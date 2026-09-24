package reportview

import (
	"strings"
	"testing"

	render "github.com/dotcommander/html/internal/render"
	"github.com/dotcommander/html/internal/report"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateReportExecutesOnce(t *testing.T) {
	t.Parallel()
	src := []byte("# Report\n\n## One\n\nText.\n\n## Two\n\nMore text.\n")
	for _, layout := range []report.LayoutOverride{report.LayoutOverrideSingle, report.LayoutOverrideTabs, report.LayoutOverrideSlides} {
		analysis, plan := report.Plan(t.Context(), src, report.Options{SourceName: "report.md", Layout: layout, Planner: report.PlannerOff})
		toc := true
		got, err := RenderReport(src, render.Options{Template: "custom", TemplateSource: `PAGE{{.Content}}END{{.TOC}}`, TOC: &toc}, analysis, plan)
		require.NoError(t, err)
		assert.Equal(t, 1, strings.Count(got, "PAGE"))
		assert.Equal(t, 1, strings.Count(got, "END"))
		assert.Contains(t, got, `<h1 id="report">Report</h1>`)
		assert.Equal(t, 1, strings.Count(got, `<nav class="toc"`))
	}
}
