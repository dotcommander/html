package open

import "testing"

// launch must fail clearly when the opener is not on PATH, instead of silently
// succeeding or panicking. (We never call Open() itself in tests — it would
// launch a real browser.)
func TestLaunch_MissingOpenerErrors(t *testing.T) {
	t.Parallel()

	if err := launch("html-no-such-launcher-xyzzy", "/tmp/whatever"); err == nil {
		t.Fatal("expected error when opener is absent from PATH, got nil")
	}
}
