package cache

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain points the cache at a throwaway dir so the suite never writes to the
// real ~/.config/html/cache. Set once before m.Run (parallel-safe; t.Setenv is
// disallowed under t.Parallel).
func TestMain(m *testing.M) {
	d, err := os.MkdirTemp("", "html-cache-test-")
	if err != nil {
		panic(err)
	}
	cacheDir := filepath.Join(d, ".config", "html", "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		panic(err)
	}
	os.Setenv("HTML_CACHE_DIR", cacheDir)
	code := m.Run()
	os.RemoveAll(d)
	os.Exit(code)
}
