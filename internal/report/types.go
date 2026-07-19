package report

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
