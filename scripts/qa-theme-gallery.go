//go:build ignore

package main

import (
	"context"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/dotcommander/html/internal/render"
	"github.com/dotcommander/html/internal/report"
)

type themeCase struct {
	Theme   string
	Palette string
	File    string
	Title   string
	Colors  themeColors
}

type themeColors struct {
	BG     string
	Paper  string
	Text   string
	Muted  string
	Accent string
}

type componentSample struct {
	Slug       string
	Title      string
	SourceName string
	Source     []byte
	Mode       report.ModeOverride
	Layout     report.LayoutOverride
	Plain      bool
}

type renderedSample struct {
	componentSample
	Path string
}

func main() {
	root, err := repoRoot()
	check(err)
	outDir := filepath.Join(root, ".work/html-qa/theme-gallery")
	check(os.RemoveAll(outDir))
	check(os.MkdirAll(outDir, 0o755))
	check(writeMediaAssets(outDir))

	cases := themeCases()
	samples := componentSamples()
	for _, c := range cases {
		rendered := renderComponentPages(outDir, c, samples)
		page, err := render.Render([]byte(themePageMarkdown(c, rendered)), render.Options{
			FallbackTitle: c.Title + " Component Gallery",
			Theme:         c.Theme,
			Palette:       c.Palette,
			SourceName:    c.File,
		})
		check(err)
		require(strings.Contains(page, `HTML_DEFAULT_THEME = "`+c.Theme+`"`), "%s missing default theme", c.File)
		require(strings.Contains(page, `HTML_DEFAULT_PALETTE = "`+c.Palette+`"`), "%s missing default palette", c.File)
		require(strings.Contains(page, `data-palette-choice="catppuccin"`), "%s missing catppuccin control", c.File)
		require(strings.Contains(page, `class="theme-controls"`), "%s missing theme controls", c.File)
		require(strings.Contains(page, `class="theme-component-grid"`), "%s missing component gallery", c.File)
		check(os.WriteFile(filepath.Join(outDir, c.File), []byte(page), 0o644))
	}

	indexPath := filepath.Join(outDir, "index.html")
	check(os.WriteFile(indexPath, []byte(renderThemeIndex(cases)), 0o644))
	fmt.Println(indexPath)
}

func renderComponentPages(outDir string, c themeCase, samples []componentSample) []renderedSample {
	pageDir := filepath.Join(outDir, "pages", strings.TrimSuffix(c.File, ".html"))
	check(os.MkdirAll(pageDir, 0o755))
	rendered := make([]renderedSample, 0, len(samples))
	for _, s := range samples {
		var page string
		var err error
		opts := render.Options{
			FallbackTitle: s.Title,
			Theme:         c.Theme,
			Palette:       c.Palette,
			SourceName:    s.SourceName,
			SourceDir:     outDir,
			Plain:         s.Plain,
		}
		if s.Mode == "" && s.Layout == "" && (strings.HasSuffix(s.SourceName, ".md") || s.Plain) {
			page, err = render.Render(s.Source, opts)
		} else {
			analysis, plan := report.Plan(context.Background(), s.Source, report.Options{
				SourceName: s.SourceName,
				Mode:       s.Mode,
				Layout:     s.Layout,
				Planner:    report.PlannerOff,
			})
			page, err = render.RenderReport(s.Source, opts, analysis, plan)
		}
		check(err)
		path := filepath.Join(pageDir, s.Slug+".html")
		check(os.WriteFile(path, []byte(page), 0o644))
		rendered = append(rendered, renderedSample{componentSample: s, Path: filepath.ToSlash(filepath.Join("pages", strings.TrimSuffix(c.File, ".html"), s.Slug+".html"))})
	}
	return rendered
}

func componentSamples() []componentSample {
	return []componentSample{
		{
			Slug:       "article-media",
			Title:      "Article, Markdown Pieces, Images, And SVG",
			SourceName: "article-media.md",
			Source: []byte(`# Article Surface

The gallery page shows headings, links, lists, quote blocks, task items, tables, inline code, keyboard input, highlighted code, and local media under the selected palette.

![Raster swatch](media-assets/raster.png)

![Vector badge](media-assets/vector.svg)

| Piece | State |
|---|---|
| Markdown table | Visible |
| Raster image | Inlined |
| SVG image | Inlined |

- [x] Theme controls
- [x] Palette controls
- [ ] Manual visual pass

> The same generated page should stay readable across every color palette.

Press <kbd>G</kbd>, read ` + "`inline code`" + `, and inspect the highlighted block.

` + "```go" + `
fmt.Println("theme gallery")
` + "```" + `
`),
		},
		{
			Slug:       "table",
			Title:      "Report Table",
			SourceName: "records.csv",
			Source:     []byte("name,score,status\nalpha,10,ready\nbeta,2,review\ngamma,7,watch\n"),
			Mode:       report.ModeOverrideTable,
		},
		{
			Slug:       "cards",
			Title:      "Record Cards",
			SourceName: "cards.json",
			Source:     []byte(`[{"name":"alpha","owner":"ops","status":"ready"},{"id":42,"score":2,"notes":"needs review"},{"title":"launch","due":"Friday","blocked":false}]`),
			Mode:       report.ModeOverrideCards,
		},
		{
			Slug:       "tabs",
			Title:      "Tabbed Report",
			SourceName: "tabs.csv",
			Source:     []byte("name,score,status\nalpha,10,ready\nbeta,2,review\n"),
			Layout:     report.LayoutOverrideTabs,
		},
		{
			Slug:       "slides",
			Title:      "Slides",
			SourceName: "slides.md",
			Source:     []byte("# Deck\n\nIntro slide.\n\n## First\n\nThe first section becomes a slide.\n\n## Second\n\nThe second section becomes another slide.\n"),
			Layout:     report.LayoutOverrideSlides,
		},
		{
			Slug:       "json",
			Title:      "Raw JSON",
			SourceName: "object.json",
			Source:     []byte(`{"project":"html","theme":{"mode":"dark","palette":"catppuccin"},"counts":{"reports":14,"components":11}}`),
		},
		{
			Slug:       "diff",
			Title:      "Diff View",
			SourceName: "change.patch",
			Source:     []byte("diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1,2 +1,2 @@\n-old line\n+new line\n context\n"),
		},
		{
			Slug:       "tree",
			Title:      "File Tree",
			SourceName: "tree.txt",
			Source:     []byte(".\n├── cmd\n│   └── html\n├── internal\n│   ├── render\n│   └── report\n└── README.md\n"),
		},
		{
			Slug:       "log",
			Title:      "Log Lines",
			SourceName: "access.log",
			Source:     []byte("127.0.0.1 - - [16/Jun/2026:12:00:00 -0400] \"GET /index.html HTTP/1.1\" 200 1234\n127.0.0.1 - - [16/Jun/2026:12:00:01 -0400] \"GET /missing HTTP/1.1\" 404 123\n127.0.0.1 - - [16/Jun/2026:12:00:02 -0400] \"POST /api HTTP/1.1\" 500 42\n"),
		},
		{
			Slug:       "plain-code",
			Title:      "Plain Code",
			SourceName: "sample.go",
			Source:     []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"html\")\n}\n"),
			Plain:      true,
		},
	}
}

func themePageMarkdown(c themeCase, samples []renderedSample) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s Component Gallery\n\n", c.Title)
	fmt.Fprintf(&b, `<div class="theme-gallery-summary" style="--sample-bg:%s;--sample-paper:%s;--sample-text:%s;--sample-muted:%s;--sample-accent:%s">
  <div><strong>%s / %s</strong><span>Generated pages rendered with this fixed theme and palette.</span></div>
  <ol aria-label="%s color tokens"><li></li><li></li><li></li><li></li><li></li></ol>
</div>
<div class="theme-component-grid">
`, html.EscapeString(c.Colors.BG), html.EscapeString(c.Colors.Paper), html.EscapeString(c.Colors.Text), html.EscapeString(c.Colors.Muted), html.EscapeString(c.Colors.Accent), html.EscapeString(c.Theme), html.EscapeString(c.Palette), html.EscapeString(c.Title))
	for _, s := range samples {
		fmt.Fprintf(&b, `  <section class="theme-component-card"><header><h2>%s</h2><a href="%s">Open</a></header><iframe title="%s" src="%s" loading="eager" onload="this.dataset.loaded='true'"></iframe></section>
`, html.EscapeString(s.Title), html.EscapeString(s.Path), html.EscapeString(s.Title), html.EscapeString(s.Path))
	}
	b.WriteString("</div>\n")
	b.WriteString(`<style>
.theme-gallery-summary { display:flex; align-items:center; justify-content:space-between; gap:.9rem; margin:0 0 1rem; padding:.85rem .95rem; color:var(--sample-text); background:var(--sample-bg); border:1px solid var(--border); border-radius:8px; }
.theme-gallery-summary strong,.theme-gallery-summary span { display:block; }
.theme-gallery-summary span { margin-top:.15rem; color:var(--sample-muted); font-size:.84rem; }
.theme-gallery-summary ol { flex:0 0 auto; display:flex; gap:.35rem; margin:0; padding:0; list-style:none; }
.theme-gallery-summary li { width:1.3rem; height:1.3rem; border:1px solid rgba(0,0,0,.14); border-radius:999px; box-shadow:0 .2rem .7rem rgba(0,0,0,.12); }
.theme-gallery-summary li:nth-child(1) { background:var(--sample-bg); }
.theme-gallery-summary li:nth-child(2) { background:var(--sample-paper); }
.theme-gallery-summary li:nth-child(3) { background:var(--sample-text); }
.theme-gallery-summary li:nth-child(4) { background:var(--sample-muted); }
.theme-gallery-summary li:nth-child(5) { background:var(--sample-accent); }
.theme-component-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,24rem),1fr)); gap:.9rem; }
.theme-component-card { min-width:0; overflow:hidden; background:var(--paper); border:1px solid var(--border); border-radius:8px; box-shadow:0 .65rem 1.8rem rgba(35,30,24,.06); }
.theme-component-card header { display:flex; align-items:center; justify-content:space-between; gap:.75rem; padding:.62rem .75rem; background:var(--panel); border-bottom:1px solid var(--border); }
.theme-component-card h2 { margin:0; font-size:.88rem; line-height:1.2; }
.theme-component-card a { font-size:.78rem; font-weight:800; }
.theme-component-card iframe { display:block; width:100%; height:20rem; border:0; background:var(--paper); }
@media (max-width:45rem) {
  .theme-gallery-summary { display:grid; border-left:0; border-right:0; border-radius:0; margin-inline:calc(var(--page-pad) * -1); }
  .theme-component-grid { margin-inline:calc(var(--page-pad) * -1); gap:.75rem; }
  .theme-component-card { border-left:0; border-right:0; border-radius:0; }
  .theme-component-card iframe { height:18rem; }
}
</style>
`)
	return b.String()
}

func themeCases() []themeCase {
	palettes := []string{"sepia", "blue", "green", "rose", "catppuccin"}
	themes := []string{"light", "dark"}
	cases := make([]themeCase, 0, len(palettes)*len(themes))
	for _, theme := range themes {
		for _, palette := range palettes {
			file := theme + "-" + palette + ".html"
			cases = append(cases, themeCase{
				Theme:   theme,
				Palette: palette,
				File:    file,
				Title:   titleCase(theme) + " " + titleCase(palette),
				Colors:  colorsFor(theme, palette),
			})
		}
	}
	return cases
}

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func renderThemeIndex(cases []themeCase) string {
	var cards strings.Builder
	for _, c := range cases {
		fmt.Fprintf(&cards, `<section data-theme-case="%s/%s"><header><div><h2>%s</h2><p>%s / %s</p></div><a href="%s">Open</a></header><div class="theme-preview" style="--sample-bg:%s;--sample-paper:%s;--sample-text:%s;--sample-muted:%s;--sample-accent:%s"><div><strong>Sample</strong><span>Reader, report, media, and component surfaces</span></div><ol aria-label="%s color tokens"><li></li><li></li><li></li><li></li><li></li></ol></div><iframe title="%s" src="%s" loading="eager" onload="this.dataset.loaded='true'"></iframe></section>`,
			html.EscapeString(c.Theme), html.EscapeString(c.Palette),
			html.EscapeString(c.Title),
			html.EscapeString(c.Theme), html.EscapeString(c.Palette),
			html.EscapeString(c.File),
			html.EscapeString(c.Colors.BG), html.EscapeString(c.Colors.Paper), html.EscapeString(c.Colors.Text), html.EscapeString(c.Colors.Muted), html.EscapeString(c.Colors.Accent),
			html.EscapeString(c.Title),
			html.EscapeString(c.Title), html.EscapeString(c.File),
		)
	}
	return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>HTML Theme Gallery</title>
  <style>
    :root { --bg:#f5f4f1; --paper:#fffefa; --text:#25221f; --muted:#6d6760; --border:#ded8cc; --accent:#8b5b2f; --panel:#f1ece3; }
    * { box-sizing:border-box; }
    body { margin:0; color:var(--text); background:var(--bg); font-family:ui-sans-serif,-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif; }
    main { width:min(calc(100% - 2rem),92rem); margin:1.25rem auto 2rem; }
    h1 { margin:0 0 .3rem; font-size:1.55rem; line-height:1.15; }
    .intro { margin:0 0 1rem; max-width:64rem; color:var(--muted); line-height:1.45; }
    .grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,32rem),1fr)); gap:1rem; }
    section { overflow:hidden; background:var(--paper); border:1px solid var(--border); border-radius:8px; box-shadow:0 .9rem 2.4rem rgba(35,30,24,.07); }
    header { display:flex; align-items:center; justify-content:space-between; gap:.75rem; padding:.75rem .85rem; background:var(--panel); border-bottom:1px solid var(--border); }
    h2 { margin:0; font-size:.95rem; line-height:1.2; }
    header p { margin:.12rem 0 0; color:var(--muted); font-size:.78rem; font-weight:700; text-transform:uppercase; }
    a { color:var(--accent); font-weight:800; text-decoration:none; }
    .theme-preview { display:flex; align-items:center; justify-content:space-between; gap:.75rem; min-height:4rem; padding:.7rem .85rem; color:var(--sample-text); background:var(--sample-bg); border-bottom:1px solid var(--border); }
    .theme-preview strong,.theme-preview span { display:block; }
    .theme-preview strong { font-size:.86rem; }
    .theme-preview span { margin-top:.12rem; color:var(--sample-muted); font-size:.76rem; line-height:1.25; }
    .theme-preview ol { flex:0 0 auto; display:flex; gap:.32rem; margin:0; padding:0; list-style:none; }
    .theme-preview li { width:1.2rem; height:1.2rem; border:1px solid rgba(0,0,0,.12); border-radius:999px; box-shadow:0 .2rem .7rem rgba(0,0,0,.12); }
    .theme-preview li:nth-child(1) { background:var(--sample-bg); }
    .theme-preview li:nth-child(2) { background:var(--sample-paper); }
    .theme-preview li:nth-child(3) { background:var(--sample-text); }
    .theme-preview li:nth-child(4) { background:var(--sample-muted); }
    .theme-preview li:nth-child(5) { background:var(--sample-accent); }
    iframe { display:block; width:100%; height:24rem; border:0; background:white; }
    @media (max-width:45rem) {
      main { width:100%; margin:0; padding:1rem 0 1.25rem; }
      h1,.intro { padding-inline:1rem; }
      .grid { gap:.75rem; }
      section { border-left:0; border-right:0; border-radius:0; }
      iframe { height:23rem; }
    }
  </style>
</head>
<body>
  <main>
    <h1>HTML Theme Gallery</h1>
    <p class="intro">Rendered from the real page wrapper with every supported palette in light and dark mode. Each frame has fixed defaults and opens a component gallery covering reader pieces, report tables, cards, tabs, slides, JSON, diffs, trees, logs, code, images, and SVGs.</p>
    <div class="grid">` + cards.String() + `</div>
  </main>
</body>
</html>
`
}

func colorsFor(theme, palette string) themeColors {
	colors := map[string]themeColors{
		"light-sepia":      {BG: "#f7f5ef", Paper: "#fffdf8", Text: "#27231f", Muted: "#6f685f", Accent: "#9f5b2d"},
		"light-blue":       {BG: "#f3f7fb", Paper: "#fcfdff", Text: "#18212f", Muted: "#5f6e82", Accent: "#2563eb"},
		"light-green":      {BG: "#f3f8f3", Paper: "#fcfefb", Text: "#1d281f", Muted: "#63715f", Accent: "#2d7a46"},
		"light-rose":       {BG: "#fbf5f7", Paper: "#fffdfd", Text: "#2b1e25", Muted: "#755f68", Accent: "#be4b75"},
		"light-catppuccin": {BG: "#eff1f5", Paper: "#f9fafc", Text: "#4c4f69", Muted: "#6c6f85", Accent: "#8839ef"},
		"dark-sepia":       {BG: "#11100e", Paper: "#181613", Text: "#eee8dd", Muted: "#afa69a", Accent: "#e0a05f"},
		"dark-blue":        {BG: "#0b111b", Paper: "#101827", Text: "#e8eef8", Muted: "#a5b3c8", Accent: "#6da8ff"},
		"dark-green":       {BG: "#0d130e", Paper: "#121a14", Text: "#e8f1e4", Muted: "#a9b8a5", Accent: "#79c48a"},
		"dark-rose":        {BG: "#160f13", Paper: "#1e151a", Text: "#f3e7ec", Muted: "#bea7b0", Accent: "#f08ab2"},
		"dark-catppuccin":  {BG: "#1e1e2e", Paper: "#242438", Text: "#cdd6f4", Muted: "#a6adc8", Accent: "#cba6f7"},
	}
	return colors[theme+"-"+palette]
}

func writeMediaAssets(outDir string) error {
	mediaDir := filepath.Join(outDir, "media-assets")
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		return err
	}
	if err := writeRaster(filepath.Join(mediaDir, "raster.png")); err != nil {
		return err
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="420" height="180" viewBox="0 0 420 180" role="img" aria-labelledby="title desc">
  <title id="title">Generated SVG badge</title>
  <desc id="desc">A green, blue, and rose vector badge for the HTML QA theme gallery.</desc>
  <rect width="420" height="180" rx="22" fill="#fffefa"/>
  <circle cx="92" cy="90" r="48" fill="#2d7a46"/>
  <rect x="172" y="46" width="76" height="88" rx="16" fill="#2563eb"/>
  <path d="M326 42 376 138 276 138Z" fill="#be4b75"/>
  <path d="M38 146 C120 104 194 192 382 126" fill="none" stroke="#9f5b2d" stroke-width="12" stroke-linecap="round" opacity=".75"/>
</svg>
`
	return os.WriteFile(filepath.Join(mediaDir, "vector.svg"), []byte(svg), 0o644)
}

func writeRaster(path string) error {
	img := image.NewRGBA(image.Rect(0, 0, 420, 180))
	for y := 0; y < 180; y++ {
		for x := 0; x < 420; x++ {
			switch {
			case x < 140:
				img.Set(x, y, color.RGBA{R: 45, G: 122, B: 70, A: 255})
			case x < 280:
				img.Set(x, y, color.RGBA{R: 37, G: 99, B: 235, A: 255})
			default:
				img.Set(x, y, color.RGBA{R: 190, G: 75, B: 117, A: 255})
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above working directory")
		}
		dir = parent
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func require(ok bool, format string, args ...any) {
	if !ok {
		panic(fmt.Sprintf(format, args...))
	}
}
