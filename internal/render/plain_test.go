package render

import (
	"strings"
	"testing"
)

func TestRenderPlain(t *testing.T) {
	t.Parallel()
	src := "# not a heading\nline two\t<tag> & \"q\"\nthird line of plain prose\n"
	out, err := Render([]byte(src), Options{FallbackTitle: "stdin", Plain: true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, `<pre><code class="language-plaintext">`) {
		t.Errorf("expected raw plaintext wrapper, got:\n%s", out)
	}
	if !strings.Contains(out, "# not a heading") {
		t.Errorf("expected literal '# not a heading'")
	}
	if strings.Contains(out, "<h1>") {
		t.Errorf("plain mode must not synthesize an <h1>")
	}
	if !strings.Contains(out, "&lt;tag&gt;") || !strings.Contains(out, "&amp;") {
		t.Errorf("expected HTML-escaped body")
	}
	if !strings.Contains(out, "<title>stdin</title>") {
		t.Errorf("expected fallback title")
	}
}

func TestRenderPlain_Highlight(t *testing.T) {
	t.Parallel()
	src := "package main\n\nfunc main() {}\n"
	out, err := Render([]byte(src), Options{FallbackTitle: "x", Plain: true, Lang: "go"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, `class="chroma`) {
		t.Errorf("expected chroma-highlighted output for Lang=go, got:\n%s", out)
	}
	if strings.Contains(out, "language-plaintext") {
		t.Errorf("highlighted output should not use the raw plaintext wrapper")
	}
}

func TestRenderPlain_LangTextForcesRaw(t *testing.T) {
	t.Parallel()
	src := "package main\nfunc main() {}\n"
	out, err := Render([]byte(src), Options{FallbackTitle: "x", Plain: true, Lang: "text"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, `<pre><code class="language-plaintext">`) {
		t.Errorf("Lang=text must force raw plaintext, got:\n%s", out)
	}
}
