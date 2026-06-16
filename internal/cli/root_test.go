package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/dotcommander/html/internal/cache"
)

func TestRoot_PlainMarkdownMutuallyExclusive(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--plain", "--markdown", "-n"})
	cmd.SetIn(strings.NewReader("hello"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected --plain/--markdown mutual-exclusion error")
	}
}

func TestRoot_PipedPlainSmoke(t *testing.T) {
	t.Parallel()
	content := "cli-smoke-uniq\nplain line\n"
	t.Cleanup(func() {
		if p, e := cache.PathForContent([]byte(content)); e == nil {
			os.Remove(p)
			os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})
	cmd := newRootCmd()
	cmd.SetArgs([]string{"-n"})
	cmd.SetIn(strings.NewReader(content))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.String(), ".html") {
		t.Errorf("expected the cache path printed to stdout, got %q", out.String())
	}
}
