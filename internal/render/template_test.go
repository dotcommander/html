package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dotcommander/html/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateData(t *testing.T) {
	t.Parallel()
	source := `{{define "item"}}<li>{{.name}}: {{.details.count}}{{if .active}} active{{else}} archived{{end}}</li>{{end}}<title>{{.Title}}</title><ul>{{range .Data.items}}{{template "item" .}}{{end}}</ul>`
	data := `{"items":[{"name":"<script>bad</script>","details":{"count":9007199254740993},"active":true},{"name":"B & C","details":{"count":2},"active":false}]}`
	for _, safe := range []bool{false, true} {
		got, err := Render([]byte(data), Options{Plain: true, Safe: safe, Template: "custom", TemplateSource: source, FallbackTitle: `<b>A & B</b>`})
		require.NoError(t, err)
		assert.Equal(t, `<title>&lt;b&gt;A &amp; B&lt;/b&gt;</title><ul><li>&lt;script&gt;bad&lt;/script&gt;: 9007199254740993 active</li><li>B &amp; C: 2 archived</li></ul>`, got)
	}
}

func TestTemplateDataIsOnlyCompleteJSON(t *testing.T) {
	t.Parallel()
	for _, src := range []string{`# Markdown`, `{"x":`, `{} {}`, `1 trailing`, "{\"x\":1}\n{\"x\":2}"} {
		got, err := Render([]byte(src), Options{Template: "custom", TemplateSource: `{{if .Data}}data{{else}}no data{{end}}`})
		require.NoError(t, err)
		assert.Equal(t, "no data", got, src)
	}
	for _, src := range []string{`9007199254740993`, `true`, `"hello"`, `[1,2]`} {
		got, err := Render([]byte(src), Options{Template: "custom", TemplateSource: `{{.Data}}`})
		require.NoError(t, err)
		assert.NotEmpty(t, got)
	}
}

func TestTemplateErrorsReturnNoOutput(t *testing.T) {
	t.Parallel()
	for _, source := range []string{`prefix{{if}}`, `prefix{{.Data.missing}}`, `prefix{{index .Data "missing"}}`, `prefix{{template "missing" .}}`, `{{trustHTML .Data}}`, `{{.Unknown}}`} {
		got, err := Render([]byte(`{"present":true}`), Options{Template: "custom", TemplateSource: source})
		require.Error(t, err, source)
		assert.Empty(t, got)
	}
}

func TestTemplateIndexAndContextualEscaping(t *testing.T) {
	t.Parallel()
	src := []byte(`{"items":[{"hyphen-key":"<tag>","empty":null}],"url":"javascript:alert(1)","text":"</script><script>bad()</script>"}`)
	for _, safe := range []bool{false, true} {
		got, err := Render(src, Options{Safe: safe, Template: "custom", TemplateSource: `<p>{{index .Data "items" 0 "hyphen-key"}}</p><span>{{index .Data "items" 0 "empty"}}</span><a href="{{.Data.url}}">link</a><script>const value = {{.Data.text}};</script>`})
		require.NoError(t, err)
		assert.Contains(t, got, `<p>&lt;tag&gt;</p>`)
		assert.Contains(t, got, `<span></span>`)
		assert.Contains(t, got, `href="#ZgotmplZ"`)
		assert.NotContains(t, got, `</script><script>bad()`)
	}
	for _, source := range []string{`{{index .Data "items" 0 "missing"}}`, `{{index .Data "items" 2}}`, `{{index .Data "items" -1}}`, `{{index .Data "items" "zero"}}`, `{{index .Data "items" 0 "empty" "missing"}}`} {
		got, err := Render(src, Options{Template: "custom", TemplateSource: source})
		require.Error(t, err)
		assert.Empty(t, got)
	}
}

func TestTemplateOutputLimit(t *testing.T) {
	t.Parallel()
	for _, source := range []string{strings.Repeat("x", 33), `prefix{{.Title}}`, `{{range .Data}}123456789{{end}}`} {
		got, err := executePageTemplate(source, templateContext{Title: strings.Repeat("x", 40), Data: []int{1, 2, 3, 4}}, 32)
		require.ErrorContains(t, err, "output exceeds")
		assert.Empty(t, got)
	}
	got, err := executePageTemplate(strings.Repeat("x", 32), templateContext{}, 32)
	require.NoError(t, err)
	assert.Len(t, got, 32)
}

func TestTemplateSlots(t *testing.T) {
	t.Parallel()
	toc := true
	src := []byte("# Title & more\n\n## Section\n\n**Content**\n\n> [!NOTE]\n> A note.\n")
	got, diagnostics, err := RenderWithDiagnostics(src, Options{Template: "custom", TemplateSource: `<!DOCTYPE html><html><head>{{.Head}}</head><body>{{.Controls}}<aside>{{.TOC}}</aside><article class="markdown-body">{{.Content}}</article>{{.Scripts}}</body></html>`, TOC: &toc})
	require.NoError(t, err)
	assert.Empty(t, diagnostics)
	assert.Contains(t, got, `<title>Title &amp; more</title>`)
	assert.Contains(t, got, `<aside><nav class="toc"`)
	assert.Equal(t, 1, strings.Count(got, `<nav class="toc"`))
	assert.Contains(t, got, `<strong>Content</strong>`)
	assert.Contains(t, got, `id="theme-toggle"`)
	assert.Contains(t, got, "copy.js")
	assert.Contains(t, got, "headings.js")
	assert.Contains(t, got, ".markdown-alert-note")
	assert.NotContains(t, got, "&lt;strong&gt;")
}

func TestTemplateSafeModeAndFraming(t *testing.T) {
	t.Parallel()
	got, diagnostics, err := RenderWithDiagnostics([]byte("# Safe\n\n![image](private.png)\n\n<script>bad()</script>"), Options{Safe: true, Template: "custom", TemplateSource: `<script>trusted()</script>{{.Content}}`, SourceDir: t.TempDir()})
	require.NoError(t, err)
	assert.Empty(t, diagnostics)
	assert.Contains(t, got, `<script>trusted()</script>`)
	assert.NotContains(t, got, "bad()")
	assert.NotContains(t, got, `<img`)
	assert.Contains(t, got, "[Image: image]")
	got, err = Render([]byte("<plain>"), Options{Plain: true, Frame: true, Lang: "text", FallbackTitle: "A&B", Template: "custom", TemplateSource: `{{.Content}}`})
	require.NoError(t, err)
	assert.Contains(t, got, `class="term-frame"`)
	assert.Contains(t, got, `A&amp;B`)
	assert.Contains(t, got, `&lt;plain&gt;`)
}

func TestTemplateReportExecutesOnce(t *testing.T) {
	t.Parallel()
	src := []byte("# Report\n\n## One\n\nText.\n\n## Two\n\nMore text.\n")
	for _, layout := range []report.LayoutOverride{report.LayoutOverrideSingle, report.LayoutOverrideTabs, report.LayoutOverrideSlides} {
		analysis, plan := report.Plan(t.Context(), src, report.Options{SourceName: "report.md", Layout: layout, Planner: report.PlannerOff})
		toc := true
		got, err := RenderReport(src, Options{Template: "custom", TemplateSource: `PAGE{{.Content}}END{{.TOC}}`, TOC: &toc}, analysis, plan)
		require.NoError(t, err)
		assert.Equal(t, 1, strings.Count(got, "PAGE"))
		assert.Equal(t, 1, strings.Count(got, "END"))
		assert.Contains(t, got, `<h1 id="report">Report</h1>`)
		assert.Equal(t, 1, strings.Count(got, `<nav class="toc"`))
	}
}

func TestReadingTemplates(t *testing.T) {
	t.Parallel()
	for _, selector := range []string{"reader", "notebook"} {
		for _, src := range []string{"Unheaded prose.", "# Heading\n\n## First\n\n> [!NOTE]\n> Remember.\n\n## Second\n\n```go\nvar x = 1\n```"} {
			toc := true
			got, err := Render([]byte(src), Options{Template: selector, TOC: &toc, FallbackTitle: "Notes"})
			require.NoError(t, err)
			assert.Contains(t, got, selector+"-page")
			assert.Contains(t, got, `id="theme-toggle"`)
			assert.Contains(t, got, "@media print")
			assert.Equal(t, 1, strings.Count(got, "<!DOCTYPE html>"))
			if strings.Contains(src, "NOTE") {
				assert.Equal(t, 1, strings.Count(got, `class="markdown-alert markdown-alert-note"`))
				assert.Equal(t, 1, strings.Count(got, `<nav class="toc"`))
			}
		}
		toc := false
		got, err := Render([]byte("# Heading\n\n## Section"), Options{Template: selector, TOC: &toc})
		require.NoError(t, err)
		assert.NotContains(t, got, `<nav class="toc"`)
		for _, opts := range []Options{{Template: selector, Plain: true}, {Template: selector, Frame: true}} {
			got, err := Render([]byte("data"), opts)
			require.ErrorContains(t, err, "ordinary Markdown")
			assert.Empty(t, got)
		}
		_, err = RenderReport([]byte("# Markdown"), Options{Template: selector}, report.Analysis{}, report.ReportPlan{})
		require.ErrorContains(t, err, "ordinary Markdown")
	}
}

func TestDefaultTemplateCompatibility(t *testing.T) {
	t.Parallel()
	src := []byte("# Default\n\n## Section\n\nParagraph.")
	want, err := Render(src, Options{})
	require.NoError(t, err)
	got, err := Render(src, Options{Template: "default"})
	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, Fingerprint(Options{}), Fingerprint(Options{Template: "default"}))
	assert.NotEqual(t, Fingerprint(Options{Template: "custom", TemplateSource: "A"}), Fingerprint(Options{Template: "custom", TemplateSource: "B"}))
}

func TestReusableTemplateExamples(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"catalog", "comparison"} {
		for _, dataset := range []string{"instruments", "field-kits"} {
			source, err := os.ReadFile(filepath.Join("..", "..", "examples", "templates", name+".html.tmpl"))
			require.NoError(t, err)
			data, err := os.ReadFile(filepath.Join("..", "..", "examples", "templates", dataset+".json"))
			require.NoError(t, err)
			got, err := Render(data, Options{Plain: true, Template: name, TemplateSource: string(source)})
			require.NoError(t, err)
			assert.Contains(t, got, `<!DOCTYPE html>`)
			assert.Contains(t, got, `data-example="`+name+`"`)
			assert.NotContains(t, got, "ZgotmplZ")
		}
	}
}
