package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportJSSortDirectionUsesActiveHeaderState(t *testing.T) {
	t.Parallel()

	js := reportJS()

	assert.Contains(t, js, `const asc = cell.getAttribute("aria-sort") !== "ascending";`)
	assert.NotContains(t, js, `let asc = true;`)
	assert.Equal(t, 1, strings.Count(js, `headers[index].setAttribute("aria-sort", asc ? "ascending" : "descending");`))
	assert.Contains(t, js, `const syncSortLabels = () => {`)
	assert.Contains(t, js, "button.setAttribute(\"aria-label\", `Sort by ${label} ${nextDirection}`);")
	assert.Contains(t, js, "mobileSort.value = `${activeIndex}:${headers[activeIndex].getAttribute(\"aria-sort\")}`;")
	assert.Contains(t, js, `const sortBy = (index, asc) => {`)
	assert.Contains(t, js, `sortBy(column, direction !== "descending");`)
	assert.Contains(t, js, `syncSortLabels();`)
}

func TestReportJSSlidesKeyboardNavigation(t *testing.T) {
	t.Parallel()

	js := reportJS()

	assert.Contains(t, js, `document.querySelectorAll("[data-report-slides]")`)
	assert.Contains(t, js, `case "PageDown":`)
	assert.Contains(t, js, `slide.hidden = !active;`)
	assert.Contains(t, js, `[data-slide-prev]`)
	assert.Contains(t, js, `[data-slide-next]`)
	assert.Contains(t, js, `[data-slide-status]`)
	assert.Contains(t, js, `prev.disabled = index === 0`)
	assert.Contains(t, js, `nextButton.disabled = index === slides.length - 1`)
}

func TestReportSlideHiddenStateSurvivesMobileCSS(t *testing.T) {
	t.Parallel()

	css := baseCSS()
	hiddenRule := `.report-slide[hidden] {
  display: none;
}`
	require.Contains(t, css, hiddenRule)
	assert.NotContains(t, css, ".report-slide,\n  .report-slide[hidden]", "mobile CSS must not force JS-hidden slides visible")
	assert.NotContains(t, css, "position: sticky;\n    bottom: 0.75rem", "mobile slide controls must not overlay slide content")
}

func TestReportTableHiddenRowsSurviveMobileCSS(t *testing.T) {
	t.Parallel()

	css := baseCSS()

	assert.Contains(t, css, ".report-table tr[hidden] {\n    display: none;\n  }")
}

func TestReportTableMobileSortCSS(t *testing.T) {
	t.Parallel()

	css := baseCSS()

	assert.Contains(t, css, ".report-mobile-sort {\n  display: none;\n}")
	assert.Contains(t, css, ".report-mobile-sort select:focus-visible")
	assert.Contains(t, css, ".report-mobile-sort {\n    display: block;\n  }")
}

func TestFileTreeCSSProvidesGuidesAndWrapping(t *testing.T) {
	t.Parallel()

	css := baseCSS()

	assert.Contains(t, css, ".file-tree {\n  display: grid;")
	assert.Contains(t, css, ".file-tree li::before")
	assert.Contains(t, css, ".file-tree li::after")
	assert.Contains(t, css, "overflow-wrap: anywhere;")
	assert.Contains(t, css, ".file-tree li:hover")
}

func TestLogCSSProvidesSeverityLayout(t *testing.T) {
	t.Parallel()

	css := baseCSS()

	assert.Contains(t, css, ".log-lines {\n  display: grid;")
	assert.Contains(t, css, ".log-line {\n  display: grid;")
	assert.Contains(t, css, ".log-level")
	assert.Contains(t, css, ".log-message")
	assert.Contains(t, css, ".log-error .log-level")
	assert.Contains(t, css, ".log-line {\n    grid-template-columns: 3.75rem minmax(0, 1fr);")
}

func TestDiffCSSProvidesSummaryAndRows(t *testing.T) {
	t.Parallel()

	css := baseCSS()

	assert.Contains(t, css, ".diff-summary {\n  display: flex;")
	assert.Contains(t, css, ".diff-summary .diff-added strong")
	assert.Contains(t, css, ".diff-summary .diff-removed strong")
	assert.Contains(t, css, ".diff-view span {\n  display: block;")
	assert.Contains(t, css, ".diff-view .add")
	assert.Contains(t, css, ".diff-view .del")
	assert.Contains(t, css, ".diff-view .hunk")
}

func TestJSONOverviewCSSProvidesPills(t *testing.T) {
	t.Parallel()

	css := baseCSS()

	assert.Contains(t, css, ".json-overview {\n  display: flex;")
	assert.Contains(t, css, ".json-overview div,\n.json-overview span")
	assert.Contains(t, css, ".json-overview dt,\n.json-overview strong")
	assert.Contains(t, css, ".json-overview dd")
}

func TestCodeOverviewCSSProvidesPills(t *testing.T) {
	t.Parallel()

	css := baseCSS()

	assert.Contains(t, css, ".code-overview {\n  display: flex;")
	assert.Contains(t, css, ".code-overview div")
	assert.Contains(t, css, ".code-overview dt")
	assert.Contains(t, css, ".code-overview dd")
}

func TestArticleOverviewCSSProvidesPills(t *testing.T) {
	t.Parallel()

	css := baseCSS()

	assert.Contains(t, css, ".article-overview {\n  display: flex;")
	assert.Contains(t, css, ".article-overview div")
	assert.Contains(t, css, ".article-overview dt")
	assert.Contains(t, css, ".article-overview dd")
}

func TestTextOverviewCSSProvidesPills(t *testing.T) {
	t.Parallel()

	css := baseCSS()

	assert.Contains(t, css, ".text-overview {\n  display: flex;")
	assert.Contains(t, css, ".text-overview div")
	assert.Contains(t, css, ".text-overview dt")
	assert.Contains(t, css, ".text-overview dd")
	assert.Contains(t, css, ".markdown-body pre.report-text")
	assert.Contains(t, css, "white-space: pre-wrap;")
	assert.Contains(t, css, "overflow-wrap: anywhere;")
}

func TestRecordCardCSSWrapsLongTitles(t *testing.T) {
	t.Parallel()

	css := baseCSS()

	assert.Contains(t, css, ".record-card h3 {\n  margin-top: 0;")
	assert.Contains(t, css, "overflow-wrap: anywhere;")
	assert.Contains(t, css, ".record-empty {\n  margin: 0;")
	assert.Contains(t, css, `font: 0.92rem ui-sans-serif`)
}

func TestTranscriptCSSProvidesTurnLayout(t *testing.T) {
	t.Parallel()

	css := baseCSS()

	assert.Contains(t, css, ".transcript-turns {\n  display: grid;")
	assert.Contains(t, css, ".transcript-turn {\n  display: grid;")
	assert.Contains(t, css, "grid-template-columns: minmax(6rem, 10rem) minmax(0, 1fr);")
	assert.Contains(t, css, ".transcript-speaker")
	assert.Contains(t, css, ".transcript-text p + p")
	assert.Contains(t, css, ".transcript-turn {\n    grid-template-columns: 1fr;")
}

func TestMobileLayoutReservesSpaceForThemeControls(t *testing.T) {
	t.Parallel()

	css := baseCSS()

	assert.Contains(t, css, ".theme-controls {\n    top: 0.75rem;")
	assert.Contains(t, css, "padding: 4.4rem 1.25rem 1.25rem;")
}
