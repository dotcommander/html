package render

import (
	"strings"
	"testing"

	"github.com/dotcommander/html/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func labeledCell(label, value string) string {
	return `<td data-label="` + escapeTableText(label) + `">` + escapeTableText(value) + `</td>`
}

func cardField(label, value string) string {
	return `<div><dt>` + escapeTableText(label) + `</dt><dd>` + escapeTableText(value) + `</dd></div>`
}

func TestRenderReport_TabInitialFocusState(t *testing.T) {
	t.Parallel()

	src := []byte("name,score\nAlpha,10\nBeta,2\n")
	analysis := report.Analyze(src, "scores.csv")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindCSVRecords,
		Layout:  report.LayoutTabbedPage,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}},
			{Type: report.ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "scores"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `role="tablist"`)
	assert.Contains(t, got, `id="report-tab-0" type="button" role="tab" aria-selected="true" aria-controls="report-panel-0" tabindex="0">Summary</button>`)
	assert.Contains(t, got, `id="report-tab-1" type="button" role="tab" aria-selected="false" aria-controls="report-panel-1" tabindex="-1">Records</button>`)
	assert.Contains(t, got, `<section id="report-panel-0" class="report-tab-panel" role="tabpanel" aria-labelledby="report-tab-0">`)
	assert.Contains(t, got, `<section id="report-panel-1" class="report-tab-panel" role="tabpanel" aria-labelledby="report-tab-1" hidden>`)
	assert.Contains(t, got, `<p class="report-filter-status" aria-live="polite">2 rows</p>`)
	assert.Equal(t, 1, strings.Count(got, `tabindex="0"`), "only the selected tab should be in the tab order")
}

func TestRenderReport_TabbedLayoutWithSingleComponent(t *testing.T) {
	t.Parallel()

	src := []byte("package main\n\nfunc main() {}\n")
	analysis := report.Analyze(src, "main.go")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindSourceCode,
		Layout:  report.LayoutTabbedPage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentCodeBlock, Source: "input", Title: "Code", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "main", SourceName: "main.go"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `class="report-tabs"`)
	assert.Contains(t, got, `role="tablist"`)
	assert.Contains(t, got, `aria-selected="true"`)
	assert.Contains(t, got, `<section id="report-panel-0" class="report-tab-panel" role="tabpanel" aria-labelledby="report-tab-0">`)
}

func TestRenderReport_SlidesLayout(t *testing.T) {
	t.Parallel()

	src := []byte("name,score\nAlpha,10\nBeta,2\n")
	analysis := report.Analyze(src, "scores.csv")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindCSVRecords,
		Layout:  report.LayoutSlides,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}},
			{Type: report.ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "scores"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `class="report-slides" data-report-slides`)
	assert.Contains(t, got, `class="report-slide" aria-label="Slide 1 of 2: Summary"`)
	assert.Contains(t, got, `class="report-slide" aria-label="Slide 2 of 2: Records"`)
	assert.Contains(t, got, `<div class="report-slide-count">1 / 2</div>`)
	assert.Contains(t, got, `<div class="report-slide-count">2 / 2</div>`)
	assert.Contains(t, got, `.report-slide`)
	assert.NotContains(t, got, `role="tablist"`)
}

func TestRenderReport_FilterStatusSingular(t *testing.T) {
	t.Parallel()

	src := []byte("name,score\nSolo,1\n")
	analysis := report.Analyze(src, "scores.csv")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindCSVRecords,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "scores"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<p class="report-filter-status" aria-live="polite">1 row</p>`)
	assert.NotContains(t, got, `>1 rows<`)
}

func TestRenderReport_PreformattedPreservesANSI(t *testing.T) {
	t.Parallel()

	src := []byte("2026-06-16 12:00:00 \x1b[31mERROR\x1b[0m fail\n2026-06-16 12:00:01 INFO ok\n")
	analysis := report.Analyze(src, "log.txt")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindLog,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeConsole,
		Components: []report.Component{
			{Type: report.ComponentPreformatted, Source: "input", Title: "Log", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "log"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `class="language-ansi"`)
	assert.Contains(t, got, `style="color:#aa0000"`)
	assert.Contains(t, got, `ERROR`)
	assert.NotContains(t, got, "\x1b[31m")
}

func TestRenderReport_CodeBlockPreservesANSI(t *testing.T) {
	t.Parallel()

	src := []byte("\x1b[31mpackage main\x1b[0m\nfunc main() {}\n")
	analysis := report.Analyze(src, "main.go")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindSourceCode,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentCodeBlock, Source: "input", Title: "Code", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "main", SourceName: "main.go"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `class="language-ansi"`)
	assert.Contains(t, got, `style="color:#aa0000"`)
	assert.Contains(t, got, `package main`)
	assert.NotContains(t, got, "\x1b[31m")
}

func TestRenderReport_ArticleForcesMarkdownRendering(t *testing.T) {
	t.Parallel()

	src := []byte("# Title\n\nBody\n")
	analysis := report.Analyze(src, "doc.md")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    analysis.Kind,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeReader,
		Components: []report.Component{
			{Type: report.ComponentArticle, Source: "input", Title: "Article", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "doc", Plain: true}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<h1 id="title">Title`)
	assert.Contains(t, got, `<p>Body</p>`)
	assert.NotContains(t, got, `class="language-plaintext"`)
	assert.NotContains(t, got, `# Title`)
}

func TestRenderReport_ArticleUsesMarkdownHeadingTitle(t *testing.T) {
	t.Parallel()

	src := []byte("# Real Title\n\nBody\n")
	analysis := report.Analyze(src, "file-name.md")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    analysis.Kind,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeReader,
		Components: []report.Component{
			{Type: report.ComponentArticle, Source: "input", Title: "Article", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "file-name"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<title>Real Title</title>`)
	assert.Contains(t, got, `<h1 id="real-title">Real Title</h1>`)
	assert.NotContains(t, got, `<title>file-name</title>`)
}

func TestRenderReport_DiffViewStripsANSIBeforeClassifying(t *testing.T) {
	t.Parallel()

	src := []byte("diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n\x1b[31m-old\x1b[0m\n\x1b[32m+new\x1b[0m\n")
	analysis := report.Analyze(src, "patch.diff")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindDiff,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeReview,
		Components: []report.Component{
			{Type: report.ComponentDiffView, Source: "input", Title: "Diff", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "patch"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<span class="del">-old</span>`)
	assert.Contains(t, got, `<span class="add">+new</span>`)
	assert.NotContains(t, got, "\x1b[31m")
	assert.NotContains(t, got, "\x1b[32m")
}

func TestRenderReport_PatchFileUsesDiffView(t *testing.T) {
	t.Parallel()

	src := []byte("diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{SourceName: "change.patch", Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "change", SourceName: "change.patch"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<h2>Diff</h2>`)
	assert.Contains(t, got, `<pre class="diff-view"><code>`)
	assert.Contains(t, got, `<span class="del">-old</span>`)
	assert.Contains(t, got, `<span class="add">+new</span>`)
	assert.NotContains(t, got, `<h2>Code</h2>`)
}

func TestRenderReport_PlainUnifiedDiffUsesDiffView(t *testing.T) {
	t.Parallel()

	src := []byte("--- old.txt\n+++ new.txt\n@@ -1 +1 @@\n-old\n+new\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "change"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dd>diff</dd>`)
	assert.Contains(t, got, `<h2>Diff</h2>`)
	assert.Contains(t, got, `<span class="file">--- old.txt</span>`)
	assert.Contains(t, got, `<span class="file">+++ new.txt</span>`)
	assert.Contains(t, got, `<span class="del">-old</span>`)
	assert.Contains(t, got, `<span class="add">+new</span>`)
	assert.NotContains(t, got, `<h2>Input</h2>`)
}

func TestRenderReport_NoNewlineDiffMarkerUsesMetadataClass(t *testing.T) {
	t.Parallel()

	src := []byte("--- old.txt\n+++ new.txt\n@@ -1 +1 @@\n-old\n\\ No newline at end of file\n+new\n\\ No newline at end of file\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "change"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dd>diff</dd>`)
	assert.Contains(t, got, `<span class="file">\ No newline at end of file</span>`)
	assert.NotContains(t, got, `<span class="ctx">\ No newline at end of file</span>`)
}

func TestRenderReport_DiffContentStartingWithFileHeaderMarkersUsesContentClass(t *testing.T) {
	t.Parallel()

	src := []byte("--- old.txt\n+++ new.txt\n@@ -1,2 +1,2 @@\n---deleted heading\n+++added heading\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "change"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dd>diff</dd>`)
	assert.Contains(t, got, `<span class="file">--- old.txt</span>`)
	assert.Contains(t, got, `<span class="file">+++ new.txt</span>`)
	assert.Contains(t, got, `<span class="del">---deleted heading</span>`)
	assert.Contains(t, got, `<span class="add">+++added heading</span>`)
}

func TestRenderReport_CombinedDiffUsesDiffView(t *testing.T) {
	t.Parallel()

	src := []byte("diff --cc main.go\nindex 1111111,2222222..3333333\n--- a/main.go\n+++ b/main.go\n@@@ -1,1 -1,1 +1,1 @@@\n- left\n -right\n++merged\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "merge"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dd>diff</dd>`)
	assert.Contains(t, got, `<span class="file">diff --cc main.go</span>`)
	assert.Contains(t, got, `<span class="hunk">@@@ -1,1 -1,1 +1,1 @@@</span>`)
	assert.Contains(t, got, `<span class="del">- left</span>`)
	assert.Contains(t, got, `<span class="del"> -right</span>`)
	assert.Contains(t, got, `<span class="add">++merged</span>`)
	assert.NotContains(t, got, `<h2>Code</h2>`)
}

func TestRenderReport_BinaryPatchUsesDiffView(t *testing.T) {
	t.Parallel()

	src := []byte("diff --git a/logo.png b/logo.png\nnew file mode 100644\nindex 0000000..1111111\nGIT binary patch\nliteral 0\nHcmV?d00001\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "logo"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dd>diff</dd>`)
	assert.Contains(t, got, `<h2>Diff</h2>`)
	assert.Contains(t, got, `<span class="file">diff --git a/logo.png b/logo.png</span>`)
	assert.Contains(t, got, `<span class="file">GIT binary patch</span>`)
	assert.Contains(t, got, `<span class="file">literal 0</span>`)
	assert.NotContains(t, got, `<h2>Code</h2>`)
}

func TestRenderReport_ModeOnlyPatchUsesDiffView(t *testing.T) {
	t.Parallel()

	src := []byte("diff --git a/script.sh b/script.sh\nold mode 100644\nnew mode 100755\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "script"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dd>diff</dd>`)
	assert.Contains(t, got, `<h2>Diff</h2>`)
	assert.Contains(t, got, `<span class="file">diff --git a/script.sh b/script.sh</span>`)
	assert.Contains(t, got, `<span class="file">old mode 100644</span>`)
	assert.Contains(t, got, `<span class="file">new mode 100755</span>`)
	assert.NotContains(t, got, `<h2>Code</h2>`)
}

func TestRenderReport_CopyOnlyPatchUsesDiffView(t *testing.T) {
	t.Parallel()

	src := []byte("diff --git a/source.txt b/copy.txt\ncopy from source.txt\ncopy to copy.txt\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "copy"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dd>diff</dd>`)
	assert.Contains(t, got, `<h2>Diff</h2>`)
	assert.Contains(t, got, `<span class="file">diff --git a/source.txt b/copy.txt</span>`)
	assert.Contains(t, got, `<span class="file">copy from source.txt</span>`)
	assert.Contains(t, got, `<span class="file">copy to copy.txt</span>`)
	assert.NotContains(t, got, `<h2>Code</h2>`)
}

func TestRenderReport_DiffViewDoesNotAddTrailingBlankLine(t *testing.T) {
	t.Parallel()

	src := []byte("diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{SourceName: "change.patch", Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "change", SourceName: "change.patch"}, analysis, plan)
	require.NoError(t, err)

	assert.NotContains(t, got, `<span class="ctx"></span>`)
}

func TestRenderReport_FileTreeCleansASCIIMarkers(t *testing.T) {
	t.Parallel()

	src := []byte(".\n|-- cmd\n|   `-- html\n`-- internal\n")
	analysis := report.Analyze(src, "tree.txt")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTreeListing,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}},
			{Type: report.ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "tree"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<li style="--depth:0"><span>cmd</span></li>`)
	assert.Contains(t, got, `<li style="--depth:1"><span>html</span></li>`)
	assert.Contains(t, got, `<li style="--depth:0"><span>internal</span></li>`)
	assert.Contains(t, got, `<dt>Files</dt><dd>3</dd>`)
	assert.NotContains(t, got, `<span>.</span>`)
	assert.NotContains(t, got, `|-- cmd`)
	assert.NotContains(t, got, "`-- html")
}

func TestRenderReport_FileTreeStripsANSI(t *testing.T) {
	t.Parallel()

	src := []byte(".\n├── \x1b[34mcmd\x1b[0m\n│   └── html\n└── internal\n")
	analysis := report.Analyze(src, "tree.txt")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTreeListing,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "tree"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<li style="--depth:0"><span>cmd</span></li>`)
	assert.Contains(t, got, `<li style="--depth:1"><span>html</span></li>`)
	assert.NotContains(t, got, "\x1b[34m")
	assert.NotContains(t, got, "\x1b[0m")
}

func TestRenderReport_FileTreeDoesNotCountSpacesInsideUnicodeName(t *testing.T) {
	t.Parallel()

	src := []byte(".\n├── Other\n└── My    File\n")
	analysis := report.Analyze(src, "tree.txt")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTreeListing,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindTreeListing, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "tree"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<li style="--depth:0"><span>My    File</span></li>`)
	assert.NotContains(t, got, `<li style="--depth:1"><span>My    File</span></li>`)
}

func TestRenderReport_FileTreeParsesANSIColoredASCIIMarkers(t *testing.T) {
	t.Parallel()

	src := []byte(".\n\x1b[34m|-- cmd\x1b[0m\n|   \x1b[34m`-- html\x1b[0m\n\x1b[34m`-- internal\x1b[0m\n")
	analysis := report.Analyze(src, "tree.txt")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTreeListing,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindTreeListing, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "tree"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<li style="--depth:0"><span>cmd</span></li>`)
	assert.Contains(t, got, `<li style="--depth:1"><span>html</span></li>`)
	assert.Contains(t, got, `<li style="--depth:0"><span>internal</span></li>`)
	assert.NotContains(t, got, `|-- cmd`)
	assert.NotContains(t, got, "`-- html")
	assert.NotContains(t, got, "\x1b[34m")
}

func TestRenderReport_FileTreeSkipsTreeSummary(t *testing.T) {
	t.Parallel()

	src := []byte(".\n├── cmd\n│   └── html\n└── internal\n\n2 directories, 1 file\n")
	analysis := report.Analyze(src, "tree.txt")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTreeListing,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}},
			{Type: report.ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindTreeListing, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "tree"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dt>Files</dt><dd>3</dd>`)
	assert.Contains(t, got, `<li style="--depth:0"><span>cmd</span></li>`)
	assert.Contains(t, got, `<li style="--depth:1"><span>html</span></li>`)
	assert.Contains(t, got, `<li style="--depth:0"><span>internal</span></li>`)
	assert.NotContains(t, got, `<span>2 directories, 1 file</span>`)
}

func TestRenderReport_FileTreeSkipsDirectoryOnlyTreeSummary(t *testing.T) {
	t.Parallel()

	src := []byte(".\n├── cmd\n│   └── html\n└── internal\n\n2 directories\n")
	analysis := report.Analyze(src, "tree.txt")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTreeListing,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}},
			{Type: report.ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindTreeListing, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "tree"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dt>Files</dt><dd>3</dd>`)
	assert.Contains(t, got, `<li style="--depth:0"><span>cmd</span></li>`)
	assert.Contains(t, got, `<li style="--depth:1"><span>html</span></li>`)
	assert.Contains(t, got, `<li style="--depth:0"><span>internal</span></li>`)
	assert.NotContains(t, got, `<span>2 directories</span>`)
}

func TestRenderReport_FileTreeTrimsDotSlashPathNames(t *testing.T) {
	t.Parallel()

	src := []byte("./cmd/html\n./internal/render\n./README.md\n")
	analysis := report.Analyze(src, "")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTreeListing,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "paths"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<li style="--depth:1"><span>cmd/html</span></li>`)
	assert.Contains(t, got, `<li style="--depth:1"><span>internal/render</span></li>`)
	assert.Contains(t, got, `<li style="--depth:0"><span>README.md</span></li>`)
	assert.NotContains(t, got, `<span>./`)
}

func TestRenderReport_FileTreeSkipsBlankLines(t *testing.T) {
	t.Parallel()

	src := []byte("./cmd/html\n\n./internal/render\n./README.md\n")
	analysis := report.Analyze(src, "")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTreeListing,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindTreeListing, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "paths"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<li style="--depth:1"><span>cmd/html</span></li>`)
	assert.Contains(t, got, `<li style="--depth:1"><span>internal/render</span></li>`)
	assert.NotContains(t, got, `<span></span>`)
}

func TestRenderReport_FileTreeIndentsWindowsPathNames(t *testing.T) {
	t.Parallel()

	src := []byte(`cmd\html
internal\render
docs\README.md
`)
	analysis := report.Analyze(src, "")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTreeListing,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindTreeListing, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "paths"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<li style="--depth:1"><span>cmd\html</span></li>`)
	assert.Contains(t, got, `<li style="--depth:1"><span>internal\render</span></li>`)
	assert.Contains(t, got, `<li style="--depth:1"><span>docs\README.md</span></li>`)
}

func TestRenderReport_FileTreeDoesNotIndentWindowsDriveRoot(t *testing.T) {
	t.Parallel()

	src := []byte("C:\\Users\\Alice\nC:\\Users\\Alice\\Documents\nD:\\Archive\\Logs\n")
	analysis := report.Analyze(src, "list.weird")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTreeListing,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindTreeListing, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "paths"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<li style="--depth:1"><span>C:\Users\Alice</span></li>`)
	assert.Contains(t, got, `<li style="--depth:2"><span>C:\Users\Alice\Documents</span></li>`)
	assert.Contains(t, got, `<li style="--depth:1"><span>D:\Archive\Logs</span></li>`)
	assert.NotContains(t, got, `<li style="--depth:3"><span>C:\Users\Alice\Documents</span></li>`)
}

func TestRenderReport_FileTreeRendersNumericPathNames(t *testing.T) {
	t.Parallel()

	src := []byte("./2024/01\n./2024/02\n./2025/01\n")
	analysis := report.Analyze(src, "list.weird")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTreeListing,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindTreeListing, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "paths"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<li style="--depth:1"><span>2024/01</span></li>`)
	assert.Contains(t, got, `<li style="--depth:1"><span>2024/02</span></li>`)
	assert.Contains(t, got, `<li style="--depth:1"><span>2025/01</span></li>`)
}

func TestRenderReport_FileTreeDoesNotIndentAbsolutePathRoot(t *testing.T) {
	t.Parallel()

	src := []byte("/usr/bin\n/usr/local/bin\n/var/log/html\n")
	analysis := report.Analyze(src, "list.weird")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTreeListing,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindTreeListing, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "paths"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<li style="--depth:1"><span>/usr/bin</span></li>`)
	assert.Contains(t, got, `<li style="--depth:2"><span>/usr/local/bin</span></li>`)
	assert.Contains(t, got, `<li style="--depth:2"><span>/var/log/html</span></li>`)
	assert.NotContains(t, got, `<li style="--depth:3"><span>/usr/local/bin</span></li>`)
}

func TestRenderReport_FileTreeDoesNotIndentParentRelativeAnchor(t *testing.T) {
	t.Parallel()

	src := []byte("../src/main.go\n../src/internal/render.go\n../docs/README.md\n")
	analysis := report.Analyze(src, "list.weird")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTreeListing,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindTreeListing, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "paths"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<li style="--depth:1"><span>../src/main.go</span></li>`)
	assert.Contains(t, got, `<li style="--depth:2"><span>../src/internal/render.go</span></li>`)
	assert.Contains(t, got, `<li style="--depth:1"><span>../docs/README.md</span></li>`)
	assert.NotContains(t, got, `<li style="--depth:3"><span>../src/internal/render.go</span></li>`)
}

func TestRenderReport_FileTreeDoesNotIndentHomeRelativeAnchor(t *testing.T) {
	t.Parallel()

	src := []byte("~/src/main.go\n~/src/internal/render.go\n~/docs/README.md\n")
	analysis := report.Analyze(src, "list.weird")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTreeListing,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindTreeListing, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "paths"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<li style="--depth:1"><span>~/src/main.go</span></li>`)
	assert.Contains(t, got, `<li style="--depth:2"><span>~/src/internal/render.go</span></li>`)
	assert.Contains(t, got, `<li style="--depth:1"><span>~/docs/README.md</span></li>`)
	assert.NotContains(t, got, `<li style="--depth:3"><span>~/src/internal/render.go</span></li>`)
}

func TestRenderReport_FileTreeRendersPathNamesWithSpaces(t *testing.T) {
	t.Parallel()

	src := []byte("./My Project/README.md\n./My Project/docs/Release Notes.md\n./Other Project/TODO.txt\n")
	analysis := report.Analyze(src, "list.weird")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTreeListing,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindTreeListing, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "paths"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<li style="--depth:1"><span>My Project/README.md</span></li>`)
	assert.Contains(t, got, `<li style="--depth:2"><span>My Project/docs/Release Notes.md</span></li>`)
	assert.Contains(t, got, `<li style="--depth:1"><span>Other Project/TODO.txt</span></li>`)
}

func TestRenderReport_JSONTablePreservesLargeNumberText(t *testing.T) {
	t.Parallel()

	src := []byte(`[{"id":9007199254740993,"name":"exact"}]`)
	analysis := report.Analyze(src, "ids.json")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindJSONRecords,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "ids"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `9007199254740993`)
	assert.NotContains(t, got, `9007199254740992`)
}

func TestRenderReport_JSONTableDistinguishesNullFromMissing(t *testing.T) {
	t.Parallel()

	src := []byte(`[{"name":"has-null","value":null},{"name":"missing"}]`)
	analysis := report.Analyze(src, "values.json")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindJSONRecords,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "values"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, labeledCell("name", "has-null")+labeledCell("value", "null"))
	assert.Contains(t, got, labeledCell("name", "missing")+labeledCell("value", ""))
}

func TestRenderReport_RecordCardsOmitEmptyFields(t *testing.T) {
	t.Parallel()

	src := []byte(`[{"name":"has-null","owner":"","value":null},{"name":"missing","owner":"Ops"}]`)
	analysis := report.Analyze(src, "values.json")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindJSONRecords,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentRecordCards, Source: "records", Title: "Details", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "values"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `class="record-card"`)
	assert.Contains(t, got, cardField("name", "has-null"))
	assert.Contains(t, got, cardField("value", "null"))
	assert.Contains(t, got, cardField("owner", "Ops"))
	assert.NotContains(t, got, cardField("owner", ""))
	assert.NotContains(t, got, cardField("value", ""))
}

func TestRenderReport_JSONTableIgnoresLeadingBOM(t *testing.T) {
	t.Parallel()

	src := []byte("\ufeff" + `[{"id":1,"name":"bom"}]`)
	analysis := report.Analyze(src, "ids.json")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindJSONRecords,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "ids"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, labeledCell("id", "1"))
	assert.Contains(t, got, labeledCell("name", "bom"))
	assert.NotContains(t, got, "\ufeff")
}

func TestRenderReport_JSONScalarFileRendersRawJSON(t *testing.T) {
	t.Parallel()

	src := []byte("9007199254740993\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{SourceName: "value.json", Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "value"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dd>json-object</dd>`)
	assert.Contains(t, got, `<h2>JSON</h2>`)
	assert.Contains(t, got, `9007199254740993`)
	assert.NotContains(t, got, `9007199254740992`)
}

func TestRenderReport_JSONLinesTableUsesAnalyzedRecords(t *testing.T) {
	t.Parallel()

	src := []byte("{\"name\":\"a\",\"score\":1}\n{\"name\":\"b\",\"score\":2}\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{SourceName: "events.jsonl", Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "events"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `class="report-table"`)
	assert.Contains(t, got, `<th scope="col"><button type="button">name</button></th>`)
	assert.Contains(t, got, `<th scope="col"><button type="button">score</button></th>`)
	assert.Contains(t, got, labeledCell("name", "a")+labeledCell("score", "1"))
	assert.Contains(t, got, labeledCell("name", "b")+labeledCell("score", "2"))
}

func TestRenderReport_SingleJSONLineRecordRendersTable(t *testing.T) {
	t.Parallel()

	src := []byte("{\"name\":\"a\",\"score\":1}\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{SourceName: "events.jsonl", Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "events"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dd>json-records</dd>`)
	assert.Contains(t, got, `class="report-table"`)
	assert.Contains(t, got, `<p class="report-filter-status" aria-live="polite">1 row</p>`)
	assert.Contains(t, got, labeledCell("name", "a")+labeledCell("score", "1"))
	assert.NotContains(t, got, `<h2>JSON</h2>`)
}

func TestRenderReport_SingleNDJSONRecordRendersTable(t *testing.T) {
	t.Parallel()

	src := []byte("{\"name\":\"a\",\"score\":1}\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{SourceName: "events.ndjson", Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "events"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dd>json-records</dd>`)
	assert.Contains(t, got, `class="report-table"`)
	assert.Contains(t, got, labeledCell("name", "a")+labeledCell("score", "1"))
	assert.NotContains(t, got, `<h2>JSON</h2>`)
}

func TestRenderReport_SingleJSONLinesRecordRendersTable(t *testing.T) {
	t.Parallel()

	src := []byte("{\"name\":\"a\",\"score\":1}\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{SourceName: "events.jsonlines", Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "events"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dd>json-records</dd>`)
	assert.Contains(t, got, `class="report-table"`)
	assert.Contains(t, got, labeledCell("name", "a")+labeledCell("score", "1"))
	assert.NotContains(t, got, `<h2>JSON</h2>`)
}

func TestRenderReport_CSVTablePreservesDuplicateHeaderCells(t *testing.T) {
	t.Parallel()

	src := []byte("tag,tag\nleft,right\n")
	analysis := report.Analyze(src, "tags.csv")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindCSVRecords,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "tags"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<button type="button">tag</button><`)
	assert.Contains(t, got, `<button type="button">tag 2</button>`)
	assert.Contains(t, got, labeledCell("tag", "left")+labeledCell("tag 2", "right"))
	assert.NotContains(t, got, labeledCell("tag", "right")+labeledCell("tag 2", "right"))
}

func TestRenderReport_CSVTableDeduplicatesHeaderLabelCollisions(t *testing.T) {
	t.Parallel()

	src := []byte("tag,tag,tag 2\nleft,middle,right\n")
	analysis := report.Analyze(src, "tags.csv")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindCSVRecords,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "tags"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<button type="button">tag</button>`)
	assert.Contains(t, got, `<button type="button">tag 2</button>`)
	assert.Contains(t, got, `<button type="button">tag 2 2</button>`)
	assert.Contains(t, got, labeledCell("tag", "left")+labeledCell("tag 2", "middle")+labeledCell("tag 2 2", "right"))
}

func TestRenderReport_CSVTableLabelsBlankHeaders(t *testing.T) {
	t.Parallel()

	src := []byte("name,\nAlpha,10\n")
	analysis := report.Analyze(src, "scores.csv")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindCSVRecords,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "scores"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<button type="button">Column 2</button>`)
	assert.NotContains(t, got, `<button type="button"></button>`)
	assert.Contains(t, got, labeledCell("name", "Alpha")+labeledCell("Column 2", "10"))
}

func TestRenderReport_CSVTablePreservesRecordSpaces(t *testing.T) {
	t.Parallel()

	src := []byte(" name,score\n Alpha,1 \n   \n")
	analysis := report.Analyze(src, "scores.csv")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindCSVRecords,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindCSVRecords, analysis.Kind)
	require.Equal(t, 1, analysis.Stats.Records)

	got, err := RenderReport(src, Options{FallbackTitle: "scores"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<button type="button"> name</button>`)
	assert.Contains(t, got, labeledCell(" name", " Alpha")+labeledCell("score", "1 "))
	assert.NotContains(t, got, `<button type="button">name</button>`)
	assert.NotContains(t, got, labeledCell(" name", "   ")+labeledCell("score", ""))
}

func TestRenderReport_TableStripsANSI(t *testing.T) {
	t.Parallel()

	src := []byte("\x1b[1mname\x1b[0m,score\n\x1b[32mAlpha\x1b[0m,10\n")
	analysis := report.Analyze(src, "scores.csv")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindCSVRecords,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindCSVRecords, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "scores"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<button type="button">name</button>`)
	assert.Contains(t, got, labeledCell("name", "Alpha")+labeledCell("score", "10"))
	assert.NotContains(t, got, "\x1b[1m")
	assert.NotContains(t, got, "\x1b[32m")
}

func TestRenderReport_TableDeduplicatesANSIHeaderLabels(t *testing.T) {
	t.Parallel()

	src := []byte("name,\x1b[1mname\x1b[0m\nAlpha,Beta\n")
	analysis := report.Analyze(src, "names.csv")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindCSVRecords,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindCSVRecords, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "names"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<button type="button">name</button>`)
	assert.Contains(t, got, `<button type="button">name 2</button>`)
	assert.NotContains(t, got, "\x1b[1m")
}

func TestRenderReport_CSVTableUsesAnalyzerRows(t *testing.T) {
	t.Parallel()

	src := []byte("name,score\nAlpha,10\n   \n")
	analysis := report.Analyze(src, "scores.csv")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindCSVRecords,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindCSVRecords, analysis.Kind)
	require.Equal(t, 1, analysis.Stats.Records)

	got, err := RenderReport(src, Options{FallbackTitle: "scores"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<p class="report-filter-status" aria-live="polite">1 row</p>`)
	assert.Contains(t, got, labeledCell("name", "Alpha")+labeledCell("score", "10"))
	assert.NotContains(t, got, labeledCell("name", "   ")+labeledCell("score", ""))
}

func TestRenderReport_TSVTableUsesAnalyzerRows(t *testing.T) {
	t.Parallel()

	src := []byte("\nname\tscore\nAlpha\t10\n   \n")
	analysis := report.Analyze(src, "scores.tsv")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTSVRecords,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindTSVRecords, analysis.Kind)
	require.Equal(t, 1, analysis.Stats.Records)

	got, err := RenderReport(src, Options{FallbackTitle: "scores"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<p class="report-filter-status" aria-live="polite">1 row</p>`)
	assert.Contains(t, got, labeledCell("name", "Alpha")+labeledCell("score", "10"))
	assert.NotContains(t, got, labeledCell("name", "   ")+labeledCell("score", ""))
}
