package render

import (
	"bytes"
	htmlpkg "html"
	"strconv"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// newMarkdown builds the goldmark pipeline. unsafe controls raw-HTML
// passthrough: true (the default local-viewer mode) renders trusted raw HTML
// verbatim; false (safe mode) makes goldmark omit raw HTML, appropriate for
// untrusted or downloaded Markdown. GFM, class-based highlighting, and auto
// heading IDs are identical in both modes — the lone difference is the
// WithUnsafe renderer option, expressed as data rather than two near-duplicate
// constructors.
func newMarkdown(unsafe bool, codeTheme string) goldmark.Markdown {
	highlightOpts := []highlighting.Option{
		// WithClasses emits CSS class names (e.g. .chroma .k) rather than
		// inline styles, so highlightCSS controls colors and dark mode works.
		highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
	}
	if ValidCodeTheme(codeTheme) {
		highlightOpts = append(highlightOpts, highlighting.WithStyle(codeTheme))
	}
	opts := []goldmark.Option{
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(highlightOpts...),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(util.Prioritized(imageInliner{}, 100)),
		),
	}
	if unsafe {
		opts = append(opts, goldmark.WithRendererOptions(goldmarkhtml.WithUnsafe()))
	}
	return goldmark.New(opts...)
}

// Package-level singletons — immutable after init. mdUnsafe (the default) passes
// raw HTML through; mdSafe omits it.
var (
	mdUnsafe = newMarkdown(true, "")
	mdSafe   = newMarkdown(false, "")
)

// ValidCodeTheme reports whether name is a Chroma style that can be used for
// code highlighting. Empty means "use the built-in github/github-dark default".
func ValidCodeTheme(name string) bool {
	if name == "" {
		return true
	}
	_, ok := styles.Registry[strings.ToLower(name)]
	return ok
}

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
	// directory local image refs resolve against ("" disables image inlining)
	SourceDir string
	// ImageFingerprint captures local image dependencies that are inlined into
	// Markdown output. It is computed by callers that have the source bytes and
	// folded into cache freshness.
	ImageFingerprint string
	// ReportTag distinguishes report-plan renders from legacy Markdown/plain
	// renders in the cache fingerprint. Empty preserves legacy cache behavior.
	ReportTag string
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
func Render(src []byte, opts Options) (string, error) {
	if opts.Plain {
		return renderPlain(src, opts), nil
	}
	md := mdUnsafe
	switch {
	case opts.CodeTheme != "":
		md = newMarkdown(!opts.Safe, opts.CodeTheme)
	case opts.Safe:
		md = mdSafe
	}
	pc := parser.NewContext()
	pc.Set(baseDirKey, opts.SourceDir)
	doc := md.Parser().Parse(text.NewReader(src), parser.WithContext(pc))
	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, src, doc); err != nil {
		return "", err
	}
	title, fromHeading, headings := analyze(doc, src, opts.FallbackTitle)
	content := buf.String()
	if !fromHeading {
		// The document has no usable level-1 heading, so it would open with no
		// visible subject. Synthesize one from the title (the source filename)
		// as a leading <h1>; .markdown-body h1 styling renders it as the
		// underlined GitHub-style banner. Documents that already start with an
		// h1 are left untouched. title is already HTML-escaped.
		content = "<h1>" + title + "</h1>\n" + content
	}
	if shouldRenderTOC(opts.TOC, len(headings)) {
		if toc := buildTOC(headings); toc != "" {
			// Place the TOC just after the first heading so navigation sits below
			// the document's title rather than above it.
			content = insertAfterFirstH1(content, toc)
		}
	}
	return wrapPage(title, content, opts), nil
}

// shouldRenderTOC decides whether to emit a table of contents: an explicit
// override wins; otherwise the TOC appears once a document has tocMinEntries or
// more h2/h3 headings.
func shouldRenderTOC(override *bool, n int) bool {
	if override != nil {
		return *override
	}
	return n >= tocMinEntries
}

// heading is one h2/h3 entry collected for the table of contents.
type heading struct {
	level int
	text  string // trimmed plain text, not yet HTML-escaped
	id    string // goldmark auto-generated heading id
}

// analyze walks an already-parsed document node and returns the page title
// (first level-1 heading text, HTML-escaped and trimmed, or the escaped
// fallback), whether that title came from a real heading, and the h2/h3
// headings (in document order) used to build the table of contents. ids are
// read straight from the parsed AST, so TOC links match the ids goldmark
// renders into the body.
func analyze(node ast.Node, src []byte, fallback string) (title string, fromHeading bool, headings []heading) {
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		switch {
		case h.Level == 1 && title == "":
			title = headingText(h, src)
		case h.Level == 2, h.Level == 3:
			headings = append(headings, heading{
				level: h.Level,
				text:  headingText(h, src),
				id:    headingID(h),
			})
		}
		// Heading children are inline nodes already consumed by headingText.
		return ast.WalkSkipChildren, nil
	})

	if title == "" {
		return htmlpkg.EscapeString(fallback), false, headings
	}
	return htmlpkg.EscapeString(title), true, headings
}

// headingText returns the concatenated plain text of a heading's inline
// descendants (Text and String nodes), trimmed. Emphasis/code/link markup is
// flattened to its text; raw inline HTML is dropped.
func headingText(n ast.Node, src []byte) string {
	var sb strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := c.(type) {
		case *ast.Text:
			sb.Write(t.Segment.Value(src))
		case *ast.String:
			sb.Write(t.Value)
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(sb.String())
}

// headingID returns the goldmark auto-generated id attribute of a heading, or
// "" if none was assigned.
func headingID(n ast.Node) string {
	v, ok := n.AttributeString("id")
	if !ok {
		return ""
	}
	switch s := v.(type) {
	case []byte:
		return string(s)
	case string:
		return s
	}
	return ""
}

// tocMinEntries is the h2/h3 count below which no table of contents is rendered
// — short documents stay uncluttered.
const tocMinEntries = 4

// buildTOC renders a compact navigation list of the given headings as same-page
// anchor links, or "" when there are none. Whether a TOC should appear at all is
// decided by shouldRenderTOC; this only formats it. Heading text and ids are
// HTML-escaped.
func buildTOC(headings []heading) string {
	if len(headings) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<nav class="toc" aria-label="Table of contents">` + "\n<ul>\n")
	for _, h := range headings {
		b.WriteString(`<li class="toc-h`)
		b.WriteString(strconv.Itoa(h.level))
		b.WriteString(`"><a href="#`)
		b.WriteString(htmlpkg.EscapeString(h.id))
		b.WriteString(`">`)
		b.WriteString(htmlpkg.EscapeString(h.text))
		b.WriteString("</a></li>\n")
	}
	b.WriteString("</ul>\n</nav>\n")
	return b.String()
}

// insertAfterFirstH1 inserts toc immediately after the first </h1> in content
// (skipping a trailing newline), or prepends it when no h1 is present.
func insertAfterFirstH1(content, toc string) string {
	const closeTag = "</h1>"
	i := strings.Index(content, closeTag)
	if i < 0 {
		return toc + content
	}
	i += len(closeTag)
	if i < len(content) && content[i] == '\n' {
		i++
	}
	return content[:i] + toc + content[i:]
}
