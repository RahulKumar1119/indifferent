package stockcache

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/rahul/indifferent/backend/internal/stockmedia"
)

// dynamoServer speaks enough of DynamoDB's JSON wire protocol (GetItem and
// PutItem) for the real SDK client to talk to it. It enforces the parts of
// DynamoDB the fake in the other tests does not: the shape of attribute values,
// base64 for binary, the 400 KB item limit, and the error a throttled table
// returns. It is a model of DynamoDB, not DynamoDB.
type dynamoServer struct {
	*httptest.Server
	mu          sync.Mutex
	items       map[string]map[string]json.RawMessage
	targets     []string
	consistent  []bool
	requests    int
	failWith    int // when set, every request gets this HTTP status
	badRequests []string
}

const itemLimit = 400 * 1024

func newDynamoServer(t *testing.T, table string) *dynamoServer {
	t.Helper()
	d := &dynamoServer{items: map[string]map[string]json.RawMessage{}}
	d.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		d.mu.Lock()
		defer d.mu.Unlock()
		d.requests++
		target := strings.TrimPrefix(r.Header.Get("X-Amz-Target"), "DynamoDB_20120810.")
		d.targets = append(d.targets, target)
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		if d.failWith != 0 {
			w.WriteHeader(d.failWith)
			_, _ = io.WriteString(w, `{"__type":"com.amazonaws.dynamodb.v20120810#ProvisionedThroughputExceededException","message":"throttled"}`)
			return
		}
		reject := func(msg string) {
			d.badRequests = append(d.badRequests, msg)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"__type":"com.amazon.coral.validate#ValidationException","message":%q}`, msg)
		}
		switch target {
		case "GetItem":
			var in struct {
				TableName      string
				Key            map[string]map[string]string
				ConsistentRead *bool
			}
			if err := json.Unmarshal(body, &in); err != nil || in.TableName != table {
				reject("bad GetItem or wrong table")
				return
			}
			pk, ok := in.Key["PK"]["S"]
			if !ok || len(in.Key) != 1 {
				reject("the key must be exactly {PK: S}")
				return
			}
			d.consistent = append(d.consistent, in.ConsistentRead != nil && *in.ConsistentRead)
			if item, found := d.items[pk]; found {
				_ = json.NewEncoder(w).Encode(map[string]any{"Item": item})
				return
			}
			_, _ = io.WriteString(w, `{}`)
		case "PutItem":
			var in struct {
				TableName string
				Item      map[string]json.RawMessage
			}
			if err := json.Unmarshal(body, &in); err != nil || in.TableName != table {
				reject("bad PutItem or wrong table")
				return
			}
			size := 0
			for name, raw := range in.Item {
				var typed map[string]string
				if err := json.Unmarshal(raw, &typed); err != nil || len(typed) != 1 {
					reject("attribute " + name + " must have exactly one type")
					return
				}
				size += len(name)
				for typ, val := range typed {
					switch typ {
					case "S":
						size += len(val)
					case "N":
						if _, err := strconv.ParseFloat(val, 64); err != nil {
							reject("attribute " + name + " is not a number")
							return
						}
						size += len(val)
					case "B":
						bin, err := base64.StdEncoding.DecodeString(val)
						if err != nil {
							reject("attribute " + name + " is not base64")
							return
						}
						size += len(bin)
					default:
						reject("unexpected attribute type " + typ)
						return
					}
				}
			}
			if size > itemLimit {
				reject("Item size has exceeded the maximum allowed size")
				return
			}
			var pk map[string]string
			_ = json.Unmarshal(in.Item["PK"], &pk)
			if pk["S"] == "" {
				reject("the item has no PK")
				return
			}
			d.items[pk["S"]] = in.Item
			_, _ = io.WriteString(w, `{}`)
		default:
			reject("unsupported operation " + target)
		}
	}))
	t.Cleanup(d.Close)
	return d
}

func (d *dynamoServer) setEpoch(e string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.items[epochItemKey] = map[string]json.RawMessage{
		"PK":           json.RawMessage(`{"S":"EPOCH"}`),
		epochAttribute: json.RawMessage(fmt.Sprintf(`{"S":%q}`, e)),
	}
}

func (d *dynamoServer) stats() (requests int, targets []string, bad []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.requests, append([]string(nil), d.targets...), append([]string(nil), d.badRequests...)
}

// realClient is the SDK's own DynamoDB client, pointed at the local server.
func realClient(srv *dynamoServer) *dynamodb.Client {
	return dynamodb.New(dynamodb.Options{
		Region:       "ap-south-1",
		BaseEndpoint: aws.String(srv.URL),
		Credentials:  aws.AnonymousCredentials{},
		HTTPClient:   srv.Client(),
	})
}

func wireCache(t *testing.T, srv *dynamoServer) *Cache {
	t.Helper()
	c, err := New(Config{DB: realClient(srv), Table: "stock-cache", Logf: (&logs{}).logf})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestTheRealSDKClientSharesEntriesThroughTheWireProtocol(t *testing.T) {
	srv := newDynamoServer(t, "stock-cache")
	a, b := wireCache(t, srv), wireCache(t, srv)

	a.Set(bg, "pixabay|photo|lighthouse", sample("1"), 24*time.Hour)
	got, ok := b.Get(bg, "pixabay|photo|lighthouse")
	if !ok {
		t.Fatal("a second client did not read the first one's entry through the wire protocol")
	}
	want := sample("1")
	if len(got.Assets) != 1 || got.Assets[0] != want.Assets[0] || got.Total != want.Total || got.Attribution != want.Attribution {
		t.Errorf("the entry changed on the way: %+v", got)
	}
	_, targets, bad := srv.stats()
	if len(bad) != 0 {
		t.Errorf("the server rejected requests: %v", bad)
	}
	// One epoch read and one write for a; one epoch read and one entry read for b.
	counts := map[string]int{}
	for _, tg := range targets {
		counts[tg]++
	}
	if counts["PutItem"] != 1 || counts["GetItem"] != 3 {
		t.Errorf("wire calls = %v, want 1 PutItem and 3 GetItem", counts)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if len(srv.consistent) < 2 || !srv.consistent[0] {
		t.Errorf("consistent flags on GetItem = %v: the epoch read must be consistent", srv.consistent)
	}
}

func TestTheRealSDKClientAndAFlushThroughTheWireProtocol(t *testing.T) {
	srv := newDynamoServer(t, "stock-cache")
	a, b := wireCache(t, srv), wireCache(t, srv)
	clock := newClock()
	a.now, b.now = clock.Now, clock.Now

	a.Set(bg, "k", sample("1"), 24*time.Hour)
	if _, ok := b.Get(bg, "k"); !ok {
		t.Fatal("setup: no hit")
	}
	srv.setEpoch("2026-11-01T09:00:00Z") // what deploy/flush-stockmedia-cache.sh writes
	clock.Advance(epochEvery)
	if _, ok := b.Get(bg, "k"); ok {
		t.Error("an entry from before the flush was served")
	}
	if _, ok := a.Get(bg, "k"); ok {
		t.Error("an entry from before the flush was served to the container that wrote it")
	}
}

func TestTheLargestEntryTheCacheWritesIsAcceptedByDynamoDBsItemLimit(t *testing.T) {
	srv := newDynamoServer(t, "stock-cache")
	a, b := wireCache(t, srv), wireCache(t, srv)
	res := sample("1")
	res.Assets[0].Alt = strings.Repeat("x", maxEntryBytes-jsonSize(t, res)+1+1)
	if n := jsonSize(t, res); n > maxEntryBytes || n < maxEntryBytes-10 {
		t.Fatalf("setup: encoded size %d is not at the limit %d", n, maxEntryBytes)
	}
	a.Set(bg, "big", res, time.Hour)
	if _, _, bad := srv.stats(); len(bad) != 0 {
		t.Fatalf("the largest entry the cache writes was rejected: %v", bad)
	}
	if got, ok := b.Get(bg, "big"); !ok || len(got.Assets[0].Alt) != len(res.Assets[0].Alt) {
		t.Error("the largest entry did not come back intact")
	}
	if maxEntryBytes >= itemLimit {
		t.Errorf("the cache's limit %d is not below DynamoDB's %d", maxEntryBytes, itemLimit)
	}
}

// The SDK retries throttled calls with growing waits. Left alone that can take
// many seconds; the cache's own 0.8 s limit must cut it short.
func TestAThrottledTableCostsTheCallerNoMoreThanTheCachesOwnTimeout(t *testing.T) {
	srv := newDynamoServer(t, "stock-cache")
	c := wireCache(t, srv)
	c.Get(bg, "warm") // epoch read while the table is healthy
	srv.mu.Lock()
	srv.failWith = http.StatusBadRequest
	srv.mu.Unlock()

	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, ok := c.Get(bg, fmt.Sprint("k", i)); ok {
			t.Fatal("hit from a throttled table")
		}
		c.Set(bg, fmt.Sprint("w", i), sample("1"), time.Hour)
	}
	if took := time.Since(start); took > 4*time.Second {
		t.Errorf("three reads and three writes against a throttled table took %v, want well under 4 s", took)
	}
	if _, ok := c.Get(bg, "w2"); !ok {
		t.Error("memory lost the entry because the table was throttled")
	}
}

func TestAServiceBuiltOnTheRealClientNeedsNoProviderCallTheSecondTime(t *testing.T) {
	srv := newDynamoServer(t, "stock-cache")
	prov := &pixabayStub{}
	mk := func() *stockmedia.Service {
		c := wireCache(t, srv)
		s, err := stockmedia.NewService([]stockmedia.Provider{prov}, stockmedia.WithCache(c),
			stockmedia.WithLogger(func(string, ...any) {}), stockmedia.WithRetry(stockmedia.RetryPolicy{}))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	ctx, cancel := context.WithTimeout(bg, 10*time.Second)
	defer cancel()
	if _, err := mk().Search(ctx, stockmedia.ProviderPixabay, photos); err != nil {
		t.Fatal(err)
	}
	got, err := mk().Search(ctx, stockmedia.ProviderPixabay, photos)
	if err != nil || !got.Cached {
		t.Errorf("second container: cached=%v err=%v", got.Cached, err)
	}
	if searches, _ := prov.calls(); searches != 1 {
		t.Errorf("provider searches = %d, want 1", searches)
	}
	if _, _, bad := srv.stats(); len(bad) != 0 {
		t.Errorf("the server rejected requests: %v", bad)
	}
}
