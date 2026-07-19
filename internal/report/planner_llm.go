package report

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dotcommander/html/internal/atomicfile"
)

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func planWithLLM(ctx context.Context, src []byte, analysis Analysis, fallback ReportPlan, opts Options) (ReportPlan, string) {
	timeout, err := time.ParseDuration(opts.LLMTimeout)
	if err != nil {
		return ReportPlan{}, "llm planner invalid timeout: " + err.Error()
	}
	key, summary := llmCacheKey(src, analysis, opts)
	securePlanCachePath(key)
	if b, err := os.ReadFile(key); err == nil {
		var p ReportPlan
		if err := json.Unmarshal(b, &p); err == nil {
			p.Planner.Cache = "hit"
			if valid, err := ValidatePlan(p); err == nil {
				if err := validatePlanForAnalysis(valid, analysis); err == nil {
					return valid, ""
				}
			}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req := chatRequest{
		Model: opts.LLMModel,
		Messages: []chatMessage{
			{Role: "system", Content: llmSystemPrompt()},
			{Role: "user", Content: llmUserPrompt(analysis, fallback, summary, src)},
		},
		Temperature: 0,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return ReportPlan{}, "llm planner request failed: " + err.Error()
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, opts.LLMURL, bytes.NewReader(body))
	if err != nil {
		return ReportPlan{}, "llm planner request failed: " + err.Error()
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return ReportPlan{}, "llm planner failed: " + err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ReportPlan{}, fmt.Sprintf("llm planner returned status %d", resp.StatusCode)
	}
	var chat chatResponse
	dec := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<20))
	if err := dec.Decode(&chat); err != nil {
		return ReportPlan{}, "llm planner invalid response: " + err.Error()
	}
	if len(chat.Choices) == 0 {
		return ReportPlan{}, "llm planner returned no choices"
	}
	var p ReportPlan
	if err := json.Unmarshal([]byte(chat.Choices[0].Message.Content), &p); err != nil {
		return ReportPlan{}, "llm planner invalid json: " + err.Error()
	}
	p.Planner = PlannerInfo{Name: "llm", Model: opts.LLMModel, Prompt: PlannerPromptVersion, Cache: "miss", Contributed: true}
	valid, err := ValidatePlan(p)
	if err != nil {
		return ReportPlan{}, "llm planner rejected: " + err.Error()
	}
	if err := validatePlanForAnalysis(valid, analysis); err != nil {
		return ReportPlan{}, "llm planner rejected: " + err.Error()
	}
	cacheDir := filepath.Dir(key)
	_ = os.MkdirAll(cacheDir, 0o700)
	_ = os.Chmod(cacheDir, 0o700)
	if b, err := json.MarshalIndent(valid, "", "  "); err == nil {
		_ = atomicfile.Write(key, b, 0o600)
	}
	return valid, ""
}

func securePlanCachePath(path string) {
	dir := filepath.Dir(path)
	if os.MkdirAll(dir, 0o700) != nil {
		return
	}
	_ = os.Chmod(dir, 0o700)
	if err := os.Chmod(path, 0o600); err != nil && !os.IsNotExist(err) {
		return
	}
}

func validatePlanForAnalysis(p ReportPlan, a Analysis) error {
	if p.Kind != a.Kind {
		return fmt.Errorf("plan kind %q is incompatible with analysis kind %q", p.Kind, a.Kind)
	}
	if hasSemanticComponents(p.Components) {
		return fmt.Errorf("semantic components are deterministic-only")
	}
	for i, c := range p.Components {
		if !componentAllowedForKind(a.Kind, c.Type) {
			return fmt.Errorf("component %d type %q is incompatible with analysis kind %q", i, c.Type, a.Kind)
		}
	}
	return nil
}

func llmCacheKey(src []byte, analysis Analysis, opts Options) (string, string) {
	abbrev := Analysis{Kind: analysis.Kind, Confidence: analysis.Confidence, Reasons: analysis.Reasons, Stats: analysis.Stats}
	summaryBytes, _ := json.Marshal(abbrev)
	h := sha256.New()
	h.Write(src)
	h.Write([]byte{0})
	h.Write([]byte(PlannerPromptVersion))
	h.Write([]byte{0})
	h.Write([]byte(opts.LLMModel))
	h.Write([]byte{0})
	h.Write([]byte(opts.LLMURL))
	h.Write([]byte{0})
	h.Write([]byte(opts.Mode))
	h.Write([]byte{0})
	h.Write([]byte(opts.Layout))
	h.Write([]byte{0})
	h.Write(summaryBytes)
	sum := hex.EncodeToString(h.Sum(nil))
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "html-report-plans", sum+".json"), string(summaryBytes)
	}
	return filepath.Join(home, ".config", "html", "plan-cache", sum+".json"), string(summaryBytes)
}

func llmSystemPrompt() string {
	return "Return only one ReportPlan JSON object. Do not return HTML, CSS, JavaScript, Markdown fences, file paths, network URLs, or template strings. Use only the enum values described by the user."
}

func llmUserPrompt(analysis Analysis, fallback ReportPlan, summary string, src []byte) string {
	sample := string(src)
	if len(sample) > 8<<10 {
		sample = sample[:8<<10]
	}
	return "Allowed kind: markdown, json-records, json-object, csv-records, tsv-records, table-records, diff, source-code, tree-listing, log, transcript, mixed, plain, binary.\n" +
		"Allowed layout: single-page, tabbed-page, slides-page.\n" +
		"Allowed mode: reader, data-browser, review, console, code, brief.\n" +
		"Allowed component type: article, preformatted, code-block, chart, data-table, record-cards, diff-view, file-tree, summary, raw-json.\n" +
		"Allowed component source: input, analysis, records.\n" +
		"Component source compatibility: summary uses analysis; chart, data-table, and record-cards use records; every other component uses input.\n" +
		"Component compatibility: article only markdown; chart, data-table, and record-cards only json-records/csv-records/tsv-records/table-records; raw-json only json-object; diff-view only diff; file-tree only tree-listing; code-block, summary, and preformatted any kind. Chart supports type=bar with optional categorical x and numeric y column names.\n" +
		"Use version 1 and at least one component. Prefer the deterministic plan unless the input is genuinely mixed or ambiguous.\n" +
		"Analysis summary: " + summary + "\n" +
		"Deterministic plan: " + fallback.Digest() + "\n" +
		"Input sample:\n" + sample
}
