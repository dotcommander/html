package render

import (
	htmlpkg "html"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// reANSI matches ANSI/VT100 CSI escape sequences — used to detect colored input
// (so renderANSI can preserve its colors) and to strip stray escapes from the
// raw-text fallback.
var reANSI = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

// lexerAnalyseCap bounds how many bytes content auto-detection inspects, so
// language analysis stays fast on large inputs.
const lexerAnalyseCap = 64 << 10

// renderPlain renders src as a preformatted, non-Markdown page body, picking the
// most faithful representation:
//   - ANSI-colored input keeps its colors (renderANSI);
//   - otherwise, when a language is detected (or forced via opts.Lang), the
//     source is syntax-highlighted with chroma, reusing the same CSS as Markdown
//     code blocks;
//   - everything else falls back to HTML-escaped raw text.
//
// goldmark, the synthesized <h1>, and the TOC are bypassed, so line structure is
// preserved exactly. The page title is the already-escaped fallback title.
func renderPlain(src []byte, opts Options) string {
	body := ""
	switch {
	case reANSI.Match(src):
		body = renderANSI(src)
	default:
		if lexer := pickLexer(opts.Lang, opts.SourceName, src); lexer != nil {
			if hl, err := highlightCode(string(src), lexer, opts.CodeTheme); err == nil {
				body = hl
			}
		}
	}
	if body == "" {
		// Raw preformatted fallback (defensively strip any stray escapes).
		clean := reANSI.ReplaceAll(src, nil)
		body = `<pre><code class="language-plaintext">` + htmlpkg.EscapeString(string(clean)) + "</code></pre>\n"
	}
	return wrapPage(htmlpkg.EscapeString(opts.FallbackTitle), body, opts)
}

// pickLexer chooses a chroma lexer for plain input, or nil to render raw escaped
// text. Precedence: an explicit language (lang) — where "text"/"none"/etc. force
// raw — then the source filename (file inputs), then bounded content analysis
// (stdin). The plaintext lexer is treated as "no highlighting" so prose and
// unknown formats stay raw rather than wrapped in an empty chroma block.
func pickLexer(lang, sourceName string, src []byte) chroma.Lexer {
	var lx chroma.Lexer
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "text", "txt", "none", "plain", "plaintext", "raw":
		return nil
	case "":
		if sourceName != "" {
			lx = lexers.Match(sourceName)
		}
		if lx == nil {
			sample := src
			if len(sample) > lexerAnalyseCap {
				sample = sample[:lexerAnalyseCap]
			}
			lx = lexers.Analyse(string(sample))
		}
	default:
		lx = lexers.Get(lang) // nil if unknown → raw
	}
	if lx == nil || strings.EqualFold(lx.Config().Name, "plaintext") {
		return nil
	}
	return lx
}

// highlightCode renders source with the given chroma lexer using the same class-
// based formatter and code theme as the Markdown code path, so the existing
// highlightCSS styles it identically. chroma emits `<pre class="chroma light">`,
// which highlightCSS targets for both light (.chroma) and dark (.chroma.light,
// scoped under data-theme=dark) themes — no post-processing needed.
func highlightCode(source string, lexer chroma.Lexer, codeTheme string) (string, error) {
	lexer = chroma.Coalesce(lexer)
	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return "", err
	}
	formatter := chromahtml.New(chromahtml.WithClasses(true))
	style := styles.Get("github")
	if ValidCodeTheme(codeTheme) && codeTheme != "" {
		style = styles.Get(codeTheme)
	}
	var buf strings.Builder
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return "", err
	}
	return buf.String() + "\n", nil
}
