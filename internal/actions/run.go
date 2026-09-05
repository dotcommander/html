package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/dotcommander/html/internal/atomicfile"
	"github.com/dotcommander/html/internal/open"
	"github.com/dotcommander/html/internal/render"
	"github.com/dotcommander/html/internal/report"
)

// maxMarkdownBytes caps source reads at 32 MiB. A source beyond this cap is
// rejected with a size error rather than silently truncated into a corrupt
// render. The same cap applies to files and to piped stdin.
const maxMarkdownBytes = 32 << 20

// errBinaryInput is returned when a source looks like binary data; rendering it
// as text would produce garbage, so it is refused — no flag overrides this.
var errBinaryInput = errors.New("input looks binary (NUL or non-text bytes); refusing to render")

// Options controls a single render-and-open invocation. Exactly one input source
// is used: Stdin when non-nil (piped data), otherwise File (a path on disk).
type Options struct {
	Context   context.Context
	File      string    // path to the source file (empty when reading Stdin)
	Stdin     io.Reader // piped source; non-nil selects stdin mode (injectable for tests)
	NoOpen    bool      // render only; do not launch the browser
	Force     bool      // rebuild even if the cache is fresh
	Safe      bool      // disable raw HTML passthrough (safe for untrusted Markdown)
	Plain     bool      // force preformatted plain-text rendering
	Markdown  bool      // force Markdown rendering (overrides auto-detection)
	Frame     bool      // --frame: wrap plain/ANSI output in terminal-window chrome (implies Plain)
	Title     string    // page title for stdin input (file input uses the basename)
	Lang      string    // force a syntax-highlight language for plain mode ("" = auto; "text" = raw)
	CodeTheme string    // chroma style for code blocks; "" = github/github-dark default
	OpenCmd   string    // launcher command (config open_command); "" = OS default
	MaxWidth  string    // reader column CSS max-width (config max_width); "" = default
	Theme     string    // initial theme (config default_theme): "light"|"dark"|"auto"|""
	Palette   string    // initial palette (config default_palette): sepia|blue|green|rose|catppuccin|""
	TOC       *bool     // TOC override (config toc): nil = automatic
	Template  string    // default, reader, notebook, or a caller-relative template file
	Output    string    // -o: write rendered HTML to this path ("-" = stdout) instead of caching+opening; "" = default

	// templateSource and templatePath are prepared once per invocation. Template
	// discovery stays in actions so the renderer only receives trusted bytes.
	templateSource string
	templatePath   string
	templateInfo   os.FileInfo

	Report     bool
	Plan       bool
	Stdout     bool
	Mode       report.ModeOverride
	Layout     report.LayoutOverride
	Planner    report.PlannerMode
	LLMURL     string
	LLMModel   string
	LLMTimeout string
}

type Result struct {
	Path        string
	Stdout      string
	Diagnostics []render.ImageDiagnostic
}

// Run renders the configured source (Options.File or Options.Stdin) to its cache
// file (reusing a fresh cache unless Force), optionally opens it in the browser,
// and returns the cache file path. The path is returned as soon as it is known
// so callers can print it even if opening the browser later fails.
func Run(opts Options) (path string, err error) {
	res, err := RunWithResult(opts)
	return res.Path, err
}

func RunWithResult(opts Options) (Result, error) {
	if opts.Frame {
		opts.Plain = true // --frame renders the body as plain text inside terminal chrome
	}
	if opts.Plan && opts.Template != "" {
		return Result{}, errors.New("--template and --plan are mutually exclusive")
	}
	if err := prepareTemplate(&opts); err != nil {
		return Result{}, err
	}
	if opts.CodeTheme != "" && !render.ValidCodeTheme(opts.CodeTheme) {
		return Result{}, fmt.Errorf("code theme: unknown chroma style %q", opts.CodeTheme)
	}
	if opts.Report || opts.Plan {
		return runReport(opts)
	}
	if opts.Stdout || opts.Output != "" {
		return runDocumentOutput(opts)
	}
	path, diagnostics, err := renderCachedDocument(opts)
	if err != nil {
		return Result{Path: path, Diagnostics: diagnostics}, err
	}
	return finalizePublication(opts, path, diagnostics)
}

func runDocumentOutput(opts Options) (Result, error) {
	if err := rejectOutputAlias(opts); err != nil {
		return Result{}, err
	}
	doc, err := prepareDocument(opts)
	if err != nil {
		return Result{}, err
	}
	htmlDoc, diagnostics, err := render.RenderWithDiagnostics(doc.src, doc.options)
	if err != nil {
		return Result{}, err
	}
	return publishExplicitOutput(opts, htmlDoc, diagnostics)
}

func runReport(opts Options) (Result, error) {
	if err := rejectOutputAlias(opts); err != nil {
		return Result{}, err
	}
	if err := render.ValidateTemplateMode(render.Options{Template: opts.Template}, true); err != nil {
		return Result{}, err
	}
	src, fallbackTitle, sourceName, err := readInput(opts)
	if err != nil {
		return Result{}, err
	}
	ctx := opts.Context
	if ctx == nil {
		ctx = context.TODO()
	}
	reportOpts := report.Options{
		Mode:          opts.Mode,
		Layout:        opts.Layout,
		Planner:       opts.Planner,
		LLMURL:        opts.LLMURL,
		LLMModel:      opts.LLMModel,
		LLMTimeout:    opts.LLMTimeout,
		FallbackTitle: fallbackTitle,
		SourceName:    sourceName,
	}
	analysis, plan := report.Plan(ctx, src, reportOpts)
	if opts.Plan {
		b, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return Result{}, fmt.Errorf("plan json: %w", err)
		}
		return Result{Stdout: string(b) + "\n"}, nil
	}
	plain := resolveMode(opts.Plain, opts.Markdown, analysis.Kind != report.KindMarkdown)
	renderOpts := buildRenderOpts(opts, fallbackTitle, sourceName, plain)
	renderOpts.ReportTag = reportCacheTag(analysis, plan, opts)
	addImageFingerprint(src, &renderOpts)
	var diagnostics []render.ImageDiagnostic
	if reportRendersArticle(plan) {
		diagnostics = render.ImageDiagnostics(src, renderOpts)
	}
	htmlDoc, err := render.RenderReport(src, renderOpts, analysis, plan)
	if err != nil {
		return Result{}, err
	}
	if opts.Stdout || opts.Output != "" {
		return publishExplicitOutput(opts, htmlDoc, diagnostics)
	}

	fp := render.Fingerprint(renderOpts)
	route := cacheRouteFor(opts, src)
	if err := route.rejectAliases(opts); err != nil {
		return Result{}, err
	}
	path, hit, err := route.lookup(fp, opts.Force)
	if err != nil {
		return Result{}, err
	}
	if !hit {
		path, err = route.write(htmlDoc, fp)
	}
	if err != nil {
		return Result{}, err
	}
	return finalizePublication(opts, path, diagnostics)
}

// publishExplicitOutput owns publication for every rendered document that
// bypasses the cache, regardless of which renderer produced it.
func publishExplicitOutput(opts Options, htmlDoc string, diagnostics []render.ImageDiagnostic) (Result, error) {
	if opts.Stdout || opts.Output == "-" {
		return Result{Stdout: htmlDoc, Diagnostics: diagnostics}, nil
	}
	if err := atomicfile.Write(opts.Output, []byte(htmlDoc), 0o644); err != nil {
		return Result{}, fmt.Errorf("write output: %w", err)
	}
	return finalizePublication(opts, opts.Output, diagnostics)
}

// finalizePublication returns the published artifact and optionally opens it.
func finalizePublication(opts Options, path string, diagnostics []render.ImageDiagnostic) (Result, error) {
	if !opts.NoOpen {
		if err := open.Open(path, opts.OpenCmd); err != nil {
			return Result{Path: path, Diagnostics: diagnostics}, fmt.Errorf("open browser: %w", err)
		}
	}
	return Result{Path: path, Diagnostics: diagnostics}, nil
}

func reportRendersArticle(plan report.ReportPlan) bool {
	for _, component := range plan.Components {
		if component.Type == report.ComponentArticle || component.Type == report.ComponentTimeline {
			return true
		}
	}
	return false
}
