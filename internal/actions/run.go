package actions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dotcommander/html/internal/cache"
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
	Output    string    // -o: write rendered HTML to this path ("-" = stdout) instead of caching+opening; "" = default

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
	Path   string
	Stdout string
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
	if opts.CodeTheme != "" && !render.ValidCodeTheme(opts.CodeTheme) {
		return Result{}, fmt.Errorf("code theme: unknown chroma style %q", opts.CodeTheme)
	}
	if opts.Report || opts.Plan {
		return runReport(opts)
	}
	if opts.Stdout || opts.Output != "" {
		return runDocumentOutput(opts)
	}
	var path string
	var err error
	if opts.Stdin != nil {
		path, err = renderStdin(opts)
	} else {
		path, err = renderFile(opts)
	}
	if err != nil {
		return Result{Path: path}, err
	}
	if !opts.NoOpen {
		if err := open.Open(path, opts.OpenCmd); err != nil {
			return Result{Path: path}, fmt.Errorf("open browser: %w", err)
		}
	}
	return Result{Path: path}, nil
}

func runDocumentOutput(opts Options) (Result, error) {
	src, fallbackTitle, sourceName, err := readInput(opts)
	if err != nil {
		return Result{}, err
	}
	if render.Detect(src) == render.KindBinary {
		return Result{}, errBinaryInput
	}
	plain := false
	if opts.Stdin != nil {
		plain = resolveMode(opts.Plain, opts.Markdown, render.Detect(src) != render.KindMarkdown)
	} else {
		plain = resolveMode(opts.Plain, opts.Markdown, !isMarkdownExt(opts.File))
	}
	renderOpts := buildRenderOpts(opts, fallbackTitle, sourceName, plain)
	addImageFingerprint(src, &renderOpts)
	htmlDoc, err := render.Render(src, renderOpts)
	if err != nil {
		return Result{}, err
	}
	if opts.Stdout || opts.Output == "-" {
		return Result{Stdout: htmlDoc}, nil
	}
	if err := os.WriteFile(opts.Output, []byte(htmlDoc), 0o644); err != nil {
		return Result{}, fmt.Errorf("write output: %w", err)
	}
	if !opts.NoOpen {
		if err := open.Open(opts.Output, opts.OpenCmd); err != nil {
			return Result{Path: opts.Output}, fmt.Errorf("open browser: %w", err)
		}
	}
	return Result{Path: opts.Output}, nil
}

func runReport(opts Options) (Result, error) {
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
	htmlDoc, err := render.RenderReport(src, renderOpts, analysis, plan)
	if err != nil {
		return Result{}, err
	}
	if opts.Stdout || opts.Output == "-" {
		return Result{Stdout: htmlDoc}, nil
	}
	if opts.Output != "" {
		if err := os.WriteFile(opts.Output, []byte(htmlDoc), 0o644); err != nil {
			return Result{}, fmt.Errorf("write output: %w", err)
		}
		if !opts.NoOpen {
			if err := open.Open(opts.Output, opts.OpenCmd); err != nil {
				return Result{Path: opts.Output}, fmt.Errorf("open browser: %w", err)
			}
		}
		return Result{Path: opts.Output}, nil
	}

	fp := render.Fingerprint(renderOpts)
	var path string
	if opts.Stdin != nil {
		fresh, err := cache.FreshContent(src, fp)
		if err != nil {
			return Result{}, err
		}
		if fresh && !opts.Force {
			path, err = cache.PathForContent(src)
		} else {
			path, err = cache.WriteContent(src, htmlDoc, fp)
		}
	} else {
		fresh, err := cache.Fresh(opts.File, fp)
		if err != nil {
			return Result{}, err
		}
		if fresh && !opts.Force {
			path, err = cache.PathFor(opts.File)
		} else {
			path, err = cache.Write(opts.File, htmlDoc, fp)
		}
	}
	if err != nil {
		return Result{}, err
	}
	if !opts.NoOpen {
		if err := open.Open(path, opts.OpenCmd); err != nil {
			return Result{Path: path}, fmt.Errorf("open browser: %w", err)
		}
	}
	return Result{Path: path}, nil
}

func readInput(opts Options) (src []byte, fallbackTitle, sourceName string, err error) {
	if opts.Stdin != nil {
		src, err = readCapped(opts.Stdin, "stdin")
		if err != nil {
			return nil, "", "", err
		}
		if len(src) == 0 {
			return nil, "", "", errors.New("no input on stdin")
		}
		return src, stdinTitle(opts.Title), "", nil
	}
	info, err := os.Stat(opts.File)
	if err != nil {
		return nil, "", "", fmt.Errorf("source file: %w", err)
	}
	if info.IsDir() {
		return nil, "", "", fmt.Errorf("source file: %s is a directory", opts.File)
	}
	f, err := os.Open(opts.File)
	if err != nil {
		return nil, "", "", fmt.Errorf("source file: %w", err)
	}
	src, err = readCapped(f, "source file "+opts.File)
	f.Close()
	if err != nil {
		return nil, "", "", err
	}
	return src, strings.TrimSuffix(filepath.Base(opts.File), filepath.Ext(opts.File)), filepath.Base(opts.File), nil
}

func reportCacheTag(analysis report.Analysis, plan report.ReportPlan, opts Options) string {
	components := make([]reportCacheComponent, 0, len(plan.Components))
	for _, c := range plan.Components {
		components = append(components, reportCacheComponent{Type: c.Type, Title: c.Title})
	}
	renderPlan := struct {
		Analysis   reportCacheAnalysis    `json:"analysis"`
		Layout     report.Layout          `json:"layout"`
		Components []reportCacheComponent `json:"components"`
	}{
		Analysis:   reportCacheAnalysis{Kind: analysis.Kind, Confidence: analysis.Confidence, Reasons: analysis.Reasons, Stats: analysis.Stats},
		Layout:     plan.Layout,
		Components: components,
	}
	b, _ := json.Marshal(renderPlan)
	h := sha256.New()
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

type reportCacheComponent struct {
	Type  report.ComponentType `json:"type"`
	Title string               `json:"title"`
}

type reportCacheAnalysis struct {
	Kind       report.Kind  `json:"kind"`
	Confidence float64      `json:"confidence"`
	Reasons    []string     `json:"reasons"`
	Stats      report.Stats `json:"stats"`
}

// renderFile renders a source file. Mode is decided by extension/flags without
// reading the file, so a fresh cache hit returns immediately without a read.
func renderFile(opts Options) (string, error) {
	info, err := os.Stat(opts.File)
	if err != nil {
		return "", fmt.Errorf("source file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("source file: %s is a directory", opts.File)
	}

	fallbackTitle := strings.TrimSuffix(filepath.Base(opts.File), filepath.Ext(opts.File))
	plain := resolveMode(opts.Plain, opts.Markdown, !isMarkdownExt(opts.File))
	renderOpts := buildRenderOpts(opts, fallbackTitle, filepath.Base(opts.File), plain)

	f, err := os.Open(opts.File)
	if err != nil {
		return "", fmt.Errorf("source file: %w", err)
	}
	src, err := readCapped(f, "source file "+opts.File)
	f.Close()
	if err != nil {
		return "", err
	}
	if render.Detect(src) == render.KindBinary {
		return "", errBinaryInput
	}
	addImageFingerprint(src, &renderOpts)
	fp := render.Fingerprint(renderOpts)

	fresh, err := cache.Fresh(opts.File, fp)
	if err != nil {
		return "", err
	}
	if fresh && !opts.Force {
		return cache.PathFor(opts.File)
	}

	htmlDoc, err := render.Render(src, renderOpts)
	if err != nil {
		return "", err
	}
	return cache.Write(opts.File, htmlDoc, fp)
}

// renderStdin renders piped data. The bytes must be read up front (to auto-detect
// the mode and to key the cache by content), so there is no mtime fast-path: the
// content hash is the cache key.
func renderStdin(opts Options) (string, error) {
	src, err := readCapped(opts.Stdin, "stdin")
	if err != nil {
		return "", err
	}
	if len(src) == 0 {
		return "", errors.New("no input on stdin")
	}

	kind := render.Detect(src)
	if kind == render.KindBinary {
		return "", errBinaryInput
	}
	plain := resolveMode(opts.Plain, opts.Markdown, kind != render.KindMarkdown)
	renderOpts := buildRenderOpts(opts, stdinTitle(opts.Title), "", plain)
	addImageFingerprint(src, &renderOpts)
	fp := render.Fingerprint(renderOpts)

	fresh, err := cache.FreshContent(src, fp)
	if err != nil {
		return "", err
	}
	if fresh && !opts.Force {
		return cache.PathForContent(src)
	}

	htmlDoc, err := render.Render(src, renderOpts)
	if err != nil {
		return "", err
	}
	return cache.WriteContent(src, htmlDoc, fp)
}

// resolveMode decides plain vs Markdown. An explicit flag wins (--markdown, then
// --plain); otherwise autoPlain — the source-specific default (file: by
// extension; stdin: by content detection) — is used.
func resolveMode(plainFlag, mdFlag, autoPlain bool) bool {
	switch {
	case mdFlag:
		return false
	case plainFlag:
		return true
	default:
		return autoPlain
	}
}

// isMarkdownExt reports whether path has a Markdown file extension.
func isMarkdownExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown":
		return true
	}
	return false
}

// buildRenderOpts assembles render.Options from the invocation options, the
// resolved fallback title, and the resolved plain/Markdown mode.
func buildRenderOpts(opts Options, fallbackTitle, sourceName string, plain bool) render.Options {
	sourceDir := ""
	if opts.File != "" {
		sourceDir = filepath.Dir(opts.File)
	}
	return render.Options{
		FallbackTitle: fallbackTitle,
		SourceName:    sourceName,
		SourceDir:     sourceDir,
		Lang:          opts.Lang,
		CodeTheme:     opts.CodeTheme,
		Safe:          opts.Safe,
		MaxWidth:      opts.MaxWidth,
		Theme:         opts.Theme,
		Palette:       opts.Palette,
		TOC:           opts.TOC,
		Plain:         plain,
		Frame:         opts.Frame,
	}
}

func addImageFingerprint(src []byte, opts *render.Options) {
	if opts.Plain || opts.SourceDir == "" {
		return
	}
	opts.ImageFingerprint = render.ImageDependencyFingerprint(src, opts.SourceDir)
}

// stdinTitle returns the page title for piped input, defaulting to "stdin".
func stdinTitle(t string) string {
	if t == "" {
		return "stdin"
	}
	return t
}

// readCapped reads at most maxMarkdownBytes from r — one byte past the cap so an
// over-cap source is detected rather than silently truncated. label names the
// source in any error ("stdin" or "source file <path>").
func readCapped(r io.Reader, label string) ([]byte, error) {
	src, err := io.ReadAll(io.LimitReader(r, maxMarkdownBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	if len(src) > maxMarkdownBytes {
		return nil, fmt.Errorf("%s is too large; cap is %d KiB", label, maxMarkdownBytes>>10)
	}
	return src, nil
}
