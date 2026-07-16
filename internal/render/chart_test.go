package render

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dotcommander/html/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chartAnalysis(src string) report.Analysis {
	return report.Analyze([]byte(src), "chart.csv")
}

func TestChartViewInfersColumnsAndIsAccessible(t *testing.T) {
	t.Parallel()
	src := "name,score\nAlpha & <one>,10\nBeta,-5\n"

	got := chartView([]byte(src), chartAnalysis(src), map[string]string{"type": "bar"})

	assert.Contains(t, got, `<svg class="report-chart"`)
	assert.Contains(t, got, `role="img"`)
	assert.Contains(t, got, `aria-label="Horizontal bar chart of score by name"`)
	assert.Contains(t, got, `<desc>2 categories. Values range from -5 to 10. The vertical line marks zero.</desc>`)
	assert.Contains(t, got, `Alpha &amp; &lt;one&gt;`)
	assert.NotContains(t, got, `Alpha & <one>`)
	assert.NotContains(t, got, "NaN")
	assert.NotContains(t, got, "Inf")
}

func TestChartViewUsesExplicitColumns(t *testing.T) {
	t.Parallel()
	src := "name,region,score\nAlpha,East,10\nBeta,West,5\n"

	got := chartView([]byte(src), chartAnalysis(src), map[string]string{"type": "bar", "x": "name", "y": "score"})

	assert.Contains(t, got, `aria-label="Horizontal bar chart of score by name"`)
	assert.NotContains(t, got, `Chart unavailable`)
}

func TestChartViewDoesNotInferAfterInvalidExplicitColumns(t *testing.T) {
	t.Parallel()
	src := "name,score\nAlpha,10\n"

	got := chartView([]byte(src), chartAnalysis(src), map[string]string{"type": "bar", "x": "missing", "y": "score"})

	assert.Contains(t, got, `Chart unavailable:`)
	assert.NotContains(t, got, `<svg`)
}

func TestChartViewRejectsPartialAndReversedExplicitColumns(t *testing.T) {
	t.Parallel()
	src := "name,score\nAlpha,10\n"
	for _, options := range []map[string]string{
		{"type": "bar", "x": "name"},
		{"type": "bar", "x": "score", "y": "name"},
	} {
		got := chartView([]byte(src), chartAnalysis(src), options)
		assert.Contains(t, got, `Chart unavailable:`)
		assert.NotContains(t, got, `<svg`)
	}
}

func TestChartViewBoundsDisplayLabelsButRetainsFullValues(t *testing.T) {
	t.Parallel()
	src := "name,score\nA category name that is intentionally much too long,00000000000000000000000000000001\n"

	got := chartView([]byte(src), chartAnalysis(src), map[string]string{"type": "bar"})

	assert.Contains(t, got, `<title>A category name that is intentionally much too long: 00000000000000000000000000000001</title>`)
	assert.Contains(t, got, `A category name tha…`)
	assert.Contains(t, got, `class="report-chart-value" x="792.00" y="47.00">1</text>`)
}

func TestChartViewRejectsLimitsAndInvalidValues(t *testing.T) {
	t.Parallel()

	rows := func(n int) string {
		var b strings.Builder
		b.WriteString("name,score\n")
		for i := range n {
			fmt.Fprintf(&b, "row-%d,%d\n", i, i)
		}
		return b.String()
	}
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "category limit", src: rows(25), want: "25 visible categories exceed the 24-category chart limit"},
		{name: "row limit", src: rows(1001), want: "1001 rows exceed the 1000-row chart limit"},
		{name: "nan", src: "name,score\nAlpha,NaN\n", want: "choose one categorical column and one fully finite numeric column"},
		{name: "infinity", src: "name,score\nAlpha,+Inf\n", want: "choose one categorical column and one fully finite numeric column"},
		{name: "too many inferred columns", src: "name,region,score\nAlpha,East,1\n", want: "automatic selection requires exactly two columns"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := chartView([]byte(tt.src), chartAnalysis(tt.src), map[string]string{"type": "bar"})
			assert.Contains(t, got, `class="report-chart-diagnostic" role="status"`)
			assert.Contains(t, got, tt.want)
			assert.NotContains(t, got, `<svg`)
		})
	}
}

func TestChartViewGeometryHandlesNumericDomains(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		src          string
		wantBaseline string
		wantBar      string
	}{
		{name: "positive", src: "name,score\nA,10\nB,5\n", wantBaseline: `x1="190.00"`, wantBar: `x="190.00" y="32.00" width="590.00"`},
		{name: "negative", src: "name,score\nA,-10\nB,-5\n", wantBaseline: `x1="780.00"`, wantBar: `x="190.00" y="32.00" width="590.00"`},
		{name: "mixed", src: "name,score\nA,-10\nB,10\n", wantBaseline: `x1="485.00"`, wantBar: `x="190.00" y="32.00" width="295.00"`},
		{name: "all zero", src: "name,score\nA,0\nB,0\n", wantBaseline: `x1="485.00"`, wantBar: `x="485.00" y="32.00" width="0.00"`},
		{name: "single row", src: "name,score\nOnly,5\n", wantBaseline: `x1="190.00"`, wantBar: `x="190.00" y="32.00" width="590.00"`},
		{name: "extreme finite range", src: "name,score\nLow,-1.7976931348623157e308\nHigh,1.7976931348623157e308\n", wantBaseline: `x1="485.00"`, wantBar: `x="190.00" y="32.00" width="295.00"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := chartView([]byte(tt.src), chartAnalysis(tt.src), map[string]string{"type": "bar"})
			assert.Contains(t, got, tt.wantBaseline)
			assert.Contains(t, got, tt.wantBar)
			if tt.name == "all zero" {
				assert.Contains(t, got, `<circle class="report-chart-zero-value" cx="485.00" cy="42.00" r="3.00"/>`)
			}
			assert.NotContains(t, got, "NaN")
			assert.NotContains(t, got, "Inf")
		})
	}
}

func TestRenderReportChartDiagnosticPreservesSiblings(t *testing.T) {
	t.Parallel()
	src := []byte("name,score\nAlpha,not-a-number\n")
	analysis := report.Analyze(src, "scores.csv")
	plan := report.ReportPlan{
		Version: report.PlanVersion, Kind: report.KindCSVRecords, Layout: report.LayoutSinglePage, Mode: report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}},
			{Type: report.ComponentChart, Source: "records", Title: "Chart", Options: map[string]string{"type": "bar"}},
			{Type: report.ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "scores"}, analysis, plan)
	require.NoError(t, err)
	assert.Contains(t, got, `class="report-summary"`)
	assert.Contains(t, got, `Chart unavailable:`)
	assert.Contains(t, got, `data-report-table`)
	assert.Contains(t, got, `not-a-number`)
}
