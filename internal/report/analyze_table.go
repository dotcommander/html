package report

import (
	"bytes"
	"strings"
)

func analyzeASCIITable(src []byte, stats Stats) (Analysis, bool) {
	text := trimOuterBlankLines(string(stripUTF8BOM(src)))
	if text == "" {
		return Analysis{}, false
	}
	records, ok := parseBoxTable(text)
	reason := "boxed ascii table"
	if !ok {
		records, ok = parseAlignedPipeTable(text)
		reason = "aligned pipe table"
	}
	if !ok || len(records) < 2 || len(records[0]) < 2 {
		return Analysis{}, false
	}
	width := len(records[0])
	for _, rec := range records[1:] {
		if len(rec) != width {
			return Analysis{}, false
		}
	}
	stats.Records = len(records) - 1
	stats.Fields = width
	return Analysis{Kind: KindTableRecords, Confidence: 0.88, Reasons: []string{reason}, Stats: stats, Data: records}, true
}

func parseBoxTable(text string) ([][]string, bool) {
	lines := nonEmptyLines(text)
	borderLines := 0
	records := make([][]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isASCIITableBorder(trimmed) {
			borderLines++
			continue
		}
		if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
			continue
		}
		row := splitPipeRow(strings.Trim(trimmed, "|"))
		if len(row) < 2 {
			return nil, false
		}
		records = append(records, row)
	}
	return records, borderLines >= 2 && len(records) >= 2
}

func parseAlignedPipeTable(text string) ([][]string, bool) {
	lines := nonEmptyLines(text)
	separator := -1
	for i, line := range lines {
		if isAlignedPipeSeparator(strings.TrimSpace(line)) {
			separator = i
			break
		}
	}
	if separator <= 0 || separator+1 >= len(lines) {
		return nil, false
	}
	header := splitPipeRow(lines[separator-1])
	if len(header) < 2 {
		return nil, false
	}
	records := [][]string{header}
	for _, line := range lines[separator+1:] {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")") {
			break
		}
		if !strings.Contains(line, "|") {
			break
		}
		row := splitPipeRow(line)
		if len(row) != len(header) {
			return nil, false
		}
		records = append(records, row)
	}
	return records, len(records) >= 2
}

func splitPipeRow(line string) []string {
	parts := strings.Split(line, "|")
	row := make([]string, 0, len(parts))
	for _, part := range parts {
		row = append(row, strings.TrimSpace(part))
	}
	return row
}

func isASCIITableBorder(line string) bool {
	if len(line) < 3 || !strings.HasPrefix(line, "+") || !strings.HasSuffix(line, "+") {
		return false
	}
	hasDash := false
	for _, r := range line {
		switch r {
		case '+', '-', '=', ' ':
			if r == '-' || r == '=' {
				hasDash = true
			}
		default:
			return false
		}
	}
	return hasDash && strings.Count(line, "+") >= 2
}

func isAlignedPipeSeparator(line string) bool {
	if !strings.Contains(line, "+") {
		return false
	}
	hasDash := false
	for _, r := range line {
		switch r {
		case '-', '+', ' ':
			if r == '-' {
				hasDash = true
			}
		default:
			return false
		}
	}
	return hasDash
}

func trimOuterBlankLines(text string) string {
	lines := strings.SplitAfter(text, "\n")
	start := 0
	for start < len(lines) && strings.Trim(lines[start], " \t\r\n") == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.Trim(lines[end-1], " \t\r\n") == "" {
		end--
	}
	return strings.Join(lines[start:end], "")
}

func stripUTF8BOM(src []byte) []byte {
	return bytes.TrimPrefix(src, utf8BOM)
}
