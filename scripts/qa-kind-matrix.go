//go:build ignore

package main

import (
	"context"
	"encoding/base64"
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

type matrixSample struct {
	Slug          string
	Title         string
	SourceName    string
	Source        []byte
	Want          report.Kind
	WantComponent report.ComponentType
	WantMode      report.Mode
	Mode          report.ModeOverride
	Layout        report.LayoutOverride
	WantLayout    report.Layout
	WantHTML      []string
	MediaPreview  []mediaPreview
}

type mediaPreview struct {
	Label string
	Src   string
	Kind  string
	Note  string
}

type matrixRendered struct {
	Sample             matrixSample
	Analysis           report.Analysis
	Plan               report.ReportPlan
	SourceFile         string
	HTMLFile           string
	RenderedMediaImage []mediaPreview
}

func main() {
	root, err := repoRoot()
	check(err)
	outDir := filepath.Join(root, ".work/html-qa/kind-matrix")
	check(os.RemoveAll(outDir))
	check(os.MkdirAll(outDir, 0o755))
	check(writeMediaAssets(outDir))

	var results []matrixRendered
	for _, s := range matrixSamples() {
		srcPath := filepath.Join(outDir, s.SourceName)
		htmlPath := filepath.Join(outDir, s.Slug+".html")
		check(os.WriteFile(srcPath, s.Source, 0o644))

		analysis, plan := report.Plan(context.Background(), s.Source, report.Options{
			SourceName: s.SourceName,
			Planner:    report.PlannerOff,
			Mode:       s.Mode,
			Layout:     s.Layout,
		})
		require(analysis.Kind == s.Want, "%s detected as %s, want %s; reasons=%v", s.Slug, analysis.Kind, s.Want, analysis.Reasons)
		if s.WantLayout != "" {
			require(plan.Layout == s.WantLayout, "%s planned layout %s, want %s", s.Slug, plan.Layout, s.WantLayout)
		}
		if s.WantMode != "" {
			require(plan.Mode == s.WantMode, "%s planned mode %s, want %s", s.Slug, plan.Mode, s.WantMode)
		}
		if s.WantComponent != "" {
			require(hasComponent(plan.Components, s.WantComponent), "%s planned components %s, want %s", s.Slug, componentList(plan.Components), s.WantComponent)
		}

		page, err := render.RenderReport(s.Source, render.Options{
			FallbackTitle: s.Title,
			SourceName:    s.SourceName,
			SourceDir:     outDir,
			ReportTag:     "kind-matrix",
		}, analysis, plan)
		check(err)
		for _, want := range s.WantHTML {
			require(strings.Contains(page, want), "%s rendered HTML missing %q", s.Slug, want)
		}
		renderedImages := renderedMediaImages(page)
		if len(s.MediaPreview) > 0 {
			require(len(renderedImages) == len(s.MediaPreview), "%s rendered media image count = %d, want %d", s.Slug, len(renderedImages), len(s.MediaPreview))
		}
		check(os.WriteFile(htmlPath, []byte(page), 0o644))
		results = append(results, matrixRendered{
			Sample:             s,
			Analysis:           analysis,
			Plan:               plan,
			SourceFile:         filepath.Base(srcPath),
			HTMLFile:           filepath.Base(htmlPath),
			RenderedMediaImage: renderedImages,
		})
	}

	indexPath := filepath.Join(outDir, "index.html")
	check(os.WriteFile(indexPath, []byte(renderMatrixIndex(results)), 0o644))
	fmt.Println(indexPath)
}

func matrixSamples() []matrixSample {
	return []matrixSample{
		{Slug: "markdown", Title: "Markdown Article", SourceName: "markdown.md", Source: []byte("# Release Notes\n\n## Added\n\n- Theme controls\n- Report cards\n\n## Fixed\n\nPlain source wrapping.\n"), Want: report.KindMarkdown},
		{
			Slug:       "media",
			Title:      "Markdown Media",
			SourceName: "media.md",
			Source:     []byte("# Markdown Media\n\nLocal raster image, inlined into the HTML:\n\n![Generated raster swatch](media-assets/raster.png)\n\nLocal SVG image, also inlined into the HTML:\n\n![Generated SVG badge](media-assets/vector.svg)\n"),
			Want:       report.KindMarkdown,
			WantHTML:   []string{`data:image/png;base64,`, `data:image/svg+xml;base64,`, `<dt>Images</dt><dd>2</dd>`},
			MediaPreview: []mediaPreview{
				{Label: "PNG raster", Src: "media-assets/raster.png", Kind: "Raster asset", Note: "Browser-rendered source PNG"},
				{Label: "SVG vector", Src: "media-assets/vector.svg", Kind: "Vector asset", Note: "Browser-rendered source SVG"},
			},
		},
		{Slug: "markdown-components", Title: "Markdown Components", SourceName: "markdown-components.md", Source: []byte("# Markdown Components\n\nIntro text for a page with richer Markdown pieces rendered together.\n\n## Table\n\n| Piece | State |\n|---|---|\n| Table | Ready |\n| Tasks | Ready |\n\n## Tasks\n\n- [x] Ship renderer\n- [ ] Capture screenshots\n\n## Quote\n\n> Keep generated pages self-contained and pleasant to scan.\n\n## Code\n\n```go\nfmt.Println(\"html\")\n```\n"), Want: report.KindMarkdown},
		{Slug: "markdown-slides", Title: "Markdown Slides", SourceName: "markdown-slides.md", Source: []byte("# Deck\n\nIntro slide for layout QA.\n\n## First\n\nThe first section becomes a slide.\n\n## Second\n\nThe second section becomes another slide.\n"), Want: report.KindMarkdown, Layout: report.LayoutOverrideSlides, WantLayout: report.LayoutSlides},
		{Slug: "markdown-article-override", Title: "Markdown Article Override", SourceName: "markdown-article-override.md", Source: []byte("# Forced Article\n\n## Summary\n\nThe article override keeps Markdown in reader mode.\n"), Want: report.KindMarkdown, WantComponent: report.ComponentArticle, WantMode: report.ModeReader, Mode: report.ModeOverrideArticle},
		{Slug: "markdown-csv-precedence", Title: "Markdown CSV Precedence", SourceName: "looks-like-csv.md", Source: []byte("name,score\na,1\nb,2\n"), Want: report.KindMarkdown, WantComponent: report.ComponentArticle, WantMode: report.ModeReader, WantHTML: []string{`class="markdown-body"`}},
		{Slug: "markdown-json-precedence", Title: "Markdown JSON Precedence", SourceName: "looks-like-json.markdown", Source: []byte(`[{"name":"a","score":1}]`), Want: report.KindMarkdown, WantComponent: report.ComponentArticle, WantMode: report.ModeReader, WantHTML: []string{`class="markdown-body"`}},
		{Slug: "markdown-unknown-structure", Title: "Markdown Unknown Structure", SourceName: "notes.mdish", Source: []byte("# Title\n\n```go\nfmt.Println()\n```\n"), Want: report.KindMarkdown, WantComponent: report.ComponentArticle, WantMode: report.ModeReader, WantHTML: []string{`class="markdown-body"`, `class="chroma light"`}},
		{Slug: "markdown-unknown-task-list", Title: "Markdown Unknown Task List", SourceName: "tasks.mdish", Source: []byte("- [x] ship renderer\n- [ ] verify screenshots\n"), Want: report.KindMarkdown, WantComponent: report.ComponentArticle, WantMode: report.ModeReader, WantHTML: []string{`class="markdown-body"`, `type="checkbox"`}},
		{Slug: "json-records", Title: "JSON Records", SourceName: "json-records.json", Source: []byte(`[{"name":"alpha","score":10,"status":"ready"},{"name":"beta","score":2,"status":"review"}]`), Want: report.KindJSONRecords},
		{Slug: "bom-json-records", Title: "BOM JSON Records", SourceName: "bom-records.json", Source: []byte("\ufeff" + `[{"name":"alpha","score":10}]`), Want: report.KindJSONRecords, WantComponent: report.ComponentDataTable, WantHTML: []string{`<dt>Kind</dt><dd>json-records</dd>`, `class="report-table"`}},
		{Slug: "json-record-cards", Title: "JSON Record Cards", SourceName: "json-record-cards.json", Source: []byte(`[{"name":"alpha","owner":"ops","status":"ready"},{"id":42,"score":2,"notes":"needs review"},{"title":"launch","due":"Friday","blocked":false}]`), Want: report.KindJSONRecords, WantComponent: report.ComponentRecordCards},
		{Slug: "ndjson-records", Title: "NDJSON Records", SourceName: "events.ndjson", Source: []byte("{\"name\":\"alpha\",\"score\":10,\"status\":\"ready\"}\n{\"name\":\"beta\",\"score\":2,\"status\":\"review\"}\n"), Want: report.KindJSONRecords, WantComponent: report.ComponentDataTable},
		{Slug: "single-jsonl-record", Title: "Single JSONL Record", SourceName: "single.jsonl", Source: []byte("{\"name\":\"alpha\",\"score\":10}\n"), Want: report.KindJSONRecords, WantComponent: report.ComponentDataTable, WantHTML: []string{`<dt>Kind</dt><dd>json-records</dd>`, `class="report-table"`}},
		{Slug: "single-jsonlines-record", Title: "Single JSONLines Record", SourceName: "single.jsonlines", Source: []byte("{\"name\":\"alpha\",\"score\":10}\n"), Want: report.KindJSONRecords, WantComponent: report.ComponentDataTable, WantHTML: []string{`<dt>Kind</dt><dd>json-records</dd>`, `class="report-table"`}},
		{Slug: "jsonl-record-cards", Title: "JSONL Record Cards", SourceName: "cards.jsonl", Source: []byte("{\"name\":\"alpha\"}\n{\"score\":2}\n"), Want: report.KindJSONRecords, WantComponent: report.ComponentRecordCards, WantHTML: []string{`<dt>Kind</dt><dd>json-records</dd>`, `class="record-cards"`}},
		{Slug: "json-object", Title: "JSON Object", SourceName: "json-object.json", Source: []byte(`{"project":"html","theme":{"mode":"dark","palette":"catppuccin"},"counts":{"reports":14,"components":11}}`), Want: report.KindJSONObject},
		{Slug: "json-scalar-array", Title: "JSON Scalar Array", SourceName: "json-scalar-array.json", Source: []byte(`[1,2,3]`), Want: report.KindJSONObject, WantComponent: report.ComponentRawJSON},
		{Slug: "json-scalar-file", Title: "JSON Scalar File", SourceName: "value.json", Source: []byte(`true`), Want: report.KindJSONObject, WantComponent: report.ComponentRawJSON, WantHTML: []string{`<dt>Kind</dt><dd>json-object</dd>`, `class="json-source"`}},
		{Slug: "empty-json-array", Title: "Empty JSON Array", SourceName: "empty-array.json", Source: []byte(`[]`), Want: report.KindJSONObject, WantComponent: report.ComponentRawJSON, WantHTML: []string{`<dt>Kind</dt><dd>json-object</dd>`, `class="json-source"`, `<strong>0</strong> items`}},
		{Slug: "json-empty-object-array", Title: "JSON Empty Object Array", SourceName: "empty-objects.json", Source: []byte(`[{},{}]`), Want: report.KindJSONObject, WantComponent: report.ComponentRawJSON, WantHTML: []string{`<dt>Kind</dt><dd>json-object</dd>`, `class="json-source"`, `<strong>2</strong> items`}},
		{Slug: "bad-json-source-code", Title: "Malformed JSON Source Code", SourceName: "bad.json", Source: []byte(`[{"a":1}] trailing`), Want: report.KindSourceCode, WantComponent: report.ComponentCodeBlock, WantMode: report.ModeCode, WantHTML: []string{`<dt>Language</dt><dd>JSON</dd>`, `<dt>Renderer</dt><dd>Chroma</dd>`, `class="chroma light"`}},
		{Slug: "bad-jsonl-source-code", Title: "Malformed JSONL Source Code", SourceName: "bad.jsonl", Source: []byte("{\"name\":\"alpha\"}\n2\n"), Want: report.KindSourceCode, WantComponent: report.ComponentCodeBlock, WantMode: report.ModeCode, WantHTML: []string{`<dt>Language</dt><dd>JSON</dd>`, `<dt>Renderer</dt><dd>Chroma</dd>`, `class="chroma light"`}},
		{Slug: "csv-records", Title: "CSV Records", SourceName: "records.csv", Source: []byte("name,score,status\nalpha,10,ready\nbeta,2,review\n"), Want: report.KindCSVRecords},
		{Slug: "bom-csv-records", Title: "BOM CSV Records", SourceName: "bom-records.csv", Source: []byte("\ufeff" + "name,score\nalpha,10\n"), Want: report.KindCSVRecords, WantComponent: report.ComponentDataTable, WantHTML: []string{`<dt>Kind</dt><dd>csv-records</dd>`, `class="report-table"`}},
		{Slug: "bad-csv-source-code", Title: "Malformed CSV Source Code", SourceName: "bad.csv", Source: []byte(" \n\ufeff" + "name,score\nalpha,10\n"), Want: report.KindSourceCode, WantComponent: report.ComponentCodeBlock, WantMode: report.ModeCode, WantHTML: []string{`<dt>Language</dt><dd>CSV</dd>`, `<dt>Renderer</dt><dd>Chroma</dd>`, `class="chroma light"`}},
		{Slug: "csv-header-only", Title: "CSV Header Only", SourceName: "empty.csv", Source: []byte("name,score\n"), Want: report.KindCSVRecords, WantComponent: report.ComponentDataTable, WantHTML: []string{`<dt>Kind</dt><dd>csv-records</dd>`, `class="report-table"`}},
		{Slug: "timestamped-csv-records", Title: "Timestamped CSV Records", SourceName: "events.csv", Source: []byte("time,level,msg\n2026-01-01 12:00:00,ERROR,oops\n2026-01-01 12:01:00,INFO,ok\n"), Want: report.KindCSVRecords, WantComponent: report.ComponentDataTable, WantHTML: []string{`<dt>Kind</dt><dd>csv-records</dd>`, `class="report-table"`}},
		{Slug: "csv-tabs", Title: "CSV Tabs", SourceName: "records-tabs.csv", Source: []byte("name,score,status\nalpha,10,ready\nbeta,2,review\n"), Want: report.KindCSVRecords, Layout: report.LayoutOverrideTabs, WantLayout: report.LayoutTabbedPage},
		{Slug: "csv-cards-override", Title: "CSV Cards Override", SourceName: "records-cards.csv", Source: []byte("name,score,status\nalpha,10,ready\nbeta,2,review\n"), Want: report.KindCSVRecords, WantComponent: report.ComponentRecordCards, WantMode: report.ModeDataBrowser, Mode: report.ModeOverrideCards},
		{Slug: "csv-code-override", Title: "CSV Code Override", SourceName: "records-code.csv", Source: []byte("name,score,status\nalpha,10,ready\nbeta,2,review\n"), Want: report.KindCSVRecords, WantComponent: report.ComponentCodeBlock, WantMode: report.ModeCode, Mode: report.ModeOverrideCode},
		{Slug: "tsv-records", Title: "TSV Records", SourceName: "records.tsv", Source: []byte("name\tscore\tstatus\nalpha\t10\tready\nbeta\t2\treview\n"), Want: report.KindTSVRecords},
		{Slug: "table-records", Title: "ASCII Table Records", SourceName: "mysql.out", Source: []byte("+----+-------+--------+\n| id | name  | state  |\n+----+-------+--------+\n| 1  | alpha | ready  |\n| 2  | beta  | review |\n+----+-------+--------+\n"), Want: report.KindTableRecords},
		{Slug: "psql-table-records", Title: "PSQL Table Records", SourceName: "psql.out", Source: []byte(" id | name\n----+-------\n  1 | alpha\n  2 | beta\n(2 rows)\n"), Want: report.KindTableRecords, WantComponent: report.ComponentDataTable, WantHTML: []string{`<dt>Kind</dt><dd>table-records</dd>`, `class="report-table"`}},
		{Slug: "diff", Title: "Unified Diff", SourceName: "change.patch", Source: []byte("diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1,2 +1,2 @@\n-old line\n+new line\n context\n"), Want: report.KindDiff},
		{Slug: "plain-diff-headers", Title: "Plain Diff Headers", SourceName: "headers.diff", Source: []byte("--- old.txt\n+++ new.txt\n@@ -1,2 +1,2 @@\n-old\n+++ added heading\n"), Want: report.KindDiff, WantComponent: report.ComponentDiffView, WantHTML: []string{`<dt>Kind</dt><dd>diff</dd>`, `class="diff-view"`}},
		{Slug: "plain-diff-multi-file", Title: "Plain Diff Multi File", SourceName: "multi.diff", Source: []byte("--- a.txt\n+++ a.txt\n@@ -1 +1 @@\n-old\n+new\n--- b.txt\n+++ b.txt\n@@ -1 +1 @@\n-left\n+right\n"), Want: report.KindDiff, WantComponent: report.ComponentDiffView, WantHTML: []string{`<dt>Kind</dt><dd>diff</dd>`, `class="diff-view"`}},
		{Slug: "combined-diff", Title: "Combined Diff", SourceName: "combined.diff", Source: []byte("diff --cc main.go\nindex 1111111,2222222..3333333\n--- a/main.go\n+++ b/main.go\n@@@ -1,1 -1,1 +1,1 @@@\n- left\n -right\n++merged\n"), Want: report.KindDiff, WantComponent: report.ComponentDiffView, WantHTML: []string{`<dt>Kind</dt><dd>diff</dd>`, `class="diff-view"`}},
		{Slug: "git-binary-patch", Title: "Git Binary Patch", SourceName: "logo.patch", Source: []byte("diff --git a/logo.png b/logo.png\nnew file mode 100644\nindex 0000000..1111111\nGIT binary patch\nliteral 0\nHcmV?d00001\n"), Want: report.KindDiff, WantComponent: report.ComponentDiffView, WantHTML: []string{`<dt>Kind</dt><dd>diff</dd>`, `class="diff-view"`}},
		{Slug: "git-mode-only-patch", Title: "Git Mode-only Patch", SourceName: "mode.patch", Source: []byte("diff --git a/script.sh b/script.sh\nold mode 100644\nnew mode 100755\n"), Want: report.KindDiff, WantComponent: report.ComponentDiffView, WantHTML: []string{`<dt>Kind</dt><dd>diff</dd>`, `class="diff-view"`}},
		{Slug: "git-copy-only-patch", Title: "Git Copy-only Patch", SourceName: "copy.patch", Source: []byte("diff --git a/source.txt b/copy.txt\ncopy from source.txt\ncopy to copy.txt\n"), Want: report.KindDiff, WantComponent: report.ComponentDiffView, WantHTML: []string{`<dt>Kind</dt><dd>diff</dd>`, `class="diff-view"`}},
		{Slug: "diff-override", Title: "Diff Override", SourceName: "change-override.patch", Source: []byte("diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new\n"), Want: report.KindDiff, WantComponent: report.ComponentDiffView, WantMode: report.ModeReview, Mode: report.ModeOverrideDiff},
		{Slug: "source-code", Title: "Source Code", SourceName: "sample.go", Source: []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"html\")\n}\n"), Want: report.KindSourceCode},
		{Slug: "yaml-source-code", Title: "YAML Source Code", SourceName: "config.yaml", Source: []byte("name: html\nitems:\n  - render\n  - verify\nmetadata:\n  theme: blue\n"), Want: report.KindSourceCode, WantComponent: report.ComponentCodeBlock, WantMode: report.ModeCode, WantHTML: []string{`<dt>Language</dt><dd>YAML</dd>`, `<dt>Renderer</dt><dd>Chroma</dd>`, `class="chroma light"`}},
		{Slug: "shell-content-source", Title: "Shell Content Source", SourceName: "script", Source: []byte("#!/bin/sh\nset -eu\necho html\n"), Want: report.KindSourceCode, WantComponent: report.ComponentCodeBlock, WantMode: report.ModeCode, WantHTML: []string{`<dt>Renderer</dt><dd>Chroma</dd>`, `class="chroma light"`}},
		{Slug: "go-source-precedence", Title: "Go Source Precedence", SourceName: "looks-like-json.go", Source: []byte("[{\"name\":\"alpha\",\"score\":10}]\n"), Want: report.KindSourceCode, WantComponent: report.ComponentCodeBlock, WantMode: report.ModeCode, WantHTML: []string{`<dt>Language</dt><dd>Go</dd>`, `<dt>Source</dt><dd>looks-like-json.go</dd>`}},
		{Slug: "go-source-fence-string", Title: "Go Source Fence String", SourceName: "fence-string.go", Source: []byte("package main\n\nvar doc = `\n```go\nfmt.Println()\n```\n`\n"), Want: report.KindSourceCode, WantComponent: report.ComponentCodeBlock, WantMode: report.ModeCode, WantHTML: []string{`<dt>Language</dt><dd>Go</dd>`, `<dt>Source</dt><dd>fence-string.go</dd>`, `class="chroma light"`}},
		{Slug: "go-source-csv-precedence", Title: "Go Source CSV Precedence", SourceName: "looks-like-csv.go", Source: []byte("name,score\na,1\nb,2\n"), Want: report.KindSourceCode, WantComponent: report.ComponentCodeBlock, WantMode: report.ModeCode, WantHTML: []string{`<dt>Language</dt><dd>Go</dd>`, `<dt>Source</dt><dd>looks-like-csv.go</dd>`, `class="chroma light"`}},
		{Slug: "tree-listing", Title: "Tree Listing", SourceName: "tree.txt", Source: []byte(".\n├── cmd\n│   └── html\n├── internal\n│   ├── render\n│   └── report\n└── README.md\n"), Want: report.KindTreeListing},
		{Slug: "ascii-tree-listing", Title: "ASCII Tree Listing", SourceName: "ascii-tree.txt", Source: []byte(".\n|-- cmd\n|   `-- html\n`-- internal\n"), Want: report.KindTreeListing, WantComponent: report.ComponentFileTree, WantHTML: []string{`<dt>Kind</dt><dd>tree-listing</dd>`, `class="file-tree"`}},
		{Slug: "tree-summary-listing", Title: "Tree Summary Listing", SourceName: "summary-tree.txt", Source: []byte(".\n├── cmd\n│   └── html\n└── internal\n\n2 directories, 1 file\n"), Want: report.KindTreeListing, WantComponent: report.ComponentFileTree, WantHTML: []string{`<dt>Kind</dt><dd>tree-listing</dd>`, `class="file-tree"`}},
		{Slug: "posix-path-listing", Title: "POSIX Path Listing", SourceName: "list.weird", Source: []byte("./cmd/html\n./internal/render\n./internal/report\n"), Want: report.KindTreeListing, WantComponent: report.ComponentFileTree, WantHTML: []string{`<dt>Kind</dt><dd>tree-listing</dd>`, `class="file-tree"`}},
		{Slug: "absolute-path-listing", Title: "Absolute Path Listing", SourceName: "absolute.weird", Source: []byte("/usr/bin\n/usr/local/bin\n/var/log/html\n"), Want: report.KindTreeListing, WantComponent: report.ComponentFileTree, WantHTML: []string{`<dt>Kind</dt><dd>tree-listing</dd>`, `class="file-tree"`}},
		{Slug: "spaced-path-listing", Title: "Spaced Path Listing", SourceName: "spaces.weird", Source: []byte("./My Project/README.md\n./My Project/docs/Release Notes.md\n./Other Project/TODO.txt\n"), Want: report.KindTreeListing, WantComponent: report.ComponentFileTree, WantHTML: []string{`<dt>Kind</dt><dd>tree-listing</dd>`, `class="file-tree"`}},
		{Slug: "windows-path-listing", Title: "Windows Path Listing", SourceName: "list.weird", Source: []byte("C:\\Users\\Alice\nC:\\Users\\Alice\\Documents\nD:\\Archive\\Logs\n"), Want: report.KindTreeListing, WantComponent: report.ComponentFileTree, WantHTML: []string{`<dt>Kind</dt><dd>tree-listing</dd>`, `class="file-tree"`}},
		{Slug: "url-list-plain", Title: "URL List Plain", SourceName: "urls.txt", Source: []byte("https://example.com/a\nhttps://example.com/b\nhttps://example.com/c\n"), Want: report.KindPlain, WantComponent: report.ComponentPreformatted, WantHTML: []string{`<dt>Kind</dt><dd>plain</dd>`, `class="report-text"`}},
		{Slug: "fractions-plain", Title: "Fractions Plain", SourceName: "fractions.txt", Source: []byte("1/2 complete\n2/3 done\n3/4 ready\n"), Want: report.KindPlain, WantComponent: report.ComponentPreformatted, WantHTML: []string{`<dt>Kind</dt><dd>plain</dd>`, `class="report-text"`}},
		{Slug: "http-request-paths-plain", Title: "HTTP Request Paths Plain", SourceName: "requests.txt", Source: []byte("GET /api/users\nPOST /api/tasks\nDELETE /api/tasks/1\n"), Want: report.KindPlain, WantComponent: report.ComponentPreformatted, WantHTML: []string{`<dt>Kind</dt><dd>plain</dd>`, `class="report-text"`}},
		{Slug: "ordinary-ok-plain", Title: "Ordinary OK Plain", SourceName: "ok.txt", Source: []byte("ok thanks\nok sure\n"), Want: report.KindPlain, WantComponent: report.ComponentPreformatted, WantHTML: []string{`<dt>Kind</dt><dd>plain</dd>`, `class="report-text"`}},
		{Slug: "config-keys-plain", Title: "Config Keys Plain", SourceName: "config.txt", Source: []byte("Host: localhost\nPort: 8080\nMode: production\n"), Want: report.KindPlain, WantComponent: report.ComponentPreformatted, WantHTML: []string{`<dt>Kind</dt><dd>plain</dd>`, `class="report-text"`}},
		{Slug: "dash-divider-plain", Title: "Dash Divider Plain", SourceName: "results.txt", Source: []byte("Results\n--------------------\nrow 1\nrow 2\n"), Want: report.KindPlain, WantComponent: report.ComponentPreformatted, WantHTML: []string{`<dt>Kind</dt><dd>plain</dd>`, `class="report-text"`}},
		{Slug: "tree-override", Title: "Tree Override", SourceName: "tree-override.txt", Source: []byte(".\n├── cmd\n│   └── html\n└── internal\n"), Want: report.KindTreeListing, WantComponent: report.ComponentFileTree, WantMode: report.ModeCode, Mode: report.ModeOverrideTree},
		{Slug: "log", Title: "HTTP Access Log", SourceName: "access.log", Source: []byte("127.0.0.1 - - [16/Jun/2026:12:00:00 -0400] \"GET /index.html HTTP/1.1\" 200 1234\n127.0.0.1 - - [16/Jun/2026:12:00:01 -0400] \"GET /missing HTTP/1.1\" 404 123\n127.0.0.1 - - [16/Jun/2026:12:00:02 -0400] \"POST /api HTTP/1.1\" 500 42\n"), Want: report.KindLog},
		{Slug: "go-test-log", Title: "Go Test Log", SourceName: "go-test.out", Source: []byte("ok\tgithub.com/dotcommander/html/internal/cache\t0.012s\nok\tgithub.com/dotcommander/html/internal/render\t0.034s\n"), Want: report.KindLog, WantComponent: report.ComponentPreformatted, WantHTML: []string{`<dt>Kind</dt><dd>log</dd>`, `class="log-lines"`}},
		{Slug: "single-severity-log", Title: "Single Severity Log", SourceName: "app.log", Source: []byte("2026-06-16 12:00:00 ERROR stop\n"), Want: report.KindLog, WantComponent: report.ComponentPreformatted, WantHTML: []string{`<dt>Kind</dt><dd>log</dd>`, `class="log-lines"`}},
		{Slug: "single-go-test-log", Title: "Single Go Test Log", SourceName: "go-test-single.out", Source: []byte("ok\tgithub.com/dotcommander/html/internal/cache\t0.012s\n"), Want: report.KindLog, WantComponent: report.ComponentPreformatted, WantHTML: []string{`<dt>Kind</dt><dd>log</dd>`, `class="log-lines"`}},
		{Slug: "plain-log-override", Title: "Plain Log Override", SourceName: "plain-log-override.txt", Source: []byte("starting\ncontinuing\nfinished\n"), Want: report.KindPlain, WantComponent: report.ComponentPreformatted, WantMode: report.ModeConsole, Mode: report.ModeOverrideLog},
		{Slug: "transcript", Title: "Transcript", SourceName: "transcript.txt", Source: []byte("Host: Welcome back.\nGuest: Thanks for having me.\nHost: Let's inspect every generated report kind.\n"), Want: report.KindTranscript},
		{Slug: "generic-speaker-transcript", Title: "Generic Speaker Transcript", SourceName: "speakers.txt", Source: []byte("Speaker 1: We should verify the HTML output.\nSpeaker 2: I have the mobile screenshots.\nSpeaker 1: The controls no longer overlap.\n"), Want: report.KindTranscript, WantComponent: report.ComponentPreformatted, WantHTML: []string{`<dt>Kind</dt><dd>transcript</dd>`, `class="transcript-turns"`}},
		{Slug: "mixed", Title: "Mixed Input", SourceName: "mixed.txt", Source: []byte("Notes\n- check deploy\n\nPayload\n{\"ok\":true}\n\nERROR failed\n"), Want: report.KindMixed},
		{Slug: "mixed-single-override", Title: "Mixed Single Layout Override", SourceName: "mixed-single.txt", Source: []byte("Notes\n- check deploy\n\nPayload\n{\"ok\":true}\n\nERROR failed\n"), Want: report.KindMixed, Layout: report.LayoutOverrideSingle, WantLayout: report.LayoutSinglePage},
		{Slug: "single-comma-plain", Title: "Single Comma Plain", SourceName: "single-comma.txt", Source: []byte("hello, world\n"), Want: report.KindPlain, WantComponent: report.ComponentPreformatted, WantHTML: []string{`<dt>Kind</dt><dd>plain</dd>`, `class="report-text"`}},
		{Slug: "plain", Title: "Plain Text", SourceName: "plain.txt", Source: []byte("ordinary prose with no structural markup\nsecond line of plain text\n"), Want: report.KindPlain},
		{Slug: "yaml-plain", Title: "YAML-like Plain Text", SourceName: "config-text", Source: []byte("name: html\nitems:\n  - render\n  - verify\nmetadata:\n  theme: blue\n"), Want: report.KindPlain, WantComponent: report.ComponentPreformatted},
		{Slug: "binary", Title: "Binary Preview", SourceName: "logo.bin", Source: []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0xff, 'h', 't', 'm', 'l'}, Want: report.KindBinary},
	}
}

func renderMatrixIndex(results []matrixRendered) string {
	var cards strings.Builder
	for _, r := range results {
		fmt.Fprintf(&cards, `<section class="kind-card" data-kind="%s">`, html.EscapeString(string(r.Analysis.Kind)))
		fmt.Fprintf(&cards, `<header><div><h2>%s</h2><p>%s</p></div><a href="%s">Open</a></header>`, html.EscapeString(r.Sample.Title), html.EscapeString(r.SourceFile), html.EscapeString(r.HTMLFile))
		fmt.Fprintf(&cards, `<dl class="meta"><div><dt>Kind</dt><dd>%s</dd></div><div><dt>Mode</dt><dd>%s</dd></div><div><dt>Layout</dt><dd>%s</dd></div>`, html.EscapeString(string(r.Analysis.Kind)), html.EscapeString(string(r.Plan.Mode)), html.EscapeString(string(r.Plan.Layout)))
		if request := requestList(r.Sample); request != "" {
			fmt.Fprintf(&cards, `<div><dt>Request</dt><dd>%s</dd></div>`, html.EscapeString(request))
		}
		fmt.Fprintf(&cards, `<div><dt>Confidence</dt><dd>%.2f</dd></div><div><dt>Components</dt><dd>%s</dd></div></dl>`, r.Analysis.Confidence, html.EscapeString(componentList(r.Plan.Components)))
		fmt.Fprintf(&cards, `<p class="reason">%s</p>`, html.EscapeString(strings.Join(r.Analysis.Reasons, "; ")))
		if len(r.Sample.MediaPreview) > 0 {
			cards.WriteString(`<div class="media-showcase" aria-label="Image and SVG rendering preview"><div class="media-showcase-head"><strong>Image And SVG Display</strong><span>Source assets beside generated report output; the report embeds the same PNG and SVG as data URIs.</span></div><div class="media-comparison">`)
			cards.WriteString(renderMediaPreviewColumn("Source Assets", "source", r.Sample.MediaPreview))
			cards.WriteString(renderMediaPreviewColumn("Rendered Data URI Output", "rendered", r.RenderedMediaImage))
			cards.WriteString(`</div></div>`)
		}
		fmt.Fprintf(&cards, `<iframe title="%s" src="%s" loading="eager" onload="this.dataset.loaded='true'"></iframe>`, html.EscapeString(r.Sample.Title), html.EscapeString(r.HTMLFile))
		cards.WriteString(`</section>`)
	}
	return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>HTML Report Kind Matrix</title>
  <style>
    :root { --bg:#f5f4f1; --paper:#fffefa; --text:#25221f; --muted:#6d6760; --border:#ded8cc; --accent:#8b5b2f; --panel:#f1ece3; color-scheme:light; }
    * { box-sizing: border-box; }
    body { margin:0; color:var(--text); background:var(--bg); font-family:ui-sans-serif,-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif; }
    main { width:min(calc(100% - 2rem),92rem); margin:1.4rem auto 2rem; }
    h1 { margin:0; font-size:1.65rem; line-height:1.15; }
    .matrix-header { display:grid; gap:.35rem; margin:0 0 1rem; }
    .matrix-header p { max-width:62rem; margin:0; color:var(--muted); line-height:1.45; }
    .coverage-panel,.media-overview,.kind-card { background:var(--paper); border:1px solid var(--border); border-radius:8px; box-shadow:0 .9rem 2.4rem rgba(35,30,24,.07); }
    .coverage-panel,.media-overview { display:grid; gap:.75rem; margin:0 0 1rem; padding:.9rem; }
    .coverage-panel h2,.media-overview h2,.coverage-group h3,h2 { margin:0; }
    .coverage-panel h2,.media-overview h2 { font-size:1rem; }
    .coverage-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,18rem),1fr)); gap:.65rem; }
    .coverage-group { min-width:0; padding:.7rem; background:var(--panel); border:1px solid var(--border); border-radius:8px; }
    .coverage-group h3 { margin:0 0 .45rem; font-size:.85rem; line-height:1.2; }
    .coverage-list { display:flex; flex-wrap:wrap; gap:.35rem; margin:0; padding:0; list-style:none; }
    .coverage-list li { min-height:1.45rem; display:inline-flex; align-items:center; gap:.25rem; padding:.12rem .42rem; background:var(--paper); border:1px solid var(--border); border-radius:999px; color:var(--muted); font-size:.74rem; font-weight:700; line-height:1.2; }
    .coverage-list li::before { content:""; width:.42rem; height:.42rem; border-radius:999px; background:#2d7a46; }
    .coverage-list li.missing::before { background:#be4b75; }
    .media-overview-head { display:flex; align-items:baseline; justify-content:space-between; gap:.75rem; }
    .media-overview-head p { margin:0; max-width:42rem; color:var(--muted); font-size:.82rem; line-height:1.4; text-align:right; }
    .media-overview-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:.75rem; align-items:stretch; }
    .media-overview-column { min-width:0; display:grid; gap:.5rem; align-content:start; }
    .media-overview-column h3 { margin:0; font-size:.84rem; line-height:1.2; color:var(--muted); }
    .media-overview-preview { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:.65rem; }
    .media-overview figure { min-width:0; margin:0; overflow:hidden; background:var(--panel); border:1px solid var(--border); border-radius:8px; }
    .media-overview img { display:block; width:100%; aspect-ratio:21/10; object-fit:contain; background:var(--paper); border-bottom:1px solid var(--border); }
    .media-overview figcaption { display:grid; gap:.12rem; padding:.5rem .55rem; color:var(--muted); font-size:.76rem; line-height:1.2; }
    .media-overview figcaption span { color:var(--accent); font-size:.68rem; font-weight:800; text-transform:uppercase; }
    .media-overview figcaption strong { color:var(--text); font-size:.83rem; }
    .media-overview figcaption em { font-style:normal; }
    .media-overview iframe { grid-column:1 / -1; height:26rem; border:1px solid var(--border); border-radius:8px; }
    .matrix-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,34rem),1fr)); gap:1rem; }
    .kind-card { min-width:0; overflow:hidden; }
    .kind-card header { display:flex; align-items:start; justify-content:space-between; gap:.75rem; padding:.85rem .95rem; border-bottom:1px solid var(--border); background:var(--panel); }
    h2 { font-size:1rem; line-height:1.2; }
    header p { margin:.18rem 0 0; color:var(--muted); font-size:.82rem; }
    header a { flex:0 0 auto; min-height:1.9rem; display:inline-flex; align-items:center; padding:0 .65rem; color:var(--accent); text-decoration:none; background:var(--paper); border:1px solid var(--border); border-radius:6px; font-size:.82rem; font-weight:700; }
    .meta { display:flex; flex-wrap:wrap; gap:.4rem; margin:0; padding:.7rem .95rem 0; font-size:.78rem; }
    .meta div { display:inline-flex; align-items:center; gap:.3rem; min-height:1.55rem; padding:.18rem .45rem; background:var(--panel); border:1px solid var(--border); border-radius:999px; }
    .meta dt,.meta dd { margin:0; }
    .meta dt { font-weight:800; }
    .meta dd,.reason { color:var(--muted); }
    .reason { margin:0; padding:.45rem .95rem .7rem; font-size:.78rem; line-height:1.35; }
    .media-showcase { display:grid; gap:.65rem; margin:0 .95rem .85rem; padding:.75rem; background:var(--panel); border:1px solid var(--border); border-radius:8px; }
    .media-showcase-head { display:flex; align-items:baseline; justify-content:space-between; gap:.75rem; }
    .media-showcase-head span { max-width:28rem; color:var(--muted); font-size:.74rem; line-height:1.3; text-align:right; }
    .media-comparison { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:.65rem; }
    .media-comparison-column { min-width:0; display:grid; gap:.45rem; }
    .media-comparison-column h3 { margin:0; color:var(--muted); font-size:.78rem; line-height:1.2; }
    .media-preview { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:.65rem; }
    .media-preview figure { min-width:0; margin:0; overflow:hidden; background:var(--panel); border:1px solid var(--border); border-radius:8px; }
    .media-preview img { display:block; width:100%; aspect-ratio:16/9; object-fit:contain; background:var(--paper); border-bottom:1px solid var(--border); }
    .media-preview figcaption { display:grid; gap:.12rem; padding:.5rem .55rem; color:var(--muted); font-size:.76rem; line-height:1.2; }
    .media-preview figcaption span { color:var(--accent); font-size:.68rem; font-weight:800; text-transform:uppercase; }
    .media-preview figcaption strong { color:var(--text); font-size:.83rem; }
    .media-preview figcaption em { font-style:normal; }
    iframe { display:block; width:100%; height:34rem; border:0; border-top:1px solid var(--border); background:white; }
    @media (max-width:45rem) { main { width:100%; margin:0; } .matrix-header { padding:1rem; margin:0; } .coverage-panel,.media-overview,.kind-card { border-left:0; border-right:0; border-radius:0; } .coverage-panel,.media-overview { margin:0 0 .75rem; } .matrix-grid,.media-overview-grid,.media-comparison { grid-template-columns:1fr; gap:.75rem; } iframe { height:30rem; } .media-overview iframe { height:28rem; border-left:0; border-right:0; border-radius:0; } .media-overview-head,.media-showcase-head { display:grid; } .media-overview-head p,.media-showcase-head span { max-width:none; text-align:left; } .media-preview,.media-overview-preview { grid-template-columns:1fr; } }
  </style>
</head>
<body>
  <main>
    <header class="matrix-header">
      <h1>HTML Report Kind Matrix</h1>
      <p>Generated browser QA target covering every deterministic report kind, component-focused samples, detection-edge samples, image and SVG rendering, and explicit output-choice overrides for article, cards, code, diff, tree, log, slides, tabs, and forced single layouts. Each card embeds the actual rendered report output for its fixture and records the detected kind, mode, layout, confidence, components, and detection reasons.</p>
    </header>
    ` + renderMediaOverview(results) + `
    ` + renderCoverage(results) + `
    <div class="matrix-grid">` + cards.String() + `</div>
  </main>
</body>
</html>
`
}

func renderMediaOverview(results []matrixRendered) string {
	for _, r := range results {
		if len(r.Sample.MediaPreview) == 0 {
			continue
		}
		return `<section class="media-overview" aria-label="Rendered image and SVG examples">
      <div class="media-overview-head">
        <h2>Image And SVG Rendering</h2>
        <p>Shows source PNG/SVG assets, then the generated report versions extracted from the rendered HTML as data URIs.</p>
      </div>
      <div class="media-overview-grid">
        ` + renderMediaPreviewColumn("Source Assets", "source", r.Sample.MediaPreview) + `
        ` + renderMediaPreviewColumn("Rendered Data URI Output", "rendered", r.RenderedMediaImage) + `
        <iframe title="Rendered Markdown media report" src="` + html.EscapeString(r.HTMLFile) + `" loading="eager" onload="this.dataset.loaded='true'"></iframe>
      </div>
    </section>`
	}
	return ""
}

func renderMediaPreviewColumn(title, group string, previews []mediaPreview) string {
	var out strings.Builder
	fmt.Fprintf(&out, `<section class="media-overview-column media-comparison-column" data-media-preview="%s"><h3>%s</h3><div class="media-overview-preview media-preview">`, html.EscapeString(group), html.EscapeString(title))
	for _, media := range previews {
		fmt.Fprintf(&out, `<figure><img src="%s" alt="%s preview"><figcaption><span>%s</span><strong>%s</strong><em>%s</em></figcaption></figure>`, html.EscapeString(media.Src), html.EscapeString(media.Label), html.EscapeString(media.Kind), html.EscapeString(media.Label), html.EscapeString(media.Note))
	}
	out.WriteString(`</div></section>`)
	return out.String()
}

func renderCoverage(results []matrixRendered) string {
	kinds := map[report.Kind]bool{}
	components := map[report.ComponentType]bool{}
	modes := map[report.Mode]bool{}
	layouts := map[report.Layout]bool{}
	for _, r := range results {
		kinds[r.Analysis.Kind] = true
		modes[r.Plan.Mode] = true
		layouts[r.Plan.Layout] = true
		for _, c := range r.Plan.Components {
			components[c.Type] = true
		}
	}
	kindValues := []string{string(report.KindMarkdown), string(report.KindJSONRecords), string(report.KindJSONObject), string(report.KindCSVRecords), string(report.KindTSVRecords), string(report.KindTableRecords), string(report.KindDiff), string(report.KindSourceCode), string(report.KindTreeListing), string(report.KindLog), string(report.KindTranscript), string(report.KindMixed), string(report.KindPlain), string(report.KindBinary)}
	componentValues := []string{string(report.ComponentArticle), string(report.ComponentPreformatted), string(report.ComponentCodeBlock), string(report.ComponentDataTable), string(report.ComponentRecordCards), string(report.ComponentDiffView), string(report.ComponentFileTree), string(report.ComponentSummary), string(report.ComponentRawJSON)}
	modeValues := []string{string(report.ModeReader), string(report.ModeDataBrowser), string(report.ModeReview), string(report.ModeConsole), string(report.ModeCode), string(report.ModeBrief)}
	layoutValues := []string{string(report.LayoutSinglePage), string(report.LayoutTabbedPage), string(report.LayoutSlides)}
	requireCoverage("kind", kindValues, func(v string) bool { return kinds[report.Kind(v)] })
	requireCoverage("component", componentValues, func(v string) bool { return components[report.ComponentType(v)] })
	requireCoverage("mode", modeValues, func(v string) bool { return modes[report.Mode(v)] })
	requireCoverage("layout", layoutValues, func(v string) bool { return layouts[report.Layout(v)] })

	var b strings.Builder
	b.WriteString(`<section class="coverage-panel" aria-label="Coverage summary"><h2>Coverage Summary</h2><div class="coverage-grid">`)
	b.WriteString(coverageGroup("Kinds", kindValues, func(v string) bool { return kinds[report.Kind(v)] }))
	b.WriteString(coverageGroup("Components", componentValues, func(v string) bool { return components[report.ComponentType(v)] }))
	b.WriteString(coverageGroup("Modes", modeValues, func(v string) bool { return modes[report.Mode(v)] }))
	b.WriteString(coverageGroup("Layouts", layoutValues, func(v string) bool { return layouts[report.Layout(v)] }))
	b.WriteString(`</div></section>`)
	return b.String()
}

func requireCoverage(name string, values []string, covered func(string) bool) {
	var missing []string
	for _, value := range values {
		if !covered(value) {
			missing = append(missing, value)
		}
	}
	require(len(missing) == 0, "kind matrix missing %s coverage: %s", name, strings.Join(missing, ", "))
}

func coverageGroup(title string, values []string, covered func(string) bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<section class="coverage-group"><h3>%s</h3><ul class="coverage-list">`, html.EscapeString(title))
	for _, value := range values {
		class := ""
		label := value
		if !covered(value) {
			class = ` class="missing"`
			label = value + " missing"
		}
		fmt.Fprintf(&b, `<li%s>%s</li>`, class, html.EscapeString(label))
	}
	b.WriteString(`</ul></section>`)
	return b.String()
}

func componentList(components []report.Component) string {
	names := make([]string, 0, len(components))
	for _, c := range components {
		names = append(names, string(c.Type))
	}
	return strings.Join(names, ", ")
}

func requestList(s matrixSample) string {
	var parts []string
	if s.Mode != "" {
		parts = append(parts, "mode="+string(s.Mode))
	}
	if s.Layout != "" {
		parts = append(parts, "layout="+string(s.Layout))
	}
	return strings.Join(parts, ", ")
}

func hasComponent(components []report.Component, want report.ComponentType) bool {
	for _, c := range components {
		if c.Type == want {
			return true
		}
	}
	return false
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
  <desc id="desc">A green, blue, and rose vector badge for the HTML QA media sample.</desc>
  <rect width="420" height="180" rx="18" fill="#fffefa"/>
  <rect x="18" y="18" width="384" height="144" rx="14" fill="#f1ece3" stroke="#ded8cc" stroke-width="2"/>
  <circle cx="96" cy="90" r="42" fill="#2d7a46"/>
  <rect x="158" y="54" width="94" height="72" rx="12" fill="#2563eb"/>
  <path d="M305 50 L354 132 L256 132 Z" fill="#be4b75"/>
  <text x="210" y="154" text-anchor="middle" font-family="Arial, sans-serif" font-size="18" font-weight="700" fill="#25221f">SVG vector image</text>
</svg>
`
	return os.WriteFile(filepath.Join(mediaDir, "vector.svg"), []byte(svg), 0o644)
}

func renderedMediaImages(page string) []mediaPreview {
	var previews []mediaPreview
	add := func(mime, label string) {
		needle := `src="data:` + mime + `;base64,`
		start := strings.Index(page, needle)
		if start < 0 {
			return
		}
		srcStart := start + len(`src="`)
		srcEnd := strings.Index(page[srcStart:], `"`)
		if srcEnd < 0 {
			return
		}
		encoded := page[srcStart+len("data:"+mime+";base64,") : srcStart+srcEnd]
		if _, err := base64.StdEncoding.DecodeString(encoded); err != nil {
			return
		}
		previews = append(previews, mediaPreview{
			Label: label,
			Src:   page[srcStart : srcStart+srcEnd],
			Kind:  "Inlined " + mime,
			Note:  "Extracted from generated report HTML",
		})
	}
	add("image/png", "Rendered PNG")
	add("image/svg+xml", "Rendered SVG")
	return previews
}

func writeRaster(path string) error {
	const width, height = 420, 180
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	bg := color.RGBA{0xff, 0xfe, 0xfa, 0xff}
	panel := color.RGBA{0xf1, 0xec, 0xe3, 0xff}
	blue := color.RGBA{0x25, 0x63, 0xeb, 0xff}
	green := color.RGBA{0x2d, 0x7a, 0x46, 0xff}
	rose := color.RGBA{0xbe, 0x4b, 0x75, 0xff}
	for y := range height {
		for x := range width {
			c := bg
			if x >= 18 && x < width-18 && y >= 18 && y < height-18 {
				c = panel
			}
			switch {
			case x >= 44 && x < 146 && y >= 52 && y < 128:
				c = blue
			case x >= 160 && x < 262 && y >= 52 && y < 128:
				c = green
			case x >= 276 && x < 378 && y >= 52 && y < 128:
				c = rose
			}
			img.SetRGBA(x, y, c)
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
