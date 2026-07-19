package report

import (
	"context"
	"strings"
)

func Plan(ctx context.Context, src []byte, opts Options) (Analysis, ReportPlan) {
	opts = withDefaults(opts)
	analysis := Analyze(src, opts.SourceName)
	base := deterministicPlan(analysis, opts)
	if analysis.Kind == KindMarkdown && opts.Mode == ModeOverrideAuto && (opts.Layout == LayoutOverrideAuto || opts.Layout == LayoutOverrideSingle) {
		if components := semanticMarkdownComponents(src); len(components) > 0 {
			base.Components = components
			if opts.Layout == LayoutOverrideAuto {
				base.Layout = LayoutSinglePage
			}
			base.Reasons = append(base.Reasons, "explicit ordered action list promoted to source-backed timeline")
		}
	}
	base.Planner = PlannerInfo{Name: "deterministic"}
	base, err := ValidatePlan(base)
	if err != nil {
		base = fallbackPlan(analysis, opts, "deterministic plan invalid: "+err.Error())
	}
	// Phase one semantic sections are deterministic and source-owned. They are
	// never offered to the LLM planner for selection or rewriting.
	if hasSemanticComponents(base.Components) || !shouldUseLLM(analysis, base, opts) {
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
	// Chart mode keeps the visualization, its diagnostic fallback, and the
	// source table visible in one document by default. An explicit --layout
	// override below still wins.
	if opts.Mode == ModeOverrideChart {
		layout = LayoutSinglePage
	}
	switch opts.Layout {
	case LayoutOverrideSingle:
		layout = LayoutSinglePage
	case LayoutOverrideTabs:
		layout = LayoutTabbedPage
	case LayoutOverrideSlides:
		layout = LayoutSlides
	case LayoutOverrideReview:
		layout = LayoutReview
	}
	if opts.Mode == ModeOverrideReview {
		layout = LayoutReview
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
	case KindCSVRecords, KindTSVRecords, KindTableRecords:
		return a.Kind, ModeDataBrowser, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{"primary": "true"}}}
	case KindDiff:
		return a.Kind, ModeReview, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: ComponentDiffView, Source: "input", Title: "Diff", Options: map[string]string{}}}
	case KindSourceCode:
		return a.Kind, ModeCode, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: ComponentCodeBlock, Source: "input", Title: "Code", Options: map[string]string{}}}
	case KindTreeListing:
		return a.Kind, ModeCode, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: ComponentFileTree, Source: "input", Title: "Files", Options: map[string]string{}}}
	case KindLog:
		return a.Kind, ModeConsole, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: ComponentPreformatted, Source: "input", Title: "Log", Options: map[string]string{}}}
	case KindTranscript:
		return a.Kind, ModeConsole, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: ComponentPreformatted, Source: "input", Title: "Transcript", Options: map[string]string{}}}
	case KindMixed:
		return a.Kind, ModeBrief, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: ComponentPreformatted, Source: "input", Title: "Input", Options: map[string]string{}}}
	case KindPlain:
		return a.Kind, ModeBrief, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: ComponentPreformatted, Source: "input", Title: "Input", Options: map[string]string{}}}
	case KindBinary:
		return a.Kind, ModeBrief, []Component{{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}}, {Type: ComponentPreformatted, Source: "input", Title: "Binary", Options: map[string]string{}}}
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
	case ModeOverrideChart:
		if !hasRecordRows(a.Kind) {
			return componentsFor(a)
		}
		return a.Kind, ModeDataBrowser, []Component{
			{Type: ComponentSummary, Source: "analysis", Title: "Summary", Options: map[string]string{}},
			{Type: ComponentChart, Source: "records", Title: "Chart", Options: map[string]string{"type": "bar"}},
			{Type: ComponentDataTable, Source: "records", Title: "Records", Options: map[string]string{"primary": "true"}},
		}
	case ModeOverrideReview:
		if !hasRecordRows(a.Kind) {
			return componentsFor(a)
		}
		return a.Kind, ModeReview, []Component{{Type: ComponentReview, Source: "records", Title: "Review", Options: map[string]string{"primary": "true"}}}
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
	return kind == KindJSONRecords || kind == KindCSVRecords || kind == KindTSVRecords || kind == KindTableRecords
}

func hasSemanticComponents(components []Component) bool {
	for _, component := range components {
		if component.Article != nil || component.Timeline != nil {
			return true
		}
	}
	return false
}

func fallbackPlan(a Analysis, opts Options, reason string) ReportPlan {
	if a.Kind == KindMarkdown {
		p := ReportPlan{
			Version: PlanVersion, Kind: a.Kind, Layout: LayoutSinglePage, Mode: ModeReader,
			Components: []Component{{Type: ComponentArticle, Source: "input", Title: "Article", Options: map[string]string{}}},
			Confidence: a.Confidence, Reasons: append(append([]string{}, a.Reasons...), reason),
			Planner: PlannerInfo{Name: "deterministic"},
		}
		p, _ = ValidatePlan(p)
		return p
	}
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
