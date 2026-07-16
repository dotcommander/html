package report

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const (
	PlanVersion          = 1
	PlannerPromptVersion = "report-plan-v9"
	maxPlanComponents    = 64
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
	ComponentTimeline     ComponentType = "timeline"
	ComponentPreformatted ComponentType = "preformatted"
	ComponentCodeBlock    ComponentType = "code-block"
	ComponentChart        ComponentType = "chart"
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
	Type     ComponentType     `json:"type"`
	Source   string            `json:"source"`
	Title    string            `json:"title"`
	Options  map[string]string `json:"options"`
	Article  *ArticleData      `json:"article,omitempty"`
	Timeline *TimelineData     `json:"timeline,omitempty"`
}

// SourceRef identifies an exact, immutable byte range in the Markdown input.
// Digest prevents a cached plan from silently selecting different source text.
type SourceRef struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Digest string `json:"digest"`
}

// ArticleData narrows an article component to a source-owned Markdown range.
// A nil Article keeps the legacy whole-document article behavior.
type ArticleData struct {
	Range SourceRef `json:"range"`
}

// TimelineData is the closed, source-backed representation of one explicit
// ordered action list. Renderers own presentation; the plan owns no HTML or
// rewritten prose.
type TimelineData struct {
	Section SourceRef   `json:"section"`
	Heading SourceRef   `json:"heading"`
	List    SourceRef   `json:"list"`
	Items   []SourceRef `json:"items"`
}

type ModeOverride string

const (
	ModeOverrideAuto    ModeOverride = "auto"
	ModeOverrideArticle ModeOverride = "article"
	ModeOverrideTable   ModeOverride = "table"
	ModeOverrideCards   ModeOverride = "cards"
	ModeOverrideChart   ModeOverride = "chart"
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
		Planner:    PlannerOff,
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
	if len(p.Components) > maxPlanComponents {
		return p, fmt.Errorf("plan has %d components; maximum is %d", len(p.Components), maxPlanComponents)
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
		if err := validateComponentData(c); err != nil {
			return p, fmt.Errorf("component %d: %w", i, err)
		}
	}
	if err := validateSemanticOwnership(p.Components); err != nil {
		return p, err
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
	case ComponentArticle, ComponentTimeline, ComponentPreformatted, ComponentCodeBlock, ComponentChart, ComponentDataTable, ComponentRecordCards, ComponentDiffView, ComponentFileTree, ComponentSummary, ComponentRawJSON, ComponentReview:
		return true
	}
	return false
}

func componentAllowedForKind(kind Kind, typ ComponentType) bool {
	switch typ {
	case ComponentSummary, ComponentPreformatted:
		return true
	case ComponentArticle, ComponentTimeline:
		return kind == KindMarkdown
	case ComponentCodeBlock:
		return true
	case ComponentChart, ComponentDataTable, ComponentRecordCards, ComponentReview:
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

func validateComponentData(c Component) error {
	if c.Type != ComponentArticle && c.Article != nil {
		return fmt.Errorf("article data is incompatible with type %q", c.Type)
	}
	if c.Type != ComponentTimeline && c.Timeline != nil {
		return fmt.Errorf("timeline data is incompatible with type %q", c.Type)
	}
	if c.Type == ComponentTimeline && c.Timeline == nil {
		return fmt.Errorf("timeline data is required")
	}
	if c.Article != nil {
		return validateSourceRef(c.Article.Range, "article")
	}
	if c.Timeline == nil {
		return nil
	}
	t := c.Timeline
	if err := validateSourceRef(t.Section, "section"); err != nil {
		return err
	}
	if err := validateSourceRef(t.Heading, "heading"); err != nil {
		return err
	}
	if err := validateSourceRef(t.List, "ordered-list"); err != nil {
		return err
	}
	if len(t.Items) < 2 {
		return fmt.Errorf("timeline requires at least two items")
	}
	if t.Heading.End > t.List.Start {
		return fmt.Errorf("timeline heading and ordered list overlap")
	}
	refs := append([]SourceRef{t.Heading, t.List}, t.Items...)
	for _, ref := range refs {
		if ref.Start < t.Section.Start || ref.End > t.Section.End {
			return fmt.Errorf("%s ref %q is outside section", ref.Kind, ref.ID)
		}
	}
	previousEnd := -1
	for _, item := range t.Items {
		if err := validateSourceRef(item, "list-item"); err != nil {
			return err
		}
		if item.Start < previousEnd {
			return fmt.Errorf("timeline item refs overlap or are out of order")
		}
		if item.Start < t.List.Start || item.End > t.List.End {
			return fmt.Errorf("timeline item ref %q is outside ordered list", item.ID)
		}
		previousEnd = item.End
	}
	return nil
}

func validateSourceRef(ref SourceRef, wantKind string) error {
	if ref.ID == "" || ref.Kind != wantKind || ref.Start < 0 || ref.End <= ref.Start {
		return fmt.Errorf("invalid %s source ref", wantKind)
	}
	if len(ref.Digest) != 64 {
		return fmt.Errorf("invalid %s source digest", wantKind)
	}
	if _, err := hex.DecodeString(ref.Digest); err != nil {
		return fmt.Errorf("invalid %s source digest", wantKind)
	}
	return nil
}

func validateSemanticOwnership(components []Component) error {
	previousEnd := -1
	seen := map[string]struct{}{}
	for i, c := range components {
		var ref *SourceRef
		switch {
		case c.Article != nil:
			ref = &c.Article.Range
		case c.Timeline != nil:
			ref = &c.Timeline.Section
		default:
			continue
		}
		if _, ok := seen[ref.ID]; ok {
			return fmt.Errorf("component %d duplicates source ref %q", i, ref.ID)
		}
		seen[ref.ID] = struct{}{}
		if ref.Start < previousEnd {
			return fmt.Errorf("component %d source refs overlap or are out of order", i)
		}
		previousEnd = ref.End
	}
	return nil
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
	case ComponentChart, ComponentDataTable, ComponentRecordCards, ComponentReview:
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
