package render

import (
	"fmt"
	"strings"
	"sync"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
)

// highlightCSSOnce caches the generated chroma CSS — immutable after first call.
var highlightCSSOnce = sync.OnceValue(func() string {
	formatter := chromahtml.New(chromahtml.WithClasses(true))

	lightStyle := styles.Get("github")
	darkStyle := styles.Get("github-dark")

	var light strings.Builder
	if err := formatter.WriteCSS(&light, lightStyle); err != nil {
		panic(fmt.Sprintf("render: chroma light CSS: %v", err))
	}

	var dark strings.Builder
	if err := formatter.WriteCSS(&dark, darkStyle); err != nil {
		panic(fmt.Sprintf("render: chroma dark CSS: %v", err))
	}

	// goldmark-highlighting emits <pre class="chroma light"> by default. Scope
	// the dark palette to data-theme=dark while targeting that emitted class.
	darkCSS := scopeDarkHighlightCSS(strings.ReplaceAll(dark.String(), ".dark", ".light"))

	return light.String() + darkCSS
})

func scopeDarkHighlightCSS(css string) string {
	var b strings.Builder
	for _, line := range strings.Split(css, "\n") {
		if line == "" {
			continue
		}
		if i := strings.Index(line, "*/ "); i >= 0 {
			b.WriteString(line[:i+3])
			b.WriteByte(' ')
			b.WriteString(`:root[data-theme="dark"] `)
			b.WriteString(line[i+3:])
		} else {
			b.WriteString(`:root[data-theme="dark"] `)
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}
	b.WriteString(`:root[data-theme="dark"] .chroma.light :is(.na, .nb, .bp, .nx, .p, .ge) { color: inherit; }` + "\n")
	return b.String()
}

// highlightCSS returns chroma CSS for light (default, unscoped) and dark
// (scoped under :root[data-theme="dark"], set by theme.js) themes.
func highlightCSS() string { return highlightCSSOnce() }

// wrapPage wraps rendered body HTML in a full HTML5 document.
// title must already be HTML-escaped by the caller. opts supplies optional
// presentation overrides (initial theme, reader width).
func wrapPage(title, body string, opts Options) string {
	// themeDefault sets the pre-paint fallback theme that theme.js reads; only
	// an explicit "light"/"dark" forces it ("" and "auto" follow the system).
	themeDefault := ""
	if opts.Theme == "light" || opts.Theme == "dark" {
		themeDefault = fmt.Sprintf("window.HTML_DEFAULT_THEME = %q;\n", opts.Theme)
	}
	// widthOverride is appended last in the <style> block; its value is
	// validated upstream (config.maxWidthRe) to a plain CSS length.
	widthOverride := ""
	if opts.MaxWidth != "" {
		widthOverride = fmt.Sprintf("\n.markdown-body { max-width: %s; }", opts.MaxWidth)
	}

	frameStyle := ""
	content := body
	if opts.Frame {
		frameStyle = "\n" + frameCSS()
		content = terminalFrame(title, body)
	}

	var w strings.Builder
	fmt.Fprintf(&w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <script>
%s%s
  </script>
  <title>%s</title>
  <style>
%s
%s%s%s
  </style>
</head>
<body>
  <button id="theme-toggle" class="theme-toggle" type="button" aria-label="Toggle color theme" aria-pressed="false">☾</button>
  <article class="markdown-body">
%s
  </article>
  <script>
%s
  </script>
  <script>
%s
  </script>
  <script>
%s
  </script>
</body>
</html>
`, themeDefault, themeJS(), title, baseCSS(), highlightCSS(), widthOverride, frameStyle, content, copyJS(), headingsJS(), reportJS())
	return w.String()
}

// terminalFrame wraps a plain/ANSI body in faux terminal-window chrome (a title
// bar with traffic-light dots over the body). title must already be HTML-escaped
// by the caller; body is the already-rendered <pre> content.
func terminalFrame(title, body string) string {
	return `<div class="term-frame"><div class="term-bar">` +
		`<span class="term-dots"><i></i><i></i><i></i></span>` +
		`<span class="term-title">` + title + `</span></div>` +
		`<div class="term-body">` + body + `</div></div>`
}
