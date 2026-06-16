package cli

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestRoot_OutputToFile(t *testing.T) {
	t.Parallel()
	sourceDir := t.TempDir()
	outputDir := t.TempDir()
	source := filepath.Join(sourceDir, "input.md")
	output := filepath.Join(outputDir, "output.html")
	content := "# output\n\nbody\n"
	if err := os.WriteFile(source, []byte(content), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	t.Cleanup(func() {
		if p, e := cache.PathFor(source); e == nil {
			os.Remove(p)
			os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})

	cmd := newRootCmd()
	cmd.SetArgs([]string{"-o", output, source})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := out.String(); strings.TrimSpace(got) != output {
		t.Fatalf("expected output path %q, got %q", output, got)
	}

	html, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !strings.Contains(string(html), "<html") {
		t.Fatalf("expected generated html in output file, got: %q", string(html))
	}
}

func TestRoot_OutputToStdout(t *testing.T) {
	t.Parallel()
	sourceDir := t.TempDir()
	source := filepath.Join(sourceDir, "input.md")
	content := "# output\n\nbody\n"
	if err := os.WriteFile(source, []byte(content), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	t.Cleanup(func() {
		if p, e := cache.PathFor(source); e == nil {
			os.Remove(p)
			os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})

	cmd := newRootCmd()
	cmd.SetArgs([]string{"-o", "-", source})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.String(), "<html") {
		t.Fatalf("expected HTML on stdout, got %q", out.String())
	}
}
