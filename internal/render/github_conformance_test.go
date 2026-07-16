package render

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const githubConformanceLiveEnv = "HTML_GITHUB_CONFORMANCE_LIVE"

var (
	classAttributeRE = regexp.MustCompile(`\bclass="([^"]*)"`)
	blockquoteRE     = regexp.MustCompile(`<blockquote(?:\s|>)`)
	tableRE          = regexp.MustCompile(`<table(?:\s|>)`)
	taskItemRE       = regexp.MustCompile(`<input\b[^>]*\btype="checkbox"`)
	strikethroughRE  = regexp.MustCompile(`<del(?:\s|>)`)
	headingRE        = regexp.MustCompile(`<h[1-6](?:\s|>)`)
	mermaidRE        = regexp.MustCompile(`<div\b[^>]*\bclass="[^"]*\bhighlight-source-mermaid\b`)
	mathRendererRE   = regexp.MustCompile(`<math-renderer(?:\s|>)`)
)

type conformanceSnapshot struct {
	AlertTypes     []string `json:"alert_types,omitempty"`
	Blockquotes    int      `json:"blockquotes,omitempty"`
	Tables         int      `json:"tables,omitempty"`
	TaskItems      int      `json:"task_items,omitempty"`
	Strikethroughs int      `json:"strikethroughs,omitempty"`
	Headings       int      `json:"headings,omitempty"`
	MermaidBlocks  int      `json:"mermaid_blocks,omitempty"`
	MathRenderers  int      `json:"math_renderers,omitempty"`
}

type conformanceExpectation struct {
	GitHub      conformanceSnapshot `json:"github"`
	Local       conformanceSnapshot `json:"local"`
	KnownDeltas []string            `json:"known_deltas"`
}

func TestGitHubMarkdownConformance(t *testing.T) {
	entries, err := filepath.Glob("testdata/github-conformance/*.md")
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	for _, markdownPath := range entries {
		markdownPath := markdownPath
		name := strings.TrimSuffix(filepath.Base(markdownPath), filepath.Ext(markdownPath))
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(markdownPath)
			require.NoError(t, err)
			expectation := readConformanceExpectation(t, strings.TrimSuffix(markdownPath, ".md")+".json")

			page, err := Render(source, Options{FallbackTitle: name})
			require.NoError(t, err)
			local := normalizeMarkdownHTML(page)
			assert.Equal(t, expectation.Local, local, "local semantic snapshot drifted")

			if os.Getenv(githubConformanceLiveEnv) != "1" {
				return
			}
			github := renderWithGitHub(t, source)
			assert.Equal(t, expectation.GitHub, github, "GitHub semantic snapshot drifted")
			checkConformanceDeltas(t, local, github, expectation.KnownDeltas)
		})
	}
}

func readConformanceExpectation(t *testing.T, path string) conformanceExpectation {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var expectation conformanceExpectation
	require.NoError(t, json.Unmarshal(data, &expectation))
	return expectation
}

func renderWithGitHub(t *testing.T, source []byte) conformanceSnapshot {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"text": string(source), "mode": "gfm"})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, "gh", "api", "--method", "POST", "-H", "Accept: application/vnd.github+json", "/markdown", "--input", "-")
	cmd.Stdin = bytes.NewReader(payload)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "gh api /markdown: %s", bytes.TrimSpace(output))
	return normalizeMarkdownHTML(string(output))
}

func normalizeMarkdownHTML(rendered string) conformanceSnapshot {
	var alertTypes []string
	for _, match := range classAttributeRE.FindAllStringSubmatch(rendered, -1) {
		classes := strings.Fields(match[1])
		if !slices.Contains(classes, "markdown-alert") {
			continue
		}
		for _, className := range classes {
			if alertType, ok := strings.CutPrefix(className, "markdown-alert-"); ok && alertType != "title" {
				alertTypes = append(alertTypes, alertType)
			}
		}
	}
	slices.Sort(alertTypes)
	return conformanceSnapshot{
		AlertTypes:     alertTypes,
		Blockquotes:    len(blockquoteRE.FindAllStringIndex(rendered, -1)),
		Tables:         len(tableRE.FindAllStringIndex(rendered, -1)),
		TaskItems:      len(taskItemRE.FindAllStringIndex(rendered, -1)),
		Strikethroughs: len(strikethroughRE.FindAllStringIndex(rendered, -1)),
		Headings:       len(headingRE.FindAllStringIndex(rendered, -1)),
		MermaidBlocks:  len(mermaidRE.FindAllStringIndex(rendered, -1)),
		MathRenderers:  len(mathRendererRE.FindAllStringIndex(rendered, -1)),
	}
}

type semanticDelta struct {
	field   string
	differs bool
}

func checkConformanceDeltas(t *testing.T, local, github conformanceSnapshot, known []string) {
	t.Helper()
	knownSet := make(map[string]bool, len(known))
	for _, field := range known {
		knownSet[field] = true
	}
	deltas := []semanticDelta{
		{"alert_types", !slices.Equal(local.AlertTypes, github.AlertTypes)},
		{"blockquotes", local.Blockquotes != github.Blockquotes},
		{"tables", local.Tables != github.Tables},
		{"task_items", local.TaskItems != github.TaskItems},
		{"strikethroughs", local.Strikethroughs != github.Strikethroughs},
		{"headings", local.Headings != github.Headings},
		{"mermaid_blocks", local.MermaidBlocks != github.MermaidBlocks},
		{"math_renderers", local.MathRenderers != github.MathRenderers},
	}
	for _, delta := range deltas {
		field, differs := delta.field, delta.differs
		switch {
		case differs && !knownSet[field]:
			t.Errorf("unexplained semantic delta: %s", field)
		case !differs && knownSet[field]:
			t.Logf("closed semantic delta: %s", field)
		case differs:
			t.Logf("known semantic delta: %s", field)
		}
	}
	for field := range knownSet {
		if !slices.ContainsFunc(deltas, func(delta semanticDelta) bool { return delta.field == field }) {
			t.Errorf("unknown known_deltas field %q; valid fields: %s", field, fmt.Sprint([]string{"alert_types", "blockquotes", "tables", "task_items", "strikethroughs", "headings", "mermaid_blocks", "math_renderers"}))
		}
	}
}
