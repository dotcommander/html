package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_Smoke(t *testing.T) {
	t.Parallel()

	src := []byte("# Hello World\n\nSome text.\n\n```go\nfmt.Println(1)\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n[link](https://example.com)\n")
	got, err := Render(src, Options{FallbackTitle: "fallback"})
	require.NoError(t, err)

	checks := []struct {
		name    string
		contain string
	}{
		{"doctype", "<!DOCTYPE html>"},
		{"title", "<title>Hello World</title>"},
		{"h1 tag", "<h1"},
		{"chroma class", "chroma"},
		{"table tag", "<table"},
		{"link href", `href="https://example.com"`},
		{"markdown-body class", ".markdown-body"},
		{"style tag", "<style>"},
		{"script tag", "<script>"},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, got, c.contain)
		})
	}
}

func assertPaletteControls(t *testing.T, got string) {
	t.Helper()

	assert.Contains(t, got, `id="theme-toggle"`)
	assert.Contains(t, got, `class="palette-switcher"`)
	for _, palette := range []string{"sepia", "blue", "green", "rose", "catppuccin"} {
		assert.Contains(t, got, `data-palette-choice="`+palette+`"`)
	}
	assert.Contains(t, got, "html-palette")
}

func TestRender_TitleFallback(t *testing.T) {
	t.Parallel()

	src := []byte("just a paragraph\n\n## subheading\n")
	got, err := Render(src, Options{FallbackTitle: "myfile"})
	require.NoError(t, err)

	assert.Contains(t, got, "<title>myfile</title>")
}

// TestRender_SynthesizedHeading verifies that a document without a level-1
// heading gets a visible subject heading synthesized from the fallback title,
// injected as the first element inside the article body.
func TestRender_SynthesizedHeading(t *testing.T) {
	t.Parallel()

	src := []byte("just a paragraph\n\n## subheading\n")
	got, err := Render(src, Options{FallbackTitle: "myfile"})
	require.NoError(t, err)

	// The synthesized <h1> sits at the top of the rendered body, before the
	// first paragraph.
	idxHeading := strings.Index(got, "<h1>myfile</h1>")
	idxBody := strings.Index(got, "just a paragraph")
	require.GreaterOrEqual(t, idxHeading, 0, "expected synthesized <h1>myfile</h1>")
	require.Less(t, idxHeading, idxBody, "synthesized heading must precede the body content")
}

// TestRender_NoDuplicateHeading verifies that a document that already begins
// with a level-1 heading is left untouched — no second heading is synthesized.
func TestRender_NoDuplicateHeading(t *testing.T) {
	t.Parallel()

	src := []byte("# Real Title\n\nBody.\n")
	got, err := Render(src, Options{FallbackTitle: "fallback"})
	require.NoError(t, err)

	assert.Equal(t, 1, strings.Count(got, "<h1"), "exactly one h1 — the document's own; none synthesized")
	assert.NotContains(t, got, "<h1>fallback</h1>", "fallback heading must not be injected when an h1 exists")
}

func TestRender_TitleEscaped(t *testing.T) {
	t.Parallel()

	// analyze walks only *ast.Text/*ast.String descendants; inline HTML nodes
	// (RawHTML) are intentionally skipped. So "# A & B <tag>" yields title
	// "A & B", which is then HTML-escaped to "A &amp; B". The <tag> is dropped.
	src := []byte("# A & B <tag>\n")
	got, err := Render(src, Options{FallbackTitle: "fallback"})
	require.NoError(t, err)

	titleStart := strings.Index(got, "<title>")
	titleEnd := strings.Index(got, "</title>")
	require.True(t, titleStart >= 0 && titleEnd > titleStart, "expected <title>...</title> in output")
	titleContent := got[titleStart : titleEnd+len("</title>")]

	// The ampersand must be escaped.
	assert.Contains(t, titleContent, "&amp;")
	// The raw "<tag>" literal must not appear inside the <title> element.
	assert.NotContains(t, titleContent, "<tag>")
}

// TestRender_FullPageFixture renders a representative document and asserts the
// full-page structure — not just isolated fragments. It guards against
// regressions in embedded-asset ordering, GFM support, heading IDs, and the
// copy/theme script hooks.
func TestRender_FullPageFixture(t *testing.T) {
	t.Parallel()

	src := []byte(`# Document Title

## Section One

Some text with ` + "`inline code`" + `.

- [x] done task
- [ ] todo task

| Col A | Col B |
|-------|-------|
| 1     | 2     |

` + "```go\nfmt.Println(\"hi\")\n```" + `

### Subsection
`)

	got, err := Render(src, Options{FallbackTitle: "fallback"})
	require.NoError(t, err)

	// Asset ordering: the theme script lives in <head>, before the inlined
	// <style> and before <body>. "html-theme" is the localStorage key, unique
	// to theme.js; "<style>"/"<body>" are structural anchors.
	idxTheme := strings.Index(got, "html-theme")
	idxStyle := strings.Index(got, "<style>")
	idxBody := strings.Index(got, "<body>")
	require.GreaterOrEqual(t, idxTheme, 0, "theme script (html-theme) must be present")
	require.GreaterOrEqual(t, idxStyle, 0, "<style> must be present")
	require.GreaterOrEqual(t, idxBody, 0, "<body> must be present")
	require.Less(t, idxTheme, idxStyle, "theme script must precede <style>")
	require.Less(t, idxStyle, idxBody, "<style> must precede <body>")
	assertPaletteControls(t, got)

	// The copy script ("Copy code to clipboard" is its unique aria-label) must
	// appear after <body> — it is the trailing <script>.
	idxCopy := strings.Index(got, "Copy code to clipboard")
	require.GreaterOrEqual(t, idxCopy, 0, "copy script must be present")
	require.Less(t, idxBody, idxCopy, "copy script must follow <body>")

	checks := []struct {
		name    string
		contain string
	}{
		{"h2 heading id", "<h2 id="},
		{"h3 heading id", "<h3 id="},
		{"gfm table", "<table"},
		{"gfm task list checkbox", `type="checkbox"`},
		{"base.css embedded", ".markdown-body"},
		{"responsive page width", "width: min(calc(100% - 2rem), 46rem);"},
		{"highlight css embedded", "chroma"},
		{"chroma code block", `class="chroma`},
		{"dark highlight targets emitted class", `:root[data-theme="dark"] .chroma.light`},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, got, c.contain)
		})
	}
}

// TestRender_RawHTMLPassthroughDefault verifies the default mode renders raw
// HTML verbatim — the trusted local-viewer behavior.
func TestRender_RawHTMLPassthroughDefault(t *testing.T) {
	t.Parallel()

	src := []byte("# Title\n\n<script>alert(1)</script>\n\nSome **bold** text.\n")
	got, err := Render(src, Options{FallbackTitle: "fallback"})
	require.NoError(t, err)

	assert.Contains(t, got, "alert(1)", "raw HTML must pass through in the default mode")
	assert.Contains(t, got, "<strong>bold</strong>", "Markdown still renders")
}

// TestRender_SafeModeStripsRawHTML verifies safe mode omits raw HTML while
// still rendering ordinary Markdown.
func TestRender_SafeModeStripsRawHTML(t *testing.T) {
	t.Parallel()

	src := []byte("# Title\n\n<script>alert(1)</script>\n\nSome **bold** text.\n")
	got, err := Render(src, Options{FallbackTitle: "fallback", Safe: true})
	require.NoError(t, err)

	assert.NotContains(t, got, "alert(1)", "raw HTML must NOT pass through in safe mode")
	assert.Contains(t, got, "<strong>bold</strong>", "Markdown still renders in safe mode")
}
