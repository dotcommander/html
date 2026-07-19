package render

import (
	"fmt"
	htmlpkg "html"
	"strconv"
	"strings"

	"github.com/dotcommander/html/internal/report"
)

func codeBlock(src []byte, opts Options) string {
	if reANSI.Match(src) {
		return renderANSI(src)
	}
	if lexer := pickLexer(opts.Lang, opts.SourceName, src); lexer != nil {
		if html, err := highlightCode(string(src), lexer, opts.CodeTheme); err == nil {
			return html
		}
	}
	return rawPre(src)
}

func codeView(src []byte, opts Options, analysis report.Analysis) string {
	return codeOverview(src, opts, analysis) + codeBlock(src, opts)
}

func codeOverview(src []byte, opts Options, analysis report.Analysis) string {
	items := make([][2]string, 0, 3)
	if lang, ok := analysis.Data.(string); ok && strings.TrimSpace(lang) != "" {
		items = append(items, [2]string{"Language", lang})
	} else if lexer := pickLexer(opts.Lang, opts.SourceName, src); lexer != nil {
		items = append(items, [2]string{"Language", lexer.Config().Name})
	} else {
		items = append(items, [2]string{"Language", "Plain text"})
	}
	if opts.SourceName != "" {
		items = append(items, [2]string{"Source", opts.SourceName})
	}
	items = append(items, [2]string{"Renderer", codeRenderer(src, opts)})
	if analysis.Stats.Lines > 0 {
		items = append(items, [2]string{"Lines", strconv.Itoa(analysis.Stats.Lines)})
	}
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<dl class="code-overview" aria-label="Code overview">`)
	for _, item := range items {
		b.WriteString(`<div><dt>`)
		b.WriteString(htmlpkg.EscapeString(item[0]))
		b.WriteString(`</dt><dd>`)
		b.WriteString(htmlpkg.EscapeString(item[1]))
		b.WriteString(`</dd></div>`)
	}
	b.WriteString(`</dl>`)
	return b.String()
}

func codeRenderer(src []byte, opts Options) string {
	if reANSI.Match(src) {
		return "ANSI"
	}
	if pickLexer(opts.Lang, opts.SourceName, src) != nil {
		return "Chroma"
	}
	return "Plain text"
}

func summary(a report.Analysis) string {
	items := []string{
		"Kind: " + string(a.Kind),
		fmt.Sprintf("Confidence: %.2f", a.Confidence),
		fmt.Sprintf("Bytes: %d", a.Stats.Bytes),
		fmt.Sprintf("Lines: %d", a.Stats.Lines),
	}
	if a.Stats.Records > 0 {
		items = append(items, fmt.Sprintf("Records: %d", a.Stats.Records))
	}
	if a.Stats.Fields > 0 {
		items = append(items, fmt.Sprintf("Fields: %d", a.Stats.Fields))
	}
	if a.Stats.Files > 0 {
		items = append(items, fmt.Sprintf("Files: %d", a.Stats.Files))
	}
	var b strings.Builder
	b.WriteString(`<section class="report-summary" aria-label="Summary"><dl>`)
	for _, item := range items {
		k, v, _ := strings.Cut(item, ": ")
		b.WriteString(`<div><dt>`)
		b.WriteString(htmlpkg.EscapeString(k))
		b.WriteString(`</dt><dd>`)
		b.WriteString(htmlpkg.EscapeString(v))
		b.WriteString(`</dd></div>`)
	}
	b.WriteString(`</dl>`)
	if len(a.Reasons) > 0 {
		b.WriteString(`<p>`)
		b.WriteString(htmlpkg.EscapeString(strings.Join(a.Reasons, "; ")))
		b.WriteString(`</p>`)
	}
	b.WriteString(`</section>`)
	return b.String()
}
