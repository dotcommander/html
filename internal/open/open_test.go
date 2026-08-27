package open

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestStartReturnsBeforeExitAndReapsProcess(t *testing.T) {
	t.Parallel()
	releasePath := filepath.Join(t.TempDir(), "release")
	cmd := exec.Command(os.Args[0], "-test.run=^TestOpenHelperProcess$")
	cmd.Env = append(os.Environ(), "HTML_OPEN_HELPER_RELEASE="+releasePath)

	started := time.Now()
	done, err := start(cmd)
	if err != nil {
		t.Fatalf("start helper process: %v", err)
	}
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("start blocked for %v instead of returning before child exit", elapsed)
	}
	if err := os.WriteFile(releasePath, nil, 0o600); err != nil {
		t.Fatalf("release helper process: %v", err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for helper process to be reaped")
	}
	if cmd.ProcessState == nil || !cmd.ProcessState.Success() {
		t.Fatalf("helper process was not successfully waited: %v", cmd.ProcessState)
	}
}

func TestStartFailureIsSynchronous(t *testing.T) {
	t.Parallel()
	done, err := start(exec.Command(filepath.Join(t.TempDir(), "missing")))
	if err == nil {
		t.Fatal("expected start error for missing executable")
	}
	if done != nil {
		t.Fatal("failed start returned a completion channel")
	}
}

func TestOpenHelperProcess(t *testing.T) {
	releasePath := os.Getenv("HTML_OPEN_HELPER_RELEASE")
	if releasePath == "" {
		return
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(releasePath); err == nil {
			return
		}
		if time.Now().After(deadline) {
			os.Exit(2)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
