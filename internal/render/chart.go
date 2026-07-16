package render

import (
	"fmt"
	htmlpkg "html"
	"math"
	"strconv"
	"strings"

	"github.com/dotcommander/html/internal/report"
)

const (
	chartMaxRows       = 1000
	chartMaxCategories = 24
	chartWidth         = 960.0
	chartPlotLeft      = 190.0
	chartPlotRight     = 780.0
	chartTop           = 42.0
	chartRowHeight     = 34.0
	chartBarHeight     = 20.0
	chartMaxLabelRunes = 20
)

type chartDatum struct {
	category string
	label    string
	value    float64
}

func chartView(src []byte, analysis report.Analysis, options map[string]string) string {
	if typ := strings.TrimSpace(options["type"]); typ != "" && typ != "bar" {
		return chartDiagnostic("only horizontal bar charts are supported")
	}
	headers, rows := tableRows(src, analysis)
	if len(headers) == 0 {
		return chartDiagnostic("record columns are unavailable")
	}
	if len(rows) == 0 {
		return chartDiagnostic("no record rows are available")
	}
	if len(rows) > chartMaxRows {
		return chartDiagnostic(fmt.Sprintf("%d rows exceed the %d-row chart limit", len(rows), chartMaxRows))
	}
	x, y, ok := chartColumns(headers, rows, options)
	if !ok {
		return chartDiagnostic("choose one categorical column and one fully finite numeric column; automatic selection requires exactly two columns")
	}
	if len(rows) > chartMaxCategories {
		return chartDiagnostic(fmt.Sprintf("%d visible categories exceed the %d-category chart limit", len(rows), chartMaxCategories))
	}

	data := make([]chartDatum, 0, len(rows))
	minValue, maxValue := 0.0, 0.0
	for _, row := range rows {
		if x >= len(row) || y >= len(row) {
			return chartDiagnostic("a record is missing a selected chart value")
		}
		category := strings.TrimSpace(cleanTableText(row[x]))
		valueText := strings.TrimSpace(cleanTableText(row[y]))
		value, valid := finiteNumber(valueText)
		if category == "" || !valid {
			return chartDiagnostic("chart categories must be nonempty and numeric values must be finite")
		}
		data = append(data, chartDatum{category: category, label: valueText, value: value})
		minValue = min(minValue, value)
		maxValue = max(maxValue, value)
	}
	return renderBarChart(headers[y], headers[x], data, minValue, maxValue)
}

func chartColumns(headers []string, rows [][]string, options map[string]string) (x, y int, ok bool) {
	// The component contract follows chart conventions: x names the category
	// dimension and y names the numeric measure, even though bars extend along
	// the horizontal axis in the rendered SVG.
	if requestedX, requestedY := strings.TrimSpace(options["x"]), strings.TrimSpace(options["y"]); requestedX != "" || requestedY != "" {
		x, xOK := chartColumnIndex(headers, requestedX)
		y, yOK := chartColumnIndex(headers, requestedY)
		if xOK && yOK && x != y && columnIsCategorical(rows, x) && !columnIsNumeric(rows, x) && columnIsNumeric(rows, y) {
			return x, y, true
		}
		// An explicit but invalid selection must not silently fall back to
		// different inferred columns.
		return 0, 0, false
	}
	if len(headers) != 2 {
		return 0, 0, false
	}
	firstNumeric := columnIsNumeric(rows, 0)
	secondNumeric := columnIsNumeric(rows, 1)
	switch {
	case firstNumeric && !secondNumeric && columnIsCategorical(rows, 1):
		return 1, 0, true
	case secondNumeric && !firstNumeric && columnIsCategorical(rows, 0):
		return 0, 1, true
	default:
		return 0, 0, false
	}
}

func chartColumnIndex(headers []string, requested string) (int, bool) {
	if requested == "" {
		return 0, false
	}
	found := -1
	for i, header := range headers {
		if strings.EqualFold(strings.TrimSpace(header), requested) {
			if found >= 0 {
				return 0, false
			}
			found = i
		}
	}
	return found, found >= 0
}

func columnIsNumeric(rows [][]string, column int) bool {
	if len(rows) == 0 {
		return false
	}
	for _, row := range rows {
		if column >= len(row) {
			return false
		}
		if _, ok := finiteNumber(strings.TrimSpace(cleanTableText(row[column]))); !ok {
			return false
		}
	}
	return true
}

func columnIsCategorical(rows [][]string, column int) bool {
	if len(rows) == 0 {
		return false
	}
	for _, row := range rows {
		if column >= len(row) || strings.TrimSpace(cleanTableText(row[column])) == "" {
			return false
		}
	}
	return true
}

func finiteNumber(value string) (float64, bool) {
	if value == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(value, 64)
	return n, err == nil && !math.IsNaN(n) && !math.IsInf(n, 0)
}

func renderBarChart(valueHeader, categoryHeader string, data []chartDatum, minValue, maxValue float64) string {
	domainMin, domainMax := minValue, maxValue
	if domainMin == 0 && domainMax == 0 {
		domainMin, domainMax = -1, 1
	}
	plotWidth := chartPlotRight - chartPlotLeft
	maxMagnitude := math.Max(math.Abs(domainMin), math.Abs(domainMax))
	normalizedMin := domainMin / maxMagnitude
	normalizedMax := domainMax / maxMagnitude
	scale := func(value float64) float64 {
		normalizedValue := value / maxMagnitude
		return chartPlotLeft + ((normalizedValue-normalizedMin)/(normalizedMax-normalizedMin))*plotWidth
	}
	baseline := scale(0)
	height := chartTop + float64(len(data))*chartRowHeight + 34
	accessibleName := fmt.Sprintf("Horizontal bar chart of %s by %s", valueHeader, categoryHeader)
	description := fmt.Sprintf("%d categories. Values range from %s to %s. The vertical line marks zero.", len(data), formatChartNumber(minValue), formatChartNumber(maxValue))

	var b strings.Builder
	fmt.Fprintf(&b, `<div class="report-chart-wrap"><svg class="report-chart" viewBox="0 0 %.0f %.0f" role="img" aria-label="%s"><desc>%s</desc>`, chartWidth, height, htmlpkg.EscapeString(accessibleName), htmlpkg.EscapeString(description))
	fmt.Fprintf(&b, `<line class="report-chart-zero" x1="%s" y1="24" x2="%s" y2="%s"/>`, chartCoord(baseline), chartCoord(baseline), chartCoord(height-22))
	for i, datum := range data {
		centerY := chartTop + float64(i)*chartRowHeight
		valueX := scale(datum.value)
		barX := math.Min(baseline, valueX)
		barWidth := math.Abs(valueX - baseline)
		categoryLabel := boundedChartLabel(datum.category, chartMaxLabelRunes)
		valueLabel := formatChartNumber(datum.value)
		fmt.Fprintf(&b, `<g class="report-chart-row"><title>%s: %s</title>`, htmlpkg.EscapeString(datum.category), htmlpkg.EscapeString(datum.label))
		fmt.Fprintf(&b, `<text class="report-chart-category" x="%s" y="%s">%s</text>`, chartCoord(chartPlotLeft-10), chartCoord(centerY+5), htmlpkg.EscapeString(categoryLabel))
		fmt.Fprintf(&b, `<rect class="report-chart-bar" x="%s" y="%s" width="%s" height="%s"/>`, chartCoord(barX), chartCoord(centerY-chartBarHeight/2), chartCoord(barWidth), chartCoord(chartBarHeight))
		if datum.value == 0 {
			fmt.Fprintf(&b, `<circle class="report-chart-zero-value" cx="%s" cy="%s" r="3.00"/>`, chartCoord(baseline), chartCoord(centerY))
		}
		fmt.Fprintf(&b, `<text class="report-chart-value" x="%s" y="%s">%s</text></g>`, chartCoord(chartPlotRight+12), chartCoord(centerY+5), htmlpkg.EscapeString(valueLabel))
	}
	fmt.Fprintf(&b, `<text class="report-chart-axis" x="%s" y="%s">%s</text>`, chartCoord(chartPlotLeft), chartCoord(height-5), htmlpkg.EscapeString(formatChartNumber(domainMin)))
	fmt.Fprintf(&b, `<text class="report-chart-axis report-chart-axis-end" x="%s" y="%s">%s</text>`, chartCoord(chartPlotRight), chartCoord(height-5), htmlpkg.EscapeString(formatChartNumber(domainMax)))
	b.WriteString(`</svg></div>`)
	return b.String()
}

func chartDiagnostic(message string) string {
	return `<p class="report-chart-diagnostic" role="status"><strong>Chart unavailable:</strong> ` + htmlpkg.EscapeString(message) + `.</p>`
}

func chartCoord(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func formatChartNumber(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func boundedChartLabel(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes-1]) + "…"
}
