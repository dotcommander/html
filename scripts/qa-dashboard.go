//go:build ignore

package main

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
)

type dashboardCard struct {
	Title       string
	Path        string
	Summary     string
	Proof       string
	HTMLCount   int
	PreviewPath string
}

func main() {
	root, err := repoRoot()
	check(err)
	outDir := filepath.Join(root, ".work/html-qa")
	check(os.MkdirAll(outDir, 0o755))
	detectionProof := detectionProof(filepath.Join(root, ".work/html-qa/detection/index.html"))

	cards := []dashboardCard{
		card(root, "Report Kind Matrix", ".work/html-qa/kind-matrix/index.html", ".work/html-qa/kind-matrix", "All deterministic report kinds, components, modes, layouts, output overrides, media previews, analyzer edge cases, and binary report rendering.", "Kind/component/mode/layout coverage plus 40 loaded report frames."),
		card(root, "CLI Report Smoke", ".work/html-qa/cli-smoke/index.html", ".work/html-qa/cli-smoke", "Public report CLI flags rendered by the real ./html binary: article, cards, tabs, code, diff, tree, log, slides, media, binary, and stdout table output.", "Stable output paths, theme controls, palette controls, stdout, and plan JSON."),
		card(root, "Document Smoke", ".work/html-qa/document-smoke/index.html", ".work/html-qa/document-smoke", "Ordinary document generation outside report mode: Markdown, output-to-stdout, forced Markdown, safe mode, plain text, source code, Go stdin, ANSI stdin, frame, and stdin Markdown.", "Normal document image inlining, binary refusal, terminal frame, stdin detection, and public flag-conflict error contracts."),
		card(root, "Config Smoke", ".work/html-qa/config-smoke/index.html", ".work/html-qa/config-smoke", "Public CLI rendering through an isolated user config: max width, default theme, default palette, and TOC override.", "Configured output reaches rendered HTML, while invalid config fails before writing output."),
		card(root, "Cache Smoke", ".work/html-qa/cache-smoke/index.html", ".work/html-qa/cache-smoke", "Default --no-open generation through an isolated HTML_CACHE_DIR for file and stdin inputs.", "File cache path is reused while fresh, --force rewrites it, and stdin content gets a content-hash cache page."),
		card(root, "Detection Matrix", ".work/html-qa/detection/index.html", ".work/html-qa/detection", "Bounded detector QA over binary, Markdown strong signals, weak Markdown false positives, source/config/log false positives, tabular inputs, CRLF/indent variants, and scan-window limits.", detectionProof),
		card(root, "Theme Gallery", ".work/html-qa/theme-gallery/index.html", ".work/html-qa/theme-gallery", "Every supported palette rendered in light and dark mode from the real page wrapper.", "10 direct theme pages with exact theme, palette, accent, one-row controls, and no overflow."),
	}
	for _, c := range cards {
		requireFile(filepath.Join(root, c.Path))
	}

	indexPath := filepath.Join(outDir, "index.html")
	check(os.WriteFile(indexPath, []byte(renderDashboard(cards)), 0o644))
	fmt.Println(indexPath)
}

func card(root, title, path, countDir, summary, proof string) dashboardCard {
	return dashboardCard{
		Title:       title,
		Path:        path,
		Summary:     summary,
		Proof:       proof,
		HTMLCount:   countHTML(filepath.Join(root, countDir)),
		PreviewPath: strings.TrimPrefix(path, ".work/html-qa/"),
	}
}

func renderDashboard(cards []dashboardCard) string {
	var b strings.Builder
	totalHTML := 0
	for _, c := range cards {
		totalHTML += c.HTMLCount
		fmt.Fprintf(&b, `<section data-dashboard-card data-dashboard-title="%s"><header><div><h2>%s</h2><p>%d HTML artifacts</p></div><a href="%s">Open</a></header><div class="card-copy"><p>%s</p><dl><div><dt>Proof</dt><dd>%s</dd></div></dl></div><iframe title="%s" src="%s" loading="eager" onload="this.dataset.loaded='true'"></iframe></section>`,
			html.EscapeString(c.Title),
			html.EscapeString(c.Title),
			c.HTMLCount,
			html.EscapeString(c.PreviewPath),
			html.EscapeString(c.Summary),
			html.EscapeString(c.Proof),
			html.EscapeString(c.Title),
			html.EscapeString(c.PreviewPath),
		)
	}
	return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>HTML QA Dashboard</title>
  <style>
    :root { --bg:#f5f4f1; --paper:#fffefa; --text:#25221f; --muted:#6d6760; --border:#ded8cc; --accent:#8b5b2f; --panel:#f1ece3; }
    * { box-sizing:border-box; }
    body { margin:0; color:var(--text); background:var(--bg); font-family:ui-sans-serif,-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif; }
    main { width:min(calc(100% - 2rem),94rem); margin:1.25rem auto 2rem; }
    h1 { margin:0 0 .3rem; font-size:1.65rem; line-height:1.15; }
    .intro { margin:0 0 1rem; max-width:66rem; color:var(--muted); line-height:1.45; }
    .summary { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,12rem),1fr)); gap:.65rem; margin:0 0 1rem; }
    .summary div,section { background:var(--paper); border:1px solid var(--border); border-radius:8px; box-shadow:0 .9rem 2.4rem rgba(35,30,24,.07); }
    .summary div { padding:.75rem; }
    .summary dt,.summary dd { margin:0; }
    .summary dt { color:var(--muted); font-size:.74rem; font-weight:800; text-transform:uppercase; }
    .summary dd { margin-top:.2rem; font-size:1.4rem; font-weight:850; }
    .grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,34rem),1fr)); gap:1rem; }
    section { min-width:0; overflow:hidden; }
    header { display:flex; align-items:center; justify-content:space-between; gap:.75rem; padding:.75rem .85rem; background:var(--panel); border-bottom:1px solid var(--border); }
    h2 { margin:0; font-size:.98rem; line-height:1.2; }
    header p { margin:.12rem 0 0; color:var(--muted); font-size:.78rem; font-weight:750; text-transform:uppercase; }
    a { color:var(--accent); font-weight:850; text-decoration:none; }
    .card-copy { display:grid; gap:.55rem; padding:.75rem .85rem; border-bottom:1px solid var(--border); }
    .card-copy p { margin:0; color:var(--muted); font-size:.84rem; line-height:1.38; }
    dl,dt,dd { margin:0; }
    dl div { display:grid; gap:.16rem; }
    dt { font-size:.72rem; font-weight:850; text-transform:uppercase; color:var(--accent); }
    dd { color:var(--text); font-size:.82rem; line-height:1.35; }
    iframe { display:block; width:100%; height:25rem; border:0; background:white; }
    @media (max-width:45rem) {
      main { width:100%; margin:0; padding:1rem 0 1.25rem; }
      h1,.intro,.summary { padding-inline:1rem; }
      .summary { grid-template-columns:repeat(2,minmax(0,1fr)); gap:.5rem; }
      .grid { gap:.75rem; }
      section { border-left:0; border-right:0; border-radius:0; }
      iframe { height:20rem; }
    }
  </style>
</head>
<body>
  <main>
    <h1>HTML QA Dashboard</h1>
    <p class="intro">Single entry point for generated QA artifacts. Each card embeds a generated artifact and links to the full page; the browser suite captures this page plus direct high-risk surfaces with chromedp.</p>
    <dl class="summary">
      <div><dt>Suites</dt><dd>` + fmt.Sprint(len(cards)) + `</dd></div>
      <div data-dashboard-total="suite-html"><dt>Suite Card HTML</dt><dd>` + fmt.Sprint(totalHTML) + `</dd></div>
      <div><dt>Browser Tool</dt><dd>chromedp</dd></div>
      <div><dt>Entry</dt><dd>just qa-browser</dd></div>
    </dl>
    <div class="grid">` + b.String() + `</div>
  </main>
</body>
</html>
`
}

func countHTML(dir string) int {
	count := 0
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".html") {
			count++
		}
		return nil
	})
	return count
}

func detectionProof(path string) string {
	src, err := os.ReadFile(path)
	check(err)
	text := string(src)
	total := strings.Count(text, `<tr data-kind="`)
	markdown := strings.Count(text, `<tr data-kind="markdown"`)
	plain := strings.Count(text, `<tr data-kind="plain"`)
	binary := strings.Count(text, `<tr data-kind="binary"`)
	return fmt.Sprintf("%d detector cases with expected binary/plain/Markdown classification (%d binary, %d plain, %d Markdown).", total, binary, plain, markdown)
}

func requireFile(path string) {
	info, err := os.Stat(path)
	if err != nil {
		panic(fmt.Sprintf("required QA artifact missing: %s: %v", path, err))
	}
	if info.IsDir() {
		panic(fmt.Sprintf("required QA artifact is a directory: %s", path))
	}
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
