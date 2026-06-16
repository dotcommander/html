package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReportJSSortDirectionUsesActiveHeaderState(t *testing.T) {
	t.Parallel()

	js := reportJS()

	assert.Contains(t, js, `const asc = cell.getAttribute("aria-sort") !== "ascending";`)
	assert.NotContains(t, js, `let asc = true;`)
	assert.Equal(t, 1, strings.Count(js, `cell.setAttribute("aria-sort", asc ? "ascending" : "descending");`))
}

func TestReportJSSlidesKeyboardNavigation(t *testing.T) {
	t.Parallel()

	js := reportJS()

	assert.Contains(t, js, `document.querySelectorAll("[data-report-slides]")`)
	assert.Contains(t, js, `case "PageDown":`)
	assert.Contains(t, js, `slide.hidden = !active;`)
}
