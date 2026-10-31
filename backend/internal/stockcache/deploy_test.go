package stockcache

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The deploy scripts create the table, turn its TTL on and write the flush epoch.
// Those names must be the ones this package reads and writes; a mismatch would
// make a cache that silently never expires, or a flush that flushes nothing. The
// scripts are plain text, so this test reads them.
func readScript(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "deploy", name))
	if err != nil {
		t.Fatalf("the deploy script %s cannot be read from the tests: %v", name, err)
	}
	return string(b)
}

func assign(t *testing.T, script, name, text string) string {
	t.Helper()
	m := regexp.MustCompile(`(?m)^` + name + `="([^"]*)"`).FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("%s: no line %s=\"...\"", script, name)
	}
	return m[1]
}

func TestDeployScriptsAgreeWithTheCode(t *testing.T) {
	setup := readScript(t, "setup-stock-cache.sh")
	flush := readScript(t, "flush-stockmedia-cache.sh")

	if got := assign(t, "setup-stock-cache.sh", "TTL_ATTR", setup); got != ttlAttribute {
		t.Errorf("setup-stock-cache.sh turns on TTL for %q, but the cache stores its expiry in %q", got, ttlAttribute)
	}
	if got := assign(t, "setup-stock-cache.sh", "API_ENV_KEY", setup); got != EnvTable {
		t.Errorf("setup-stock-cache.sh sets %q, but the code reads %q", got, EnvTable)
	}
	if got := assign(t, "flush-stockmedia-cache.sh", "EPOCH_KEY", flush); got != epochItemKey {
		t.Errorf("flush-stockmedia-cache.sh writes the epoch to item %q, but the code reads %q", got, epochItemKey)
	}

	// Both scripts name the same table.
	tableOf := func(script, text string) string {
		return strings.Replace(assign(t, script, "TABLE", text), "${PROJECT}", "indifferent-fun", 1)
	}
	if a, b := tableOf("setup", setup), tableOf("flush", flush); a != b || a != "indifferent-fun-stock-cache" {
		t.Errorf("tables differ or changed: setup %q, flush %q", a, b)
	}

	// The key the table is created with, and the epoch attribute the flush writes.
	if !strings.Contains(setup, "AttributeName="+hashKey+",AttributeType=S") || !strings.Contains(setup, "AttributeName="+hashKey+",KeyType=HASH") {
		t.Errorf("setup-stock-cache.sh does not create a table with a single string hash key %q", hashKey)
	}
	if !strings.Contains(flush, `\"`+hashKey+`\":{\"S\":\"${EPOCH_KEY}\"},\"`+epochAttribute+`\":{\"S\":`) {
		t.Errorf("flush-stockmedia-cache.sh does not write item {%s: EPOCH, %s: ...}", hashKey, epochAttribute)
	}
	if !strings.Contains(flush, "--consistent-read") {
		t.Error("the flush script should read the current epoch with a consistent read")
	}
}
