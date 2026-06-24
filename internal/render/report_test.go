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

func sortHeaderButton(label string) string {
	escaped := escapeTableText(label)
	return `<button type="button" data-sort-label="` + escaped + `" aria-label="Sort by ` + escaped + ` ascending">` + escaped + `</button>`
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
	assert.Contains(t, got, `id="report-tab-0" type="button" role="tab" aria-selected="true" aria-controls="report-panel-0" tabindex="0" title="Summary"><span>Summary</span></button>`)
	assert.Contains(t, got, `id="report-tab-1" type="button" role="tab" aria-selected="false" aria-controls="report-panel-1" tabindex="-1" title="Records"><span>Records</span></button>`)
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
	assert.Contains(t, got, `<nav class="report-slide-controls" aria-label="Slide controls">`)
	assert.Contains(t, got, `data-slide-prev aria-label="Previous slide" title="Previous slide"><span aria-hidden="true">‹</span>`)
	assert.Contains(t, got, `data-slide-status>1 / 2</span>`)
	assert.Contains(t, got, `data-slide-next aria-label="Next slide" title="Next slide"><span aria-hidden="true">›</span>`)
	assert.Contains(t, got, `.report-slide`)
	assert.NotContains(t, got, `role="tablist"`)
}

func TestRenderReport_MixedSummaryShowsSignalNames(t *testing.T) {
	t.Parallel()

	src := []byte("Notes\n- check deploy\n\nPayload\n{\"ok\":true}\n\nERROR failed\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "mixed"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dt>Kind</dt><dd>mixed</dd>`)
	assert.Contains(t, got, `multiple weak format signals: markdown-like prose, json-like block, log severity`)
	assert.Contains(t, got, `role="tablist"`)
	assert.Contains(t, got, `title="Summary"><span>Summary</span></button>`)
	assert.Contains(t, got, `title="Input"><span>Input</span></button>`)
	assert.Contains(t, got, `<dl class="text-overview" aria-label="Text overview">`)
	assert.Contains(t, got, `ERROR failed`)
}

func TestRenderReport_PlainUsesTextOverview(t *testing.T) {
	t.Parallel()

	src := []byte("ordinary prose with no structural markup\nsecond line\n")
	analysis := report.Analyze(src, "notes.txt")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindPlain,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeBrief,
		Components: []report.Component{
			{Type: report.ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}},
			{Type: report.ComponentPreformatted, Source: "input", Title: "Input", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindPlain, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "notes"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dl class="text-overview" aria-label="Text overview">`)
	assert.Contains(t, got, `<dt>Lines</dt><dd>2</dd>`)
	assert.Contains(t, got, `<dt>Words</dt><dd>8</dd>`)
	assert.Contains(t, got, `<dt>Characters</dt><dd>53</dd>`)
	assert.Contains(t, got, `<pre class="report-text"><code class="language-plaintext">ordinary prose`)
}

func TestRenderReport_BinaryUsesSafePreview(t *testing.T) {
	t.Parallel()

	src := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0xff, 'h', 't', 'm', 'l'}
	analysis, plan := report.Plan(t.Context(), src, report.Options{SourceName: "logo.png", Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "logo"}, analysis, plan)
	require.NoError(t, err)

	assert.Equal(t, report.KindBinary, analysis.Kind)
	assert.Contains(t, got, `<dd>binary</dd>`)
	assert.Contains(t, got, `<dl class="binary-overview" aria-label="Binary overview">`)
	assert.Contains(t, got, `<dt>Bytes</dt><dd>14</dd>`)
	assert.Contains(t, got, `<dt>Preview</dt><dd>14 bytes</dd>`)
	assert.Contains(t, got, `<dt>Reason</dt><dd>binary bytes detected</dd>`)
	assert.Contains(t, got, `<pre class="binary-preview" aria-label="Binary byte preview"><code>`)
	assert.Contains(t, got, `00000000  89 50 4e 47 0d 0a 1a 0a 00 ff 68 74 6d 6c`)
	assert.Contains(t, got, `|.PNG......html|`)
	assert.NotContains(t, got, "\x00")
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
	assert.Contains(t, got, `<div class="report-mobile-sort"><select aria-label="Sort rows"><option value="">Sort rows</option><option value="0:ascending">name ↑</option><option value="0:descending">name ↓</option><option value="1:ascending">score ↑</option><option value="1:descending">score ↓</option></select></div>`)
	assert.Contains(t, got, `<button type="button" data-sort-label="name" aria-label="Sort by name ascending">name</button>`)
	assert.Contains(t, got, `<button type="button" data-sort-label="score" aria-label="Sort by score ascending">score</button>`)
	assert.NotContains(t, got, `>1 rows<`)
}

func TestRenderReport_DataTableIncludesFilterEmptyRow(t *testing.T) {
	t.Parallel()

	src := []byte("name,score\nAlpha,10\nBeta,20\n")
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

	assert.Contains(t, got, `<tr class="report-empty-row" data-report-empty-row hidden><td colspan="2">No rows match</td></tr>`)
}

func TestRenderReport_DataTableShowsEmptyInputRow(t *testing.T) {
	t.Parallel()

	src := []byte("name,score\n")
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

	assert.Contains(t, got, `<p class="report-filter-status" aria-live="polite">0 rows</p>`)
	assert.Contains(t, got, `<tr class="report-empty-row" data-report-empty-row><td colspan="2">No rows</td></tr>`)
}

func TestRenderReport_TranscriptUsesStructuredTurns(t *testing.T) {
	t.Parallel()

	src := []byte("Host: Welcome back.\nGuest: Thanks for having me.\ncontinued answer\nHost: Let's begin <now>.\n")
	analysis := report.Analyze(src, "transcript.txt")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTranscript,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeConsole,
		Components: []report.Component{
			{Type: report.ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}},
			{Type: report.ComponentPreformatted, Source: "input", Title: "Transcript", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindTranscript, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "transcript"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dl class="transcript-overview" aria-label="Transcript overview"><div><dt>Turns</dt><dd>3</dd></div><div><dt>Speakers</dt><dd>2</dd></div></dl>`)
	assert.Contains(t, got, `<ol class="transcript-turns">`)
	assert.Contains(t, got, `<li class="transcript-turn"><span class="transcript-speaker">Host</span><div class="transcript-text"><p>Welcome back.</p></div></li>`)
	assert.Contains(t, got, `<span class="transcript-speaker">Guest</span><div class="transcript-text"><p>Thanks for having me.</p><p>continued answer</p></div>`)
	assert.Contains(t, got, `<p>Let&#39;s begin &lt;now&gt;.</p>`)
	assert.NotContains(t, got, `<pre><code class="language-plaintext">`)
}

func TestRenderReport_TranscriptStripsANSI(t *testing.T) {
	t.Parallel()

	src := []byte("Host: \x1b[32mWelcome back\x1b[0m.\nGuest: Thanks.\nHost: Done.\n")
	analysis := report.Analyze(src, "transcript.txt")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTranscript,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeConsole,
		Components: []report.Component{
			{Type: report.ComponentPreformatted, Source: "input", Title: "Transcript", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindTranscript, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "transcript"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<p>Welcome back.</p>`)
	assert.NotContains(t, got, "\x1b[32m")
	assert.NotContains(t, got, "\x1b[0m")
}

func TestRenderReport_LogUsesStructuredLines(t *testing.T) {
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

	assert.Contains(t, got, `<dl class="log-overview" aria-label="Log overview"><div><dt>Lines</dt><dd>2</dd></div><div><dt>Errors</dt><dd>1</dd></div><div><dt>Info</dt><dd>1</dd></div></dl>`)
	assert.Contains(t, got, `<ol class="log-lines">`)
	assert.Contains(t, got, `<li class="log-line log-error"><span class="log-level">ERROR</span><span class="log-message">2026-06-16 12:00:00 ERROR fail</span></li>`)
	assert.Contains(t, got, `<li class="log-line log-info"><span class="log-level">INFO</span><span class="log-message">2026-06-16 12:00:01 INFO ok</span></li>`)
	assert.NotContains(t, got, `<pre><code class="language-plaintext">`)
	assert.NotContains(t, got, "\x1b[31m")
}

func TestRenderReport_LogEscapesAndClassifiesGoTestLines(t *testing.T) {
	t.Parallel()

	src := []byte("--- FAIL: TestThing (0.00s)\n    thing_test.go:10: got <bad>\nok\tgithub.com/example/project\t0.123s\n")
	analysis := report.Analyze(src, "go-test.log")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindLog,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeConsole,
		Components: []report.Component{
			{Type: report.ComponentPreformatted, Source: "input", Title: "Log", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindLog, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "go-test"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dl class="log-overview" aria-label="Log overview"><div><dt>Lines</dt><dd>3</dd></div><div><dt>Errors</dt><dd>1</dd></div><div><dt>Info</dt><dd>1</dd></div></dl>`)
	assert.Contains(t, got, `<li class="log-line log-error"><span class="log-level">ERROR</span><span class="log-message">--- FAIL: TestThing (0.00s)</span></li>`)
	assert.Contains(t, got, `<span class="log-message">    thing_test.go:10: got &lt;bad&gt;</span>`)
	assert.Contains(t, got, `<li class="log-line log-info"><span class="log-level">INFO</span><span class="log-message">ok	github.com/example/project	0.123s</span></li>`)
}

func TestRenderReport_AccessLogClassifiesHTTPStatus(t *testing.T) {
	t.Parallel()

	src := []byte("127.0.0.1 - - [16/Jun/2026:12:00:00 -0400] \"GET /index.html HTTP/1.1\" 200 1234\n" +
		"127.0.0.1 - - [16/Jun/2026:12:00:01 -0400] \"GET /missing HTTP/1.1\" 404 123\n" +
		"127.0.0.1 - - [16/Jun/2026:12:00:02 -0400] \"POST /api HTTP/1.1\" 500 42\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{SourceName: "access.log", Planner: report.PlannerOff})

	require.Equal(t, report.KindLog, analysis.Kind)
	require.Contains(t, analysis.Reasons, "http access log markers")

	got, err := RenderReport(src, Options{FallbackTitle: "access"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dl class="log-overview" aria-label="Log overview"><div><dt>Lines</dt><dd>3</dd></div><div><dt>Errors</dt><dd>1</dd></div><div><dt>Warnings</dt><dd>1</dd></div><div><dt>Info</dt><dd>1</dd></div></dl>`)
	assert.Contains(t, got, `<li class="log-line log-info"><span class="log-level">INFO</span><span class="log-message">127.0.0.1 - - [16/Jun/2026:12:00:00 -0400] &#34;GET /index.html HTTP/1.1&#34; 200 1234</span></li>`)
	assert.Contains(t, got, `<li class="log-line log-warn"><span class="log-level">WARN</span><span class="log-message">127.0.0.1 - - [16/Jun/2026:12:00:01 -0400] &#34;GET /missing HTTP/1.1&#34; 404 123</span></li>`)
	assert.Contains(t, got, `<li class="log-line log-error"><span class="log-level">ERROR</span><span class="log-message">127.0.0.1 - - [16/Jun/2026:12:00:02 -0400] &#34;POST /api HTTP/1.1&#34; 500 42</span></li>`)
	assert.NotContains(t, got, `<pre class="report-text"`)
}

func TestRenderReport_ForcedLogModeUsesStructuredLines(t *testing.T) {
	t.Parallel()

	src := []byte("starting\nplain line without timestamp\nERROR forced mode still classifies\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{Mode: report.ModeOverrideLog, Planner: report.PlannerOff})

	require.Equal(t, report.KindPlain, analysis.Kind)
	require.Equal(t, report.ModeConsole, plan.Mode)

	got, err := RenderReport(src, Options{FallbackTitle: "forced-log"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<h2>Log</h2>`)
	assert.Contains(t, got, `<ol class="log-lines">`)
	assert.Contains(t, got, `<li class="log-line"><span class="log-message">starting</span></li>`)
	assert.Contains(t, got, `<li class="log-line log-error"><span class="log-level">ERROR</span><span class="log-message">ERROR forced mode still classifies</span></li>`)
	assert.NotContains(t, got, `<dl class="text-overview"`)
	assert.NotContains(t, got, `<pre class="report-text"`)
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

func TestRenderReport_SourceCodeShowsOverview(t *testing.T) {
	t.Parallel()

	src := []byte("package main\n\nfunc main() {}\n")
	analysis := report.Analyze(src, "main.go")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindSourceCode,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeCode,
		Components: []report.Component{
			{Type: report.ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}},
			{Type: report.ComponentCodeBlock, Source: "input", Title: "Code", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindSourceCode, analysis.Kind)

	got, err := RenderReport(src, Options{FallbackTitle: "main", SourceName: "main.go"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dl class="code-overview" aria-label="Code overview">`)
	assert.Contains(t, got, `<dt>Language</dt><dd>Go</dd>`)
	assert.Contains(t, got, `<dt>Source</dt><dd>main.go</dd>`)
	assert.Contains(t, got, `<dt>Renderer</dt><dd>Chroma</dd>`)
	assert.Contains(t, got, `<dt>Lines</dt><dd>3</dd>`)
	assert.Contains(t, got, `class="chroma light"`)
}

func TestRenderReport_ForcedCodeModeShowsOverview(t *testing.T) {
	t.Parallel()

	src := []byte("plain text\nwithout a source lexer\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{Mode: report.ModeOverrideCode, Planner: report.PlannerOff})

	require.Equal(t, report.KindPlain, analysis.Kind)
	require.Equal(t, report.ModeCode, plan.Mode)

	got, err := RenderReport(src, Options{FallbackTitle: "forced-code", SourceName: "notes.txt"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<h2>Code</h2>`)
	assert.Contains(t, got, `<dl class="code-overview" aria-label="Code overview">`)
	assert.Contains(t, got, `<dt>Language</dt><dd>Plain text</dd>`)
	assert.Contains(t, got, `<dt>Source</dt><dd>notes.txt</dd>`)
	assert.Contains(t, got, `<dt>Renderer</dt><dd>Plain text</dd>`)
	assert.Contains(t, got, `<dt>Lines</dt><dd>2</dd>`)
	assert.Contains(t, got, `<pre><code class="language-plaintext">plain text`)
	assert.NotContains(t, got, `<dl class="text-overview"`)
}

func TestRenderReport_CodeOverviewReportsANSIRenderer(t *testing.T) {
	t.Parallel()

	src := []byte("\x1b[31mred\x1b[0m\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{Mode: report.ModeOverrideCode, Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "ansi", SourceName: "ansi.txt"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dt>Renderer</dt><dd>ANSI</dd>`)
	assert.Contains(t, got, `class="language-ansi"`)
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
	assert.Contains(t, got, `<dl class="article-overview" aria-label="Article overview">`)
	assert.Contains(t, got, `<dt>Lines</dt><dd>3</dd>`)
	assert.Contains(t, got, `<dt>Headings</dt><dd>1</dd>`)
	assert.Contains(t, got, `<p>Body</p>`)
	assert.NotContains(t, got, `class="language-plaintext"`)
	assert.NotContains(t, got, `# Title`)
}

func TestRenderReport_ArticleOverviewCountsSections(t *testing.T) {
	t.Parallel()

	src := []byte("# Title\n\nIntro.\n\n## One\n\nBody.\n\n## Two\n\nMore.\n")
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

	got, err := RenderReport(src, Options{FallbackTitle: "doc"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dt>Lines</dt><dd>11</dd>`)
	assert.Contains(t, got, `<dt>Headings</dt><dd>3</dd>`)
	assert.Contains(t, got, `<dt>Sections</dt><dd>2</dd>`)
	assert.Contains(t, got, `<h2 id="one">One</h2>`)
	assert.Contains(t, got, `<h2 id="two">Two</h2>`)
}

func TestRenderReport_ArticleOverviewCountsImages(t *testing.T) {
	t.Parallel()

	src := []byte("# Media\n\n![Raster](raster.png)\n\n![Vector](vector.svg)\n")
	analysis := report.Analyze(src, "media.md")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    analysis.Kind,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeReader,
		Components: []report.Component{
			{Type: report.ComponentArticle, Source: "input", Title: "Article", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "media"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dt>Images</dt><dd>2</dd>`)
	assert.Contains(t, got, `<img src="raster.png" alt="Raster">`)
	assert.Contains(t, got, `<img src="vector.svg" alt="Vector">`)
}

func TestRenderReport_ArticleOverviewCountsMarkdownPieces(t *testing.T) {
	t.Parallel()

	src := []byte("# Components\n\n## Table\n\n| Piece | State |\n|---|---|\n| Table | Ready |\n\n## Tasks\n\n- [x] Ship renderer\n- [ ] Capture screenshots\n\n## Quote\n\n> Keep generated pages self-contained.\n\n## Code\n\n```go\nfmt.Println(\"html\")\n```\n")
	analysis := report.Analyze(src, "components.md")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    analysis.Kind,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeReader,
		Components: []report.Component{
			{Type: report.ComponentArticle, Source: "input", Title: "Article", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "components"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dt>Tables</dt><dd>1</dd>`)
	assert.Contains(t, got, `<dt>Code blocks</dt><dd>1</dd>`)
	assert.Contains(t, got, `<dt>Tasks</dt><dd>2</dd>`)
	assert.Contains(t, got, `<dt>Quotes</dt><dd>1</dd>`)
	assert.Contains(t, got, `<nav class="toc" aria-label="Table of contents">`)
	assert.Contains(t, got, `<table>`)
	assert.Contains(t, got, `type="checkbox"`)
	assert.Contains(t, got, `<blockquote>`)
	assert.Contains(t, got, `<pre`)
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

	assert.Contains(t, got, `<div class="diff-summary" aria-label="Diff summary"><span><strong>1</strong> file</span><span><strong>1</strong> hunk</span><span class="diff-added"><strong>+1</strong> addition</span><span class="diff-removed"><strong>-1</strong> deletion</span></div>`)
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
	assert.Contains(t, got, `<div class="diff-summary" aria-label="Diff summary"><span><strong>1</strong> file</span><span><strong>1</strong> hunk</span><span class="diff-added"><strong>+1</strong> addition</span><span class="diff-removed"><strong>-1</strong> deletion</span></div>`)
	assert.Contains(t, got, `<span class="file">--- old.txt</span>`)
	assert.Contains(t, got, `<span class="file">+++ new.txt</span>`)
	assert.Contains(t, got, `<span class="del">-old</span>`)
	assert.Contains(t, got, `<span class="add">+new</span>`)
	assert.NotContains(t, got, `<h2>Input</h2>`)
}

func TestRenderReport_PlainUnifiedDiffCountsMultipleFiles(t *testing.T) {
	t.Parallel()

	src := []byte("--- a.txt\n+++ a.txt\n@@ -1 +1 @@\n-old\n+new\n--- b.txt\n+++ b.txt\n@@ -1 +1 @@\n-left\n+right\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "change"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dd>diff</dd>`)
	assert.Contains(t, got, `<span><strong>2</strong> files</span>`)
	assert.Contains(t, got, `<span><strong>2</strong> hunks</span>`)
	assert.Contains(t, got, `<span class="diff-added"><strong>+2</strong> additions</span>`)
	assert.Contains(t, got, `<span class="diff-removed"><strong>-2</strong> deletions</span>`)
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
	assert.Contains(t, got, `<span class="diff-added"><strong>+1</strong> addition</span>`)
	assert.Contains(t, got, `<span class="diff-removed"><strong>-2</strong> deletions</span>`)
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
	assert.Contains(t, got, `<dl class="file-tree-overview" aria-label="File tree overview"><div><dt>Entries</dt><dd>3</dd></div><div><dt>Max depth</dt><dd>1</dd></div></dl>`)
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
	assert.Contains(t, got, `<dl class="record-cards-overview" aria-label="Record cards overview"><div><dt>Cards</dt><dd>2</dd></div><div><dt>Visible fields</dt><dd>4</dd></div></dl>`)
	assert.Contains(t, got, `<h3>Record 1: has-null</h3>`)
	assert.Contains(t, got, `<h3>Record 2: missing</h3>`)
	assert.Contains(t, got, cardField("name", "has-null"))
	assert.Contains(t, got, cardField("value", "null"))
	assert.Contains(t, got, cardField("owner", "Ops"))
	assert.NotContains(t, got, cardField("owner", ""))
	assert.NotContains(t, got, cardField("value", ""))
}

func TestRenderReport_RecordCardsShowEmptyState(t *testing.T) {
	t.Parallel()

	src := []byte("name,score\n")
	analysis := report.Analysis{
		Kind:       report.KindCSVRecords,
		Confidence: 0.8,
		Reasons:    []string{"csv header"},
		Stats:      report.Stats{Bytes: len(src), Lines: 1, Records: 0, Fields: 2},
		Data:       [][]string{{"name", "score"}},
	}
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindCSVRecords,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentRecordCards, Source: "records", Title: "Details", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "empty"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<p class="record-empty" aria-live="polite">No records</p>`)
	assert.NotContains(t, got, `<div class="record-cards">`)
	assert.NotContains(t, got, `<article class="record-card">`)
}

func TestRenderReport_RecordCardsPreferTitleFields(t *testing.T) {
	t.Parallel()

	src := []byte(`[{"id":"A-1","title":"First Title"},{"id":"B-2","key":"fallback-key"},{"key":"C-3","value":"kept"},{"value":"untitled"}]`)
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

	assert.Contains(t, got, `<h3>Record 1: First Title</h3>`)
	assert.Contains(t, got, `<h3>Record 2: B-2</h3>`)
	assert.Contains(t, got, `<h3>Record 3: C-3</h3>`)
	assert.Contains(t, got, `<h3>Record 4</h3>`)
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
	assert.Contains(t, got, `<pre class="json-source"><code class="language-json">`)
	assert.Contains(t, got, `9007199254740993`)
	assert.NotContains(t, got, `9007199254740992`)
}

func TestRenderReport_JSONObjectShowsOverview(t *testing.T) {
	t.Parallel()

	src := []byte(`{"name":"alpha","score":10,"tags":["qa","html"],"meta":{"ok":true},"active":false}`)
	analysis, plan := report.Plan(t.Context(), src, report.Options{SourceName: "object.json", Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "object"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dd>json-object</dd>`)
	assert.Contains(t, got, `<dl class="json-overview" aria-label="JSON overview">`)
	assert.Contains(t, got, `<dt>active</dt><dd>boolean</dd>`)
	assert.Contains(t, got, `<dt>meta</dt><dd>object (1)</dd>`)
	assert.Contains(t, got, `<dt>name</dt><dd>string</dd>`)
	assert.Contains(t, got, `<dt>score</dt><dd>number</dd>`)
	assert.Contains(t, got, `<dt>tags</dt><dd>array (2)</dd>`)
	assert.Contains(t, got, `<pre class="json-source"><code class="language-json">`)
	assert.Contains(t, got, `&#34;score&#34;: 10`)
}

func TestRenderReport_JSONArrayShowsOverview(t *testing.T) {
	t.Parallel()

	src := []byte(`[1,"two",true]`)
	analysis, plan := report.Plan(t.Context(), src, report.Options{SourceName: "array.json", Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "array"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<dd>json-object</dd>`)
	assert.Contains(t, got, `<div class="json-overview" aria-label="JSON overview"><span><strong>3</strong> items</span><span>array (3)</span></div>`)
	assert.Contains(t, got, `<pre class="json-source"><code class="language-json">`)
}

func TestRenderReport_JSONLinesTableUsesAnalyzedRecords(t *testing.T) {
	t.Parallel()

	src := []byte("{\"name\":\"a\",\"score\":1}\n{\"name\":\"b\",\"score\":2}\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{SourceName: "events.jsonl", Planner: report.PlannerOff})

	got, err := RenderReport(src, Options{FallbackTitle: "events"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `class="report-table"`)
	assert.Contains(t, got, `<th scope="col">`+sortHeaderButton("name")+`</th>`)
	assert.Contains(t, got, `<th scope="col">`+sortHeaderButton("score")+`</th>`)
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

	assert.Contains(t, got, sortHeaderButton("tag")+`<`)
	assert.Contains(t, got, sortHeaderButton("tag 2"))
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

	assert.Contains(t, got, sortHeaderButton("tag"))
	assert.Contains(t, got, sortHeaderButton("tag 2"))
	assert.Contains(t, got, sortHeaderButton("tag 2 2"))
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

	assert.Contains(t, got, sortHeaderButton("Column 2"))
	assert.NotContains(t, got, `data-sort-label=""`)
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

	assert.Contains(t, got, sortHeaderButton(" name"))
	assert.Contains(t, got, labeledCell(" name", " Alpha")+labeledCell("score", "1 "))
	assert.NotContains(t, got, sortHeaderButton("name"))
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

	assert.Contains(t, got, sortHeaderButton("name"))
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

	assert.Contains(t, got, sortHeaderButton("name"))
	assert.Contains(t, got, sortHeaderButton("name 2"))
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

func TestRenderReport_ASCIITableUsesAnalyzerRows(t *testing.T) {
	t.Parallel()

	src := []byte("+----+-------+\n| id | name  |\n+----+-------+\n| 1  | alpha |\n| 2  | beta  |\n+----+-------+\n")
	analysis := report.Analyze(src, "mysql.out")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindTableRecords,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeDataBrowser,
		Components: []report.Component{
			{Type: report.ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{}},
		},
	}

	require.Equal(t, report.KindTableRecords, analysis.Kind)
	require.Equal(t, 2, analysis.Stats.Records)

	got, err := RenderReport(src, Options{FallbackTitle: "mysql"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `<p class="report-filter-status" aria-live="polite">2 rows</p>`)
	assert.Contains(t, got, labeledCell("id", "1")+labeledCell("name", "alpha"))
	assert.Contains(t, got, labeledCell("id", "2")+labeledCell("name", "beta"))
	assert.NotContains(t, got, `+----+-------+`)
}

func TestRenderReport_SlidesSplitsMarkdownByH2(t *testing.T) {
	t.Parallel()

	src := []byte("# Deck\n\n## One\n\nfirst section\n\n## Two\n\nsecond section\n")
	analysis := report.Analyze(src, "deck.md")
	plan := report.ReportPlan{
		Version:    report.PlanVersion,
		Kind:       report.KindMarkdown,
		Layout:     report.LayoutSlides,
		Mode:       report.ModeReader,
		Components: []report.Component{{Type: report.ComponentArticle, Source: "input", Title: "Deck", Options: map[string]string{}}},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "deck"}, analysis, plan)
	require.NoError(t, err)

	// h1+intro slide, then one slide per h2 = 3 slides.
	assert.Equal(t, 3, strings.Count(got, `class="report-slide"`), "intro + one slide per h2")
	assert.Contains(t, got, `aria-label="Slide 1 of 3: Deck"`)
	assert.Contains(t, got, `aria-label="Slide 2 of 3: One"`)
	assert.Contains(t, got, `aria-label="Slide 3 of 3: Two"`)
	assert.Contains(t, got, `<div class="report-slide-count">2 / 3</div>`)
}

func TestRenderReport_ReviewCardsRenderTextareasAndCopy(t *testing.T) {
	t.Parallel()

	src := []byte(`[{"name":"alpha","note":"first"},{"name":"beta","note":"second"}]`)
	analysis := report.Analyze(src, "values.json")
	plan := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindJSONRecords,
		Layout:  report.LayoutReview,
		Mode:    report.ModeReview,
		Components: []report.Component{
			{Type: report.ComponentReview, Source: "records", Title: "Review", Options: map[string]string{}},
		},
	}

	got, err := RenderReport(src, Options{FallbackTitle: "values"}, analysis, plan)
	require.NoError(t, err)

	assert.Contains(t, got, `class="review-card"`)
	assert.Contains(t, got, `class="review-copy"`)
	assert.Contains(t, got, `<textarea class="review-comment" data-review-id="alpha"`)
	assert.Contains(t, got, `<textarea class="review-comment" data-review-id="beta"`)
	assert.Contains(t, got, cardField("note", "first"))
}
