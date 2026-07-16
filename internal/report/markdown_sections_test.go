package report

import (
	"strings"
	"testing"
)

func TestPlanPromotesOnlyExplicitOrderedListSection(t *testing.T) {
	t.Parallel()

	src := []byte("# Deploy\n\nIntro with `v1.2.3`.\n\n## Steps\n\n1. Run `go test ./...`.\n2. Publish **v1.2.3**.\n\n## Notes\n\n- Keep the tag.\n- Keep the logs.\n")
	_, plan := Plan(t.Context(), src, Options{Planner: PlannerOff})

	if len(plan.Components) != 3 {
		t.Fatalf("components = %#v, want article, timeline, article", plan.Components)
	}
	if plan.Components[0].Type != ComponentArticle || plan.Components[1].Type != ComponentTimeline || plan.Components[2].Type != ComponentArticle {
		t.Fatalf("component types = %s, %s, %s", plan.Components[0].Type, plan.Components[1].Type, plan.Components[2].Type)
	}
	timeline := plan.Components[1].Timeline
	if timeline == nil || len(timeline.Items) != 2 {
		t.Fatalf("timeline = %#v, want two items", timeline)
	}
	if plan.Components[1].Title != "Timeline" {
		t.Fatalf("title = %q, want Timeline", plan.Components[1].Title)
	}
	if got := string(src[timeline.Items[0].Start:timeline.Items[0].End]); !strings.Contains(got, "`go test ./...`") {
		t.Fatalf("first item lost technical token: %q", got)
	}
	if err := ValidateComponentSources(src, plan.Components); err != nil {
		t.Fatalf("validate component sources: %v", err)
	}
}

func TestSemanticPlanKeepsSourceHeadingOutOfPlanText(t *testing.T) {
	t.Parallel()

	src := []byte("# Deploy\n\n## {{https://example.com/steps}}\n\n1. Test.\n2. Publish.\n")
	analysis, plan := Plan(t.Context(), src, Options{Planner: PlannerOff})

	if len(plan.Components) != 2 || plan.Components[1].Type != ComponentTimeline {
		t.Fatalf("components = %#v, want article and timeline", plan.Components)
	}
	if plan.Components[1].Title != "Timeline" {
		t.Fatalf("title = %q, want fixed renderer-owned title", plan.Components[1].Title)
	}
	if _, err := ValidatePlan(plan); err != nil {
		t.Fatalf("validate deterministic semantic plan: %v", err)
	}
	if err := validatePlanForAnalysis(plan, analysis); err == nil || !strings.Contains(err.Error(), "deterministic-only") {
		t.Fatalf("LLM semantic validation error = %v", err)
	}
}

func TestPlanLeavesOrderedListWithSectionProseAsArticle(t *testing.T) {
	t.Parallel()

	src := []byte("# Deploy\n\n## Steps\n\nDo this carefully.\n\n1. Test.\n2. Publish.\n")
	_, plan := Plan(t.Context(), src, Options{Planner: PlannerOff})

	if len(plan.Components) != 1 || plan.Components[0].Type != ComponentArticle || plan.Components[0].Article != nil {
		t.Fatalf("components = %#v, want legacy whole-document article", plan.Components)
	}
}

func TestValidateComponentSourcesRejectsStaleDuplicateAndOverlap(t *testing.T) {
	t.Parallel()

	src := []byte("# Deploy\n\n## Steps\n\n1. Test.\n2. Publish.\n")
	_, plan := Plan(t.Context(), src, Options{Planner: PlannerOff})
	if len(plan.Components) != 2 {
		t.Fatalf("components = %#v, want article and timeline", plan.Components)
	}

	stale := append([]byte{}, src...)
	stale[0] = 'X'
	if err := ValidateComponentSources(stale, plan.Components); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale validation error = %v", err)
	}

	duplicate := append([]Component(nil), plan.Components...)
	duplicate[1].Timeline = cloneTimeline(plan.Components[1].Timeline)
	duplicate[1].Timeline.Items[1].ID = duplicate[1].Timeline.Items[0].ID
	if err := ValidateComponentSources(src, duplicate); err == nil || !strings.Contains(err.Error(), "duplicates") {
		t.Fatalf("duplicate validation error = %v", err)
	}

	overlap := append([]Component(nil), plan.Components...)
	overlap[1].Timeline = cloneTimeline(plan.Components[1].Timeline)
	overlap[1].Timeline.Section.Start--
	if err := ValidateComponentSources(src, overlap); err == nil {
		t.Fatal("expected overlap to be rejected")
	}
}

func TestValidatePlanRejectsTooManyComponents(t *testing.T) {
	t.Parallel()

	components := make([]Component, maxPlanComponents+1)
	for i := range components {
		components[i] = Component{Type: ComponentArticle, Source: "input", Title: "Article", Options: map[string]string{}}
	}
	plan := ReportPlan{
		Version: PlanVersion, Kind: KindMarkdown, Layout: LayoutSinglePage, Mode: ModeReader,
		Components: components, Confidence: 1,
	}
	if _, err := ValidatePlan(plan); err == nil || !strings.Contains(err.Error(), "maximum") {
		t.Fatalf("component limit error = %v", err)
	}
}

func cloneTimeline(in *TimelineData) *TimelineData {
	out := *in
	out.Items = append([]SourceRef(nil), in.Items...)
	return &out
}
