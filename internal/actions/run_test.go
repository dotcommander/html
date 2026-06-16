package actions

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dotcommander/html/internal/cache"
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
	require.Equal(t, reportCacheTag(base, opts), reportCacheTag(metadataOnly, opts))
}

func TestReportCacheTagIncludesRenderedPlanFields(t *testing.T) {
	t.Parallel()

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

	require.NotEqual(t, reportCacheTag(base, Options{}), reportCacheTag(tabs, Options{}))
	require.NotEqual(t, reportCacheTag(base, Options{}), reportCacheTag(slides, Options{}))
	require.NotEqual(t, reportCacheTag(tabs, Options{}), reportCacheTag(slides, Options{}))
	require.NotEqual(t, reportCacheTag(base, Options{}), reportCacheTag(titled, Options{}))
	require.NotEqual(t, reportCacheTag(base, Options{}), reportCacheTag(typed, Options{}))
}

func readRenderedFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}
