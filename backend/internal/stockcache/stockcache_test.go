package stockcache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/stockmedia"
)

// fakeDB is a DynamoDB table in memory. It counts calls and can be made to fail
// or hang, which is how the tests show the cache never makes a search worse.
type fakeDB struct {
	mu    sync.Mutex
	items map[string]map[string]types.AttributeValue

	entryGets, epochGets, puts int
	putAttempts                int
	getErr, putErr             error
	hang                       bool // calls wait for their context, as a stalled connection does
	consistentEpoch            bool // whether every epoch read asked for a consistent read
	sawEpochRead               bool
	putSizes                   []int
	epochGate                  chan struct{} // when set, epoch reads wait for it to be closed
}

func newFakeDB() *fakeDB {
	return &fakeDB{items: map[string]map[string]types.AttributeValue{}, consistentEpoch: true}
}

func pkOf(key map[string]types.AttributeValue) string {
	return key["PK"].(*types.AttributeValueMemberS).Value
}

func (f *fakeDB) GetItem(ctx context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	f.mu.Lock()
	hang, err := f.hang, f.getErr
	pk := pkOf(in.Key)
	if pk == epochItemKey {
		f.epochGets++
		f.sawEpochRead = true
		if in.ConsistentRead == nil || !*in.ConsistentRead {
			f.consistentEpoch = false
		}
	} else {
		f.entryGets++
	}
	item := f.items[pk]
	gate := f.epochGate
	f.mu.Unlock()
	if pk == epochItemKey && gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if hang {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	return &dynamodb.GetItemOutput{Item: cloneItem(item)}, nil
}

func (f *fakeDB) PutItem(ctx context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	f.mu.Lock()
	hang, err := f.hang, f.putErr
	f.putAttempts++
	f.mu.Unlock()
	if hang {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.puts++
	size := 0
	for k, v := range in.Item {
		size += len(k)
		switch x := v.(type) {
		case *types.AttributeValueMemberB:
			size += len(x.Value)
		case *types.AttributeValueMemberS:
			size += len(x.Value)
		}
	}
	f.putSizes = append(f.putSizes, size)
	f.items[pkOf(in.Item)] = cloneItem(in.Item)
	return &dynamodb.PutItemOutput{}, nil
}

func cloneItem(m map[string]types.AttributeValue) map[string]types.AttributeValue {
	if m == nil {
		return nil
	}
	out := make(map[string]types.AttributeValue, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (f *fakeDB) setEpoch(e string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items[epochItemKey] = map[string]types.AttributeValue{"PK": &types.AttributeValueMemberS{Value: epochItemKey}, "epoch": &types.AttributeValueMemberS{Value: e}}
}

func (f *fakeDB) counts() (entryGets, epochGets, puts int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.entryGets, f.epochGets, f.puts
}

// entryCalls is every call about an entry, whether or not it worked: the calls
// a resting table must not receive.
func (f *fakeDB) entryCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.entryGets + f.putAttempts
}

func (f *fakeDB) entryKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var keys []string
	for k := range f.items {
		if k != epochItemKey {
			keys = append(keys, k)
		}
	}
	return keys
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *fakeClock { return &fakeClock{t: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)} }
func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type logs struct {
	mu    sync.Mutex
	lines []string
}

func (l *logs) logf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}
func (l *logs) has(substr string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, s := range l.lines {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
func (l *logs) count(substr string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, s := range l.lines {
		if strings.Contains(s, substr) {
			n++
		}
	}
	return n
}

// container makes a cache that shares db with others, as a second Lambda
// container does.
func container(t *testing.T, db *fakeDB, clock *fakeClock) (*Cache, *logs) {
	t.Helper()
	l := &logs{}
	c, err := New(Config{DB: db, Table: "stock-cache", Logf: l.logf})
	if err != nil {
		t.Fatal(err)
	}
	c.now = clock.Now
	return c, l
}

func sample(id string) stockmedia.SearchResult {
	return stockmedia.SearchResult{
		Provider: stockmedia.ProviderPixabay, Type: stockmedia.MediaPhoto, Total: 321, Page: 2, PerPage: 20,
		Assets: []stockmedia.Asset{{
			Provider: stockmedia.ProviderPixabay, ID: id, Type: stockmedia.MediaPhoto,
			PageURL: "https://pixabay.com/photos/x-" + id + "/", MediaURL: "https://pixabay.com/get/" + id + "_1280.jpg",
			Format: "image/jpeg", Width: 720, Height: 1280, Orientation: stockmedia.OrientationPortrait,
			Author: "Jo", Credit: "Image by Jo on Pixabay", Alt: "a lighthouse",
		}},
		Skipped: 3, SkipReasons: map[string]int{"host": 3},
		Attribution: stockmedia.Attribution{Text: "Images provided by Pixabay", URL: "https://pixabay.com"},
		Quota:       &stockmedia.Quota{Remaining: 5, ResetAt: time.Unix(1, 0)},
	}
}

var bg = context.Background()

func TestNew_RefusesWhatCannotWork(t *testing.T) {
	if _, err := New(Config{Table: "t"}); err == nil {
		t.Error("no client was accepted")
	}
	for _, table := range []string{"", "  "} {
		if _, err := New(Config{DB: newFakeDB(), Table: table}); err == nil {
			t.Errorf("table %q was accepted", table)
		}
	}
}

func TestFromEnv_IsOffUnlessATableIsNamed(t *testing.T) {
	for _, v := range []string{"", "   ", "\t"} {
		c, err := FromEnv(newFakeDB(), func(string) string { return v }, nil)
		if c != nil || err != nil {
			t.Errorf("STOCK_CACHE_TABLE=%q: got %v, %v; want the cache off", v, c, err)
		}
	}
	c, err := FromEnv(newFakeDB(), func(k string) string {
		if k != EnvTable {
			t.Errorf("read %q, want %q", k, EnvTable)
		}
		return " stock-cache "
	}, nil)
	if c == nil || err != nil || c.table != "stock-cache" {
		t.Errorf("got %+v, %v; want a cache on the trimmed table name", c, err)
	}
}

func TestServiceOptions_AreEmptyUnlessATableIsNamed(t *testing.T) {
	l := &logs{}
	for _, v := range []string{"", " "} {
		if opts := ServiceOptions(newFakeDB(), func(string) string { return v }, l.logf); len(opts) != 0 {
			t.Errorf("STOCK_CACHE_TABLE=%q gave %d options, want none", v, len(opts))
		}
	}
	if l.has("stockcache_enabled") {
		t.Error("the cache was announced as enabled with no table")
	}
	// A nil client is a mistake in the caller; it must not stop the service.
	if opts := ServiceOptions(nil, func(string) string { return "t" }, l.logf); len(opts) != 0 || !l.has("stockcache_disabled") {
		t.Errorf("a nil client gave %d options (logged: %v)", len(opts), l.lines)
	}
}

func TestServiceOptions_GiveAServiceTheSharedCache(t *testing.T) {
	db, l := newFakeDB(), &logs{}
	opts := ServiceOptions(db, func(string) string { return "stock-cache" }, l.logf)
	if len(opts) != 1 || !l.has(`stockcache_enabled table="stock-cache"`) {
		t.Fatalf("options = %d, logs = %v", len(opts), l.lines)
	}
	prov := &pixabayStub{}
	s, err := stockmedia.NewService([]stockmedia.Provider{prov}, append(opts, stockmedia.WithLogger(func(string, ...any) {}))...)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Search(bg, stockmedia.ProviderPixabay, photos); err != nil {
		t.Fatal(err)
	}
	if _, _, puts := db.counts(); puts != 1 {
		t.Errorf("table writes = %d, want 1: the service did not use the shared cache", puts)
	}
}

func TestACacheEntryIsSharedBetweenContainers(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, _ := container(t, db, clock)
	b, _ := container(t, db, clock)

	a.Set(bg, "pixabay|photo|lighthouse", sample("1"), 24*time.Hour)
	got, ok := b.Get(bg, "pixabay|photo|lighthouse")
	if !ok {
		t.Fatal("a second container did not see the first one's entry")
	}
	want := sample("1")
	if len(got.Assets) != 1 || got.Assets[0] != want.Assets[0] || got.Total != 321 || got.Page != 2 || got.PerPage != 20 ||
		got.Skipped != 3 || got.SkipReasons["host"] != 3 || got.Attribution != want.Attribution || got.Provider != want.Provider || got.Type != want.Type {
		t.Errorf("the entry changed on the way through the table:\n got  %+v\n want %+v", got, want)
	}
	if got.Quota != nil {
		t.Errorf("a quota reading is only true at the moment it was made, and must not be shared: %+v", got.Quota)
	}
	if _, ok := b.Get(bg, "pixabay|photo|something else"); ok {
		t.Error("a different key hit")
	}
}

func TestAHitInTheTableIsKeptInMemoryForTheRestOfItsLife(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, _ := container(t, db, clock)
	b, logs := container(t, db, clock)
	a.Set(bg, "k", sample("1"), 24*time.Hour)

	if _, ok := b.Get(bg, "k"); !ok {
		t.Fatal("no hit from the table")
	}
	before, _, _ := db.counts()
	for i := 0; i < 5; i++ {
		if _, ok := b.Get(bg, "k"); !ok {
			t.Fatal("lost the entry")
		}
	}
	if after, _, _ := db.counts(); after != before {
		t.Errorf("%d more table reads for an entry already held in memory", after-before)
	}
	if !logs.has("stockcache_shared_hit") {
		t.Error("the hit was not logged")
	}

}

// Memory keeps a copy fetched from the table only for the time the table's copy
// has left, not for a fresh 24 hours.
func TestAMemoryCopyOfATableHitDoesNotOutliveTheTableCopy(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, _ := container(t, db, clock)
	b, _ := container(t, db, clock)
	a.Set(bg, "k", sample("1"), time.Second) // the table's copy has one second left
	if _, ok := b.Get(bg, "k"); !ok {
		t.Fatal("no hit from the table")
	}
	db.mu.Lock()
	db.items = map[string]map[string]types.AttributeValue{} // the table forgets it; only memory could answer
	db.mu.Unlock()
	if _, ok := b.Get(bg, "k"); !ok {
		t.Fatal("memory did not keep the copy at all")
	}
	time.Sleep(1200 * time.Millisecond)
	if _, ok := b.Get(bg, "k"); ok {
		t.Error("memory still serves a copy whose lifetime in the table is over")
	}
}

func TestAnEntryTheTableHasNotYetDeletedButThatHasExpiredIsAMiss(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, _ := container(t, db, clock)
	b, _ := container(t, db, clock)
	a.Set(bg, "k", sample("1"), time.Hour)

	clock.Advance(time.Hour - time.Second)
	if _, ok := b.Get(bg, "k"); !ok {
		t.Error("a miss one second before expiry")
	}
	c, _ := container(t, db, clock)
	clock.Advance(time.Second) // exactly at expiry: DynamoDB's own TTL has not removed it yet
	if _, ok := c.Get(bg, "k"); ok {
		t.Error("an expired entry was served because the table still had it")
	}
	d, _ := container(t, db, clock)
	clock.Advance(47 * time.Hour)
	if _, ok := d.Get(bg, "k"); ok {
		t.Error("an entry two days old was served")
	}
}

func TestTheExpiryStoredIsNeverEarlierThanTheLifetimeAskedFor(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, _ := container(t, db, clock)
	a.Set(bg, "k", sample("1"), 1500*time.Millisecond)
	var got int64
	for _, k := range db.entryKeys() {
		got, _ = strconv.ParseInt(db.items[k]["expiresAt"].(*types.AttributeValueMemberN).Value, 10, 64)
	}
	if want := clock.Now().Add(1500 * time.Millisecond); got < want.Unix()+1 {
		t.Errorf("expiresAt = %d, want at least %d (rounded up)", got, want.Unix()+1)
	}
	// A whole number of seconds is not pushed a second further.
	a.Set(bg, "j", sample("2"), 2*time.Second)
	for _, k := range db.entryKeys() {
		v, _ := strconv.ParseInt(db.items[k]["expiresAt"].(*types.AttributeValueMemberN).Value, 10, 64)
		if v == clock.Now().Add(2*time.Second).Unix()+1 {
			t.Errorf("a whole-second lifetime was rounded up a further second")
		}
	}
}

func TestNothingIsStoredForANonPositiveLifetime(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, _ := container(t, db, clock)
	for _, ttl := range []time.Duration{0, -time.Second} {
		a.Set(bg, "k", sample("1"), ttl)
		a.Seed(bg, "s", sample("1"), ttl)
	}
	if _, ok := a.Get(bg, "k"); ok {
		t.Error("memory kept an entry with no lifetime")
	}
	if _, _, puts := db.counts(); puts != 0 {
		t.Errorf("%d writes for entries with no lifetime", puts)
	}
}

func TestAFlushMakesEveryEarlierEntryUnreachableEverywhere(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, aLogs := container(t, db, clock)
	b, _ := container(t, db, clock)
	a.Set(bg, "k", sample("1"), 24*time.Hour)
	if _, ok := b.Get(bg, "k"); !ok {
		t.Fatal("setup: b should see a's entry")
	}

	db.setEpoch("after-pixabay-approved") // what deploy/flush-stockmedia-cache.sh does

	// Containers learn of it within the minute, not at once.
	clock.Advance(epochEvery - time.Second)
	if _, ok := b.Get(bg, "k"); !ok {
		t.Error("the flush was seen before the epoch was due to be re-read")
	}
	clock.Advance(time.Second)
	if _, ok := b.Get(bg, "k"); ok {
		t.Error("an entry from before the flush was served from memory after it")
	}
	if _, ok := a.Get(bg, "k"); ok {
		t.Error("the container that wrote the entry still serves it after the flush")
	}
	fresh, _ := container(t, db, clock)
	if _, ok := fresh.Get(bg, "k"); ok {
		t.Error("an entry from before the flush was served from the table after it")
	}
	if !aLogs.has("stockcache_epoch_changed") {
		t.Error("the flush was not logged")
	}

	// New entries are written under the new epoch and shared as before.
	b.Set(bg, "k", sample("2"), 24*time.Hour)
	if got, ok := fresh.Get(bg, "k"); !ok || got.Assets[0].ID != "2" {
		t.Errorf("after the flush: %+v, %v; want the new entry", got, ok)
	}
}

func TestAContainerThatMissedTheFlushCannotBringBackOldResults(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	stale, _ := container(t, db, clock)
	stale.Set(bg, "warm", sample("old"), 24*time.Hour) // reads the epoch: "" (no item)
	db.setEpoch("e2")
	clock.Advance(epochEvery - time.Second) // stale has not re-read the epoch yet
	stale.Set(bg, "k", sample("old"), 24*time.Hour)

	fresh, _ := container(t, db, clock)
	if _, ok := fresh.Get(bg, "k"); ok {
		t.Error("a write by a container that had not heard of the flush came back after it")
	}
}

func TestTheEpochIsReadOnceAMinuteNotOnEveryCall(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, _ := container(t, db, clock)
	for i := 0; i < 20; i++ {
		a.Get(bg, "k")
		a.Set(bg, "k", sample("1"), time.Hour)
	}
	if _, reads, _ := db.counts(); reads != 1 {
		t.Errorf("epoch reads = %d in one minute, want 1", reads)
	}
	clock.Advance(epochEvery)
	a.Get(bg, "k")
	if _, reads, _ := db.counts(); reads != 2 {
		t.Errorf("epoch reads = %d a minute later, want 2", reads)
	}
	if !db.consistentEpoch {
		t.Error("the epoch was read without a consistent read, so a flush could be missed for a while")
	}
}

func TestCallersArrivingTogetherShareOneEpochReadAndUseMemoryUntilItReturns(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	db.epochGate = make(chan struct{})
	a, _ := container(t, db, clock)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.Get(bg, "k")
		}()
	}
	// Wait until the one reader is inside the table call, then let it finish.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, reads, _ := db.counts(); reads > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // time for any second reader to show itself
	if _, reads, _ := db.counts(); reads != 1 {
		t.Errorf("epoch reads while one was under way = %d, want 1: every caller at start-up asked the table", reads)
	}
	close(db.epochGate)
	wg.Wait()
	if entryGets, reads, _ := db.counts(); reads != 1 || entryGets > 1 {
		t.Errorf("epoch reads = %d, entry reads = %d; want 1 epoch read, and no entry read by callers that did not know the epoch", reads, entryGets)
	}
}

func TestWithoutAnEpochTheTableIsNotUsedAtAll(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	db.getErr = errors.New("ProvisionedThroughputExceededException")
	a, logs := container(t, db, clock)

	a.Set(bg, "k", sample("1"), time.Hour)
	if _, ok := a.Get(bg, "k"); !ok {
		t.Error("memory should still hold the entry")
	}
	if entryGets, _, puts := db.counts(); entryGets != 0 || puts != 0 {
		t.Errorf("table used under a guessed epoch: %d entry reads, %d writes", entryGets, puts)
	}
	if !logs.has("stockcache_epoch_read_failed") {
		t.Error("the failure was not logged")
	}

	// Ten seconds on, a working table is found and used.
	db.mu.Lock()
	db.getErr = nil
	db.mu.Unlock()
	clock.Advance(epochRetry - time.Second)
	a.Set(bg, "k2", sample("2"), time.Hour)
	if _, _, puts := db.counts(); puts != 0 {
		t.Error("the epoch was retried sooner than the retry interval")
	}
	clock.Advance(time.Second)
	a.Set(bg, "k3", sample("3"), time.Hour)
	if _, _, puts := db.counts(); puts != 1 {
		t.Errorf("writes = %d once the epoch could be read, want 1", puts)
	}
	if !logs.has("stockcache_ready") {
		t.Error("recovery was not logged")
	}
}

func TestAFailingTableNeverFailsASearchAndIsLeftAloneAfterRepeatedFailures(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, logs := container(t, db, clock)
	a.Get(bg, "warm") // reads the epoch
	base := db.entryCalls()
	db.mu.Lock()
	db.getErr, db.putErr = errors.New("boom"), errors.New("boom")
	db.mu.Unlock()

	for i := 0; i < 10; i++ {
		if _, ok := a.Get(bg, fmt.Sprint("k", i)); ok {
			t.Fatal("a miss became a hit")
		}
		a.Set(bg, fmt.Sprint("w", i), sample("1"), time.Hour) // must not panic or block
	}
	// A read, a write and a read fail: three in a row open the breaker, and none
	// of the remaining 17 operations reaches the table.
	if n := db.entryCalls() - base; n != breakerThreshold {
		t.Errorf("table calls after it started failing = %d, want %d: the breaker should have opened", n, breakerThreshold)
	}
	if !logs.has("stockcache_table_resting") {
		t.Error("the breaker opening was not logged")
	}
	if n := logs.count("_failed"); n != breakerThreshold {
		t.Errorf("%d failures logged, want %d: a broken table must not fill the log", n, breakerThreshold)
	}
	// What was written is still in memory.
	if _, ok := a.Get(bg, "w9"); !ok {
		t.Error("memory lost an entry because the table failed")
	}

	// After the rest, one probe goes out; success closes the breaker.
	db.mu.Lock()
	db.getErr, db.putErr = nil, nil
	db.mu.Unlock()
	clock.Advance(breakerOpen - time.Second)
	a.Get(bg, "again")
	if n := db.entryCalls() - base; n != breakerThreshold {
		t.Error("the table was tried before the rest was over")
	}
	clock.Advance(time.Second)
	a.Get(bg, "again")
	if n := db.entryCalls() - base; n != breakerThreshold+1 {
		t.Errorf("table calls = %d after the rest, want %d (one probe)", n, breakerThreshold+1)
	}
	if !logs.has("stockcache_table_recovered") {
		t.Error("recovery was not logged")
	}
}

func TestAProbeThatFailsAfterTheRestClosesTheTableOffAgain(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, _ := container(t, db, clock)
	a.Get(bg, "warm")
	db.mu.Lock()
	db.getErr = errors.New("still down")
	db.mu.Unlock()
	for i := 0; i < breakerThreshold; i++ {
		a.Get(bg, fmt.Sprint("k", i))
	}
	clock.Advance(breakerOpen)
	a.Get(bg, "probe") // the one probe, which fails
	opened := db.entryCalls()
	for i := 0; i < 10; i++ {
		a.Get(bg, fmt.Sprint("later", i))
	}
	if n := db.entryCalls(); n != opened {
		t.Errorf("%d more table calls after a failed probe: the table was not left alone again", n-opened)
	}
}

// A table that stays broken is probed again after every rest. Those probes fail
// too, and must not each write a line: a day of them would be 2,880 lines.
func TestATableThatStaysBrokenDoesNotFillTheLog(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, logs := container(t, db, clock)
	a.Get(bg, "warm")
	db.mu.Lock()
	db.getErr = errors.New("still down")
	db.mu.Unlock()
	for i := 0; i < 20; i++ { // twenty rests, each ending in a failed probe
		clock.Advance(breakerOpen)
		a.Get(bg, fmt.Sprint("k", i))
		a.Get(bg, fmt.Sprint("k", i, "b"))
	}
	if n := logs.count("_failed"); n > breakerThreshold {
		t.Errorf("%d failure lines for a table that stayed broken, want at most %d", n, breakerThreshold)
	}
	if n := logs.count("stockcache_table_resting"); n != 1 {
		t.Errorf("%d 'resting' lines over twenty failed probes, want 1: it is said once per outage", n)
	}
	// When the table comes back and then fails again, that is a new outage.
	db.mu.Lock()
	db.getErr = nil
	db.mu.Unlock()
	clock.Advance(breakerOpen)
	a.Get(bg, "back")
	db.mu.Lock()
	db.getErr = errors.New("down again")
	db.mu.Unlock()
	for i := 0; i < breakerThreshold; i++ {
		a.Get(bg, fmt.Sprint("again", i))
	}
	if n := logs.count("stockcache_table_resting"); n != 2 {
		t.Errorf("%d 'resting' lines after a second outage, want 2", n)
	}
}

func TestSuccessBetweenFailuresKeepsTheBreakerClosed(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, _ := container(t, db, clock)
	a.Get(bg, "warm")
	for round := 0; round < 4; round++ {
		db.mu.Lock()
		db.getErr = errors.New("blip")
		db.mu.Unlock()
		a.Get(bg, "x1")
		a.Get(bg, "x2") // two failures: below the threshold
		db.mu.Lock()
		db.getErr = nil
		db.mu.Unlock()
		a.Get(bg, "x3") // a success resets the count
	}
	before, _, _ := db.counts()
	a.Get(bg, "x4")
	if after, _, _ := db.counts(); after != before+1 {
		t.Error("the breaker opened although no three failures came in a row")
	}
}

func TestAHungTableCostsOneShortTimeoutNotTheSearch(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, _ := container(t, db, clock)
	a.Get(bg, "warm")
	db.mu.Lock()
	db.hang = true
	db.mu.Unlock()
	a.opTimeout = 30 * time.Millisecond

	start := time.Now()
	if _, ok := a.Get(bg, "k"); ok {
		t.Fatal("hit from a hung table")
	}
	a.Set(bg, "k", sample("1"), time.Hour)
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("a hung table held the caller for %v", took)
	}
}

func TestACallerWhoLeavesIsNotCountedAsATableFailure(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, logs := container(t, db, clock)
	a.Get(bg, "warm")
	db.mu.Lock()
	db.hang = true
	db.mu.Unlock()
	for i := 0; i < 2*breakerThreshold; i++ {
		ctx, cancel := context.WithCancel(bg)
		go func() { time.Sleep(5 * time.Millisecond); cancel() }()
		a.Get(ctx, fmt.Sprint("k", i))
	}
	if logs.has("stockcache_table_resting") || logs.has("stockcache_read_failed") {
		t.Errorf("cancellations by callers were blamed on the table: %v", logs.lines)
	}
}

func TestSeededEntriesStayInMemory(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, _ := container(t, db, clock)
	b, _ := container(t, db, clock)
	a.Seed(bg, "get|pixabay|photo|1", sample("1"), time.Hour)
	if _, ok := a.Get(bg, "get|pixabay|photo|1"); !ok {
		t.Error("a seeded entry is not found by the container that seeded it")
	}
	if _, ok := b.Get(bg, "get|pixabay|photo|1"); ok {
		t.Error("a seeded entry reached another container")
	}
	if _, _, puts := db.counts(); puts != 0 {
		t.Errorf("%d table writes for a seeded entry", puts)
	}
}

func TestAnOversizedResultIsKeptInMemoryButNotWrittenToTheTable(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, logs := container(t, db, clock)
	big := sample("1")
	big.Assets[0].Alt = strings.Repeat("x", maxEntryBytes)
	a.Set(bg, "big", big, time.Hour)
	if _, ok := a.Get(bg, "big"); !ok {
		t.Error("memory lost the large entry")
	}
	if _, _, puts := db.counts(); puts != 0 {
		t.Error("a result over the size limit was written to the table")
	}
	if !logs.has("stockcache_write_skipped reason=too_large") {
		t.Error("the skip was not logged")
	}
	// Just under the limit is written, and DynamoDB's own item limit is far above it.
	ok := sample("2")
	ok.Assets[0].Alt = strings.Repeat("y", maxEntryBytes-2000)
	a.Set(bg, "ok", ok, time.Hour)
	if _, _, puts := db.counts(); puts != 1 {
		t.Errorf("writes = %d for a result just under the limit, want 1", puts)
	}
	if maxEntryBytes >= 400*1024 {
		t.Error("maxEntryBytes must stay below DynamoDB's 400 KB item limit")
	}
}

// jsonSize is the size of the encoded result, which is what the limit is on.
func jsonSize(t *testing.T, res stockmedia.SearchResult) int {
	t.Helper()
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	return len(b)
}

func TestTheSizeLimitIsExactlyMaxEntryBytes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		extra  int
		stored bool
	}{{"one byte under", -1, true}, {"exactly at the limit", 0, true}, {"one byte over", 1, false}} {
		t.Run(tc.name, func(t *testing.T) {
			db, clock := newFakeDB(), newClock()
			a, _ := container(t, db, clock)
			res := sample("1")
			res.Quota = nil
			// Alt is omitted from the JSON when empty, so measure with one character
			// in it and pad from there.
			res.Assets[0].Alt = "x"
			pad := 1 + maxEntryBytes - jsonSize(t, res) + tc.extra
			res.Assets[0].Alt = strings.Repeat("x", pad)
			if got := jsonSize(t, res); got != maxEntryBytes+tc.extra {
				t.Fatalf("setup: encoded size %d, want %d", got, maxEntryBytes+tc.extra)
			}
			a.Set(bg, "k", res, time.Hour)
			if _, _, puts := db.counts(); (puts == 1) != tc.stored {
				t.Errorf("writes = %d, stored = %v", puts, tc.stored)
			}
		})
	}
}

// A seed written before a flush must not survive it: it holds the same old links
// as the search it came from.
func TestASeedWrittenBeforeAFlushIsGoneAfterIt(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	db.setEpoch("before")
	a, _ := container(t, db, clock)
	a.Get(bg, "warm") // the container learns the epoch
	a.Seed(bg, "get|k", sample("old"), 24*time.Hour)
	if _, ok := a.Get(bg, "get|k"); !ok {
		t.Fatal("setup: the seed is not found")
	}
	db.setEpoch("after")
	clock.Advance(epochEvery)
	if _, ok := a.Get(bg, "get|k"); ok {
		t.Error("a seeded entry survived a flush")
	}
}

func TestNoSearchPhraseOrKeyIsEverPartOfATableKey(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, logs := container(t, db, clock)
	phrase := "pixabay|photo|portrait|1|20|my private story about grandma"
	a.Set(bg, phrase, sample("1"), time.Hour)
	for _, k := range db.entryKeys() {
		if !strings.HasPrefix(k, entryPrefix) || len(k) != len(entryPrefix)+64 || strings.Contains(k, "grandma") {
			t.Errorf("table key %q is not a prefix plus a SHA-256", k)
		}
	}
	if len(db.entryKeys()) != 1 {
		t.Fatalf("entries = %d, want 1", len(db.entryKeys()))
	}
	if logs.has("grandma") {
		t.Error("the phrase reached a log line")
	}
	// Two keys that differ only by epoch must not collide.
	if entryPK("a", "bc") == entryPK("ab", "c") {
		t.Error("the epoch and key run together, so two different pairs share an entry")
	}
}

func TestEntriesFromAnotherFormatOrDamagedEntriesAreMisses(t *testing.T) {
	good := func() map[string]types.AttributeValue {
		db, clock := newFakeDB(), newClock()
		a, _ := container(t, db, clock)
		a.Set(bg, "k", sample("1"), time.Hour)
		return db.items[db.entryKeys()[0]]
	}
	for name, mutate := range map[string]func(m map[string]types.AttributeValue){
		"another version": func(m map[string]types.AttributeValue) { m["v"] = &types.AttributeValueMemberN{Value: "2"} },
		"no version":      func(m map[string]types.AttributeValue) { delete(m, "v") },
		"no data":         func(m map[string]types.AttributeValue) { delete(m, "data") },
		"no expiry":       func(m map[string]types.AttributeValue) { delete(m, "expiresAt") },
		"bad expiry":      func(m map[string]types.AttributeValue) { m["expiresAt"] = &types.AttributeValueMemberN{Value: "soon"} },
		"damaged json": func(m map[string]types.AttributeValue) {
			m["data"] = &types.AttributeValueMemberB{Value: []byte("{not json")}
		},
		"data of the wrong type": func(m map[string]types.AttributeValue) {
			m["data"] = &types.AttributeValueMemberS{Value: "text"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			db, clock := newFakeDB(), newClock()
			a, _ := container(t, db, clock)
			a.Set(bg, "k", sample("1"), time.Hour)
			item := cloneItem(good())
			mutate(item)
			db.items[db.entryKeys()[0]] = item
			b, _ := container(t, db, clock)
			if _, ok := b.Get(bg, "k"); ok {
				t.Error("an unreadable entry was served")
			}
		})
	}
}

func TestManyGoroutinesShareOneCacheSafely(t *testing.T) {
	db, clock := newFakeDB(), newClock()
	a, _ := container(t, db, clock)
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				k := fmt.Sprint("k", i%7)
				a.Set(bg, k, sample(fmt.Sprint(i)), time.Hour)
				a.Get(bg, k)
				a.Seed(bg, "s"+k, sample("s"), time.Hour)
				if g == 0 && i%10 == 0 {
					clock.Advance(epochEvery)
				}
			}
		}()
	}
	wg.Wait()
}
