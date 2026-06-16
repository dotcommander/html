package render

import (
	"bytes"
	"regexp"
	"unicode/utf8"
)

// Kind classifies a source blob so the renderer can pick a faithful output:
// binary input is refused upstream, Markdown goes through goldmark, everything
// else is shown as preformatted plain text.
type Kind int

const (
	KindPlain Kind = iota
	KindMarkdown
	KindBinary
)

// Detection scans only the first detectScanBytes AND at most detectScanLines
// lines (whichever comes first), so a single multi-megabyte line cannot make
// detection super-linear. Classification is bounded, not whole-input.
const (
	detectScanBytes = 64 << 10 // 64 KiB
	detectScanLines = 256
)

// Detect classifies src as binary, Markdown, or plain text. Precedence:
//  1. binary (a NUL byte, or >maxControlRatio non-text bytes in the scan window);
//  2. Markdown — but ONLY on a high-confidence structural signal (fenced code,
//     a GFM table, a GFM task list, or a setext heading);
//  3. plain text (the default).
//
// Markdown detection is deliberately strong-signal-only. Every weaker inline cue
// (a "# comment" line, "__dunder__", `command` backticks, "arr[i](x)" reading as
// a link) has a common code/config/log doppelgänger, so relying on them would
// misclassify scripts, diffs, JSON/YAML, and command output as Markdown and
// mangle them. The cost is that a heading-and-prose-only Markdown doc piped in
// renders as plain text unless the caller passes --markdown.
func Detect(src []byte) Kind {
	window := src
	if len(window) > detectScanBytes {
		window = window[:detectScanBytes]
	}
	if isBinary(window) {
		return KindBinary
	}
	if looksLikeMarkdown(window) {
		return KindMarkdown
	}
	return KindPlain
}

// maxControlRatio is the share of disallowed control / invalid-UTF-8 bytes above
// which a NUL-free window is still treated as binary.
const maxControlRatio = 0.10

// isBinary reports whether window is non-text: any NUL byte is decisive; else a
// count of disallowed control bytes / UTF-8 decode errors exceeding
// maxControlRatio. Tab, newline, carriage return, form feed, and ESC (so ANSI
// color codes don't trip the guard) are allowed.
func isBinary(window []byte) bool {
	if len(window) == 0 {
		return false
	}
	if bytes.IndexByte(window, 0) >= 0 {
		return true
	}
	bad := 0
	b := window
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		switch {
		case r == utf8.RuneError && size == 1:
			bad++ // invalid UTF-8 byte
		case r < 0x20 && r != '\t' && r != '\n' && r != '\r' && r != '\f' && r != 0x1b:
			bad++ // disallowed C0 control
		}
		b = b[size:]
	}
	return float64(bad) > maxControlRatio*float64(len(window))
}

var (
	// A fenced code block at line start (optional indent). Rare outside Markdown.
	reFence = regexp.MustCompile("(?m)^[ \t]*(```|~~~)")
	// A GFM table delimiter row: pipes + dashes/colons only.
	reTableDelim = regexp.MustCompile(`^\s*\|?[ :|-]*\|[ :|-]*$`)
	// A GFM task-list item. The marker is specific enough to avoid ordinary
	// bullets, command output, and prose lists.
	reTaskList = regexp.MustCompile(`(?m)^[ \t]{0,3}[-+*][ \t]+\[[ xX]\][ \t]+`)
)

// looksLikeMarkdown reports a high-confidence Markdown structure in the (already
// known-text) window: a fenced code block, a GFM table, a GFM task list, or a
// setext heading.
func looksLikeMarkdown(window []byte) bool {
	scan := limitLines(window, detectScanLines)
	return reFence.Match(scan) || hasTable(scan) || reTaskList.Match(scan) || hasSetextEq(scan)
}

// hasTable reports a GFM table: a delimiter row (pipes + dashes) with at least
// one "-", immediately under a line that itself contains a "|" (the header).
func hasTable(scan []byte) bool {
	lines := bytes.Split(scan, []byte("\n"))
	for i := 1; i < len(lines); i++ {
		if reTableDelim.Match(lines[i]) &&
			bytes.IndexByte(lines[i], '-') >= 0 &&
			bytes.IndexByte(lines[i-1], '|') >= 0 {
			return true
		}
	}
	return false
}

// hasSetextEq reports a setext heading underline. A "-" underline needs a blank
// or EOF after it so plain command-output dividers stay plain.
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

// allByte reports whether b is non-empty and every byte equals c.
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

// limitLines returns the prefix of b containing at most n lines, so the regex
// scan stays bounded even when b is one enormous line.
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
