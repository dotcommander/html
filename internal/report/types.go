package report

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const (
	PlanVersion          = 1
	PlannerPromptVersion = "report-plan-v8"
)

type Kind string

const (
	KindMarkdown     Kind = "markdown"
	KindJSONRecords  Kind = "json-records"
	KindJSONObject   Kind = "json-object"
	KindCSVRecords   Kind = "csv-records"
	KindTSVRecords   Kind = "tsv-records"
	KindTableRecords Kind = "table-records"
	KindDiff         Kind = "diff"
	KindSourceCode   Kind = "source-code"
	KindTreeListing  Kind = "tree-listing"
	KindLog          Kind = "log"
	KindTranscript   Kind = "transcript"
	KindMixed        Kind = "mixed"
	KindPlain        Kind = "plain"
	KindBinary       Kind = "binary"
)

type Layout string

const (
	LayoutSinglePage Layout = "single-page"
	LayoutTabbedPage Layout = "tabbed-page"
	LayoutSlides     Layout = "slides-page"
	LayoutReview     Layout = "review-page"
)

type Mode string

const (
	ModeReader      Mode = "reader"
	ModeDataBrowser Mode = "data-browser"
	ModeReview      Mode = "review"
	ModeConsole     Mode = "console"
	ModeCode        Mode = "code"
	ModeBrief       Mode = "brief"
)

type ComponentType string

const (
	ComponentArticle      ComponentType = "article"
	ComponentPreformatted ComponentType = "preformatted"
	ComponentCodeBlock    ComponentType = "code-block"
	ComponentDataTable    ComponentType = "data-table"
	ComponentRecordCards  ComponentType = "record-cards"
	ComponentDiffView     ComponentType = "diff-view"
	ComponentFileTree     ComponentType = "file-tree"
	ComponentTabs         ComponentType = "tabs"
	ComponentTOC          ComponentType = "toc"
	ComponentSummary      ComponentType = "summary"
	ComponentRawJSON      ComponentType = "raw-json"
	ComponentReview       ComponentType = "review"
)

type Stats struct {
	Bytes   int `json:"bytes"`
	Lines   int `json:"lines"`
	Records int `json:"records,omitempty"`
	Fields  int `json:"fields,omitempty"`
	Files   int `json:"files,omitempty"`
}

type Analysis struct {
	Kind       Kind     `json:"kind"`
	Confidence float64  `json:"confidence"`
	Reasons    []string `json:"reasons"`
	Stats      Stats    `json:"stats"`
	Data       any      `json:"data,omitempty"`
}

type PlannerInfo struct {
	Name        string `json:"name"`
	Model       string `json:"model,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
	Cache       string `json:"cache,omitempty"`
	Contributed bool   `json:"contributed"`
}

type ReportPlan struct {
	Version    int         `json:"version"`
	Kind       Kind        `json:"kind"`
	Layout     Layout      `json:"layout"`
	Mode       Mode        `json:"mode"`
	Components []Component `json:"components"`
	Confidence float64     `json:"confidence"`
	Reasons    []string    `json:"reasons"`
	Planner    PlannerInfo `json:"planner"`
}

type Component struct {
	Type    ComponentType     `json:"type"`
	Source  string            `json:"source"`
	Title   string            `json:"title"`
	Options map[string]string `json:"options"`
}

type ModeOverride string

const (
	ModeOverrideAuto    ModeOverride = "auto"
	ModeOverrideArticle ModeOverride = "article"
	ModeOverrideTable   ModeOverride = "table"
	ModeOverrideCards   ModeOverride = "cards"
	ModeOverrideReview  ModeOverride = "review"
	ModeOverrideDiff    ModeOverride = "diff"
	ModeOverrideLog     ModeOverride = "log"
	ModeOverrideCode    ModeOverride = "code"
	ModeOverrideTree    ModeOverride = "tree"
)

type LayoutOverride string

const (
	LayoutOverrideAuto   LayoutOverride = "auto"
	LayoutOverrideSingle LayoutOverride = "single"
	LayoutOverrideTabs   LayoutOverride = "tabs"
	LayoutOverrideSlides LayoutOverride = "slides"
	LayoutOverrideReview LayoutOverride = "review"
)

type PlannerMode string

const (
	PlannerAuto PlannerMode = "auto"
	PlannerOff  PlannerMode = "off"
	PlannerLLM  PlannerMode = "llm"
)

type Options struct {
	Mode          ModeOverride
	Layout        LayoutOverride
	Planner       PlannerMode
	LLMURL        string
	LLMModel      string
	LLMTimeout    string
	FallbackTitle string
	SourceName    string
}

func DefaultOptions() Options {
	return Options{
		Mode:       ModeOverrideAuto,
		Layout:     LayoutOverrideAuto,
		Planner:    PlannerAuto,
		LLMURL:     "http://localhost:8000/v1/chat/completions",
		LLMModel:   "Qwen3.6-35B-A3B-oQ4-fp16-mtp",
		LLMTimeout: "10s",
	}
}

func (p ReportPlan) Digest() string {
	b, _ := json.Marshal(p)
	return string(b)
}

func ValidatePlan(p ReportPlan) (ReportPlan, error) {
	if p.Version != PlanVersion {
		return p, fmt.Errorf("version must be %d", PlanVersion)
	}
	if !validKind(p.Kind) {
		return p, fmt.Errorf("unknown kind %q", p.Kind)
	}
	if !validLayout(p.Layout) {
		return p, fmt.Errorf("unknown layout %q", p.Layout)
	}
	if !validMode(p.Mode) {
		return p, fmt.Errorf("unknown mode %q", p.Mode)
	}
	if len(p.Components) == 0 {
		return p, fmt.Errorf("plan has no components")
	}
	if math.IsNaN(p.Confidence) || math.IsInf(p.Confidence, 0) {
		return p, fmt.Errorf("confidence must be finite")
	}
	for i, c := range p.Components {
		if !validComponent(c.Type) {
			return p, fmt.Errorf("component %d has unknown type %q", i, c.Type)
		}
		if !componentAllowedForKind(p.Kind, c.Type) {
			return p, fmt.Errorf("component %d type %q is incompatible with kind %q", i, c.Type, p.Kind)
		}
		if strings.TrimSpace(c.Title) == "" {
			return p, fmt.Errorf("component %d has blank title", i)
		}
		if strings.ContainsAny(c.Source, `/\`) {
			return p, fmt.Errorf("component %d source looks like a file path", i)
		}
		if !validComponentSource(c.Source) {
			return p, fmt.Errorf("component %d has unknown source %q", i, c.Source)
		}
		if !sourceAllowedForComponent(c.Type, c.Source) {
			return p, fmt.Errorf("component %d source %q is incompatible with type %q", i, c.Source, c.Type)
		}
		if unsafeLLMText(c.Source) || unsafeLLMText(c.Title) {
			return p, fmt.Errorf("component %d contains unsafe text", i)
		}
		for k, v := range c.Options {
			if unsafeLLMText(k) || unsafeLLMText(v) {
				return p, fmt.Errorf("component %d contains unsafe option", i)
			}
		}
	}
	p.Confidence = clamp01(p.Confidence)
	return p, nil
}

func validKind(v Kind) bool {
	switch v {
	case KindMarkdown, KindJSONRecords, KindJSONObject, KindCSVRecords, KindTSVRecords, KindTableRecords, KindDiff, KindSourceCode, KindTreeListing, KindLog, KindTranscript, KindMixed, KindPlain, KindBinary:
		return true
	}
	return false
}

func validLayout(v Layout) bool {
	return v == LayoutSinglePage || v == LayoutTabbedPage || v == LayoutSlides || v == LayoutReview
}

func validMode(v Mode) bool {
	switch v {
	case ModeReader, ModeDataBrowser, ModeReview, ModeConsole, ModeCode, ModeBrief:
		return true
	}
	return false
}

func validComponent(v ComponentType) bool {
	switch v {
	case ComponentArticle, ComponentPreformatted, ComponentCodeBlock, ComponentDataTable, ComponentRecordCards, ComponentDiffView, ComponentFileTree, ComponentSummary, ComponentRawJSON, ComponentReview:
		return true
	}
	return false
}

func componentAllowedForKind(kind Kind, typ ComponentType) bool {
	switch typ {
	case ComponentSummary, ComponentPreformatted:
		return true
	case ComponentArticle:
		return kind == KindMarkdown
	case ComponentCodeBlock:
		return true
	case ComponentDataTable, ComponentRecordCards, ComponentReview:
		return kind == KindJSONRecords || kind == KindCSVRecords || kind == KindTSVRecords || kind == KindTableRecords
	case ComponentDiffView:
		return kind == KindDiff
	case ComponentFileTree:
		return kind == KindTreeListing
	case ComponentRawJSON:
		return kind == KindJSONObject
	default:
		return false
	}
}

func validComponentSource(source string) bool {
	switch source {
	case "input", "analysis", "records":
		return true
	default:
		return false
	}
}

func sourceAllowedForComponent(typ ComponentType, source string) bool {
	switch typ {
	case ComponentSummary:
		return source == "analysis"
	case ComponentDataTable, ComponentRecordCards, ComponentReview:
		return source == "records"
	default:
		return source == "input"
	}
}

func unsafeLLMText(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "<html") ||
		strings.Contains(lower, "<script") ||
		strings.Contains(lower, "<style") ||
		strings.Contains(lower, "javascript:") ||
		strings.Contains(lower, "http://") ||
		strings.Contains(lower, "https://") ||
		strings.Contains(lower, "{{") ||
		strings.Contains(lower, "}}")
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}
