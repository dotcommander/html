package cli

import (
	"fmt"

	"github.com/dotcommander/html/internal/report"
)

type modeValue report.ModeOverride

func (v *modeValue) String() string { return string(*v) }
func (v *modeValue) Type() string   { return "mode" }
func (v *modeValue) Set(s string) error {
	m := report.ModeOverride(s)
	switch m {
	case report.ModeOverrideAuto, report.ModeOverrideArticle, report.ModeOverrideTable, report.ModeOverrideCards, report.ModeOverrideChart, report.ModeOverrideReview, report.ModeOverrideDiff, report.ModeOverrideLog, report.ModeOverrideCode, report.ModeOverrideTree:
		*v = modeValue(m)
		return nil
	default:
		return fmt.Errorf("unsupported mode %q", s)
	}
}

type layoutValue report.LayoutOverride

func (v *layoutValue) String() string { return string(*v) }
func (v *layoutValue) Type() string   { return "layout" }
func (v *layoutValue) Set(s string) error {
	l := report.LayoutOverride(s)
	switch l {
	case report.LayoutOverrideAuto, report.LayoutOverrideSingle, report.LayoutOverrideTabs, report.LayoutOverrideSlides, report.LayoutOverrideReview:
		*v = layoutValue(l)
		return nil
	default:
		return fmt.Errorf("unsupported layout %q", s)
	}
}

type plannerValue report.PlannerMode

func (v *plannerValue) String() string { return string(*v) }
func (v *plannerValue) Type() string   { return "planner" }
func (v *plannerValue) Set(s string) error {
	p := report.PlannerMode(s)
	switch p {
	case report.PlannerAuto, report.PlannerOff, report.PlannerLLM:
		*v = plannerValue(p)
		return nil
	default:
		return fmt.Errorf("unsupported planner %q", s)
	}
}

func validateReportFlags(mode report.ModeOverride, layout report.LayoutOverride, planner report.PlannerMode) error {
	if err := (*modeValue)(&mode).Set(string(mode)); err != nil {
		return err
	}
	if err := (*layoutValue)(&layout).Set(string(layout)); err != nil {
		return err
	}
	if err := (*plannerValue)(&planner).Set(string(planner)); err != nil {
		return err
	}
	return nil
}
