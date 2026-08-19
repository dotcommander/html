package render

import (
	"bytes"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
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
	return newMarkdownWithImageLimit(unsafe, codeTheme, maxInlineImages)
}

func newMarkdownWithImageLimit(unsafe bool, codeTheme string, imageLimit int64, extraTransformers ...parser.ASTTransformer) goldmark.Markdown {
	highlightOpts := []highlighting.Option{
		// WithClasses emits CSS class names (e.g. .chroma .k) rather than
		// inline styles, so highlightCSS controls colors and dark mode works.
		highlighting.WithFormatOptions(chromahtml.WithClasses(true), chromahtml.WithModeClasses(true)),
	}
	if ValidCodeTheme(codeTheme) {
		style := codeTheme
		if style == "" {
			style = "github"
		}
		highlightOpts = append(highlightOpts, highlighting.WithStyle(style))
	}
	imageTransformer := parser.ASTTransformer(imageInliner{maxTotal: imageLimit})
	if !unsafe {
		imageTransformer = safeImagePlaceholder{}
	}
	transformers := []util.PrioritizedValue{
		util.Prioritized(imageTransformer, 100),
		util.Prioritized(localLinkRebaser{}, 110),
	}
	for i, transformer := range extraTransformers {
		transformers = append(transformers, util.Prioritized(transformer, 200+i))
	}
	opts := []goldmark.Option{
		goldmark.WithExtensions(
			extension.GFM,
			alertExtension{},
			highlighting.NewHighlighting(highlightOpts...),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(transformers...),
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

// Render converts source into a complete, self-contained HTML document — as
// Markdown by default, or as preformatted plain text when opts.Plain is set.
func Render(src []byte, opts Options) (string, error) {
	html, _, err := RenderWithDiagnostics(src, opts)
	return html, err
}

// RenderWithDiagnostics renders src and reports images that could not be
// embedded. The diagnostics are deterministic and deduplicated by destination
// and reason. Plain and safe rendering return no diagnostics; safe mode retains
// its no-filesystem-I/O guarantee.
func RenderWithDiagnostics(src []byte, opts Options) (string, []ImageDiagnostic, error) {
	if opts.Plain {
		return renderPlain(src, opts), nil, nil
	}
	md := mdUnsafe
	switch {
	case len(opts.semanticLists) > 0:
		md = newMarkdownWithImageLimit(!opts.Safe, opts.CodeTheme, maxInlineImages, semanticListTransformer{refs: opts.semanticLists})
	case opts.CodeTheme != "":
		md = newMarkdown(!opts.Safe, opts.CodeTheme)
	case opts.Safe:
		md = mdSafe
	}
	pc := parser.NewContext()
	pc.Set(baseDirKey, opts.SourceDir)
	if opts.RebaseLocalLinks {
		pc.Set(linkBaseDirKey, opts.SourceDir)
	}
	doc := md.Parser().Parse(text.NewReader(src), parser.WithContext(pc))
	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, src, doc); err != nil {
		return "", nil, err
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
	return wrapPage(title, content, opts), imageDiagnostics(pc), nil
}

// ImageDiagnostics returns the image diagnostics for a Markdown render without
// producing HTML. It is used to preserve warnings on fresh cache hits.
func ImageDiagnostics(src []byte, opts Options) []ImageDiagnostic {
	if opts.Plain || opts.Safe {
		return nil
	}
	md := mdUnsafe
	if opts.CodeTheme != "" {
		md = newMarkdown(true, opts.CodeTheme)
	}
	pc := parser.NewContext()
	pc.Set(baseDirKey, opts.SourceDir)
	md.Parser().Parse(text.NewReader(src), parser.WithContext(pc))
	return imageDiagnostics(pc)
}

func imageDiagnostics(pc parser.Context) []ImageDiagnostic {
	state, _ := pc.Get(imageInlinerStateKey).(*imageInlinerState)
	if state == nil || len(state.diagnostics) == 0 {
		return nil
	}
	return append([]ImageDiagnostic(nil), state.diagnostics...)
}
