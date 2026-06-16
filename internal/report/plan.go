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
	"strings"
	"time"
)

func Plan(ctx context.Context, src []byte, opts Options) (Analysis, ReportPlan) {
	opts = withDefaults(opts)
	analysis := Analyze(src, opts.SourceName)
	base := deterministicPlan(analysis, opts)
	base.Planner = PlannerInfo{Name: "deterministic"}
	base, err := ValidatePlan(base)
	if err != nil {
		base = fallbackPlan(analysis, opts, "deterministic plan invalid: "+err.Error())
	}
	if !shouldUseLLM(analysis, base, opts) {
		return analysis, base
	}
	llmPlan, reason := planWithLLM(ctx, src, analysis, base, opts)
	if reason != "" {
		base.Reasons = append(base.Reasons, reason)
		return analysis, base
	}
	return analysis, llmPlan
}

func withDefaults(opts Options) Options {
	def := DefaultOptions()
	if opts.Mode == "" {
		opts.Mode = def.Mode
	}
	if opts.Layout == "" {
		opts.Layout = def.Layout
	}
	if opts.Planner == "" {
		opts.Planner = def.Planner
	}
	if opts.LLMURL == "" {
		opts.LLMURL = def.LLMURL
	}
	if opts.LLMModel == "" {
		opts.LLMModel = def.LLMModel
	}
	if opts.LLMTimeout == "" {
		opts.LLMTimeout = def.LLMTimeout
	}
	if opts.FallbackTitle == "" {
		opts.FallbackTitle = "stdin"
	}
	return opts
}

func deterministicPlan(a Analysis, opts Options) ReportPlan {
	kind, mode, components := componentsFor(a)
	if opts.Mode != ModeOverrideAuto {
		kind, mode, components = overrideComponents(opts.Mode, a)
	}
	layout := LayoutSinglePage
	if len(components) > 2 || a.Kind == KindMixed {
		layout = LayoutTabbedPage
	}
	switch opts.Layout {
	case LayoutOverrideSingle:
		layout = LayoutSinglePage
	case LayoutOverrideTabs:
		layout = LayoutTabbedPage
	case LayoutOverrideSlides:
		layout = LayoutSlides
	}
	return ReportPlan{
		Version:    PlanVersion,
		Kind:       kind,
		Layout:     layout,
		Mode:       mode,
		Components: components,
		Confidence: a.Confidence,
		Reasons:    append([]string{}, a.Reasons...),
	}
}

func componentsFor(a Analysis) (Kind, Mode, []Component) {
	switch a.Kind {
	case KindMarkdown:
		return a.Kind, ModeReader, []Component{{Type: ComponentArticle, Source: "input", Title: "Article", Options: map[string]string{}}}
	case KindJSONRecords:
		component := ComponentDataTable
		title := "Records"
		if strings.Contains(strings.Join(a.Reasons, " "), "heterogeneous") {
			component = ComponentRecordCards
			title = "Details"
		}
		return a.Kind, ModeDataBrowser, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: component, Source: "records", Title: title, Options: map[string]string{"primary": "true"}}}
	case KindJSONObject:
		return a.Kind, ModeDataBrowser, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: ComponentRawJSON, Source: "input", Title: "JSON", Options: map[string]string{}}}
	case KindCSVRecords, KindTSVRecords:
		return a.Kind, ModeDataBrowser, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{"primary": "true"}}}
	case KindDiff:
		return a.Kind, ModeReview, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: ComponentDiffView, Source: "input", Title: "Diff", Options: map[string]string{}}}
	case KindSourceCode:
		return a.Kind, ModeCode, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: ComponentCodeBlock, Source: "input", Title: "Code", Options: map[string]string{}}}
	case KindTreeListing:
		return a.Kind, ModeCode, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}}}
	case KindLog, KindTranscript:
		return a.Kind, ModeConsole, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: ComponentPreformatted, Source: "input", Title: "Log", Options: map[string]string{}}}
	case KindMixed:
		return a.Kind, ModeBrief, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: ComponentPreformatted, Source: "input", Title: "Input", Options: map[string]string{}}}
	default:
		return a.Kind, ModeBrief, []Component{{Type: ComponentPreformatted, Source: "input", Title: "Input", Options: map[string]string{}}}
	}
}

func overrideComponents(mode ModeOverride, a Analysis) (Kind, Mode, []Component) {
	switch mode {
	case ModeOverrideArticle:
		if a.Kind != KindMarkdown {
			return componentsFor(a)
		}
		return a.Kind, ModeReader, []Component{{Type: ComponentArticle, Source: "input", Title: "Article", Options: map[string]string{}}}
	case ModeOverrideTable:
		if !hasRecordRows(a.Kind) {
			return componentsFor(a)
		}
		return a.Kind, ModeDataBrowser, []Component{{Type: ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{"primary": "true"}}}
	case ModeOverrideCards:
		if !hasRecordRows(a.Kind) {
			return componentsFor(a)
		}
		return a.Kind, ModeDataBrowser, []Component{{Type: ComponentRecordCards, Source: "records", Title: "Details", Options: map[string]string{"primary": "true"}}}
	case ModeOverrideDiff:
		if a.Kind != KindDiff {
			return componentsFor(a)
		}
		return a.Kind, ModeReview, []Component{{Type: ComponentDiffView, Source: "input", Title: "Diff", Options: map[string]string{}}}
	case ModeOverrideLog:
		return a.Kind, ModeConsole, []Component{{Type: ComponentPreformatted, Source: "input", Title: "Log", Options: map[string]string{}}}
	case ModeOverrideCode:
		return a.Kind, ModeCode, []Component{{Type: ComponentCodeBlock, Source: "input", Title: "Code", Options: map[string]string{}}}
	case ModeOverrideTree:
		if a.Kind != KindTreeListing {
			return componentsFor(a)
		}
		return a.Kind, ModeCode, []Component{{Type: ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}}}
	default:
		return componentsFor(a)
	}
}

func hasRecordRows(kind Kind) bool {
	return kind == KindJSONRecords || kind == KindCSVRecords || kind == KindTSVRecords
}

func fallbackPlan(a Analysis, opts Options, reason string) ReportPlan {
	p := ReportPlan{
		Version:    PlanVersion,
		Kind:       a.Kind,
		Layout:     LayoutSinglePage,
		Mode:       ModeBrief,
		Components: []Component{{Type: ComponentPreformatted, Source: "input", Title: opts.FallbackTitle, Options: map[string]string{}}},
		Confidence: a.Confidence,
		Reasons:    append(append([]string{}, a.Reasons...), reason),
		Planner:    PlannerInfo{Name: "deterministic"},
	}
	p, _ = ValidatePlan(p)
	return p
}

func shouldUseLLM(a Analysis, p ReportPlan, opts Options) bool {
	if opts.Planner == PlannerOff || a.Kind == KindBinary {
		return false
	}
	if opts.Planner == PlannerLLM {
		return true
	}
	if opts.Mode != ModeOverrideAuto {
		return false
	}
	if opts.Layout != LayoutOverrideAuto && a.Kind != KindPlain && a.Kind != KindMixed {
		return false
	}
	if a.Confidence >= 0.80 {
		return false
	}
	if a.Confidence < 0.65 || a.Kind == KindMixed {
		return true
	}
	return p.Mode == ModeDataBrowser && a.Stats.Fields > 12
}

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
	_ = os.MkdirAll(filepath.Dir(key), 0o755)
	if b, err := json.MarshalIndent(valid, "", "  "); err == nil {
		_ = os.WriteFile(key, b, 0o644)
	}
	return valid, ""
}

func validatePlanForAnalysis(p ReportPlan, a Analysis) error {
	if p.Kind != a.Kind {
		return fmt.Errorf("plan kind %q is incompatible with analysis kind %q", p.Kind, a.Kind)
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
	if len(sample) > 8000 {
		sample = sample[:8000]
	}
	return "Allowed kind: markdown, json-records, json-object, csv-records, tsv-records, diff, source-code, tree-listing, log, transcript, mixed, plain, binary.\n" +
		"Allowed layout: single-page, tabbed-page, slides-page.\n" +
		"Allowed mode: reader, data-browser, review, console, code, brief.\n" +
		"Allowed component type: article, preformatted, code-block, data-table, record-cards, diff-view, file-tree, summary, raw-json.\n" +
		"Allowed component source: input, analysis, records.\n" +
		"Component source compatibility: summary uses analysis; data-table and record-cards use records; every other component uses input.\n" +
		"Component compatibility: article only markdown; data-table and record-cards only json-records/csv-records/tsv-records; raw-json only json-object; diff-view only diff; file-tree only tree-listing; code-block, summary, and preformatted any kind.\n" +
		"Use version 1 and at least one component. Prefer the deterministic plan unless the input is genuinely mixed or ambiguous.\n" +
		"Analysis summary: " + summary + "\n" +
		"Deterministic plan: " + fallback.Digest() + "\n" +
		"Input sample:\n" + sample
}
