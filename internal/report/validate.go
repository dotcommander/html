package report

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

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
