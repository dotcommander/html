package render

import (
	"fmt"
	htmlpkg "html"
	"path"
	"strconv"
	"strings"
)

func fileTree(src []byte) string {
	lines := nonEmptyTreeLines(string(src))
	if len(lines) == 0 {
		return rawPre(src)
	}
	entries := make([]treeEntry, 0, len(lines))
	stats := treeStats{}
	for _, line := range lines {
		name, depth := treeLine(line)
		entries = append(entries, treeEntry{Name: name, Depth: depth})
		stats.add(depth)
	}
	var b strings.Builder
	b.WriteString(fileTreeOverview(stats))
	b.WriteString(`<ul class="file-tree">`)
	for _, entry := range entries {
		fmt.Fprintf(&b, `<li style="--depth:%d"><span>%s</span></li>`, entry.Depth, htmlpkg.EscapeString(entry.Name))
	}
	b.WriteString(`</ul>`)
	return b.String()
}

type treeEntry struct {
	Name  string
	Depth int
}

type treeStats struct {
	Entries  int
	MaxDepth int
}

func (s *treeStats) add(depth int) {
	s.Entries++
	if depth > s.MaxDepth {
		s.MaxDepth = depth
	}
}

func fileTreeOverview(stats treeStats) string {
	var b strings.Builder
	b.WriteString(`<dl class="file-tree-overview" aria-label="File tree overview">`)
	for _, item := range [][2]string{
		{"Entries", strconv.Itoa(stats.Entries)},
		{"Max depth", strconv.Itoa(stats.MaxDepth)},
	} {
		b.WriteString(`<div><dt>`)
		b.WriteString(htmlpkg.EscapeString(item[0]))
		b.WriteString(`</dt><dd>`)
		b.WriteString(htmlpkg.EscapeString(item[1]))
		b.WriteString(`</dd></div>`)
	}
	b.WriteString(`</dl>`)
	return b.String()
}

func nonEmptyTreeLines(text string) []string {
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		clean := string(reANSI.ReplaceAll([]byte(line), nil))
		if strings.TrimSpace(clean) != "" && !isTreeRootLine(clean) && !isTreeSummaryLine(clean) {
			lines = append(lines, clean)
		}
	}
	return lines
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

func treeLine(line string) (string, int) {
	if name, depth, ok := asciiTreeLine(line); ok {
		return name, depth
	}
	name := strings.TrimSpace(strings.TrimLeft(line, "├└│─ "))
	if name == "" {
		name = line
	}
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimPrefix(name, `.\`)
	return name, pathDepth(line)
}

func asciiTreeLine(line string) (string, int, bool) {
	markers := []string{"|-- ", "`-- ", "+-- "}
	for _, marker := range markers {
		if i := strings.Index(line, marker); i >= 0 {
			name := strings.TrimSpace(line[i+len(marker):])
			if name == "" {
				name = line
			}
			return name, strings.Count(line[:i], "|   ") + strings.Count(line[:i], "    "), true
		}
	}
	return "", 0, false
}

func pathDepth(line string) int {
	if strings.ContainsAny(line, "├└│") {
		return unicodeTreeDepth(line)
	}
	clean := strings.TrimSpace(line)
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "../")
	clean = strings.TrimPrefix(clean, "~/")
	clean = strings.TrimPrefix(clean, `.\`)
	clean = strings.TrimPrefix(clean, `..\`)
	clean = strings.ReplaceAll(clean, `\`, "/")
	clean = trimWindowsVolumeAnchor(clean)
	clean = strings.TrimLeft(clean, "/")
	return strings.Count(path.Clean(clean), "/")
}

func unicodeTreeDepth(line string) int {
	marker := strings.IndexAny(line, "├└")
	if marker < 0 {
		marker = len(line)
	}
	prefix := line[:marker]
	return strings.Count(prefix, "│") + strings.Count(prefix, "    ")
}

func trimWindowsVolumeAnchor(path string) string {
	if len(path) >= 2 && path[1] == ':' && isASCIILetter(path[0]) {
		return path[2:]
	}
	return path
}

func isASCIILetter(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
}

func extractArticle(doc string) string {
	start := strings.Index(doc, `<article class="markdown-body">`)
	if start < 0 {
		return doc
	}
	start += len(`<article class="markdown-body">`)
	end := strings.LastIndex(doc, `</article>`)
	if end <= start {
		return doc
	}
	return doc[start:end]
}

func sortedStringKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
