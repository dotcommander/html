package render

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tocNav returns the substring of got spanning the first <nav class="toc"> …
// </nav>, failing the test if no TOC is present.
func tocNav(t *testing.T, got string) string {
	t.Helper()
	start := strings.Index(got, `<nav class="toc"`)
	end := strings.Index(got, "</nav>")
	require.True(t, start >= 0 && end > start, "expected a <nav class=\"toc\"> … </nav>")
	return got[start:end]
}

// assertTOCHrefsMatchBodyIDs checks every TOC anchor targets an id that exists
// in the rendered document — i.e. TOC links and body heading ids agree.
func assertTOCHrefsMatchBodyIDs(t *testing.T, got string) {
	t.Helper()
	re := regexp.MustCompile(`href="#([^"]+)"`)
	for _, m := range re.FindAllStringSubmatch(tocNav(t, got), -1) {
		assert.Contains(t, got, `id="`+m[1]+`"`,
			"TOC link #%s must match a heading id in the body", m[1])
	}
}

// TestRender_TOCForLongDoc verifies a document with enough headings gets a TOC
// of same-page anchor links, nested by level, placed after the first heading.
func TestRender_TOCForLongDoc(t *testing.T) {
	t.Parallel()

	src := []byte("# Title\n\n## Alpha\n\ntext\n\n### Alpha One\n\ntext\n\n## Beta\n\ntext\n\n## Gamma\n\ntext\n")
	got, err := Render(src, Options{FallbackTitle: "f"})
	require.NoError(t, err)

	nav := tocNav(t, got)
	assert.Equal(t, 4, strings.Count(nav, "</a></li>"), "TOC lists all four h2/h3 headings")
	assert.Contains(t, nav, `class="toc-h2"`)
	assert.Contains(t, nav, `class="toc-h3"`, "nested h3 must appear in the TOC")
	assertTOCHrefsMatchBodyIDs(t, got)

	// The TOC sits after the first heading, not above it.
	idxH1End := strings.Index(got, "</h1>")
	idxTOC := strings.Index(got, `<nav class="toc"`)
	require.True(t, idxH1End >= 0 && idxTOC > idxH1End, "TOC must follow the first heading")
}

// TestRender_TOCAbsentForShortDoc verifies short documents stay uncluttered.
func TestRender_TOCAbsentForShortDoc(t *testing.T) {
	t.Parallel()

	src := []byte("# Title\n\n## Only One\n\ntext\n\n## Only Two\n\ntext\n")
	got, err := Render(src, Options{FallbackTitle: "f"})
	require.NoError(t, err)

	assert.NotContains(t, got, `<nav class="toc"`, "a 2-heading doc must not gain a TOC")
}

// TestRender_TOCDuplicateHeadingIDs verifies duplicate heading text yields
// distinct ids and the TOC links match them exactly.
func TestRender_TOCDuplicateHeadingIDs(t *testing.T) {
	t.Parallel()

	src := []byte("# T\n\n## Notes\n\na\n\n## Other\n\nb\n\n## Notes\n\nc\n\n## More\n\nd\n")
	got, err := Render(src, Options{FallbackTitle: "f"})
	require.NoError(t, err)

	assertTOCHrefsMatchBodyIDs(t, got)

	re := regexp.MustCompile(`href="#([^"]+)"`)
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(tocNav(t, got), -1) {
		assert.False(t, seen[m[1]], "duplicate TOC href #%s — heading ids must be unique", m[1])
		seen[m[1]] = true
	}
	assert.Len(t, seen, 4, "four distinct heading ids expected")
}

// TestRender_TOCEscapesHeadingText verifies heading text is HTML-escaped in the
// TOC and raw inline HTML does not leak.
func TestRender_TOCEscapesHeadingText(t *testing.T) {
	t.Parallel()

	src := []byte("# T\n\n## A & B <x>\n\na\n\n## C\n\nb\n\n## D\n\nc\n\n## E\n\nd\n")
	got, err := Render(src, Options{FallbackTitle: "f"})
	require.NoError(t, err)

	nav := tocNav(t, got)
	assert.Contains(t, nav, "&amp;", "ampersand in heading text must be escaped in the TOC")
	assert.NotContains(t, nav, "<x>", "raw inline HTML must not leak into the TOC")
}

// TestRender_HeadingAnchorScriptEmbedded verifies the heading-anchor enhancement
// script is shipped in the page.
func TestRender_HeadingAnchorScriptEmbedded(t *testing.T) {
	t.Parallel()

	got, err := Render([]byte("# T\n\n## S\n\ntext\n"), Options{FallbackTitle: "f"})
	require.NoError(t, err)

	assert.Contains(t, got, "heading-anchor", "headings.js (or its CSS) must be present")
}

// TestRender_MaxWidthOverride verifies the config max_width reaches the page CSS.
func TestRender_MaxWidthOverride(t *testing.T) {
	t.Parallel()

	got, err := Render([]byte("# T\n\ntext\n"), Options{FallbackTitle: "f", MaxWidth: "55rem"})
	require.NoError(t, err)

	assert.Contains(t, got, "max-width: 55rem")
}

// TestRender_DefaultThemeInjected verifies a configured theme is injected as the
// pre-paint default read by theme.js.
func TestRender_DefaultThemeInjected(t *testing.T) {
	t.Parallel()

	got, err := Render([]byte("# T\n\ntext\n"), Options{FallbackTitle: "f", Theme: "dark"})
	require.NoError(t, err)

	assert.Contains(t, got, `HTML_DEFAULT_THEME = "dark"`)
}

// TestRender_TOCForceOnShortDoc verifies toc:true forces a TOC on a short doc.
func TestRender_TOCForceOnShortDoc(t *testing.T) {
	t.Parallel()

	on := true
	got, err := Render([]byte("# T\n\n## A\n\nx\n\n## B\n\nx\n"), Options{FallbackTitle: "f", TOC: &on})
	require.NoError(t, err)

	assert.Contains(t, got, `<nav class="toc"`, "toc:true must force a TOC even below the auto threshold")
}

// TestRender_TOCForceOff verifies toc:false suppresses an otherwise-automatic TOC.
func TestRender_TOCForceOff(t *testing.T) {
	t.Parallel()

	off := false
	src := []byte("# T\n\n## A\n\nx\n\n## B\n\nx\n\n## C\n\nx\n\n## D\n\nx\n")
	got, err := Render(src, Options{FallbackTitle: "f", TOC: &off})
	require.NoError(t, err)

	assert.NotContains(t, got, `<nav class="toc"`, "toc:false must suppress the TOC despite enough headings")
}
