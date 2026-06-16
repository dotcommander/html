package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
	assertPaletteControls(t, out)
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

func TestRenderPlain_Frame(t *testing.T) {
	t.Parallel()
	src := "build started\nok\n"
	out, err := Render([]byte(src), Options{FallbackTitle: "build.log", Plain: true, Frame: true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, `class="term-frame"`) {
		t.Errorf("expected terminal-window frame wrapper, got:\n%s", out)
	}
	if !strings.Contains(out, `class="term-title">build.log<`) {
		t.Errorf("expected the title bar to show the page title")
	}
	if !strings.Contains(out, `class="term-body"`) || !strings.Contains(out, "build started") {
		t.Errorf("expected the framed body to contain the rendered content, got:\n%s", out)
	}
	if !strings.Contains(out, ".term-frame {") {
		t.Errorf("expected frame CSS to be injected when Frame is set")
	}
	assertPaletteControls(t, out)
}

func TestOptions_FrameCacheTag(t *testing.T) {
	t.Parallel()
	plain := Options{Plain: true}
	framed := Options{Plain: true, Frame: true}
	if plain.cacheTag() == framed.cacheTag() {
		t.Errorf("frame must change the cache tag; both = %q", plain.cacheTag())
	}
	if !strings.Contains(framed.cacheTag(), "+frame") {
		t.Errorf("frame cacheTag should contain +frame, got %q", framed.cacheTag())
	}
}

func TestOptions_PaletteCacheTag(t *testing.T) {
	t.Parallel()

	blue := Options{Palette: "blue"}
	rose := Options{Palette: "rose"}
	if blue.cacheTag() == rose.cacheTag() {
		t.Errorf("palette must change the cache tag; both = %q", blue.cacheTag())
	}
	assert.Contains(t, blue.cacheTag(), "+palette:4:blue")
}
