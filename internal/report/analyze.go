package report

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"slices"
	"strings"
)

const (
	analyzeScanBytes = 64 << 10
	analyzeScanLines = 256
)

var utf8BOM = []byte{0xef, 0xbb, 0xbf}

func Analyze(src []byte, sourceName string) Analysis {
	stats := baseStats(src)
	window := src
	if len(window) > analyzeScanBytes {
		window = window[:analyzeScanBytes]
	}
	if isBinary(window) {
		return Analysis{Kind: KindBinary, Confidence: 1, Reasons: []string{"binary bytes detected"}, Stats: stats}
	}
	if analyzeMarkdownName(sourceName) {
		return Analysis{Kind: KindMarkdown, Confidence: 0.9, Reasons: []string{"markdown filename extension"}, Stats: stats}
	}
	if !analyzeDataName(sourceName) && !analyzeDiffName(sourceName) {
		if a, ok := analyzeSourceName(sourceName, stats, false); ok {
			return a
		}
	}
	if a, ok := analyzeJSONLines(src, stats, sourceName); ok {
		return a
	}
	if a, ok := analyzeJSON(src, stats, sourceName); ok {
		return a
	}
	text := string(limitLines(window, analyzeScanLines))
	if a, ok := analyzeDiff(text, stats); ok {
		return a
	}
	if a, ok := analyzeGoTestLog(text, stats); ok {
		return a
	}
	if a, ok := analyzeDelimited(src, stats, sourceName, ',', KindCSVRecords); ok {
		return a
	}
	if a, ok := analyzeDelimited(src, stats, sourceName, '\t', KindTSVRecords); ok {
		return a
	}
	if a, ok := analyzeASCIITable(src, stats); ok {
		return a
	}
	if a, ok := analyzeTranscript(text, stats); ok {
		return a
	}
	if a, ok := analyzeAccessLog(text, stats); ok {
		return a
	}
	if a, ok := analyzeLog(text, stats); ok {
		return a
	}
	if a, ok := analyzeSourceName(sourceName, stats, false); ok {
		return a
	}
	if looksLikeMarkdown(window) {
		return Analysis{Kind: KindMarkdown, Confidence: 0.92, Reasons: []string{"markdown structural signal"}, Stats: stats}
	}
	if a, ok := analyzeTree(text, stats); ok {
		return a
	}
	if signals := mixedSignals(text); len(signals) >= 2 {
		return Analysis{Kind: KindMixed, Confidence: 0.55, Reasons: []string{"multiple weak format signals: " + strings.Join(signals, ", ")}, Stats: stats}
	}
	if a, ok := analyzeSourceContent(src, stats); ok {
		return a
	}
	if a, ok := analyzeTranscript(text, stats); ok {
		return a
	}
	if a, ok := analyzeLog(text, stats); ok {
		return a
	}
	if a, ok := analyzeSourceName(sourceName, stats, true); ok {
		return a
	}
	return Analysis{Kind: KindPlain, Confidence: 0.62, Reasons: []string{"no high-confidence structured format detected"}, Stats: stats}
}

func baseStats(src []byte) Stats {
	lines := 0
	if len(src) > 0 {
		lines = bytes.Count(src, []byte("\n"))
		if !bytes.HasSuffix(src, []byte("\n")) {
			lines++
		}
	}
	return Stats{Bytes: len(src), Lines: lines}
}

func analyzeJSON(src []byte, stats Stats, sourceName string) (Analysis, bool) {
	trimmed := bytes.TrimSpace(stripUTF8BOM(src))
	if len(trimmed) == 0 {
		return Analysis{}, false
	}
	if trimmed[0] != '[' && trimmed[0] != '{' && !analyzeJSONName(sourceName) {
		return Analysis{}, false
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return Analysis{}, false
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return Analysis{}, false
	}
	switch x := v.(type) {
	case []any:
		keys, homogeneous, records := recordKeys(x)
		if !records {
			return Analysis{Kind: KindJSONObject, Confidence: 0.9, Reasons: []string{"json array"}, Stats: stats, Data: x}, true
		}
		stats.Records = len(x)
		stats.Fields = len(keys)
		reasons := []string{"json array of objects"}
		conf := 0.95
		if !homogeneous {
			reasons = append(reasons, "heterogeneous record keys")
			conf = 0.9
		}
		return Analysis{Kind: KindJSONRecords, Confidence: conf, Reasons: reasons, Stats: stats, Data: x}, true
	case map[string]any:
		stats.Fields = len(x)
		return Analysis{Kind: KindJSONObject, Confidence: 0.94, Reasons: []string{"json object"}, Stats: stats, Data: x}, true
	default:
		return Analysis{Kind: KindJSONObject, Confidence: 0.82, Reasons: []string{"json scalar"}, Stats: stats, Data: x}, true
	}
}

func analyzeJSONLines(src []byte, stats Stats, sourceName string) (Analysis, bool) {
	src = stripUTF8BOM(src)
	lines := bytes.Split(src, []byte("\n"))
	records := make([]any, 0, len(lines))
	nonBlank := 0
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		nonBlank++
		var v any
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.UseNumber()
		if err := dec.Decode(&v); err != nil {
			return Analysis{}, false
		}
		if dec.Decode(&struct{}{}) != io.EOF {
			return Analysis{}, false
		}
		obj, ok := v.(map[string]any)
		if !ok {
			return Analysis{}, false
		}
		records = append(records, obj)
	}
	if nonBlank < 2 && !analyzeJSONLinesName(sourceName) {
		return Analysis{}, false
	}
	keys, homogeneous, ok := recordKeys(records)
	if !ok {
		return Analysis{}, false
	}
	stats.Records = len(records)
	stats.Fields = len(keys)
	reasons := []string{"json lines of objects"}
	conf := 0.94
	if !homogeneous {
		reasons = append(reasons, "heterogeneous record keys")
		conf = 0.89
	}
	return Analysis{Kind: KindJSONRecords, Confidence: conf, Reasons: reasons, Stats: stats, Data: records}, true
}

func recordKeys(records []any) ([]string, bool, bool) {
	if len(records) == 0 {
		return nil, false, false
	}
	var first []string
	homogeneous := true
	seen := map[string]bool{}
	for i, rec := range records {
		obj, ok := rec.(map[string]any)
		if !ok {
			return nil, false, false
		}
		keys := sortedKeys(obj)
		for _, k := range keys {
			seen[k] = true
		}
		if i == 0 {
			first = keys
			continue
		}
		if !slices.Equal(first, keys) {
			homogeneous = false
		}
	}
	if len(seen) == 0 {
		return nil, false, false
	}
	return sortedBoolKeys(seen), homogeneous, true
}

func analyzeDelimited(src []byte, stats Stats, sourceName string, comma rune, kind Kind) (Analysis, bool) {
	text := trimOuterBlankLines(string(stripUTF8BOM(src)))
	if strings.HasPrefix(strings.TrimLeft(text, " \t\r\n"), string(utf8BOM)) {
		return Analysis{}, false
	}
	if text == "" || strings.ContainsRune(firstLine(text), 0) {
		return Analysis{}, false
	}
	if comma == ',' && !strings.Contains(firstLine(text), ",") {
		return Analysis{}, false
	}
	if comma == '\t' && !strings.Contains(firstLine(text), "\t") {
		return Analysis{}, false
	}
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = comma
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil || len(records) == 0 {
		return Analysis{}, false
	}
	width := len(records[0])
	if width < 2 {
		return Analysis{}, false
	}
	if len(records) == 1 && !analyzeDataName(sourceName) {
		return Analysis{}, false
	}
	for _, rec := range records[1:] {
		if len(rec) != width {
			return Analysis{}, false
		}
	}
	stats.Records = len(records) - 1
	stats.Fields = width
	reason := "csv records with header"
	if kind == KindTSVRecords {
		reason = "tsv records with header"
	}
	return Analysis{Kind: kind, Confidence: 0.92, Reasons: []string{reason}, Stats: stats, Data: records}, true
}
