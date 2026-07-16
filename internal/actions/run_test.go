package actions

import (
	"encoding/base64"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dotcommander/html/internal/cache"
	"github.com/dotcommander/html/internal/render"
	"github.com/dotcommander/html/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_NoOpen(t *testing.T) {
	t.Parallel()

	// Write a minimal markdown source into a temp file.
	f, err := os.CreateTemp(t.TempDir(), "src*.md")
	require.NoError(t, err)
	_, err = f.WriteString("# Test Heading\n\nA paragraph.\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// Cleanup the cache entry this test creates.
	t.Cleanup(func() {
		p, cerr := cache.PathFor(f.Name())
		if cerr == nil {
			_ = os.Remove(p)
			_ = os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})

	path, err := Run(Options{File: f.Name(), NoOpen: true})
	require.NoError(t, err)
	assert.NotEmpty(t, path)

	// Cache file must exist and contain rendered HTML.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(data), "<!DOCTYPE html>"),
		"expected rendered HTML in cache file")
}

func TestRun_CachedFileRebasesRelativeLinks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "doc.md")
	require.NoError(t, os.WriteFile(src, []byte("[next](next%20page.md?raw=1#details)\n"), 0o644))
	t.Cleanup(func() {
		p, err := cache.PathFor(src)
		if err == nil {
			_ = os.Remove(p)
			_ = os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})

	path, err := Run(Options{File: src, NoOpen: true, Force: true})
	require.NoError(t, err)
	html := readRenderedFile(t, path)
	want := (&url.URL{
		Scheme:   "file",
		Path:     filepath.ToSlash(filepath.Join(dir, "next page.md")),
		RawQuery: "raw=1",
		Fragment: "details",
	}).String()
	assert.Contains(t, html, `href="`+want+`"`)
}

func TestRun_ExplicitOutputKeepsRelativeLinks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "doc.md")
	out := filepath.Join(dir, "out.html")
	require.NoError(t, os.WriteFile(src, []byte("[next](next.md#details)\n"), 0o644))

	_, err := RunWithResult(Options{File: src, Output: out, NoOpen: true})
	require.NoError(t, err)
	assert.Contains(t, readRenderedFile(t, out), `href="next.md#details"`)

	fileStdout, err := RunWithResult(Options{File: src, Stdout: true, NoOpen: true})
	require.NoError(t, err)
	assert.Contains(t, fileStdout.Stdout, `href="next.md#details"`)

	stdinStdout, err := RunWithResult(Options{
		Stdin:    strings.NewReader("[next](next.md#details)\n"),
		Markdown: true,
		Stdout:   true,
		NoOpen:   true,
	})
	require.NoError(t, err)
	assert.Contains(t, stdinStdout.Stdout, `href="next.md#details"`)
}

func TestRun_SafeCachedFileKeepsRelativeLinks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "doc.md")
	require.NoError(t, os.WriteFile(src, []byte("[next](next.md#details)\n"), 0o644))
	t.Cleanup(func() {
		p, err := cache.PathFor(src)
		if err == nil {
			_ = os.Remove(p)
			_ = os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})

	path, err := Run(Options{File: src, Safe: true, NoOpen: true, Force: true})
	require.NoError(t, err)
	assert.Contains(t, readRenderedFile(t, path), `href="next.md#details"`)
}

func TestRun_CachedFileLinkBaseInvalidatesAcrossSymlinkAliases(t *testing.T) {
	t.Parallel()

	realDir := t.TempDir()
	source := filepath.Join(realDir, "doc.md")
	require.NoError(t, os.WriteFile(source, []byte("[next](next.md)\n"), 0o644))

	aliasDir := t.TempDir()
	alias := filepath.Join(aliasDir, "doc.md")
	if err := os.Symlink(source, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	t.Cleanup(func() {
		p, err := cache.PathFor(source)
		if err == nil {
			_ = os.Remove(p)
			_ = os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})

	aliasCache, err := Run(Options{File: alias, NoOpen: true, Force: true})
	require.NoError(t, err)
	aliasURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(aliasDir, "next.md"))}).String()
	assert.Contains(t, readRenderedFile(t, aliasCache), `href="`+aliasURL+`"`)

	realCache, err := Run(Options{File: source, NoOpen: true})
	require.NoError(t, err)
	require.Equal(t, aliasCache, realCache, "symlink aliases deliberately share one cache path")
	realURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(realDir, "next.md"))}).String()
	realHTML := readRenderedFile(t, realCache)
	assert.Contains(t, realHTML, `href="`+realURL+`"`)
	assert.NotContains(t, realHTML, `href="`+aliasURL+`"`)
}

func TestRun_CachedReportRebasesRelativeLinks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "doc.md")
	require.NoError(t, os.WriteFile(src, []byte("# Report\n\n[next](next.md#details)\n"), 0o644))
	t.Cleanup(func() {
		p, err := cache.PathFor(src)
		if err == nil {
			_ = os.Remove(p)
			_ = os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})

	res, err := RunWithResult(Options{
		File:    src,
		Report:  true,
		Planner: report.PlannerOff,
		NoOpen:  true,
		Force:   true,
	})
	require.NoError(t, err)
	want := (&url.URL{
		Scheme:   "file",
		Path:     filepath.ToSlash(filepath.Join(dir, "next.md")),
		Fragment: "details",
	}).String()
	assert.Contains(t, readRenderedFile(t, res.Path), `href="`+want+`"`)
}

func TestRun_OutputIncludesConfiguredPalette(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	out := filepath.Join(dir, "out.html")
	res, err := RunWithResult(Options{
		Stdin:    strings.NewReader("# Palette\n\nBody\n"),
		Output:   out,
		Markdown: true,
		Palette:  "catppuccin",
		NoOpen:   true,
	})
	require.NoError(t, err)
	require.Equal(t, out, res.Path)

	html := readRenderedFile(t, out)
	assert.Contains(t, html, `HTML_DEFAULT_PALETTE = "catppuccin"`)
	assert.Contains(t, html, `data-palette-choice="catppuccin"`)
	assert.Contains(t, html, `class="palette-switcher"`)
}

func TestRun_OutputIncludesConfiguredCodeTheme(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	out := filepath.Join(dir, "out.html")
	res, err := RunWithResult(Options{
		Stdin:     strings.NewReader("```go\npackage main\n```\n"),
		Output:    out,
		Markdown:  true,
		CodeTheme: "dracula",
		NoOpen:    true,
	})
	require.NoError(t, err)
	require.Equal(t, out, res.Path)

	html := readRenderedFile(t, out)
	assert.Contains(t, html, `class="chroma`)
	assert.Contains(t, html, "#282a36")
}

func TestRun_ThreadsImageDiagnosticsOnRenderAndCacheHit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "source.md")
	require.NoError(t, os.WriteFile(source, []byte("![missing](missing.png)\n"), 0o644))
	t.Cleanup(func() {
		if p, err := cache.PathFor(source); err == nil {
			_ = os.Remove(p)
			_ = os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})

	for _, force := range []bool{true, false} {
		res, err := RunWithResult(Options{File: source, NoOpen: true, Force: force})
		require.NoError(t, err)
		require.NotEmpty(t, res.Path)
		assert.Equal(t, []render.ImageDiagnostic{{
			Code:        render.DiagnosticImageMissing,
			Destination: "missing.png",
		}}, res.Diagnostics)
	}
}

func TestRun_ThreadsImageDiagnosticsForTimelineOnlyReport(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "timeline.md")
	src := "## Steps\n\n1. Review ![missing](missing.png).\n2. Publish.\n"
	require.NoError(t, os.WriteFile(source, []byte(src), 0o644))
	t.Cleanup(func() {
		if p, err := cache.PathFor(source); err == nil {
			_ = os.Remove(p)
			_ = os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})

	res, err := RunWithResult(Options{File: source, Report: true, NoOpen: true, Force: true})
	require.NoError(t, err)
	assert.Equal(t, []render.ImageDiagnostic{{
		Code:        render.DiagnosticImageMissing,
		Destination: "missing.png",
	}}, res.Diagnostics)
}

func TestRun_RejectsOutputAliasesSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		outputPath func(t *testing.T, dir, source string) string
	}{
		{
			name: "lexical",
			outputPath: func(_ *testing.T, dir, _ string) string {
				return filepath.Join(dir, ".", "source.md")
			},
		},
		{
			name: "symlink",
			outputPath: func(t *testing.T, dir, source string) string {
				output := filepath.Join(dir, "source-link.html")
				require.NoError(t, os.Symlink(source, output))
				return output
			},
		},
		{
			name: "hardlink",
			outputPath: func(t *testing.T, dir, source string) string {
				output := filepath.Join(dir, "source-hardlink.html")
				require.NoError(t, os.Link(source, output))
				return output
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			source := filepath.Join(dir, "source.md")
			original := []byte("# Preserve me\n")
			require.NoError(t, os.WriteFile(source, original, 0o644))
			output := tt.outputPath(t, dir, source)

			_, err := RunWithResult(Options{File: source, Output: output, NoOpen: true})
			require.ErrorContains(t, err, "output path aliases source file")
			got, readErr := os.ReadFile(source)
			require.NoError(t, readErr)
			assert.Equal(t, original, got)
		})
	}
}

func TestRun_AtomicOutputPreservesExistingDestinationOnFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "source.md")
	output := filepath.Join(dir, "output.html")
	require.NoError(t, os.WriteFile(source, []byte("# Source\n"), 0o644))
	require.NoError(t, os.Mkdir(output, 0o755))
	sentinel := filepath.Join(output, "sentinel")
	require.NoError(t, os.WriteFile(sentinel, []byte("keep"), 0o644))

	_, err := RunWithResult(Options{File: source, Output: output, NoOpen: true})
	require.ErrorContains(t, err, "write output")
	got, readErr := os.ReadFile(sentinel)
	require.NoError(t, readErr)
	assert.Equal(t, "keep", string(got))

	entries, readDirErr := os.ReadDir(dir)
	require.NoError(t, readDirErr)
	for _, entry := range entries {
		assert.NotContains(t, entry.Name(), ".output.html.tmp-", "failed atomic write left a temporary file")
	}
}

func TestRun_OutputUsesStableMode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	output := filepath.Join(dir, "output.html")
	require.NoError(t, os.WriteFile(output, []byte("old"), 0o600))

	_, err := RunWithResult(Options{
		Stdin:  strings.NewReader("# Source\n"),
		Output: output,
		NoOpen: true,
	})
	require.NoError(t, err)
	info, err := os.Stat(output)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode()&0o777)
}

func TestRun_RejectsUnknownCodeTheme(t *testing.T) {
	t.Parallel()

	_, err := RunWithResult(Options{
		Stdin:     strings.NewReader("package main\n"),
		Plain:     true,
		CodeTheme: "not-a-style",
		NoOpen:    true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown chroma style "not-a-style"`)
}

func TestRun_ModeLogForcesStructuredLogView(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "notes.txt")
	out := filepath.Join(dir, "log.html")
	require.NoError(t, os.WriteFile(src, []byte("starting\nERROR forced view\n"), 0o644))

	res, err := RunWithResult(Options{
		File:    src,
		Output:  out,
		Report:  true,
		Mode:    report.ModeOverrideLog,
		Planner: report.PlannerOff,
		NoOpen:  true,
	})
	require.NoError(t, err)
	require.Equal(t, out, res.Path)

	html := readRenderedFile(t, out)
	assert.Contains(t, html, `<h2>Log</h2>`)
	assert.Contains(t, html, `class="log-lines"`)
	assert.Contains(t, html, `class="log-line log-error"`)
	assert.NotContains(t, html, `class="text-overview"`)
	assert.NotContains(t, html, `class="report-text"`)
}

func TestRun_ModeCodeForcesCodeOverview(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "notes.txt")
	out := filepath.Join(dir, "code.html")
	require.NoError(t, os.WriteFile(src, []byte("plain text\nwithout a source lexer\n"), 0o644))

	res, err := RunWithResult(Options{
		File:    src,
		Output:  out,
		Report:  true,
		Mode:    report.ModeOverrideCode,
		Planner: report.PlannerOff,
		NoOpen:  true,
	})
	require.NoError(t, err)
	require.Equal(t, out, res.Path)

	html := readRenderedFile(t, out)
	assert.Contains(t, html, `<h2>Code</h2>`)
	assert.Contains(t, html, `class="code-overview"`)
	assert.Contains(t, html, `<dt>Language</dt><dd>Plain text</dd>`)
	assert.NotContains(t, html, `class="text-overview"`)
	assert.NotContains(t, html, `class="report-text"`)
}

func TestRun_ReportBinaryOutputUsesSafePreview(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "logo.bin")
	out := filepath.Join(dir, "binary.html")
	require.NoError(t, os.WriteFile(src, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0xff, 'h', 't', 'm', 'l'}, 0o644))

	res, err := RunWithResult(Options{
		File:    src,
		Output:  out,
		Report:  true,
		Planner: report.PlannerOff,
		NoOpen:  true,
	})
	require.NoError(t, err)
	require.Equal(t, out, res.Path)

	html := readRenderedFile(t, out)
	assert.Contains(t, html, `<dt>Kind</dt><dd>binary</dd>`)
	assert.Contains(t, html, `<dl class="binary-overview" aria-label="Binary overview">`)
	assert.Contains(t, html, `<dt>Bytes</dt><dd>14</dd>`)
	assert.Contains(t, html, `<pre class="binary-preview" aria-label="Binary byte preview"><code>`)
	assert.Contains(t, html, `00000000  89 50 4e 47 0d 0a 1a 0a 00 ff 68 74 6d 6c`)
	assert.NotContains(t, html, "\x00")
}

func TestRun_ReportOutputCoversRepresentativeKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		fileName string
		want     []string
	}{
		{
			name:     "markdown",
			input:    "# Report\n\n## Section\n\nBody\n",
			fileName: "report.md",
			want:     []string{`class="article-overview"`, `<dt>Sections</dt><dd>1</dd>`, `<h1 id="report">Report`, `Section`},
		},
		{
			name:     "json-records",
			input:    `[{"name":"alpha","score":10},{"name":"beta","score":2}]`,
			fileName: "records.json",
			want:     []string{`<dt>Kind</dt><dd>json-records</dd>`, `data-report-table`, `alpha`},
		},
		{
			name:     "json-object",
			input:    `{"name":"alpha","score":10}`,
			fileName: "object.json",
			want:     []string{`<dt>Kind</dt><dd>json-object</dd>`, `<h2>JSON</h2>`, `class="json-overview"`, `<dt>score</dt><dd>number</dd>`, `&#34;score&#34;: 10`},
		},
		{
			name:     "csv-records",
			input:    "name,score\nalpha,10\nbeta,2\n",
			fileName: "records.csv",
			want:     []string{`<dt>Kind</dt><dd>csv-records</dd>`, `data-label="name">alpha`, `2 rows`},
		},
		{
			name:     "tsv-records",
			input:    "name\tscore\nalpha\t10\nbeta\t2\n",
			fileName: "records.tsv",
			want:     []string{`<dt>Kind</dt><dd>tsv-records</dd>`, `data-label="score">10`, `2 rows`},
		},
		{
			name: "table-records",
			input: "+----+-------+\n" +
				"| id | name  |\n" +
				"+----+-------+\n" +
				"| 1  | alpha |\n" +
				"| 2  | beta  |\n" +
				"+----+-------+\n",
			fileName: "mysql.out",
			want:     []string{`<dt>Kind</dt><dd>table-records</dd>`, `data-label="name">alpha`, `2 rows`},
		},
		{
			name:     "diff",
			input:    "diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new\n",
			fileName: "change.patch",
			want:     []string{`<dt>Kind</dt><dd>diff</dd>`, `class="diff-summary"`, `class="diff-view"`, `class="add">+new`},
		},
		{
			name:     "source-code",
			input:    "package main\n\nfunc main() {}\n",
			fileName: "main.go",
			want:     []string{`<dt>Kind</dt><dd>source-code</dd>`, `<h2>Code</h2>`, `class="code-overview"`, `<dt>Language</dt><dd>Go</dd>`, `class="chroma light"`},
		},
		{
			name:     "tree-listing",
			input:    ".\n├── cmd\n│   └── html\n└── internal\n",
			fileName: "tree.txt",
			want:     []string{`<dt>Kind</dt><dd>tree-listing</dd>`, `class="file-tree"`, `internal`},
		},
		{
			name:     "log",
			input:    "2026-06-16 12:00:00 ERROR stop\n2026-06-16 12:00:01 INFO ok\n",
			fileName: "run.log",
			want:     []string{`<dt>Kind</dt><dd>log</dd>`, `<h2>Log</h2>`, `class="log-lines"`, `class="log-line log-error"`},
		},
		{
			name: "access-log",
			input: "127.0.0.1 - - [16/Jun/2026:12:00:00 -0400] \"GET /index.html HTTP/1.1\" 200 1234\n" +
				"127.0.0.1 - - [16/Jun/2026:12:00:01 -0400] \"POST /api HTTP/1.1\" 500 42\n",
			fileName: "access.log",
			want:     []string{`<dt>Kind</dt><dd>log</dd>`, `http access log markers`, `<dt>Errors</dt><dd>1</dd>`, `<dt>Info</dt><dd>1</dd>`, `class="log-line log-error"`},
		},
		{
			name:     "transcript",
			input:    "Host: Welcome back.\nGuest: Thanks for having me.\nHost: Let's begin.\n",
			fileName: "transcript.txt",
			want:     []string{`<dt>Kind</dt><dd>transcript</dd>`, `<h2>Transcript</h2>`, `class="transcript-turns"`, `<span class="transcript-speaker">Guest</span>`},
		},
		{
			name:     "mixed",
			input:    "Notes\n- check deploy\n\nPayload\n{\"ok\":true}\n\nERROR failed\n",
			fileName: "mixed.txt",
			want:     []string{`<dt>Kind</dt><dd>mixed</dd>`, `multiple weak format signals`, `title="Input"><span>Input</span></button>`, `class="text-overview"`},
		},
		{
			name:     "plain",
			input:    "ordinary prose with no structural markup\nsecond line\n",
			fileName: "notes.txt",
			want:     []string{`<dt>Kind</dt><dd>plain</dd>`, `<h2>Input</h2>`, `class="text-overview"`, `ordinary prose`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			src := filepath.Join(dir, tt.fileName)
			out := filepath.Join(dir, tt.name+".html")
			require.NoError(t, os.WriteFile(src, []byte(tt.input), 0o644))

			res, err := RunWithResult(Options{
				File:    src,
				Output:  out,
				Report:  true,
				Planner: report.PlannerOff,
				NoOpen:  true,
			})
			require.NoError(t, err)
			require.Equal(t, out, res.Path)

			html := readRenderedFile(t, out)
			assert.Contains(t, html, `<!DOCTYPE html>`)
			assert.Contains(t, html, `class="theme-controls"`)
			for _, palette := range []string{"sepia", "blue", "green", "rose", "catppuccin"} {
				assert.Contains(t, html, `data-palette-choice="`+palette+`"`)
			}
			for _, want := range tt.want {
				assert.Contains(t, html, want)
			}
		})
	}
}

func TestRun_ImageCacheInvalidatesWhenImageChanges(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "doc.md")
	img := filepath.Join(dir, "dot.png")
	oldImage := []byte("old-image-bytes")
	newImage := []byte("new-image-bytes")
	require.NoError(t, os.WriteFile(src, []byte("# Image\n\n![dot](dot.png)\n"), 0o644))
	require.NoError(t, os.WriteFile(img, oldImage, 0o644))
	t.Cleanup(func() {
		p, cerr := cache.PathFor(src)
		if cerr == nil {
			_ = os.Remove(p)
			_ = os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})

	path, err := Run(Options{File: src, NoOpen: true, Force: true})
	require.NoError(t, err)
	html := readRenderedFile(t, path)
	require.Contains(t, html, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(oldImage))

	require.NoError(t, os.WriteFile(img, newImage, 0o644))
	path, err = Run(Options{File: src, NoOpen: true})
	require.NoError(t, err)
	html = readRenderedFile(t, path)
	assert.Contains(t, html, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(newImage))
	assert.NotContains(t, html, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(oldImage))
}

func TestReportCacheTagIgnoresNonRenderedPlanMetadata(t *testing.T) {
	t.Parallel()

	analysis := report.Analysis{
		Kind:       report.KindPlain,
		Confidence: 0.62,
		Reasons:    []string{"no high-confidence structured format detected"},
		Stats:      report.Stats{Bytes: 10, Lines: 1},
	}
	base := report.ReportPlan{
		Version:    report.PlanVersion,
		Kind:       report.KindPlain,
		Layout:     report.LayoutSinglePage,
		Mode:       report.ModeBrief,
		Confidence: 0.1,
		Reasons:    []string{"first reason"},
		Components: []report.Component{
			{Type: report.ComponentPreformatted, Source: "input", Title: "Input", Options: map[string]string{}},
		},
		Planner: report.PlannerInfo{Name: "llm", Model: "test-model", Prompt: report.PlannerPromptVersion, Cache: "miss", Contributed: true},
	}
	metadataOnly := base
	metadataOnly.Kind = report.KindMixed
	metadataOnly.Mode = report.ModeConsole
	metadataOnly.Confidence = 0.9
	metadataOnly.Reasons = []string{"different reason"}
	metadataOnly.Planner = report.PlannerInfo{Name: "deterministic", Cache: "hit"}
	metadataOnly.Components = append([]report.Component(nil), base.Components...)
	metadataOnly.Components[0].Source = "records"
	metadataOnly.Components[0].Options = map[string]string{"ignored": "true"}

	opts := Options{Planner: report.PlannerLLM, LLMModel: "test-model"}
	require.Equal(t, reportCacheTag(analysis, base, opts), reportCacheTag(analysis, metadataOnly, opts))
}

func TestReportCacheTagIncludesRenderedAnalysisFields(t *testing.T) {
	t.Parallel()

	analysis := report.Analysis{
		Kind:       report.KindPlain,
		Confidence: 0.62,
		Reasons:    []string{"no high-confidence structured format detected"},
		Stats:      report.Stats{Bytes: 10, Lines: 1},
	}
	kindChanged := analysis
	kindChanged.Kind = report.KindMixed
	reasonChanged := analysis
	reasonChanged.Reasons = []string{"multiple weak format signals: markdown-like prose, json-like block"}
	statsChanged := analysis
	statsChanged.Stats = report.Stats{Bytes: 11, Lines: 2}
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

	base := reportCacheTag(analysis, plan, Options{})
	require.NotEqual(t, base, reportCacheTag(kindChanged, plan, Options{}))
	require.NotEqual(t, base, reportCacheTag(reasonChanged, plan, Options{}))
	require.NotEqual(t, base, reportCacheTag(statsChanged, plan, Options{}))
}

func TestReportCacheTagIncludesRenderedPlanFields(t *testing.T) {
	t.Parallel()

	analysis := report.Analysis{
		Kind:       report.KindPlain,
		Confidence: 0.62,
		Reasons:    []string{"no high-confidence structured format detected"},
		Stats:      report.Stats{Bytes: 10, Lines: 1},
	}
	base := report.ReportPlan{
		Version: report.PlanVersion,
		Kind:    report.KindPlain,
		Layout:  report.LayoutSinglePage,
		Mode:    report.ModeBrief,
		Components: []report.Component{
			{Type: report.ComponentPreformatted, Source: "input", Title: "Input", Options: map[string]string{}},
		},
	}
	tabs := base
	tabs.Layout = report.LayoutTabbedPage
	slides := base
	slides.Layout = report.LayoutSlides
	titled := base
	titled.Components = append([]report.Component(nil), base.Components...)
	titled.Components[0].Title = "Different"
	typed := base
	typed.Components = append([]report.Component(nil), base.Components...)
	typed.Components[0].Type = report.ComponentCodeBlock
	withSummary := base
	withSummary.Components = append([]report.Component{
		{Type: report.ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}},
	}, base.Components...)

	require.NotEqual(t, reportCacheTag(analysis, base, Options{}), reportCacheTag(analysis, tabs, Options{}))
	require.NotEqual(t, reportCacheTag(analysis, base, Options{}), reportCacheTag(analysis, slides, Options{}))
	require.NotEqual(t, reportCacheTag(analysis, tabs, Options{}), reportCacheTag(analysis, slides, Options{}))
	require.NotEqual(t, reportCacheTag(analysis, base, Options{}), reportCacheTag(analysis, titled, Options{}))
	require.NotEqual(t, reportCacheTag(analysis, base, Options{}), reportCacheTag(analysis, typed, Options{}))
	require.NotEqual(t, reportCacheTag(analysis, base, Options{}), reportCacheTag(analysis, withSummary, Options{}))
}

func readRenderedFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}
