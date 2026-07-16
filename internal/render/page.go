package render

import (
	"fmt"
	"strings"
	"sync"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
)

var highlightCSSCache sync.Map

// highlightCSS caches generated chroma CSS by style name. Empty preserves the
// original github light / github-dark pair; a custom style is used in both
// light and dark page themes so code blocks keep the requested identity.
func highlightCSS(codeTheme string) string {
	key := codeTheme
	if key == "" {
		key = "github/github-dark"
	}
	if css, ok := highlightCSSCache.Load(key); ok {
		return css.(string)
	}
	css := buildHighlightCSS(codeTheme)
	actual, _ := highlightCSSCache.LoadOrStore(key, css)
	return actual.(string)
}

func buildHighlightCSS(codeTheme string) string {
	formatter := chromahtml.New(chromahtml.WithClasses(true))

	lightStyle := styles.Get("github")
	darkStyle := styles.Get("github-dark")
	if ValidCodeTheme(codeTheme) && codeTheme != "" {
		style := styles.Get(codeTheme)
		lightStyle = style
		darkStyle = style
	}

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
}

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
	paletteDefault := ""
	if validPalette(opts.Palette) {
		paletteDefault = fmt.Sprintf("window.HTML_DEFAULT_PALETTE = %q;\n", opts.Palette)
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
	alertStyle := ""
	if strings.Contains(body, `class="markdown-alert `) {
		alertStyle = "\n" + alertCSS()
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
%s%s%s%s
  </style>
</head>
<body>
  <div class="theme-controls" role="group" aria-label="Appearance controls">
    <button id="theme-toggle" class="theme-toggle" type="button" aria-label="Toggle light or dark theme" aria-pressed="false">☾</button>
    <div class="palette-switcher" aria-label="Color palette">
      <button class="palette-button" type="button" data-palette-choice="sepia" aria-label="Sepia palette" aria-pressed="false"></button>
      <button class="palette-button" type="button" data-palette-choice="blue" aria-label="Blue palette" aria-pressed="false"></button>
      <button class="palette-button" type="button" data-palette-choice="green" aria-label="Green palette" aria-pressed="false"></button>
      <button class="palette-button" type="button" data-palette-choice="rose" aria-label="Rose palette" aria-pressed="false"></button>
      <button class="palette-button" type="button" data-palette-choice="catppuccin" aria-label="Catppuccin palette" aria-pressed="false"></button>
    </div>
  </div>
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
`, themeDefault+paletteDefault, themeJS(), title, baseCSS(), highlightCSS(opts.CodeTheme), widthOverride, frameStyle, alertStyle, content, copyJS(), headingsJS(), reportJS())
	return w.String()
}

func validPalette(palette string) bool {
	switch palette {
	case "sepia", "blue", "green", "rose", "catppuccin":
		return true
	default:
		return false
	}
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
