package actions

import (
	"os"
	"strings"
	"testing"

	"github.com/dotcommander/html/internal/cache"
)

// pipeToHTML runs Run with content piped via Stdin, registers cache cleanup, and
// returns the cache file path. Each caller uses unique content so its content
// hash (the cache key) is distinct.
func pipeToHTML(t *testing.T, content string, opts Options) string {
	t.Helper()
	opts.Stdin = strings.NewReader(content)
	opts.NoOpen = true
	t.Cleanup(func() {
		if p, e := cache.PathForContent([]byte(content)); e == nil {
			os.Remove(p)
			os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})
	path, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return path
}

func readRendered(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	return string(data)
}

func TestRun_StdinPlainByDetection(t *testing.T) {
	t.Parallel()
	content := "tree-detect-uniq\n.\n├── cmd\n│   └── html\n└── internal\n"
	html := readRendered(t, pipeToHTML(t, content, Options{}))
	if !strings.Contains(html, `<pre><code class="language-plaintext">`) {
		t.Errorf("expected plain wrapper for tree output")
	}
	if !strings.Contains(html, "├── cmd") {
		t.Errorf("expected tree characters preserved literally")
	}
}

func TestRun_StdinMarkdownByDetection(t *testing.T) {
	t.Parallel()
	content := "md-detect-uniq\n\n# Heading\n\nbody\n\n```go\nx := 1\n```\n"
	html := readRendered(t, pipeToHTML(t, content, Options{}))
	if strings.Contains(html, "language-plaintext") {
		t.Errorf("expected Markdown render, got plain wrapper")
	}
	if !strings.Contains(html, "<h1") {
		t.Errorf("expected heading rendered as <h1>")
	}
}

func TestRun_StdinTaskListMarkdownByDetection(t *testing.T) {
	t.Parallel()
	content := "task-list-detect-uniq\n\n- [x] ship renderer\n- [ ] verify screenshots\n"
	html := readRendered(t, pipeToHTML(t, content, Options{}))
	if strings.Contains(html, "language-plaintext") {
		t.Errorf("expected Markdown render, got plain wrapper")
	}
	if !strings.Contains(html, `type="checkbox"`) {
		t.Errorf("expected GFM task list checkboxes")
	}
}

func TestRun_StdinATXHeadingMarkdownByDetection(t *testing.T) {
	t.Parallel()
	content := "atx-heading-detect-uniq\n\n# Heading\n\nbody\n"
	html := readRendered(t, pipeToHTML(t, content, Options{}))
	if strings.Contains(html, "language-plaintext") {
		t.Errorf("expected Markdown render, got plain wrapper")
	}
	if !strings.Contains(html, "<h1") {
		t.Errorf("expected ATX heading rendered as <h1>")
	}
}

func TestRun_StdinTitleInvalidatesContentCache(t *testing.T) {
	t.Parallel()
	content := "stdin-title-cache-uniq\nplain text\n"

	first := readRendered(t, pipeToHTML(t, content, Options{Title: "First Title"}))
	if !strings.Contains(first, "<title>First Title</title>") {
		t.Fatalf("expected first title in cached render")
	}

	second := readRendered(t, pipeToHTML(t, content, Options{Title: "Second Title"}))
	if !strings.Contains(second, "<title>Second Title</title>") {
		t.Fatalf("expected changed title to invalidate cached render")
	}
	if strings.Contains(second, "<title>First Title</title>") {
		t.Fatalf("stale cached title reused after title changed")
	}
}

func TestRun_StdinForceMarkdown(t *testing.T) {
	t.Parallel()
	content := "force-md-uniq just some plain words\n"
	html := readRendered(t, pipeToHTML(t, content, Options{Markdown: true}))
	if strings.Contains(html, "language-plaintext") {
		t.Errorf("--markdown must not use the plain wrapper")
	}
	if !strings.Contains(html, "<p>") {
		t.Errorf("expected a paragraph from the Markdown render")
	}
}

func TestRun_StdinForcePlain(t *testing.T) {
	t.Parallel()
	// Markdownish content forced down the plain path: it must NOT be rendered as
	// Markdown (no real <h1> element), though it may be syntax-highlighted.
	content := "force-plain-uniq\n# Heading\n\n```go\nx := 1\n```\n"
	html := readRendered(t, pipeToHTML(t, content, Options{Plain: true}))
	if !strings.Contains(html, "<pre") {
		t.Errorf("--plain must produce a <pre> block, got:\n%s", html)
	}
	if strings.Contains(html, "<h1") {
		t.Errorf("--plain must not render Markdown headings as <h1>")
	}
}

func TestRun_StdinBinaryRefused(t *testing.T) {
	t.Parallel()
	_, err := Run(Options{Stdin: strings.NewReader("bin-uniq\x00\x01\x02data"), Plain: true, NoOpen: true})
	if err == nil {
		t.Fatalf("expected an error for binary input")
	}
	if !strings.Contains(err.Error(), "binary") {
		t.Errorf("expected a binary error, got: %v", err)
	}
}

func TestRun_StdinEmpty(t *testing.T) {
	t.Parallel()
	_, err := Run(Options{Stdin: strings.NewReader(""), NoOpen: true})
	if err == nil {
		t.Fatalf("expected an error for empty stdin")
	}
}
