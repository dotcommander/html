package render

import (
	"strconv"
	"strings"

	"github.com/dotcommander/html/internal/report"
)

// Options controls a single Render call.
type Options struct {
	// FallbackTitle is used for <title> when the document has no level-1
	// heading (typically the source filename without extension).
	FallbackTitle string
	// Safe omits raw HTML passthrough — appropriate for untrusted or downloaded
	// Markdown. The zero value (false) preserves trusted local raw HTML.
	Safe bool
	// MaxWidth overrides the reader column CSS max-width (e.g. "48rem"); "" keeps
	// the stylesheet default.
	MaxWidth string
	// Theme is the initial color theme fallback: "light" or "dark" forces that
	// theme before any localStorage choice; "" or "auto" follows the system.
	Theme string
	// Palette is the initial color-family fallback. Valid values are sepia,
	// blue, green, rose, and catppuccin; "" uses sepia.
	Palette string
	// TOC overrides the automatic table of contents: nil = automatic (by heading
	// count), true = always, false = never.
	TOC *bool
	// Plain renders src as preformatted plain text (HTML-escaped, wrapped in
	// <pre><code>) instead of Markdown — for piped command output, logs, code,
	// and other non-Markdown input. Bypasses goldmark, the synthesized <h1>, and
	// the TOC.
	Plain bool
	// Frame wraps plain/ANSI output in faux terminal-window chrome (title bar +
	// traffic-light dots) for share-ready "screenshots". Only meaningful with
	// Plain; the CLI's --frame implies plain rendering.
	Frame bool
	// Lang forces a chroma syntax-highlight language for plain mode ("" = auto-
	// detect; "text"/"none"/"plain" = no highlighting / raw escaped text).
	Lang string
	// CodeTheme is a Chroma style name for code blocks. Empty keeps the built-in
	// github/github-dark default.
	CodeTheme string
	// SourceName is the input's file name (with extension) when known, used to
	// detect the highlight language by filename; "" for stdin (content-detected).
	SourceName string
	// SourceDir is the directory trusted local image refs resolve against. Safe
	// rendering ignores it and never reads image destinations.
	SourceDir string
	// RebaseLocalLinks rewrites relative Markdown links against SourceDir. It is
	// intended for cached file renders whose output lives outside SourceDir.
	RebaseLocalLinks bool
	// ImageFingerprint captures local image dependencies that are inlined into
	// Markdown output. It is computed by callers that have the source bytes and
	// folded into cache freshness.
	ImageFingerprint string
	// ReportTag distinguishes report-plan renders from legacy Markdown/plain
	// renders in the cache fingerprint. Empty preserves legacy cache behavior.
	ReportTag string
	// semanticLists is internal report-renderer state. Each ref identifies an
	// ordered list styled during the same full-document Markdown parse.
	semanticLists []report.SourceRef
}

// cacheTag encodes the Options fields that change rendered output, so the
// renderer Fingerprint distinguishes renders that would differ. Launch-only
// options (which do not affect the HTML, e.g. the open command) must NOT appear
// here. Extend this when adding a new output-affecting option.
func (o Options) cacheTag() string {
	var b strings.Builder
	if o.Safe {
		b.WriteString("+safe")
	}
	if o.Plain {
		b.WriteString("+plain")
	}
	if o.Frame {
		b.WriteString("+frame")
	}
	if o.Lang != "" {
		appendCacheTag(&b, "lang", o.Lang)
	}
	if o.CodeTheme != "" {
		appendCacheTag(&b, "code-theme", o.CodeTheme)
	}
	if o.FallbackTitle != "" {
		appendCacheTag(&b, "title", o.FallbackTitle)
	}
	if o.SourceName != "" {
		appendCacheTag(&b, "source", o.SourceName)
	}
	if o.MaxWidth != "" {
		appendCacheTag(&b, "w", o.MaxWidth)
	}
	if o.Theme != "" {
		appendCacheTag(&b, "theme", o.Theme)
	}
	if o.Palette != "" {
		appendCacheTag(&b, "palette", o.Palette)
	}
	if o.TOC != nil {
		if *o.TOC {
			b.WriteString("+toc=on")
		} else {
			b.WriteString("+toc=off")
		}
	}
	if o.ReportTag != "" {
		appendCacheTag(&b, "report", o.ReportTag)
	}
	if o.ImageFingerprint != "" {
		appendCacheTag(&b, "img", o.ImageFingerprint)
	}
	if o.RebaseLocalLinks {
		appendCacheTag(&b, "links", o.SourceDir)
	}
	return b.String()
}

func appendCacheTag(b *strings.Builder, key, value string) {
	b.WriteString("+")
	b.WriteString(key)
	b.WriteString(":")
	b.WriteString(strconv.Itoa(len(value)))
	b.WriteString(":")
	b.WriteString(value)
}

// Render converts source into a complete, self-contained HTML document — as
// Markdown by default, or as preformatted plain text when opts.Plain is set.
