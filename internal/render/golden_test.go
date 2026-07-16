package render

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dotcommander/html/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const updateGoldenEnv = "UPDATE_GOLDEN"

func TestRender_Golden(t *testing.T) {
	tests := []struct {
		name   string
		render func(*testing.T) string
	}{
		{name: "markdown", render: renderMarkdownGolden},
		{name: "plain", render: renderPlainGolden},
		{name: "report", render: renderReportGolden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := htmlSnapshot(t, tt.render(t))
			path := filepath.Join("testdata", "golden", tt.name+".snapshot")
			if os.Getenv(updateGoldenEnv) != "" {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte(snapshot), 0o644))
			}
			want, err := os.ReadFile(path)
			require.NoError(t, err, "missing golden; run %s=1 go test ./internal/render -run TestRender_Golden", updateGoldenEnv)
			assert.Equal(t, string(want), snapshot, "rendered output changed; review the article diff before updating the golden")
		})
	}
}

func renderMarkdownGolden(t *testing.T) string {
	t.Helper()
	toc := true
	src := []byte(`# Render Contract

## Overview

Portable output with **strong text**, ` + "`inline code`" + `, and a [link](https://example.com).

- [x] deterministic
- [ ] reviewable

## Data

| Name | Value |
|:-----|------:|
| alpha | 1 |
| beta | 22 |

` + "```go\nfmt.Println(\"stable\")\n```" + `
`)
	got, err := Render(src, Options{FallbackTitle: "render-contract", TOC: &toc})
	require.NoError(t, err)
	return got
}

func renderPlainGolden(t *testing.T) string {
	t.Helper()
	src := []byte("\x1b[32mPASS\x1b[0m render contract\nnext line\n")
	got, err := Render(src, Options{FallbackTitle: "command-output", Plain: true, Frame: true})
	require.NoError(t, err)
	return got
}

func renderReportGolden(t *testing.T) string {
	t.Helper()
	src := []byte("name,score,status\nalpha,10,ready\nbeta,2,review\n")
	analysis, plan := report.Plan(t.Context(), src, report.Options{
		SourceName: "scores.csv",
		Planner:    report.PlannerOff,
	})
	got, err := RenderReport(src, Options{FallbackTitle: "scores", SourceName: "scores.csv"}, analysis, plan)
	require.NoError(t, err)
	return got
}

func htmlSnapshot(t *testing.T, html string) string {
	t.Helper()
	article := snapshotArticle(t, html)
	digest := sha256.Sum256([]byte(html))
	return fmt.Sprintf("bytes=%d\nsha256=%x\n--- article ---\n%s\n", len(html), digest, article)
}

func snapshotArticle(t *testing.T, html string) string {
	t.Helper()
	const open = `<article class="markdown-body">`
	start := strings.Index(html, open)
	require.GreaterOrEqual(t, start, 0, "rendered document has no markdown article")
	relEnd := strings.Index(html[start:], "</article>")
	require.GreaterOrEqual(t, relEnd, 0, "rendered document has no closing article")
	return html[start : start+relEnd+len("</article>")]
}

type renderLCG struct {
	state uint64
}

func (r *renderLCG) next(n int) int {
	r.state = r.state*6364136223846793005 + 1442695040888963407
	return int(r.state>>32) % n
}

func TestRender_Metamorphic(t *testing.T) {
	blocks := []string{
		"# Heading\n",
		"Paragraph with **strong**, *emphasis*, `code`, and [link](https://example.com).\n",
		"> quoted\n> block\n",
		"- one\n- [x] done\n  - nested\n",
		"| Name | Value |\n|---|---:|\n| alpha | 1 |\n| beta | 2 |\n",
		"```text\nvalue <unsafe> & literal\n```\n",
		"---\n",
	}
	rng := renderLCG{state: 0xf0d0cafe5eed1234}
	for i := 0; i < 120; i++ {
		var src strings.Builder
		count := 1 + rng.next(7)
		for j := 0; j < count; j++ {
			if j > 0 {
				src.WriteByte('\n')
			}
			src.WriteString(blocks[rng.next(len(blocks))])
		}
		first, err := Render([]byte(src.String()), Options{FallbackTitle: "generated"})
		require.NoError(t, err, "case %d", i)
		second, err := Render([]byte(src.String()), Options{FallbackTitle: "generated"})
		require.NoError(t, err, "case %d repeat", i)
		assert.Equal(t, first, second, "case %d must be byte-deterministic", i)
		assert.True(t, strings.HasPrefix(first, "<!DOCTYPE html>"), "case %d doctype", i)
		assert.True(t, strings.HasSuffix(first, "</html>\n"), "case %d closing document", i)
		assert.Equal(t, 1, strings.Count(first, `<article class="markdown-body">`), "case %d article count", i)
	}
}

func TestRender_MetamorphicEquivalentLineEndings(t *testing.T) {
	lf := "# Title\n\nParagraph with **strong** text.\n\n- one\n- two\n"
	variants := []string{
		strings.ReplaceAll(lf, "\n", "\r\n"),
		strings.TrimSuffix(lf, "\n"),
	}
	want, err := Render([]byte(lf), Options{FallbackTitle: "line-endings"})
	require.NoError(t, err)
	for _, variant := range variants {
		got, err := Render([]byte(variant), Options{FallbackTitle: "line-endings"})
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
}

func TestRender_MetamorphicSafeModeNeutralizesRawMarkup(t *testing.T) {
	src := []byte("# Safe\n\n<script>alert('xss')</script>\n\n<iframe src=evil></iframe>\n\n**kept**\n")
	got, err := Render(src, Options{FallbackTitle: "safe", Safe: true})
	require.NoError(t, err)
	article := snapshotArticle(t, got)
	assert.NotContains(t, article, "<script")
	assert.NotContains(t, article, "<iframe")
	assert.NotContains(t, article, "alert('xss')")
	assert.Contains(t, article, "<strong>kept</strong>")
}
