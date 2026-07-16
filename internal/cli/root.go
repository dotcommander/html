package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"runtime/debug"
	"strings"

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

// interspersedArgs allows flags before or after the optional file argument,
// preserves --, and expands grouped boolean shorthands before flag.Parse.
func interspersedArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, 1)
	parsing := true
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if parsing && arg == "--" {
			parsing = false
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !parsing || !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) > 2 && !strings.Contains(arg, "=") {
			expanded := make([]string, 0, len(arg)-1)
			allBool := true
			for _, short := range arg[1:] {
				f := fs.Lookup(string(short))
				if f == nil {
					allBool = false
					break
				}
				boolean, ok := f.Value.(interface{ IsBoolFlag() bool })
				if !ok || !boolean.IsBoolFlag() {
					allBool = false
					break
				}
				expanded = append(expanded, "-"+string(short))
			}
			if allBool {
				flags = append(flags, expanded...)
				continue
			}
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-")
		f := fs.Lookup(name)
		if f == nil || strings.Contains(arg, "=") {
			continue
		}
		if boolean, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && boolean.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		} else if f != nil {
			return nil, fmt.Errorf("flag needs an argument: -%s", name)
		}
	}
	return append(append(flags, "--"), positionals...), nil
}

func boolFlag(fs *flag.FlagSet, target *bool, name, shorthand, usage string) {
	fs.BoolVar(target, name, false, usage)
	fs.BoolVar(target, shorthand, false, usage)
}

func stringFlag(fs *flag.FlagSet, target *string, name, shorthand, value, usage string) {
	fs.StringVar(target, name, value, usage)
	fs.StringVar(target, shorthand, value, usage)
}

func changedFlags(fs *flag.FlagSet) map[string]bool {
	changed := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { changed[f.Name] = true })
	return changed
}

func printUsage(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprintln(w, "Render Markdown or piped text to a clean HTML page and open it in the browser")
	fmt.Fprintln(w, "\nUsage:\n  html [flags] [file]\n\nFlags:")
	fs.VisitAll(func(f *flag.Flag) {
		switch f.Name {
		}
		prefix := "--"
		if len(f.Name) == 1 {
			prefix = "-"
		}
		fmt.Fprintf(w, "  %s%s\n    \t%s (default %s)\n", prefix, f.Name, f.Usage, f.DefValue)
	})
}

func versionString() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || localBuildVersion(info.Main.Version) {
		return "html devel"
	}
	return formatVersion(info.Main.Version)
}

var pseudoVersionRE = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+-(0\.)?[0-9]{14}-[0-9a-f]+(\+.*)?$`)

func localBuildVersion(version string) bool {
	return version == "" || version == "(devel)" || strings.Contains(version, "+dirty") || pseudoVersionRE.MatchString(version)
}

func formatVersion(version string) string {
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return "html " + version
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
