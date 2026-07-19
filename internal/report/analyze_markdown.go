package report

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

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
