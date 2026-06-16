package report

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2/lexers"
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
	if a, ok := analyzeDelimited(src, stats, ',', KindCSVRecords); ok {
		return a
	}
	if a, ok := analyzeDelimited(src, stats, '\t', KindTSVRecords); ok {
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
	if a, ok := analyzeSourceContent(src, stats); ok {
		return a
	}
	if a, ok := analyzeLog(text, stats); ok {
		return a
	}
	if looksMixed(text) {
		return Analysis{Kind: KindMixed, Confidence: 0.55, Reasons: []string{"multiple weak format signals"}, Stats: stats}
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

func analyzeDelimited(src []byte, stats Stats, comma rune, kind Kind) (Analysis, bool) {
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
	if err != nil || len(records) < 2 {
		return Analysis{}, false
	}
	width := len(records[0])
	if width < 2 {
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

var (
	reHunk       = regexp.MustCompile(`(?m)^@@@? .+ @@@?`)
	reDiffHeader = regexp.MustCompile(`(?m)^diff --(?:git|cc|combined) `)
	reDiffMeta   = regexp.MustCompile(`(?m)^(?:GIT binary patch|Binary files .+ differ|old mode |new mode |new file mode |deleted file mode |rename from |rename to |copy from |copy to |similarity index |dissimilarity index )`)
	reDiffOldNew = regexp.MustCompile(`(?m)^--- .*\n\+\+\+ `)
)

func analyzeDiff(text string, stats Stats) (Analysis, bool) {
	switch {
	case reDiffHeader.MatchString(text) && reHunk.MatchString(text):
	case reDiffHeader.MatchString(text) && reDiffMeta.MatchString(text):
	case reDiffOldNew.MatchString(text) && reHunk.MatchString(text):
	default:
		return Analysis{}, false
	}
	stats.Files = diffFileCount(text)
	return Analysis{Kind: KindDiff, Confidence: 0.96, Reasons: []string{"unified diff markers"}, Stats: stats}, true
}

func diffFileCount(text string) int {
	if n := len(reDiffHeader.FindAllStringIndex(text, -1)); n > 0 {
		return n
	}
	count := 0
	inHunk := false
	sawOldHeader := false
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "@@"):
			inHunk = true
		case !inHunk && strings.HasPrefix(line, "--- "):
			sawOldHeader = true
		case !inHunk && sawOldHeader && strings.HasPrefix(line, "+++ "):
			count++
			sawOldHeader = false
		case !inHunk && strings.TrimSpace(line) != "":
			sawOldHeader = false
		}
	}
	return count
}

func analyzeSourceName(sourceName string, stats Stats, fallbackExt bool) (Analysis, bool) {
	ext := strings.ToLower(filepath.Ext(sourceName))
	if sourceName != "" {
		if lx := lexers.Match(sourceName); lx != nil && !strings.EqualFold(lx.Config().Name, "plaintext") {
			return Analysis{Kind: KindSourceCode, Confidence: 0.88, Reasons: []string{"source filename matched lexer " + lx.Config().Name}, Stats: stats, Data: lx.Config().Name}, true
		}
	}
	if fallbackExt && ext != "" && ext != ".txt" && ext != ".log" {
		return Analysis{Kind: KindSourceCode, Confidence: 0.75, Reasons: []string{"source-like file extension"}, Stats: stats}, true
	}
	return Analysis{}, false
}

func analyzeMarkdownName(sourceName string) bool {
	switch strings.ToLower(filepath.Ext(sourceName)) {
	case ".md", ".markdown":
		return true
	default:
		return false
	}
}

func analyzeJSONName(sourceName string) bool {
	return strings.EqualFold(filepath.Ext(sourceName), ".json")
}

func analyzeJSONLinesName(sourceName string) bool {
	switch strings.ToLower(filepath.Ext(sourceName)) {
	case ".jsonl", ".ndjson", ".jsonlines":
		return true
	default:
		return false
	}
}

func analyzeDiffName(sourceName string) bool {
	switch strings.ToLower(filepath.Ext(sourceName)) {
	case ".diff", ".patch":
		return true
	default:
		return false
	}
}

func analyzeDataName(sourceName string) bool {
	switch strings.ToLower(filepath.Ext(sourceName)) {
	case ".json", ".jsonl", ".ndjson", ".jsonlines", ".csv", ".tsv":
		return true
	default:
		return false
	}
}

func analyzeSourceContent(src []byte, stats Stats) (Analysis, bool) {
	sample := string(src)
	if len(sample) > analyzeScanBytes {
		sample = sample[:analyzeScanBytes]
	}
	if lx := lexers.Analyse(sample); lx != nil && !strings.EqualFold(lx.Config().Name, "plaintext") {
		return Analysis{Kind: KindSourceCode, Confidence: 0.72, Reasons: []string{"content matched lexer " + lx.Config().Name}, Stats: stats, Data: lx.Config().Name}, true
	}
	return Analysis{}, false
}

func analyzeTree(text string, stats Stats) (Analysis, bool) {
	lines := nonEmptyLines(text)
	if len(lines) < 2 {
		return Analysis{}, false
	}
	treeGlyphs := 0
	pathLike := 0
	for _, line := range lines {
		if strings.ContainsAny(line, "├└│") || containsASCIITreeMarker(line) {
			treeGlyphs++
		}
		s := strings.TrimSpace(line)
		if isPathListingLine(s) {
			pathLike++
		}
	}
	if treeGlyphs >= 2 || pathLike >= 3 && float64(pathLike)/float64(len(lines)) >= 0.55 {
		stats.Files = treeFileCount(lines)
		return Analysis{Kind: KindTreeListing, Confidence: 0.86, Reasons: []string{"tree or repeated path listing"}, Stats: stats}, true
	}
	return Analysis{}, false
}

func treeFileCount(lines []string) int {
	count := 0
	for _, line := range lines {
		if !isTreeRootLine(line) && !isTreeSummaryLine(line) {
			count++
		}
	}
	return count
}

func isTreeRootLine(line string) bool {
	switch strings.TrimSpace(line) {
	case ".", "./", `.\`:
		return true
	default:
		return false
	}
}

func isTreeSummaryLine(line string) bool {
	fields := strings.Fields(strings.ReplaceAll(strings.TrimSpace(line), ",", ""))
	if len(fields) == 2 {
		return isUnsignedInt(fields[0]) && isTreeDirectoryWord(fields[1])
	}
	return len(fields) == 4 &&
		isUnsignedInt(fields[0]) &&
		isTreeDirectoryWord(fields[1]) &&
		isUnsignedInt(fields[2]) &&
		isTreeFileWord(fields[3])
}

func isUnsignedInt(text string) bool {
	if text == "" {
		return false
	}
	_, err := strconv.Atoi(text)
	return err == nil
}

func isTreeDirectoryWord(text string) bool {
	return text == "directory" || text == "directories"
}

func isTreeFileWord(text string) bool {
	return text == "file" || text == "files"
}

func isPathListingLine(line string) bool {
	line = strings.TrimSpace(line)
	if strings.Contains(line, "://") || strings.Contains(line, "\t") || isHTTPRequestLine(line) {
		return false
	}
	explicitAnchor := hasExplicitPathAnchor(line)
	if strings.Contains(line, " ") && !explicitAnchor {
		return false
	}
	normalized := strings.ReplaceAll(line, `\`, "/")
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, "../")
	normalized = strings.TrimLeft(normalized, "/")
	if !strings.Contains(normalized, "/") && !explicitAnchor {
		return false
	}
	if !explicitAnchor && isNumericOnlySlashPath(normalized) {
		return false
	}
	for _, part := range strings.Split(normalized, "/") {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		for _, r := range part {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' {
				return true
			}
		}
	}
	return false
}

func isNumericOnlySlashPath(line string) bool {
	hasDigit := false
	for _, part := range strings.Split(line, "/") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		for _, r := range part {
			if unicode.IsDigit(r) {
				hasDigit = true
				continue
			}
			if r == '.' || r == '-' {
				continue
			}
			return false
		}
	}
	return hasDigit
}

func hasExplicitPathAnchor(line string) bool {
	return strings.HasPrefix(line, "./") ||
		strings.HasPrefix(line, "../") ||
		strings.HasPrefix(line, "~/") ||
		strings.HasPrefix(line, "/") ||
		strings.HasPrefix(line, `.\`) ||
		strings.HasPrefix(line, `..\`) ||
		strings.Contains(line, `\`)
}

func isHTTPRequestLine(line string) bool {
	method, _, ok := strings.Cut(line, " ")
	if !ok {
		return false
	}
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "CONNECT", "TRACE":
		return true
	default:
		return false
	}
}

func containsASCIITreeMarker(line string) bool {
	return strings.Contains(line, "|-- ") ||
		strings.Contains(line, "`-- ") ||
		strings.Contains(line, "+-- ")
}

var (
	reTimestamp  = regexp.MustCompile(`(?m)(^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}|^\d{2}:\d{2}:\d{2})`)
	reSeverity   = regexp.MustCompile(`(?i)\b(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|PASS|FAIL|panic:)\b`)
	reGoTest     = regexp.MustCompile(`(?m)^(ok|FAIL|\?)\s+\S+\s+((\d+(\.\d+)?s)|\(cached\)|\[no test files\])\s*$`)
	reGoTestMark = regexp.MustCompile(`(?m)^(--- FAIL:|PASS$|FAIL$)`)
)

func analyzeGoTestLog(text string, stats Stats) (Analysis, bool) {
	if reGoTest.MatchString(text) || reGoTestMark.MatchString(text) || strings.Contains(text, "\nFAIL\t") {
		return Analysis{Kind: KindLog, Confidence: 0.82, Reasons: []string{"log/test/console markers"}, Stats: stats}, true
	}
	return Analysis{}, false
}

func analyzeLog(text string, stats Stats) (Analysis, bool) {
	lines := nonEmptyLines(text)
	if len(lines) == 0 {
		return Analysis{}, false
	}
	score := 0
	if reTimestamp.MatchString(text) {
		score++
	}
	if reSeverity.MatchString(text) {
		score++
	}
	if reGoTestMark.MatchString(text) || strings.Contains(text, "\nFAIL\t") {
		score++
	}
	if reGoTest.MatchString(text) {
		score += 2
	}
	if strings.Contains(text, "$ ") || strings.Contains(text, "> ") {
		score++
	}
	if score >= 2 {
		return Analysis{Kind: KindLog, Confidence: 0.82, Reasons: []string{"log/test/console markers"}, Stats: stats}, true
	}
	return Analysis{}, false
}

func looksMixed(text string) bool {
	signals := 0
	if strings.Contains(text, "\n```") {
		signals++
	}
	if strings.Contains(text, "\n{") || strings.Contains(text, "\n[") {
		signals++
	}
	if strings.Contains(text, "\n- ") || strings.Contains(text, "\n# ") {
		signals++
	}
	if reSeverity.MatchString(text) {
		signals++
	}
	return signals >= 2
}

func isBinary(window []byte) bool {
	if len(window) == 0 {
		return false
	}
	if bytes.IndexByte(window, 0) >= 0 {
		return true
	}
	total := len(window)
	bad := 0
	for len(window) > 0 {
		r, size := utf8.DecodeRune(window)
		if r == utf8.RuneError && size == 1 || r < 0x20 && r != '\t' && r != '\n' && r != '\r' && r != '\f' && r != 0x1b {
			bad++
		}
		window = window[size:]
	}
	return float64(bad) > 0.10*float64(total)
}

var (
	reFence      = regexp.MustCompile("(?m)^[ \t]*(```|~~~)")
	reTableDelim = regexp.MustCompile(`^\s*\|?[ :|-]*\|[ :|-]*$`)
	reTaskList   = regexp.MustCompile(`(?m)^[ \t]{0,3}[-+*][ \t]+\[[ xX]\][ \t]+`)
)

func looksLikeMarkdown(window []byte) bool {
	scan := limitLines(window, analyzeScanLines)
	return reFence.Match(scan) || hasTable(scan) || reTaskList.Match(scan) || hasSetextEq(scan) || hasATXHeading(scan)
}

func hasTable(scan []byte) bool {
	lines := bytes.Split(scan, []byte("\n"))
	for i := 1; i < len(lines); i++ {
		if reTableDelim.Match(lines[i]) && bytes.IndexByte(lines[i], '-') >= 0 && bytes.IndexByte(lines[i-1], '|') >= 0 {
			return true
		}
	}
	return false
}

func hasSetextEq(scan []byte) bool {
	lines := bytes.Split(scan, []byte("\n"))
	for i := 1; i < len(lines); i++ {
		prev := bytes.TrimSpace(lines[i-1])
		cur := bytes.TrimSpace(lines[i])
		if len(prev) == 0 || len(cur) == 0 {
			continue
		}
		if allByte(cur, '=') || allByte(cur, '-') && (i+1 == len(lines) || len(bytes.TrimSpace(lines[i+1])) == 0) {
			return true
		}
	}
	return false
}

func hasATXHeading(scan []byte) bool {
	lines := bytes.Split(scan, []byte("\n"))
	for i, line := range lines {
		trimmed := bytes.TrimLeft(line, " \t")
		if len(line)-len(trimmed) > 3 || len(trimmed) == 0 || trimmed[0] != '#' {
			continue
		}
		hashes := 0
		for hashes < len(trimmed) && trimmed[hashes] == '#' {
			hashes++
		}
		if hashes > 6 || hashes >= len(trimmed) || trimmed[hashes] != ' ' && trimmed[hashes] != '\t' {
			continue
		}
		if len(bytes.TrimSpace(trimmed[hashes:])) == 0 {
			continue
		}
		if i+1 == len(lines) || len(bytes.TrimSpace(lines[i+1])) == 0 {
			return true
		}
	}
	return false
}

func allByte(b []byte, c byte) bool {
	if len(b) == 0 {
		return false
	}
	for _, x := range b {
		if x != c {
			return false
		}
	}
	return true
}

func limitLines(b []byte, n int) []byte {
	count := 0
	for i := 0; i < len(b); i++ {
		if b[i] == '\n' {
			count++
			if count >= n {
				return b[:i]
			}
		}
	}
	return b
}

func firstLine(s string) string {
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return before
	}
	return s
}

func nonEmptyLines(s string) []string {
	raw := strings.Split(s, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func sortedBoolKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
