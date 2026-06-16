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

func TestRoot_HelpHidesLLMPlumbing(t *testing.T) {
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
	if !strings.Contains(got, "--planner") {
		t.Fatalf("help should still expose planner policy, got:\n%s", got)
	}
	for _, hidden := range []string{"--llm-url", "--llm-model", "--llm-timeout", "Qwen", "localhost:8000"} {
		if strings.Contains(got, hidden) {
			t.Fatalf("help should hide %q, got:\n%s", hidden, got)
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

func TestRoot_HiddenLLMFlagsRequestReportHTML(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--stdout", "--llm-url", "http://127.0.0.1:9/v1/chat/completions"})
	cmd.SetIn(strings.NewReader(`[{"name":"a","score":1},{"name":"b","score":2}]`))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `class="report-table"`) {
		t.Fatalf("hidden LLM planner flags must request report HTML, got:\n%s", got)
	}
	if strings.Contains(got, `class="language-plaintext"`) {
		t.Fatalf("hidden LLM planner flags must not fall back to plain rendering, got:\n%s", got)
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
