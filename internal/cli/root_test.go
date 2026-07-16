package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dotcommander/html/internal/cache"
	"github.com/dotcommander/html/internal/report"
)

func TestRoot_PlainMarkdownMutuallyExclusive(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--plain", "--markdown", "-n"})
	cmd.SetIn(strings.NewReader("hello"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected --plain/--markdown mutual-exclusion error")
	}
}

func TestRoot_PipedPlainSmoke(t *testing.T) {
	t.Parallel()
	content := "cli-smoke-uniq\nplain line\n"
	t.Cleanup(func() {
		if p, e := cache.PathForContent([]byte(content)); e == nil {
			os.Remove(p)
			os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})
	cmd := newRootCmd()
	cmd.SetArgs([]string{"-n"})
	cmd.SetIn(strings.NewReader(content))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.String(), ".html") {
		t.Errorf("expected the cache path printed to stdout, got %q", out.String())
	}
}

func TestRoot_PrintsImageDiagnosticsToStderr(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "source.md")
	if err := os.WriteFile(source, []byte("![missing](missing.png)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if p, err := cache.PathFor(source); err == nil {
			_ = os.Remove(p)
			_ = os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})

	cmd := newRootCmd()
	cmd.SetArgs([]string{"-nf", source})
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.String(), ".html") {
		t.Fatalf("expected cache path on stdout, got %q", out.String())
	}
	want := `html: warning: [image-missing] image "missing.png" was not embedded` + "\n"
	if errOut.String() != want {
		t.Fatalf("stderr = %q, want %q", errOut.String(), want)
	}
}

func TestRoot_HelpExposesPlannerConfiguration(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--help"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "chart") {
		t.Fatalf("help should expose chart report mode, got:\n%s", got)
	}
	if !strings.Contains(got, "--planner") {
		t.Fatalf("help should still expose planner policy, got:\n%s", got)
	}
	for _, visible := range []string{"--llm-url", "--llm-model", "--llm-timeout", "--version"} {
		if !strings.Contains(got, visible) {
			t.Fatalf("help should expose %q, got:\n%s", visible, got)
		}
	}
}

func TestRoot_VersionIsSideEffectFree(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--version"})
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "html devel" {
		t.Fatalf("version = %q, want %q", got, "html devel")
	}
	if errOut.Len() != 0 {
		t.Fatalf("version wrote stderr: %q", errOut.String())
	}
}

func TestFormatVersion_TaggedBuild(t *testing.T) {
	t.Parallel()
	if got := formatVersion("v0.1.0"); got != "html v0.1.0" {
		t.Fatalf("formatVersion = %q", got)
	}
}

func TestLocalBuildVersion_PseudoVersion(t *testing.T) {
	t.Parallel()
	for _, version := range []string{
		"(devel)",
		"v0.0.0-20260709181523-9e8e331c8d68",
		"v1.2.4-0.20260709181523-9e8e331c8d68+dirty",
	} {
		if !localBuildVersion(version) {
			t.Fatalf("%q should be classified as a local build", version)
		}
	}
	if localBuildVersion("v0.1.0") {
		t.Fatal("release tag classified as local build")
	}
}

func TestRoot_GroupedBooleanShorthandsAndFlagsAfterFile(t *testing.T) {
	t.Parallel()
	source := filepath.Join(t.TempDir(), "input.md")
	if err := os.WriteFile(source, []byte("# grouped\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	cmd.SetArgs([]string{source, "-nf"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.String(), ".html") {
		t.Fatalf("expected cache path, got %q", out.String())
	}
}

func TestRoot_EndOfOptionsAllowsDashPrefixedInput(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.WriteFile("-notes.md", []byte("# notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	cmd.SetArgs([]string{"-n", "--", "-notes.md"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestRoot_DefaultPlannerDoesNotRequestReport(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--stdout"})
	cmd.SetIn(strings.NewReader("ambiguous plain input\n"))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.Contains(out.String(), `class="report-`) {
		t.Fatalf("default invocation unexpectedly selected report mode")
	}
}

func TestRoot_PlannerRequiresExplicitEndpointAndModel(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"--planner", "auto"},
		{"--planner", "llm", "--llm-url", "ftp://example.com", "--llm-model", "test"},
		{"--llm-url", "https://example.com/v1/chat/completions"},
	} {
		cmd := newRootCmd()
		cmd.SetArgs(args)
		cmd.SetIn(strings.NewReader("input\n"))
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := cmd.Execute(); err == nil {
			t.Fatalf("args %q unexpectedly succeeded", args)
		}
	}
}

func TestRoot_PlanPrintsReportPlanJSON(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--plan", "--planner", "off"})
	cmd.SetIn(strings.NewReader(`[{"name":"a","score":1},{"name":"b","score":2}]`))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var p report.ReportPlan
	if err := json.Unmarshal(out.Bytes(), &p); err != nil {
		t.Fatalf("plan json: %v\n%s", err, out.String())
	}
	if p.Kind != report.KindJSONRecords {
		t.Fatalf("kind = %s, want %s", p.Kind, report.KindJSONRecords)
	}
	if strings.Contains(out.String(), ".html") {
		t.Fatalf("--plan must not print a cache path: %q", out.String())
	}
}

func TestRoot_PlanAcceptsSlidesLayout(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--plan", "--layout", "slides", "--planner", "off"})
	cmd.SetIn(strings.NewReader(`[{"name":"a","score":1},{"name":"b","score":2}]`))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var p report.ReportPlan
	if err := json.Unmarshal(out.Bytes(), &p); err != nil {
		t.Fatalf("plan json: %v\n%s", err, out.String())
	}
	if p.Layout != report.LayoutSlides {
		t.Fatalf("layout = %s, want %s", p.Layout, report.LayoutSlides)
	}
}

func TestRoot_PlanOutputMutuallyExclusive(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--plan", "--output", filepath.Join(t.TempDir(), "plan.html"), "--planner", "off"})
	cmd.SetIn(strings.NewReader(`[{"name":"a","score":1}]`))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected --plan/--output mutual-exclusion error")
	}
	if !strings.Contains(err.Error(), "--plan and --output are mutually exclusive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_PlainReportFlagsMutuallyExclusive(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--plain", "--mode", "table"})
	cmd.SetIn(strings.NewReader("name,score\nAlpha,10\n"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected --plain/report mutual-exclusion error")
	}
	if !strings.Contains(err.Error(), "--plain and report flags are mutually exclusive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_PlainHiddenLLMFlagsMutuallyExclusive(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--plain", "--llm-url", "http://127.0.0.1:9/v1/chat/completions"})
	cmd.SetIn(strings.NewReader(`[{"name":"a","score":1}]`))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected --plain/report mutual-exclusion error")
	}
	if !strings.Contains(err.Error(), "--plain and report flags are mutually exclusive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_MarkdownReportFlagsMutuallyExclusive(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--markdown", "--mode", "table"})
	cmd.SetIn(strings.NewReader("# Title\n\nname,score\nAlpha,10\n"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected --markdown/report mutual-exclusion error")
	}
	if !strings.Contains(err.Error(), "--markdown and report flags are mutually exclusive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_StdoutOutputMutuallyExclusive(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--stdout", "--output", filepath.Join(t.TempDir(), "out.html")})
	cmd.SetIn(strings.NewReader("# title\n\nbody\n"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected --stdout/--output mutual-exclusion error")
	}
	if !strings.Contains(err.Error(), "--stdout and --output are mutually exclusive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_StdoutPrintsHTML(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--stdout", "--markdown"})
	cmd.SetIn(strings.NewReader(`# stdout

body
`))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	if !strings.HasPrefix(got, "<!DOCTYPE html>") {
		t.Fatalf("--stdout must print HTML, got %q", got[:min(len(got), 80)])
	}
	if strings.Contains(got, "/cache/") {
		t.Fatalf("--stdout must not print a cache path")
	}
	if !strings.Contains(got, `<h1 id="stdout">stdout</h1>`) || !strings.Contains(got, "<p>body</p>") {
		t.Fatalf("--stdout must preserve normal Markdown rendering, got:\n%s", got)
	}
	if strings.Contains(got, `class="language-plaintext"`) {
		t.Fatalf("--stdout must not switch Markdown into report/code rendering")
	}
}

func TestRoot_ReportStdoutPrintsReportHTML(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--stdout", "--planner", "off"})
	cmd.SetIn(strings.NewReader(`[{"name":"a","score":1},{"name":"b","score":2}]`))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	if !strings.HasPrefix(got, "<!DOCTYPE html>") {
		t.Fatalf("--stdout must print HTML, got %q", got[:min(len(got), 80)])
	}
	if !strings.Contains(got, `class="report-table"`) {
		t.Fatalf("explicit report flags must still render report HTML, got:\n%s", got)
	}
}

func TestRoot_LLMFlagsRequireExplicitPlanner(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--stdout", "--llm-url", "http://127.0.0.1:9/v1/chat/completions"})
	cmd.SetIn(strings.NewReader(`[{"name":"a","score":1},{"name":"b","score":2}]`))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "require explicit --planner") {
		t.Fatalf("expected explicit planner error, got %v", err)
	}
}

func TestRoot_OutputToFile(t *testing.T) {
	t.Parallel()
	sourceDir := t.TempDir()
	outputDir := t.TempDir()
	source := filepath.Join(sourceDir, "input.md")
	output := filepath.Join(outputDir, "output.html")
	content := "# output\n\nbody\n"
	if err := os.WriteFile(source, []byte(content), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	t.Cleanup(func() {
		if p, e := cache.PathFor(source); e == nil {
			os.Remove(p)
			os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})

	cmd := newRootCmd()
	cmd.SetArgs([]string{"-n", "-o", output, source})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := out.String(); strings.TrimSpace(got) != output {
		t.Fatalf("expected output path %q, got %q", output, got)
	}

	html, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !strings.Contains(string(html), "<html") {
		t.Fatalf("expected generated html in output file, got: %q", string(html))
	}
	if !strings.Contains(string(html), `<h1 id="output">output</h1>`) || !strings.Contains(string(html), "<p>body</p>") {
		t.Fatalf("expected normal Markdown render in output file, got: %q", string(html))
	}
	if strings.Contains(string(html), `class="report-`) {
		t.Fatalf("--output without report flags must not switch to report rendering")
	}
}

func TestRoot_OutputToStdout(t *testing.T) {
	t.Parallel()
	sourceDir := t.TempDir()
	source := filepath.Join(sourceDir, "input.md")
	content := "# output\n\nbody\n"
	if err := os.WriteFile(source, []byte(content), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	t.Cleanup(func() {
		if p, e := cache.PathFor(source); e == nil {
			os.Remove(p)
			os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})

	cmd := newRootCmd()
	cmd.SetArgs([]string{"-o", "-", source})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.String(), "<html") {
		t.Fatalf("expected HTML on stdout, got %q", out.String())
	}
}

func TestRoot_CodeThemeOutputToStdout(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--markdown", "--code-theme", "dracula", "--stdout"})
	cmd.SetIn(strings.NewReader("```go\npackage main\n```\n"))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `class="chroma`) {
		t.Fatalf("expected highlighted code block, got:\n%s", got)
	}
	if !strings.Contains(got, "#282a36") {
		t.Fatalf("expected dracula CSS in output, got:\n%s", got)
	}
}

func TestRoot_CodeThemeRejectsUnknownStyle(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--plain", "--code-theme", "not-a-style", "--stdout"})
	cmd.SetIn(strings.NewReader("package main\n"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected unknown code theme error")
	}
	if !strings.Contains(err.Error(), `unknown chroma style "not-a-style"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_FrameMarkdownMutuallyExclusive(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--frame", "--markdown", "-n"})
	cmd.SetIn(strings.NewReader("hello"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected --frame/--markdown mutual-exclusion error")
	}
	if !strings.Contains(err.Error(), "--frame and --markdown are mutually exclusive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoot_FrameWrapsPlainOutput(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--frame", "--stdout"})
	cmd.SetIn(strings.NewReader("plain frame line\n"))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `class="term-frame"`) {
		t.Fatalf("--frame must wrap output in a terminal-window frame, got:\n%s", got)
	}
	if !strings.Contains(got, "plain frame line") {
		t.Fatalf("--frame must preserve the body content, got:\n%s", got)
	}
}
