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

func TestRenderPlain_CodeTheme(t *testing.T) {
	t.Parallel()
	src := "package main\n\nfunc main() {}\n"
	out, err := Render([]byte(src), Options{FallbackTitle: "x", Plain: true, Lang: "go", CodeTheme: "dracula"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assert.Contains(t, out, `class="chroma`)
	assert.Contains(t, out, "#282a36")
}

func TestRenderPlain_CSVTable(t *testing.T) {
	t.Parallel()
	src := "name,status,count\nalpha,ready,2\nbeta,<blocked> & queued,5\n"
	out, err := Render([]byte(src), Options{FallbackTitle: "data.csv", Plain: true, SourceName: "data.csv"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assert.Contains(t, out, `<table class="plain-data-table">`)
	assert.Contains(t, out, `<th scope="col">status</th>`)
	assert.Contains(t, out, `<td>&lt;blocked&gt; &amp; queued</td>`)
	assert.NotContains(t, out, `language-plaintext`)
}

func TestRenderPlain_WhitespaceTable(t *testing.T) {
	t.Parallel()
	src := "NAME      PID   CPU\napi       123   4.5\nworker    456   0.1\n"
	out, err := Render([]byte(src), Options{FallbackTitle: "ps", Plain: true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assert.Contains(t, out, `<table class="plain-data-table">`)
	assert.Contains(t, out, `<th scope="col">PID</th>`)
	assert.Contains(t, out, `<td>worker</td>`)
	assert.NotContains(t, out, `language-plaintext`)
}

func TestRenderPlain_MultiSectionAlignedTables(t *testing.T) {
	t.Parallel()
	src := `codex skill leaderboard
files: 1446  events: 8745
cache: 1404 hit  42 scanned
rank skill                                  uses  loads matches sessions
1    next                                   1447   1447       0       50
2    go-dev-patterns                        1128   1128       0      467

claude skill leaderboard
files: 11349  events: 302
cache: 10961 hit  388 scanned
rank skill                                  uses  loads matches sessions
1    repo-audit-deep                          29     29       0       29
2    audit                                    20     20       0       19
`
	out, err := Render([]byte(src), Options{FallbackTitle: "skills", Plain: true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assert.Contains(t, out, `<section class="plain-table-section">`)
	assert.Contains(t, out, `<h2>codex skill leaderboard</h2>`)
	assert.Contains(t, out, `<pre class="plain-table-meta"><code class="language-plaintext">files: 1446  events: 8745`)
	assert.Contains(t, out, `<table class="plain-data-table">`)
	assert.Contains(t, out, `<th scope="col">sessions</th>`)
	assert.Contains(t, out, `<td>go-dev-patterns</td>`)
	assert.Contains(t, out, `<h2>claude skill leaderboard</h2>`)
	assert.NotContains(t, out, `class="chroma`)
}

func TestRenderPlain_ExplicitLangKeepsPre(t *testing.T) {
	t.Parallel()
	src := "name,status\nalpha,ready\nbeta,queued\n"
	out, err := Render([]byte(src), Options{FallbackTitle: "x", Plain: true, Lang: "text"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assert.Contains(t, out, `<pre><code class="language-plaintext">`)
	assert.NotContains(t, out, `<table class="plain-data-table">`)
}

func TestRenderPlain_CommandOutputStaysPre(t *testing.T) {
	t.Parallel()
	src := "total 8\ndrwxr-xr-x  4 u g 128 .\n-rw-r--r--  1 u g 10 a.txt\n"
	out, err := Render([]byte(src), Options{FallbackTitle: "ls", Plain: true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assert.Contains(t, out, `<pre><code class="language-plaintext">`)
	assert.NotContains(t, out, `<table class="plain-data-table">`)
}

func TestRenderPlain_TwoColumnProseStaysPre(t *testing.T) {
	t.Parallel()
	src := "hello world\nplain words\n"
	out, err := Render([]byte(src), Options{FallbackTitle: "notes", Plain: true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assert.Contains(t, out, `<pre><code class="language-plaintext">`)
	assert.NotContains(t, out, `<table class="plain-data-table">`)
}

func TestRenderPlain_ProseCommandOutputStaysPre(t *testing.T) {
	t.Parallel()
	src := "hey, metrix. sounds like a solid brand name. short, punchy, tech-forward.\n\n" +
		"if you're building something:\n" +
		"- it fits the metrics or matrix vibe well\n" +
		"- easy to spell, easy to remember\n"
	out, err := Render([]byte(src), Options{FallbackTitle: "stdin", Plain: true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assert.Contains(t, out, `<pre><code class="language-plaintext">`)
	assert.NotContains(t, out, `class="chroma`)
}

func TestRenderPlain_AutoDetectsGoCode(t *testing.T) {
	t.Parallel()
	src := "package main\n\nfunc main() { println(\"hi\") }\n"
	out, err := Render([]byte(src), Options{FallbackTitle: "stdin", Plain: true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assert.Contains(t, out, `class="chroma`)
	assert.NotContains(t, out, `language-plaintext`)
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
