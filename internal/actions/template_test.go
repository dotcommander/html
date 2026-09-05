package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dotcommander/html/internal/cache"
	"github.com/dotcommander/html/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_CustomTemplateWorksAcrossCompositions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	templatePath := filepath.Join(dir, "page.tmpl")
	require.NoError(t, os.WriteFile(templatePath, []byte(`<!doctype html><body data-template="custom">{{.Content}}</body>`), 0o644))

	for _, tt := range []struct {
		name string
		opts Options
	}{
		{
			name: "markdown",
			opts: Options{Stdin: strings.NewReader("# Source\n\nBody\n"), Markdown: true, Stdout: true, NoOpen: true},
		},
		{
			name: "plain",
			opts: Options{Stdin: strings.NewReader("plain text\n"), Plain: true, Stdout: true, NoOpen: true},
		},
		{
			name: "framed",
			opts: Options{Stdin: strings.NewReader("plain text\n"), Frame: true, Stdout: true, NoOpen: true},
		},
		{
			name: "report",
			opts: Options{Stdin: strings.NewReader("# Source\n\nBody\n"), Report: true, Planner: report.PlannerOff, Stdout: true, NoOpen: true},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.Template = templatePath
			res, err := RunWithResult(tt.opts)
			require.NoError(t, err)
			assert.Empty(t, res.Path)
			assert.Contains(t, res.Stdout, `data-template="custom"`)
		})
	}
}

func TestRun_CustomTemplateInvalidatesCacheWhenSourceChanges(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "source.md")
	templatePath := filepath.Join(dir, "page.tmpl")
	require.NoError(t, os.WriteFile(source, []byte("# Source\n\nBody\n"), 0o644))
	require.NoError(t, os.WriteFile(templatePath, []byte(`<!doctype html><body data-version="one">{{.Content}}</body>`), 0o644))
	cachePath, err := cache.PathFor(source)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Remove(cachePath)
		_ = os.Remove(strings.TrimSuffix(cachePath, ".html") + ".fp")
	})

	first, err := RunWithResult(Options{File: source, Template: templatePath, NoOpen: true, Force: true})
	require.NoError(t, err)
	assert.Equal(t, cachePath, first.Path)
	assert.Contains(t, readRenderedFile(t, first.Path), `data-version="one"`)

	require.NoError(t, os.WriteFile(templatePath, []byte(`<!doctype html><body data-version="two">{{.Content}}</body>`), 0o644))
	second, err := RunWithResult(Options{File: source, Template: templatePath, NoOpen: true})
	require.NoError(t, err)
	assert.Equal(t, cachePath, second.Path)
	assert.Contains(t, readRenderedFile(t, second.Path), `data-version="two"`)

	third, err := RunWithResult(Options{File: source, Template: templatePath, NoOpen: true})
	require.NoError(t, err)
	assert.Equal(t, cachePath, third.Path)
	assert.Contains(t, readRenderedFile(t, third.Path), `data-version="two"`)
}

func TestRun_TemplateFailuresPublishNothing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "source.md")
	templatePath := filepath.Join(dir, "broken.tmpl")
	output := filepath.Join(dir, "output.html")
	require.NoError(t, os.WriteFile(source, []byte("# Source\n"), 0o644))
	require.NoError(t, os.WriteFile(templatePath, []byte(`{{if .Title}}`), 0o644))
	require.NoError(t, os.WriteFile(output, []byte("preserve"), 0o644))
	cachePath, err := cache.PathFor(source)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Remove(cachePath)
		_ = os.Remove(strings.TrimSuffix(cachePath, ".html") + ".fp")
	})

	for _, tt := range []struct {
		name string
		opts Options
	}{
		{
			name: "explicit-output",
			opts: Options{File: source, Template: templatePath, Output: output, NoOpen: true},
		},
		{
			name: "stdout",
			opts: Options{File: source, Template: templatePath, Stdout: true, NoOpen: true},
		},
		{
			name: "cache",
			opts: Options{File: source, Template: templatePath, NoOpen: true, Force: true},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := RunWithResult(tt.opts)
			require.ErrorContains(t, err, "parse page template")
			assert.Empty(t, res.Path)
			assert.Empty(t, res.Stdout)
			got, readErr := os.ReadFile(output)
			require.NoError(t, readErr)
			assert.Equal(t, "preserve", string(got))
			_, statErr := os.Stat(cachePath)
			assert.True(t, os.IsNotExist(statErr), "invalid template must not write a cache entry")
		})
	}
}

func TestRun_RejectsOutputAliasesTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		outputPath func(t *testing.T, dir, templatePath string) string
	}{
		{
			name: "lexical",
			outputPath: func(_ *testing.T, dir, _ string) string {
				return filepath.Join(dir, ".", "page.tmpl")
			},
		},
		{
			name: "symlink",
			outputPath: func(t *testing.T, dir, templatePath string) string {
				output := filepath.Join(dir, "page-link.html")
				require.NoError(t, os.Symlink(templatePath, output))
				return output
			},
		},
		{
			name: "hardlink",
			outputPath: func(t *testing.T, dir, templatePath string) string {
				output := filepath.Join(dir, "page-hardlink.html")
				require.NoError(t, os.Link(templatePath, output))
				return output
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			templatePath := filepath.Join(dir, "page.tmpl")
			original := []byte(`<!doctype html><body>{{.Content}}</body>`)
			require.NoError(t, os.WriteFile(templatePath, original, 0o644))
			output := tt.outputPath(t, dir, templatePath)

			_, err := RunWithResult(Options{
				Stdin:    strings.NewReader("# Source\n"),
				Markdown: true,
				Template: templatePath,
				Output:   output,
				NoOpen:   true,
			})
			require.ErrorContains(t, err, "output path aliases template file")
			got, readErr := os.ReadFile(templatePath)
			require.NoError(t, readErr)
			assert.Equal(t, original, got)
		})
	}
}

func TestRun_RejectsOversizeTemplateBeforeOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	templatePath := filepath.Join(dir, "large.tmpl")
	output := filepath.Join(dir, "output.html")
	require.NoError(t, os.WriteFile(templatePath, make([]byte, maxTemplateBytes+1), 0o644))
	require.NoError(t, os.WriteFile(output, []byte("preserve"), 0o644))

	res, err := RunWithResult(Options{
		Stdin:    strings.NewReader("# Source\n"),
		Markdown: true,
		Template: templatePath,
		Output:   output,
		NoOpen:   true,
	})
	require.ErrorContains(t, err, "template file")
	require.ErrorContains(t, err, "cap is 1024 KiB")
	assert.Empty(t, res.Path)
	assert.Empty(t, res.Stdout)
	got, readErr := os.ReadFile(output)
	require.NoError(t, readErr)
	assert.Equal(t, "preserve", string(got))
}

func TestRun_ReadingTemplateRejectsIncompatibleModes(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		opts Options
	}{
		{name: "plain", opts: Options{Stdin: strings.NewReader("text\n"), Template: "reader", Plain: true, NoOpen: true}},
		{name: "frame", opts: Options{Stdin: strings.NewReader("text\n"), Template: "notebook", Frame: true, NoOpen: true}},
		{name: "report", opts: Options{Stdin: strings.NewReader("# Source\n"), Template: "reader", Report: true, NoOpen: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := RunWithResult(tt.opts)
			require.ErrorContains(t, err, "requires ordinary Markdown")
			assert.Empty(t, res.Path)
			assert.Empty(t, res.Stdout)
		})
	}
}
