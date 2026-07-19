package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/dotcommander/html/internal/actions"
	"github.com/dotcommander/html/internal/config"
	"github.com/dotcommander/html/internal/report"
)

func Execute() error { return newRootCmd().Execute() }

type command struct {
	args   []string
	in     io.Reader
	out    io.Writer
	errOut io.Writer
	ctx    context.Context
}

func newRootCmd() *command {
	return &command{args: os.Args[1:], in: os.Stdin, out: os.Stdout, errOut: os.Stderr, ctx: context.Background()}
}

func (cmd *command) SetArgs(args []string) { cmd.args = args }
func (cmd *command) SetIn(in io.Reader)    { cmd.in = in }
func (cmd *command) SetOut(out io.Writer)  { cmd.out = out }
func (cmd *command) SetErr(out io.Writer)  { cmd.errOut = out }

func (cmd *command) Execute() error {
	var noOpen, force, safe, plain, markdown, frame bool
	var plan, stdout, version bool
	var title, lang, codeTheme, output string
	reportDefaults := report.DefaultOptions()
	mode := reportDefaults.Mode
	layout := reportDefaults.Layout
	planner := reportDefaults.Planner
	llmURL := reportDefaults.LLMURL
	llmModel := reportDefaults.LLMModel
	llmTimeout := reportDefaults.LLMTimeout
	fs := flag.NewFlagSet("html", flag.ContinueOnError)
	// The flag package otherwise prints a diagnostic and returns the same error;
	// main owns the single user-facing diagnostic.
	fs.SetOutput(io.Discard)
	fs.Usage = func() { printUsage(cmd.out, fs) }
	boolFlag(fs, &noOpen, "no-open", "n", "render only; print the cache path without opening the browser")
	boolFlag(fs, &force, "force", "f", "rebuild even if the cached HTML is fresh")
	fs.BoolVar(&safe, "safe", false, "disable raw HTML passthrough (safe for untrusted Markdown)")
	boolFlag(fs, &plain, "plain", "p", "render input as preformatted plain text, not Markdown")
	boolFlag(fs, &markdown, "markdown", "m", "render input as Markdown (overrides stdin auto-detection)")
	fs.BoolVar(&frame, "frame", false, "wrap plain/ANSI output in a terminal-window frame (implies --plain)")
	stringFlag(fs, &title, "title", "t", "stdin", "page title for piped input")
	stringFlag(fs, &lang, "lang", "l", "", "syntax-highlight language for plain mode (e.g. go, json; \"text\" = no highlighting)")
	fs.StringVar(&codeTheme, "code-theme", "", "chroma style for code blocks (e.g. dracula, monokai, nord; empty = github/github-dark)")
	stringFlag(fs, &output, "output", "o", "", "write the final HTML document to a stable path (\"-\" writes stdout)")
	fs.BoolVar(&plan, "plan", false, "print the report plan JSON without rendering")
	fs.BoolVar(&stdout, "stdout", false, "write the final HTML document to stdout without opening")
	fs.BoolVar(&version, "version", false, "print version and exit")
	fs.Var((*modeValue)(&mode), "mode", "report mode: auto, article, table, cards, chart, review, diff, log, code, tree")
	fs.Var((*layoutValue)(&layout), "layout", "report layout: auto, single, tabs, slides, review")
	fs.Var((*plannerValue)(&planner), "planner", "planner policy: off, auto, llm")
	fs.StringVar(&llmURL, "llm-url", reportDefaults.LLMURL, "HTTP(S) OpenAI-compatible chat completions endpoint")
	fs.StringVar(&llmModel, "llm-model", reportDefaults.LLMModel, "model name for the optional planner")
	fs.StringVar(&llmTimeout, "llm-timeout", reportDefaults.LLMTimeout, "timeout for the optional planner")

	normalized, err := interspersedArgs(fs, cmd.args)
	if err != nil {
		return err
	}
	if err := fs.Parse(normalized); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	args := fs.Args()
	if len(args) > 1 {
		return fmt.Errorf("accepts at most 1 arg(s), received %d", len(args))
	}
	changed := changedFlags(fs)
	if version {
		fmt.Fprintln(cmd.out, versionString())
		return nil
	}

	if plain && markdown {
		return fmt.Errorf("--plain and --markdown are mutually exclusive")
	}
	if frame && markdown {
		return fmt.Errorf("--frame and --markdown are mutually exclusive")
	}
	reportRequested := plan ||
		changed["mode"] || changed["layout"] || changed["planner"] ||
		changed["llm-url"] || changed["llm-model"] || changed["llm-timeout"]
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
	if planner != report.PlannerOff {
		if !changed["planner"] {
			return fmt.Errorf("--llm-url and --llm-model require explicit --planner auto or --planner llm")
		}
		if llmModel == "" {
			return fmt.Errorf("--planner %s requires nonempty --llm-model", planner)
		}
		u, err := url.ParseRequestURI(llmURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("--planner %s requires an HTTP(S) --llm-url", planner)
		}
	} else if changed["llm-url"] || changed["llm-model"] || changed["llm-timeout"] {
		return fmt.Errorf("LLM flags require explicit --planner auto or --planner llm")
	}
	// Optional user preferences; a missing file yields a zero Config and
	// reproduces current behavior. A malformed file errors here.
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !changed["code-theme"] {
		codeTheme = cfg.DefaultCodeTheme
	}
	opts := actions.Options{
		Context:    cmd.ctx,
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
	case isPiped(cmd.in):
		opts.Stdin = cmd.in
	default:
		return fmt.Errorf("no input: provide a file path or pipe data (e.g. `tree -d | html`)")
	}

	res, err := actions.RunWithResult(opts)
	for _, diagnostic := range res.Diagnostics {
		fmt.Fprintf(cmd.errOut, "html: warning: [%s] image %q was not embedded\n", diagnostic.Code, diagnostic.Destination)
	}
	if res.Stdout != "" {
		fmt.Fprint(cmd.out, res.Stdout)
	} else if res.Path != "" {
		fmt.Fprintln(cmd.out, res.Path)
	}
	return err
}
