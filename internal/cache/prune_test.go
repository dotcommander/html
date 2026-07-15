package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPrune_RemovesStaleKeepsFresh(t *testing.T) {
	t.Parallel()

	d := t.TempDir()
	stale := filepath.Join(d, "stale.html")
	fresh := filepath.Join(d, "fresh.html")
	for _, p := range []string{stale, fresh} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Backdate the stale entry well past the cutoff.
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	prune(d, time.Hour)

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale entry should have been pruned; stat err = %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("fresh entry should remain; stat err = %v", err)
	}
}

func TestPrune_DisabledWhenAgeNonPositive(t *testing.T) {
	t.Parallel()

	d := t.TempDir()
	f := filepath.Join(d, "x.html")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-100 * time.Hour)
	if err := os.Chtimes(f, old, old); err != nil {
		t.Fatal(err)
	}

	prune(d, 0) // disabled

	if _, err := os.Stat(f); err != nil {
		t.Errorf("prune(maxAge<=0) must delete nothing; stat err = %v", err)
	}
}
