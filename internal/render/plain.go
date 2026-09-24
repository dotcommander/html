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

// ReANSI matches ANSI/VT100 CSI escape sequences — used to detect colored input
// (so RenderANSI can preserve its colors) and to strip stray escapes from the
// raw-text fallback.
var ReANSI = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

// lexerAnalyseCap bounds how many bytes content auto-detection inspects, so
// language analysis stays fast on large inputs.
const lexerAnalyseCap = 64 << 10

// renderPlain renders src as a preformatted, non-Markdown page Body, picking the
// most faithful representation:
//   - ANSI-colored input keeps its colors (RenderANSI);
//   - otherwise, when a language is detected (or forced via opts.Lang), the
//     source is syntax-highlighted with chroma, reusing the same CSS as Markdown
//     code blocks;
//   - everything else falls back to HTML-escaped raw text.
//
// goldmark, the synthesized <h1>, and the TOC are bypassed, so line structure is
// preserved exactly. Page assembly and framing belong to AssemblePage.
func renderPlain(src []byte, opts Options) string {
	Body := ""
	switch {
	case ReANSI.Match(src):
		Body = RenderANSI(src)
	default:
		if opts.Lang == "" {
			if tableBody, ok := renderPlainTableDocument(src, opts.SourceName); ok {
				Body = tableBody
			} else if table, ok := detectPlainTable(src, opts.SourceName); ok {
				Body = renderPlainTable(table)
			}
		}
		if Body == "" {
			if lexer := PickLexer(opts.Lang, opts.SourceName, src); lexer != nil {
				if hl, err := HighlightCode(string(src), lexer, opts.CodeTheme); err == nil {
					Body = hl
				}
			}
		}
	}
	if Body == "" {
		// Raw preformatted fallback (defensively strip any stray escapes).
		clean := ReANSI.ReplaceAll(src, nil)
		Body = `<pre><code class="language-plaintext">` + htmlpkg.EscapeString(string(clean)) + "</code></pre>\n"
	}
	return Body
}

// PickLexer chooses a chroma lexer for plain input, or nil to render raw escaped
// text. Precedence: an explicit language (lang) — where "text"/"none"/etc. force
// raw — then the source filename (file inputs), then bounded content analysis
// (stdin). The plaintext lexer is treated as "no highlighting" so prose and
// unknown formats stay raw rather than wrapped in an empty chroma block.
func PickLexer(lang, sourceName string, src []byte) chroma.Lexer {
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
			if looksLikeHumanText(sample) {
				return nil
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

func looksLikeHumanText(src []byte) bool {
	text := strings.TrimSpace(string(src))
	if text == "" || hasStrongCodeSignal(text) {
		return false
	}
	lines := strings.Split(text, "\n")
	proseLines := 0
	for _, line := range lines {
		if looksLikeProseLine(strings.TrimSpace(line)) {
			proseLines++
		}
	}
	if proseLines >= 2 {
		return true
	}
	return proseLines == 1 && len(strings.Fields(text)) >= 8
}

func hasStrongCodeSignal(text string) bool {
	for _, needle := range []string{
		"\npackage ", "package main", "\nfunc ", "func main", "\nimport ", "#!",
		"{", "}", ";", ":=", "=>", "::", "</", "<?", "def ", "class ",
		"SELECT ", "FROM ",
	} {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func looksLikeProseLine(line string) bool {
	if line == "" {
		return false
	}
	trimmed := strings.TrimLeft(line, "-*0123456789. )")
	words := strings.Fields(trimmed)
	if len(words) < 3 {
		return false
	}
	return strings.ContainsAny(trimmed, " .,?!'\"`*")
}

// HighlightCode renders source with the given chroma lexer using the same class-
// based formatter and code theme as the Markdown code path, so the existing
// highlightCSS styles it identically. Mode classes keep the light wrapper
// explicit so the page can switch to its scoped dark palette at runtime.
func HighlightCode(source string, lexer chroma.Lexer, codeTheme string) (string, error) {
	lexer = chroma.Coalesce(lexer)
	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return "", err
	}
	formatter := chromahtml.New(chromahtml.WithClasses(true), chromahtml.WithModeClasses(true))
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
