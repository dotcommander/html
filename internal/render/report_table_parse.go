package render

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dotcommander/html/internal/report"
)

func tableRows(src []byte, analysis report.Analysis) ([]string, [][]string) {
	src = stripUTF8BOM(src)
	switch analysis.Kind {
	case report.KindJSONRecords:
		if records, ok := analysis.Data.([]any); ok {
			if headers, rows := jsonRecordRows(records); len(headers) > 0 {
				return headers, rows
			}
		}
		var records []any
		dec := json.NewDecoder(bytes.NewReader(src))
		dec.UseNumber()
		if err := dec.Decode(&records); err == nil {
			return jsonRecordRows(records)
		}
	case report.KindCSVRecords, report.KindTSVRecords, report.KindTableRecords:
		if records, ok := analysis.Data.([][]string); ok {
			return delimitedRows(records)
		}
		if analysis.Kind == report.KindTableRecords {
			return nil, nil
		}
		comma := ','
		if analysis.Kind == report.KindTSVRecords {
			comma = '\t'
		}
		text := trimOuterBlankLines(string(stripUTF8BOM(src)))
		if strings.HasPrefix(strings.TrimLeft(text, " \t\r\n"), string(utf8BOM)) {
			return nil, nil
		}
		r := csv.NewReader(strings.NewReader(text))
		r.Comma = comma
		r.FieldsPerRecord = -1
		records, err := r.ReadAll()
		if err == nil {
			return delimitedRows(records)
		}
	}
	return nil, nil
}

func jsonRecordRows(records []any) ([]string, [][]string) {
	seen := map[string]bool{}
	objects := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		obj, ok := rec.(map[string]any)
		if !ok {
			return nil, nil
		}
		objects = append(objects, obj)
		for k := range obj {
			seen[k] = true
		}
	}
	headers := sortedStringKeys(seen)
	if len(headers) == 0 {
		return nil, nil
	}
	rows := make([][]string, 0, len(objects))
	for _, rec := range objects {
		row := make([]string, len(headers))
		for i, h := range headers {
			if v, ok := rec[h]; ok {
				row[i] = stringify(v)
			}
		}
		rows = append(rows, row)
	}
	return headers, rows
}

func delimitedRows(records [][]string) ([]string, [][]string) {
	if len(records) == 0 {
		return nil, nil
	}
	headers := records[0]
	if len(headers) == 0 {
		return nil, nil
	}
	rows := make([][]string, 0, len(records)-1)
	for _, rec := range records[1:] {
		row := make([]string, len(headers))
		for i := range headers {
			if i < len(rec) {
				row[i] = rec[i]
			}
		}
		rows = append(rows, row)
	}
	return headers, rows
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

func headerLabels(headers []string) []string {
	labels := make([]string, len(headers))
	counts := map[string]int{}
	used := map[string]bool{}
	for i, header := range headers {
		base := headerLabel(header, i)
		counts[base]++
		label := base
		if counts[base] > 1 {
			label = fmt.Sprintf("%s %d", base, counts[base])
		}
		for used[label] {
			counts[base]++
			label = fmt.Sprintf("%s %d", base, counts[base])
		}
		labels[i] = label
		used[label] = true
	}
	return labels
}

func headerLabel(header string, index int) string {
	header = cleanTableText(header)
	if strings.TrimSpace(header) == "" {
		return fmt.Sprintf("Column %d", index+1)
	}
	return header
}

func stripUTF8BOM(src []byte) []byte {
	return bytes.TrimPrefix(src, utf8BOM)
}

func stringify(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case json.Number:
		return x.String()
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprint(x)
		}
		return string(b)
	}
}
