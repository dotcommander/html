package cli

import (
	"flag"
	"fmt"
	"io"
	"net/url"

	"github.com/dotcommander/html/internal/actions"
	"github.com/dotcommander/html/internal/config"
	"github.com/dotcommander/html/internal/report"
)

// flagValues owns the root command's flag surface: the values bound to the
// FlagSet, the combination policy between them, and their translation into
// actions.Options once user config is layered in. Input selection and the
// context stay with the command.
type flagValues struct {
	noOpen, force, safe, plain, markdown, frame bool
	plan, stdout, version                       bool
	title, lang, codeTheme, output, template    string
	mode                                        report.ModeOverride
	layout                                      report.LayoutOverride
	planner                                     report.PlannerMode
	llmURL, llmModel, llmTimeout                string
}

// newRootFlagSet builds the root FlagSet bound to fresh flagValues seeded from
// the report defaults. Registration order is observable — printUsage lists
// flags in this order — so reordering changes --help output.
func newRootFlagSet() (*flag.FlagSet, *flagValues) {
	defaults := report.DefaultOptions()
	v := &flagValues{
		mode:       defaults.Mode,
		layout:     defaults.Layout,
		planner:    defaults.Planner,
		llmURL:     defaults.LLMURL,
		llmModel:   defaults.LLMModel,
		llmTimeout: defaults.LLMTimeout,
	}
	fs := flag.NewFlagSet("html", flag.ContinueOnError)
	// The flag package otherwise prints a diagnostic and returns the same error;
	// main owns the single user-facing diagnostic.
	fs.SetOutput(io.Discard)
	boolFlag(fs, &v.noOpen, "no-open", "n", "render only; print the cache path without opening the browser")
	boolFlag(fs, &v.force, "force", "f", "rebuild even if the cached HTML is fresh")
	fs.BoolVar(&v.safe, "safe", false, "disable raw HTML passthrough (safe for untrusted Markdown)")
	boolFlag(fs, &v.plain, "plain", "p", "render input as preformatted plain text, not Markdown")
	boolFlag(fs, &v.markdown, "markdown", "m", "render input as Markdown (overrides stdin auto-detection)")
	fs.BoolVar(&v.frame, "frame", false, "wrap plain/ANSI output in a terminal-window frame (implies --plain)")
	stringFlag(fs, &v.title, "title", "t", "stdin", "page title for piped input")
	stringFlag(fs, &v.lang, "lang", "l", "", "syntax-highlight language for plain mode (e.g. go, json; \"text\" = no highlighting)")
	fs.StringVar(&v.codeTheme, "code-theme", "", "chroma style for code blocks (e.g. dracula, monokai, nord; empty = github/github-dark)")
	fs.StringVar(&v.template, "template", "", "page template: default, reader, notebook, or a local template file")
	stringFlag(fs, &v.output, "output", "o", "", "write the final HTML document to a stable path (\"-\" writes stdout)")
	fs.BoolVar(&v.plan, "plan", false, "print the report plan JSON without rendering")
	fs.BoolVar(&v.stdout, "stdout", false, "write the final HTML document to stdout without opening")
	fs.BoolVar(&v.version, "version", false, "print version and exit")
	fs.Var((*modeValue)(&v.mode), "mode", "report mode: auto, article, table, cards, chart, review, diff, log, code, tree")
	fs.Var((*layoutValue)(&v.layout), "layout", "report layout: auto, single, tabs, slides, review")
	fs.Var((*plannerValue)(&v.planner), "planner", "planner policy: off, auto, llm")
	fs.StringVar(&v.llmURL, "llm-url", defaults.LLMURL, "HTTP(S) OpenAI-compatible chat completions endpoint")
	fs.StringVar(&v.llmModel, "llm-model", defaults.LLMModel, "model name for the optional planner")
	fs.StringVar(&v.llmTimeout, "llm-timeout", defaults.LLMTimeout, "timeout for the optional planner")
	return fs, v
}

// validate enforces the combination policy between root flags. Check order is
// observable: the first violated rule names the returned error.
func (v flagValues) validate(changed map[string]bool) error {
	if v.plain && v.markdown {
		return fmt.Errorf("--plain and --markdown are mutually exclusive")
	}
	if v.frame && v.markdown {
		return fmt.Errorf("--frame and --markdown are mutually exclusive")
	}
	reportRequested := v.reportRequested(changed)
	if v.plain && reportRequested {
		return fmt.Errorf("--plain and report flags are mutually exclusive")
	}
	if v.markdown && reportRequested {
		return fmt.Errorf("--markdown and report flags are mutually exclusive")
	}
	if v.frame && reportRequested {
		return fmt.Errorf("--frame and report flags are mutually exclusive")
	}
	if v.plan && v.output != "" {
		return fmt.Errorf("--plan and --output are mutually exclusive")
	}
	if v.plan && changed["template"] {
		return fmt.Errorf("--template and --plan are mutually exclusive")
	}
	if v.stdout && v.output != "" {
		return fmt.Errorf("--stdout and --output are mutually exclusive")
	}
	if err := validateReportFlags(v.mode, v.layout, v.planner); err != nil {
		return err
	}
	return v.validatePlannerFlags(changed)
}

// reportRequested reports whether any report-surface flag was set explicitly.
func (v flagValues) reportRequested(changed map[string]bool) bool {
	return v.plan ||
		changed["mode"] || changed["layout"] || changed["planner"] ||
		changed["llm-url"] || changed["llm-model"] || changed["llm-timeout"]
}

// validatePlannerFlags enforces the LLM planner contract: explicit --planner
// for any LLM flag, and a model plus HTTP(S) endpoint when the planner is on.
func (v flagValues) validatePlannerFlags(changed map[string]bool) error {
	if v.planner == report.PlannerOff {
		if changed["llm-url"] || changed["llm-model"] || changed["llm-timeout"] {
			return fmt.Errorf("LLM flags require explicit --planner auto or --planner llm")
		}
		return nil
	}
	if !changed["planner"] {
		return fmt.Errorf("--llm-url and --llm-model require explicit --planner auto or --planner llm")
	}
	if v.llmModel == "" {
		return fmt.Errorf("--planner %s requires nonempty --llm-model", v.planner)
	}
	u, err := url.ParseRequestURI(v.llmURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("--planner %s requires an HTTP(S) --llm-url", v.planner)
	}
	return nil
}

// actionsOptions layers user config over the parsed flag values and translates
// them into actions.Options. Context and input source are owned by the command.
func (v flagValues) actionsOptions(cfg config.Config, changed map[string]bool) actions.Options {
	opts := actions.Options{
		NoOpen:     v.noOpen,
		Force:      v.force,
		Safe:       v.safe,
		Plain:      v.plain,
		Markdown:   v.markdown,
		Frame:      v.frame,
		Title:      v.title,
		Lang:       v.lang,
		CodeTheme:  v.codeTheme,
		OpenCmd:    cfg.OpenCommand,
		MaxWidth:   cfg.MaxWidth,
		Theme:      cfg.DefaultTheme,
		Palette:    cfg.DefaultPalette,
		TOC:        cfg.TOC,
		Template:   v.template,
		Output:     v.output,
		Report:     v.reportRequested(changed),
		Plan:       v.plan,
		Stdout:     v.stdout,
		Mode:       v.mode,
		Layout:     v.layout,
		Planner:    v.planner,
		LLMURL:     v.llmURL,
		LLMModel:   v.llmModel,
		LLMTimeout: v.llmTimeout,
	}
	if !changed["code-theme"] {
		opts.CodeTheme = cfg.DefaultCodeTheme
	}
	return opts
}
