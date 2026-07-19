package report

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/alecthomas/chroma/v2/lexers"
)

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
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		nextLine := ""
		if i+1 < len(lines) {
			nextLine = lines[i+1]
		}
		switch {
		case strings.HasPrefix(line, "@@"):
			inHunk = true
		case inHunk && strings.HasPrefix(line, "--- ") && strings.HasPrefix(nextLine, "+++ "):
			count++
			inHunk = false
			sawOldHeader = false
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
