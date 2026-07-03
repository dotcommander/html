package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/dotcommander/html/internal/actions"
	"github.com/dotcommander/html/internal/config"
	"github.com/dotcommander/html/internal/report"
	"github.com/spf13/cobra"
)

func Execute() error { return newRootCmd().Execute() }

func newRootCmd() *cobra.Command {
	var noOpen, force, safe, plain, markdown, frame bool
	var plan, stdout bool
	var title, lang, codeTheme, output string
	reportDefaults := report.DefaultOptions()
	mode := reportDefaults.Mode
	layout := reportDefaults.Layout
	planner := reportDefaults.Planner
	llmURL := reportDefaults.LLMURL
	llmModel := reportDefaults.LLMModel
	llmTimeout := reportDefaults.LLMTimeout
	cmd := &cobra.Command{
		Use:   "html [file]",
		Short: "Render Markdown or piped text to a clean HTML page and open it in the browser",
		Long: "Render a Markdown file — or data piped on stdin (e.g. `tree -d | html`) — to a\n" +
			"self-contained HTML page and open it in the browser. Piped input is auto-\n" +
			"detected as Markdown or plain text; use --plain/--markdown to force a mode.",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true, // don't dump usage text on a runtime error
		SilenceErrors: true, // cmd/html/main.go prints the error
		RunE: func(cmd *cobra.Command, args []string) error {
			if plain && markdown {
				return fmt.Errorf("--plain and --markdown are mutually exclusive")
			}
			if frame && markdown {
				return fmt.Errorf("--frame and --markdown are mutually exclusive")
			}
			reportRequested := plan ||
				cmd.Flags().Changed("mode") ||
				cmd.Flags().Changed("layout") ||
				cmd.Flags().Changed("planner") ||
				cmd.Flags().Changed("llm-url") ||
				cmd.Flags().Changed("llm-model") ||
				cmd.Flags().Changed("llm-timeout")
			if plain && reportRequested {
				return fmt.Errorf("--plain and report flags are mutually exclusive")
			}
			if markdown && reportRequested {
				return fmt.Errorf("--markdown and report flags are mutually exclusive")
			}
			if frame && reportRequested {
				return fmt.Errorf("--frame and report flags are mutually exclusive")
			}
			if plan && output != "" {
				return fmt.Errorf("--plan and --output are mutually exclusive")
			}
			if stdout && output != "" {
				return fmt.Errorf("--stdout and --output are mutually exclusive")
			}
			if err := validateReportFlags(mode, layout, planner); err != nil {
				return err
			}
			// Optional user preferences; a missing file yields a zero Config and
			// reproduces current behavior. A malformed file errors here.
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("code-theme") {
				codeTheme = cfg.DefaultCodeTheme
			}
			opts := actions.Options{
				Context:    cmd.Context(),
				NoOpen:     noOpen,
				Force:      force,
				Safe:       safe,
				Plain:      plain,
				Markdown:   markdown,
				Frame:      frame,
				Title:      title,
				Lang:       lang,
				CodeTheme:  codeTheme,
				OpenCmd:    cfg.OpenCommand,
				MaxWidth:   cfg.MaxWidth,
				Theme:      cfg.DefaultTheme,
				Palette:    cfg.DefaultPalette,
				TOC:        cfg.TOC,
				Output:     output,
				Report:     reportRequested,
				Plan:       plan,
				Stdout:     stdout,
				Mode:       mode,
				Layout:     layout,
				Planner:    planner,
				LLMURL:     llmURL,
				LLMModel:   llmModel,
				LLMTimeout: llmTimeout,
			}
			switch {
			case len(args) == 1:
				opts.File = args[0]
			case isPiped(cmd.InOrStdin()):
				opts.Stdin = cmd.InOrStdin()
			default:
				return fmt.Errorf("no input: provide a file path or pipe data (e.g. `tree -d | html`)")
			}

			res, err := actions.RunWithResult(opts)
			if res.Stdout != "" {
				fmt.Fprint(cmd.OutOrStdout(), res.Stdout)
			} else if res.Path != "" {
				fmt.Fprintln(cmd.OutOrStdout(), res.Path)
			}
			return err
		},
	}
	cmd.Flags().BoolVarP(&noOpen, "no-open", "n", false, "render only; print the cache path without opening the browser")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "rebuild even if the cached HTML is fresh")
	cmd.Flags().BoolVar(&safe, "safe", false, "disable raw HTML passthrough (safe for untrusted Markdown)")
	cmd.Flags().BoolVarP(&plain, "plain", "p", false, "render input as preformatted plain text, not Markdown")
	cmd.Flags().BoolVarP(&markdown, "markdown", "m", false, "render input as Markdown (overrides stdin auto-detection)")
	cmd.Flags().BoolVar(&frame, "frame", false, "wrap plain/ANSI output in a terminal-window frame (implies --plain)")
	cmd.Flags().StringVarP(&title, "title", "t", "stdin", "page title for piped input")
	cmd.Flags().StringVarP(&lang, "lang", "l", "", "syntax-highlight language for plain mode (e.g. go, json; \"text\" = no highlighting)")
	cmd.Flags().StringVar(&codeTheme, "code-theme", "", "chroma style for code blocks (e.g. dracula, monokai, nord; empty = github/github-dark)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "write the final HTML document to a stable path (\"-\" writes stdout)")
	cmd.Flags().BoolVar(&plan, "plan", false, "print the report plan JSON without rendering")
	cmd.Flags().BoolVar(&stdout, "stdout", false, "write the final HTML document to stdout without opening")
	cmd.Flags().Var((*modeValue)(&mode), "mode", "report mode: auto, article, table, cards, review, diff, log, code, tree")
	cmd.Flags().Var((*layoutValue)(&layout), "layout", "report layout: auto, single, tabs, slides, review")
	cmd.Flags().Var((*plannerValue)(&planner), "planner", "planner policy: off, auto, llm")
	cmd.Flags().StringVar(&llmURL, "llm-url", reportDefaults.LLMURL, "OpenAI-compatible chat completions URL for the optional planner")
	cmd.Flags().StringVar(&llmModel, "llm-model", reportDefaults.LLMModel, "model name for the optional planner")
	cmd.Flags().StringVar(&llmTimeout, "llm-timeout", reportDefaults.LLMTimeout, "timeout for the optional planner")
	mustMarkHidden(cmd, "llm-url")
	mustMarkHidden(cmd, "llm-model")
	mustMarkHidden(cmd, "llm-timeout")
	return cmd
}

func mustMarkHidden(cmd *cobra.Command, name string) {
	if err := cmd.Flags().MarkHidden(name); err != nil {
		panic(fmt.Sprintf("hide flag %s: %v", name, err))
	}
}

// isPiped reports whether r carries piped/redirected data rather than an
// interactive terminal. Only an *os.File (e.g. os.Stdin) can be a TTY; any other
// reader (such as a test buffer) is treated as piped.
func isPiped(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return true
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}

type modeValue report.ModeOverride

func (v *modeValue) String() string { return string(*v) }
func (v *modeValue) Type() string   { return "mode" }
func (v *modeValue) Set(s string) error {
	m := report.ModeOverride(s)
	switch m {
	case report.ModeOverrideAuto, report.ModeOverrideArticle, report.ModeOverrideTable, report.ModeOverrideCards, report.ModeOverrideReview, report.ModeOverrideDiff, report.ModeOverrideLog, report.ModeOverrideCode, report.ModeOverrideTree:
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
