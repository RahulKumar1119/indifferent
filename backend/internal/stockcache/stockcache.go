// Package stockcache is a stockmedia.Cache that every container shares.
//
// Pixabay's API terms require requests to be cached for 24 hours. The default
// stockmedia.MemoryCache does that only inside one process, and on Lambda a
// process is one warm container, so ten containers can each ask Pixabay the
// same question inside the same day. This cache keeps a copy in a small
// DynamoDB table as well, so the first container to ask answers for the rest.
//
// How it is laid out:
//
//   - Two tiers. A read tries the process's own memory first, then the table,
//     and a hit in the table is copied into memory for the rest of its life. A
//     write goes to both.
//   - Per-asset entries that stockmedia derives from a search (see
//     stockmedia.Seeder) stay in memory only. A search would otherwise cost up
//     to thirty extra writes, and an entry like that is one provider call to
//     recreate.
//   - A table entry's key is a SHA-256 of the flush epoch and the cache key, so
//     a search phrase never appears in a key or a log line. Each entry carries
//     its own expiry (expiresAt, UNIX seconds), which is checked on every read:
//     DynamoDB removes expired items only some time after they expire (up to 48
//     hours), so the table's TTL setting is housekeeping, not correctness.
//
// Flushing. Changing the epoch makes every entry written under the old one
// unreachable, in every container, without a deploy: the epoch is one item in
// the same table, re-read by each container every minute. Run
// deploy/flush-stockmedia-cache.sh when something that changes what a provider
// returns has changed, such as Pixabay approving the API key for full access,
// after which results carry larger image links than the cached ones.
//
// Failure. The cache is an optimisation and must never make a search worse.
// Every table call has a short timeout, errors are logged and treated as a
// miss, and after repeated failures the table is left alone for 30 seconds so a
// broken or throttled table does not add its timeout to every search. Until the
// epoch has been read once, the table is not used at all: an entry written
// under a guessed epoch could bring back results a flush was meant to remove.
package stockcache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/stockmedia"
)

// EnvTable names the table that turns the shared cache on. Unset, a service
// keeps only its in-memory cache.
const EnvTable = "STOCK_CACHE_TABLE"

const (
	// formatVersion is stored with every entry. An entry written by a different
	// version of this code is read as a miss.
	formatVersion = 1

	// The names below are also written by deploy/setup-stock-cache.sh and
	// deploy/flush-stockmedia-cache.sh; deploy_test.go fails if they differ.
	epochItemKey   = "EPOCH"
	epochAttribute = "epoch"
	ttlAttribute   = "expiresAt"
	hashKey        = "PK"

	entryPrefix = "E#"

	// defaultOpTimeout bounds one table call. A same-region call takes a few
	// milliseconds and a first connection about a hundred, while the provider
	// call a cache hit saves takes a few hundred. A table slower than this is
	// costing more than it saves. Until the circuit breaker opens, a failing
	// table can cost three of these.
	defaultOpTimeout = 500 * time.Millisecond
	// epochEvery is how often a container re-reads the epoch, and so how long a
	// flush can take to reach it.
	epochEvery = time.Minute
	// epochRetry is how soon a failed epoch read is tried again.
	epochRetry = 10 * time.Second

	// breakerThreshold consecutive failures leave the table alone for breakerOpen.
	breakerThreshold = 3
	breakerOpen      = 30 * time.Second

	// maxEntryBytes keeps an entry well inside DynamoDB's 400 KB item limit. A
	// result this large is not stored in the table; memory still has it.
	maxEntryBytes = 300_000
)

// DB is the part of the DynamoDB client the cache uses.
type DB interface {
	GetItem(ctx context.Context, in *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, in *dynamodb.PutItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
}

// Config configures a Cache.
type Config struct {
	DB    DB
	Table string
	// Logf receives notes. It defaults to discarding them.
	Logf func(format string, args ...any)
	// MemoryEntries bounds the in-memory tier; zero takes stockmedia's default.
	MemoryEntries int
}

// Cache implements stockmedia.Cache and stockmedia.Seeder. It is safe for
// concurrent use.
type Cache struct {
	db    DB
	table string
	logf  func(string, ...any)
	local *stockmedia.MemoryCache

	now       func() time.Time
	opTimeout time.Duration

	mu           sync.Mutex
	epoch        string
	epochKnown   bool
	epochCheckAt time.Time
	refreshing   bool
	failures     int
	openUntil    time.Time
}

var (
	_ stockmedia.Cache  = (*Cache)(nil)
	_ stockmedia.Seeder = (*Cache)(nil)
)

// New returns a Cache over the table. The table must have a string hash key
// named PK and no sort key.
func New(cfg Config) (*Cache, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("stockcache: no DynamoDB client")
	}
	table := strings.TrimSpace(cfg.Table)
	if table == "" {
		return nil, fmt.Errorf("stockcache: no table name")
	}
	logf := cfg.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Cache{
		db:        cfg.DB,
		table:     table,
		logf:      logf,
		local:     stockmedia.NewMemoryCache(cfg.MemoryEntries),
		now:       time.Now,
		opTimeout: defaultOpTimeout,
	}, nil
}

// FromEnv returns a Cache when STOCK_CACHE_TABLE is set, and nil when it is not
// (or is blank), which means "keep the in-memory cache".
func FromEnv(db DB, getenv func(string) string, logf func(string, ...any)) (*Cache, error) {
	table := strings.TrimSpace(getenv(EnvTable))
	if table == "" {
		return nil, nil
	}
	return New(Config{DB: db, Table: table, Logf: logf})
}

// ServiceOptions returns what gives a stockmedia.Service the shared cache: one
// option when STOCK_CACHE_TABLE names a table, and none when it does not. It
// exists so that the API and the video worker switch the cache on the same way.
//
// It never fails. The cache is an optimisation, so a setting that cannot be
// used is logged and the service starts with its in-memory cache, rather than
// the service not starting. (A nil *Cache must never reach stockmedia.WithCache:
// it would be a non-nil interface that panics on first use, which is why this
// returns options and not a cache.)
func ServiceOptions(db DB, getenv func(string) string, logf func(string, ...any)) []stockmedia.Option {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	c, err := FromEnv(db, getenv, logf)
	switch {
	case err != nil:
		logf("stockcache_disabled reason=%q", err.Error())
		return nil
	case c == nil:
		return nil
	}
	logf("stockcache_enabled table=%q", c.table)
	return []stockmedia.Option{stockmedia.WithCache(c)}
}

// Get implements stockmedia.Cache.
func (c *Cache) Get(ctx context.Context, key string) (stockmedia.SearchResult, bool) {
	epoch, shared := c.currentEpoch(ctx)
	lk := localKey(epoch, key)
	if res, ok := c.local.Get(ctx, lk); ok {
		return res, true
	}
	if !shared || !c.tableAllowed() {
		return stockmedia.SearchResult{}, false
	}

	var out *dynamodb.GetItemOutput
	err := c.call(ctx, "read", func(ctx context.Context) error {
		var err error
		out, err = c.db.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String(c.table),
			Key:       map[string]types.AttributeValue{hashKey: &types.AttributeValueMemberS{Value: entryPK(epoch, key)}},
		})
		return err
	})
	if err != nil {
		return stockmedia.SearchResult{}, false
	}
	res, expires, ok := c.decode(out.Item)
	if !ok {
		return stockmedia.SearchResult{}, false
	}
	left := expires.Sub(c.now())
	if left <= 0 {
		return stockmedia.SearchResult{}, false
	}
	c.logf("stockcache_shared_hit remaining_s=%d", int(left.Seconds()))
	c.local.Set(ctx, lk, res, left)
	return res, true
}

// Set implements stockmedia.Cache. A result is kept in memory and, when the
// table can be used, in the table. A table failure is logged and otherwise
// ignored.
func (c *Cache) Set(ctx context.Context, key string, res stockmedia.SearchResult, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	epoch, shared := c.currentEpoch(ctx)
	c.local.Set(ctx, localKey(epoch, key), res, ttl)
	if !shared || !c.tableAllowed() {
		return
	}
	data, err := json.Marshal(res)
	if err != nil {
		c.logf("stockcache_write_skipped reason=encode err=%q", err.Error())
		return
	}
	if len(data) > maxEntryBytes {
		c.logf("stockcache_write_skipped reason=too_large bytes=%d", len(data))
		return
	}
	// Whole seconds, rounded up: an entry is never stored with an expiry earlier
	// than the lifetime asked for.
	expires := c.now().Add(ttl)
	expiresAt := expires.Unix()
	if expires.Nanosecond() > 0 {
		expiresAt++
	}
	_ = c.call(ctx, "write", func(ctx context.Context) error {
		_, err := c.db.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(c.table),
			Item: map[string]types.AttributeValue{
				hashKey:      &types.AttributeValueMemberS{Value: entryPK(epoch, key)},
				"v":          &types.AttributeValueMemberN{Value: strconv.Itoa(formatVersion)},
				"data":       &types.AttributeValueMemberB{Value: data},
				ttlAttribute: &types.AttributeValueMemberN{Value: strconv.FormatInt(expiresAt, 10)},
			},
		})
		return err
	})
}

// Seed implements stockmedia.Seeder: memory only.
func (c *Cache) Seed(ctx context.Context, key string, res stockmedia.SearchResult, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	epoch, _ := c.currentEpoch(ctx)
	c.local.Set(ctx, localKey(epoch, key), res, ttl)
}

// decode reads an entry. Anything wrong with it, such as a missing item, another
// format version or damaged JSON, is a miss, never an error.
func (c *Cache) decode(item map[string]types.AttributeValue) (stockmedia.SearchResult, time.Time, bool) {
	if len(item) == 0 {
		return stockmedia.SearchResult{}, time.Time{}, false
	}
	ver, _ := item["v"].(*types.AttributeValueMemberN)
	exp, _ := item[ttlAttribute].(*types.AttributeValueMemberN)
	data, _ := item["data"].(*types.AttributeValueMemberB)
	if ver == nil || exp == nil || data == nil {
		c.logf("stockcache_entry_unreadable reason=missing_field")
		return stockmedia.SearchResult{}, time.Time{}, false
	}
	if ver.Value != strconv.Itoa(formatVersion) {
		return stockmedia.SearchResult{}, time.Time{}, false
	}
	secs, err := strconv.ParseInt(exp.Value, 10, 64)
	if err != nil {
		c.logf("stockcache_entry_unreadable reason=expiry")
		return stockmedia.SearchResult{}, time.Time{}, false
	}
	var res stockmedia.SearchResult
	if err := json.Unmarshal(data.Value, &res); err != nil {
		c.logf("stockcache_entry_unreadable reason=json")
		return stockmedia.SearchResult{}, time.Time{}, false
	}
	return res, time.Unix(secs, 0), true
}

// currentEpoch returns the flush epoch and whether the table may be used with
// it. It re-reads the epoch at most once a minute (every ten seconds after a
// failed read); a call that finds a read already under way, or the table
// resting, uses what is known. The table is used only once an epoch has been
// read.
func (c *Cache) currentEpoch(ctx context.Context) (string, bool) {
	c.mu.Lock()
	if c.now().Before(c.epochCheckAt) || c.refreshing || c.now().Before(c.openUntil) {
		e, known := c.epoch, c.epochKnown
		c.mu.Unlock()
		return e, known
	}
	c.refreshing = true
	c.mu.Unlock()

	var out *dynamodb.GetItemOutput
	err := c.call(ctx, "epoch_read", func(ctx context.Context) error {
		var err error
		out, err = c.db.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String(c.table),
			Key:       map[string]types.AttributeValue{hashKey: &types.AttributeValueMemberS{Value: epochItemKey}},
			// A flush should be seen as soon as the item is written.
			ConsistentRead: aws.Bool(true),
		})
		return err
	})

	c.mu.Lock()
	defer c.mu.Unlock()
	c.refreshing = false
	switch {
	case err == nil:
		next := ""
		if s, ok := out.Item[epochAttribute].(*types.AttributeValueMemberS); ok {
			next = s.Value
		}
		switch {
		case !c.epochKnown:
			c.logf("stockcache_ready epoch=%q", shorten(next))
		case next != c.epoch:
			c.logf("stockcache_epoch_changed from=%q to=%q", shorten(c.epoch), shorten(next))
		}
		c.epoch, c.epochKnown = next, true
		c.epochCheckAt = c.now().Add(epochEvery)
	case ctx.Err() != nil:
		// The caller left; nothing is learned about the table.
	default:
		c.epochCheckAt = c.now().Add(epochRetry)
	}
	return c.epoch, c.epochKnown
}

// call runs one table operation, named for the log, under the per-call timeout.
// It keeps the failure count the circuit breaker uses and logs failures: here,
// in one place, so that the two things a failure does (count and log) cannot
// disagree about what counts as one.
//
// A caller giving up (its context ended) is not a failure of the table: it is
// neither counted nor logged. Only the first failures of a run are logged, then
// the breaker's own line says the table is resting, so a broken table cannot
// fill the log.
func (c *Cache) call(ctx context.Context, name string, op func(ctx context.Context) error) error {
	cctx, cancel := context.WithTimeout(ctx, c.opTimeout)
	defer cancel()
	err := op(cctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case err == nil:
		if c.failures >= breakerThreshold {
			c.logf("stockcache_table_recovered")
		}
		c.failures = 0
	case ctx.Err() != nil:
	default:
		c.failures++
		if c.failures <= breakerThreshold {
			c.logf("stockcache_%s_failed err=%q", name, shorten(err.Error()))
		}
		if c.failures >= breakerThreshold {
			// Also when a probe after a rest fails: it closes the table off again.
			c.openUntil = c.now().Add(breakerOpen)
			if c.failures == breakerThreshold {
				// Said once per outage, not at every failed probe: a table that stays
				// down would otherwise add a line every 30 seconds.
				c.logf("stockcache_table_resting failures=%d seconds=%d", c.failures, int(breakerOpen.Seconds()))
			}
		}
	}
	return err
}

// tableAllowed is false while the circuit breaker is open.
func (c *Cache) tableAllowed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.now().Before(c.openUntil)
}

func shorten(s string) string {
	if r := []rune(s); len(r) > 200 {
		return string(r[:200]) + "..."
	}
	return s
}

func localKey(epoch, key string) string { return epoch + "\x00" + key }

// entryPK is the table key of an entry: a hash, so that no search phrase is
// ever part of a key.
func entryPK(epoch, key string) string {
	sum := sha256.Sum256([]byte(localKey(epoch, key)))
	return entryPrefix + hex.EncodeToString(sum[:])
}
