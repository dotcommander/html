package render

import (
	"fmt"
	htmlpkg "html"
	"strings"
)

func diffView(src []byte) string {
	lines := strings.Split(string(src), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var b strings.Builder
	stats := diffStats{}
	var code strings.Builder
	code.WriteString(`<pre class="diff-view"><code>`)
	combinedPrefixCols := 0
	inHunk := false
	for i, line := range lines {
		clean := string(reANSI.ReplaceAll([]byte(line), nil))
		nextClean := ""
		if i+1 < len(lines) {
			nextClean = string(reANSI.ReplaceAll([]byte(lines[i+1]), nil))
		}
		class := "ctx"
		switch {
		case isDiffBoundaryLine(clean):
			class = "file"
			stats.addExplicitFile()
			combinedPrefixCols = 0
			inHunk = false
		case strings.HasPrefix(clean, "@@"):
			class = "hunk"
			stats.hunks++
			combinedPrefixCols = combinedDiffPrefixCols(clean)
			inHunk = true
		case inHunk && isPlainUnifiedOldFileHeader(clean, nextClean):
			class = "file"
			combinedPrefixCols = 0
			inHunk = false
		case inHunk && combinedPrefixCols > 1:
			class = combinedDiffLineClass(clean, combinedPrefixCols)
		case inHunk && strings.HasPrefix(clean, "+"):
			class = "add"
		case inHunk && strings.HasPrefix(clean, "-"):
			class = "del"
		case strings.HasPrefix(clean, "+") && !strings.HasPrefix(clean, "+++"):
			class = "add"
		case strings.HasPrefix(clean, "-") && !strings.HasPrefix(clean, "---"):
			class = "del"
		case isDiffMetadataLine(clean):
			class = "file"
			combinedPrefixCols = 0
		}
		stats.observeLine(clean, class)
		switch class {
		case "add":
			stats.additions++
		case "del":
			stats.deletions++
		}
		fmt.Fprintf(&code, `<span class="%s">%s</span>`+"\n", class, htmlpkg.EscapeString(clean))
	}
	code.WriteString(`</code></pre>`)
	b.WriteString(diffSummary(stats))
	b.WriteString(code.String())
	return b.String()
}

type diffStats struct {
	files         int
	hunks         int
	additions     int
	deletions     int
	oldHeader     bool
	explicitFiles bool
}

func (s *diffStats) addExplicitFile() {
	s.files++
	s.explicitFiles = true
	s.oldHeader = false
}

func (s *diffStats) observeLine(line, class string) {
	if class != "file" && class != "hunk" {
		if strings.TrimSpace(line) != "" {
			s.oldHeader = false
		}
		return
	}
	switch {
	case strings.HasPrefix(line, "--- "):
		s.oldHeader = true
	case strings.HasPrefix(line, "+++ ") && s.oldHeader:
		if !s.explicitFiles {
			s.files++
		}
		s.oldHeader = false
	case strings.HasPrefix(line, "@@"):
		s.oldHeader = false
	case isDiffBoundaryLine(line):
		s.oldHeader = false
	case strings.TrimSpace(line) != "":
		if !strings.HasPrefix(line, "index ") {
			s.oldHeader = false
		}
	}
}

func diffSummary(stats diffStats) string {
	var b strings.Builder
	b.WriteString(`<div class="diff-summary" aria-label="Diff summary">`)
	fmt.Fprintf(&b, `<span><strong>%d</strong> %s</span>`, stats.files, plural(stats.files, "file", "files"))
	fmt.Fprintf(&b, `<span><strong>%d</strong> %s</span>`, stats.hunks, plural(stats.hunks, "hunk", "hunks"))
	fmt.Fprintf(&b, `<span class="diff-added"><strong>+%d</strong> %s</span>`, stats.additions, plural(stats.additions, "addition", "additions"))
	fmt.Fprintf(&b, `<span class="diff-removed"><strong>-%d</strong> %s</span>`, stats.deletions, plural(stats.deletions, "deletion", "deletions"))
	b.WriteString(`</div>`)
	return b.String()
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func isDiffMetadataLine(line string) bool {
	for _, prefix := range diffMetadataPrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func isDiffBoundaryLine(line string) bool {
	return strings.HasPrefix(line, "diff --git") ||
		strings.HasPrefix(line, "diff --cc") ||
		strings.HasPrefix(line, "diff --combined")
}

func isPlainUnifiedOldFileHeader(line, nextLine string) bool {
	return strings.HasPrefix(line, "--- ") && strings.HasPrefix(nextLine, "+++ ")
}

var diffMetadataPrefixes = []string{
	"diff --git",
	"diff --cc",
	"diff --combined",
	"---",
	"+++",
	"index ",
	"old mode ",
	"new mode ",
	"new file mode ",
	"deleted file mode ",
	"rename from ",
	"rename to ",
	"copy from ",
	"copy to ",
	"similarity index ",
	"dissimilarity index ",
	"GIT binary patch",
	"literal ",
	"delta ",
	"Binary files ",
	`\ No newline at end of file`,
}

func combinedDiffPrefixCols(line string) int {
	if !strings.HasPrefix(line, "@@@") {
		return 0
	}
	count := 0
	for count < len(line) && line[count] == '@' {
		count++
	}
	return count - 1
}

func combinedDiffLineClass(line string, prefixCols int) string {
	if len(line) < prefixCols {
		return "ctx"
	}
	prefix := line[:prefixCols]
	for _, r := range prefix {
		if r != ' ' && r != '+' && r != '-' {
			return "ctx"
		}
	}
	if strings.Contains(prefix, "+") {
		return "add"
	}
	if strings.Contains(prefix, "-") {
		return "del"
	}
	return "ctx"
}
