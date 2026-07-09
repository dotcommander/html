package render

import (
	htmlpkg "html"
	"strings"
	"unicode"

	"github.com/dotcommander/html/internal/report"
)

type plainTable struct {
	headers []string
	rows    [][]string
}

type plainTableSection struct {
	title string
	notes []string
	table plainTable
}

func detectPlainTable(src []byte, sourceName string) (plainTable, bool) {
	analysis := report.Analyze(src, sourceName)
	switch analysis.Kind {
	case report.KindCSVRecords, report.KindTSVRecords, report.KindTableRecords:
		headers, rows := tableRows(src, analysis)
		if len(headers) >= 2 && len(rows) > 0 {
			return plainTable{headers: headers, rows: rows}, true
		}
	}

	return parseWhitespaceTable(string(stripUTF8BOM(src)))
}

func renderPlainTableDocument(src []byte, sourceName string) (string, bool) {
	if _, ok := detectPlainTable(src, sourceName); ok {
		return "", false
	}
	sections, ok := parsePlainTableSections(string(stripUTF8BOM(src)))
	if !ok {
		return "", false
	}
	var b strings.Builder
	for _, section := range sections {
		b.WriteString(`<section class="plain-table-section">`)
		if strings.TrimSpace(section.title) != "" {
			b.WriteString(`<h2>`)
			b.WriteString(htmlpkg.EscapeString(section.title))
			b.WriteString(`</h2>`)
		}
		if len(section.notes) > 0 {
			b.WriteString(`<pre class="plain-table-meta"><code class="language-plaintext">`)
			b.WriteString(htmlpkg.EscapeString(strings.Join(section.notes, "\n")))
			b.WriteString(`</code></pre>`)
		}
		b.WriteString(renderPlainTable(section.table))
		b.WriteString(`</section>`)
	}
	return b.String(), true
}

func parsePlainTableSections(text string) ([]plainTableSection, bool) {
	blocks := nonEmptyBlocks(trimOuterBlankLines(text))
	if len(blocks) == 0 {
		return nil, false
	}
	sections := make([]plainTableSection, 0, len(blocks))
	for _, block := range blocks {
		section, ok := parsePlainTableSection(block)
		if !ok {
			return nil, false
		}
		sections = append(sections, section)
	}
	return sections, true
}

func parsePlainTableSection(lines []string) (plainTableSection, bool) {
	headerIndex := -1
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && hasColumnSpacing(line) && looksLikeTableHeader(fields) {
			headerIndex = i
			break
		}
	}
	if headerIndex < 0 || headerIndex == len(lines)-1 {
		return plainTableSection{}, false
	}

	header := strings.Fields(lines[headerIndex])
	rows := make([][]string, 0, len(lines)-headerIndex-1)
	for _, line := range lines[headerIndex+1:] {
		if !hasColumnSpacing(line) {
			return plainTableSection{}, false
		}
		fields := strings.Fields(line)
		if len(fields) != len(header) || !looksLikeTableDataRow(fields) {
			return plainTableSection{}, false
		}
		rows = append(rows, fields)
	}
	if len(rows) == 0 {
		return plainTableSection{}, false
	}

	notes := append([]string(nil), lines[:headerIndex]...)
	title := ""
	if len(notes) > 0 && !strings.Contains(notes[0], ":") {
		title = strings.TrimSpace(notes[0])
		notes = notes[1:]
	}
	return plainTableSection{
		title: title,
		notes: notes,
		table: plainTable{headers: header, rows: rows},
	}, true
}

func looksLikeTableDataRow(fields []string) bool {
	for _, field := range fields {
		if strings.TrimSpace(field) == "" {
			return false
		}
	}
	return true
}

func nonEmptyBlocks(text string) [][]string {
	var blocks [][]string
	var block []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			if len(block) > 0 {
				blocks = append(blocks, block)
				block = nil
			}
			continue
		}
		block = append(block, line)
	}
	if len(block) > 0 {
		blocks = append(blocks, block)
	}
	return blocks
}

func parseWhitespaceTable(text string) (plainTable, bool) {
	lines := nonBlankTableLines(trimOuterBlankLines(text))
	if len(lines) < 2 {
		return plainTable{}, false
	}

	records := make([][]string, 0, len(lines))
	width := 0
	for _, line := range lines {
		if strings.ContainsAny(line, "|,{}[]<>") || strings.HasPrefix(strings.TrimSpace(line), "#") {
			return plainTable{}, false
		}
		if !hasColumnSpacing(line) {
			return plainTable{}, false
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || len(fields) > 12 {
			return plainTable{}, false
		}
		if width == 0 {
			width = len(fields)
		} else if len(fields) != width {
			return plainTable{}, false
		}
		records = append(records, fields)
	}
	if !looksLikeTableHeader(records[0]) {
		return plainTable{}, false
	}
	return plainTable{headers: records[0], rows: records[1:]}, true
}

func hasColumnSpacing(line string) bool {
	return strings.Contains(line, "\t") || strings.Contains(line, "  ")
}

func nonBlankTableLines(text string) []string {
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func looksLikeTableHeader(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	for _, field := range fields {
		if !looksLikeHeaderCell(field) {
			return false
		}
	}
	return true
}

func looksLikeHeaderCell(field string) bool {
	field = strings.TrimSpace(field)
	if field == "" || len(field) > 40 {
		return false
	}
	hasLetter := false
	for _, r := range field {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r), r == '_', r == '-':
		default:
			return false
		}
	}
	return hasLetter
}

func renderPlainTable(table plainTable) string {
	labels := headerLabels(table.headers)
	var b strings.Builder
	b.WriteString(`<table class="plain-data-table"><thead><tr>`)
	for _, label := range labels {
		b.WriteString(`<th scope="col">`)
		b.WriteString(htmlpkg.EscapeString(label))
		b.WriteString(`</th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)
	for _, row := range table.rows {
		b.WriteString(`<tr>`)
		for i := range labels {
			b.WriteString(`<td>`)
			if i < len(row) {
				b.WriteString(htmlpkg.EscapeString(cleanTableText(row[i])))
			}
			b.WriteString(`</td>`)
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}
