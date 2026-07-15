package report

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSecurePlanCachePathTightensExistingArtifacts(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "plan-cache")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	securePlanCachePath(path)
	for target, want := range map[string]os.FileMode{dir: 0o700, path: 0o600} {
		info, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Fatalf("%s mode = %o, want %o", target, got, want)
		}
	}
}
