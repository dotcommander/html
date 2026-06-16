package open

import (
	"strings"
	"testing"
)

// launch must fail clearly when the opener is not on PATH, instead of silently
// succeeding or panicking. (We never call Open() itself in tests — it would
// launch a real browser.)
func TestLaunch_MissingOpenerErrors(t *testing.T) {
	t.Parallel()

	if err := launch("html-no-such-launcher-xyzzy", "/tmp/whatever"); err == nil {
		t.Fatal("expected error when opener is absent from PATH, got nil")
	}
}

func TestOpen_MissingLauncherSurfacesError(t *testing.T) {
	t.Parallel()
	// A launcher name that cannot exist on PATH must produce an error, not a
	// silent no-op — open failures are surfaced to the caller.
	err := Open("/some/file/path", "html-no-such-launcher-zzz")
	if err == nil {
		t.Fatal("expected an error for a missing launcher, got nil")
	}
	if !strings.Contains(err.Error(), "launcher") {
		t.Fatalf("error should mention the missing launcher, got: %v", err)
	}
}
