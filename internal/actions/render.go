package actions

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/dotcommander/html/internal/cache"
	"github.com/dotcommander/html/internal/render"
)

// renderCachedDocument renders normal document input through the appropriate
// file or content cache without changing the renderer's source-specific policy.
func renderCachedDocument(opts Options) (string, []render.ImageDiagnostic, error) {
	doc, err := prepareDocument(opts)
	if err != nil {
		return "", nil, err
	}
	fp := render.Fingerprint(doc.options)
	route := cacheRouteFor(opts, doc.src)
	if err := route.rejectAliases(opts); err != nil {
		return "", nil, err
	}

	path, hit, err := route.lookup(fp, opts.Force)
	if err != nil {
		return "", nil, err
	}
	if hit {
		return path, render.ImageDiagnostics(doc.src, doc.options), nil
	}

	htmlDoc, diagnostics, err := render.RenderWithDiagnostics(doc.src, doc.options)
	if err != nil {
		return "", nil, err
	}
	path, err = route.write(htmlDoc, fp)
	return path, diagnostics, err
}

type preparedDocument struct {
	src     []byte
	options render.Options
}

// prepareDocument owns source-specific document preparation shared by cached
// and explicit-output rendering.
func prepareDocument(opts Options) (preparedDocument, error) {
	src, fallbackTitle, sourceName, err := readInput(opts)
	if err != nil {
		return preparedDocument{}, err
	}
	kind := render.Detect(src)
	if kind == render.KindBinary {
		return preparedDocument{}, errBinaryInput
	}
	autoPlain := !isMarkdownExt(opts.File)
	if opts.Stdin != nil {
		autoPlain = kind != render.KindMarkdown
	}
	plain := resolveMode(opts.Plain, opts.Markdown, autoPlain)
	renderOpts := buildRenderOpts(opts, fallbackTitle, sourceName, plain)
	if err := render.ValidateTemplateMode(renderOpts, false); err != nil {
		return preparedDocument{}, err
	}
	addImageFingerprint(src, &renderOpts)
	return preparedDocument{src: src, options: renderOpts}, nil
}

// cacheRoute preserves the cache API distinction between source files and piped
// content while giving each renderer one cache lifecycle to call.
type cacheRoute struct {
	file  string
	src   []byte
	stdin bool
}

func cacheRouteFor(opts Options, src []byte) cacheRoute {
	return cacheRoute{file: opts.File, src: src, stdin: opts.Stdin != nil}
}

// lookup resolves a reusable cache entry, leaving writes with the caller.
func (route cacheRoute) lookup(fingerprint string, force bool) (string, bool, error) {
	fresh, err := route.fresh(fingerprint)
	if err != nil {
		return "", false, err
	}
	if !fresh || force {
		return "", false, nil
	}
	path, err := route.path()
	if err != nil {
		return "", false, err
	}
	return path, true, nil
}

func (route cacheRoute) fresh(fingerprint string) (bool, error) {
	if route.stdin {
		return cache.FreshContent(route.src, fingerprint)
	}
	return cache.Fresh(route.file, route.src, fingerprint)
}

func (route cacheRoute) path() (string, error) {
	if route.stdin {
		return cache.PathForContent(route.src)
	}
	return cache.PathFor(route.file)
}

// Cache publication has two destinations. Check both before lookup as even a
// cache hit can tighten their permissions, and a miss replaces their contents.
func (route cacheRoute) rejectAliases(opts Options) error {
	path, err := route.path()
	if err != nil {
		return err
	}
	for _, target := range cache.PublicationPaths(path) {
		opts.Output = target
		if err := rejectOutputAlias(opts); err != nil {
			return err
		}
	}
	return nil
}

func (route cacheRoute) write(htmlDoc, fingerprint string) (string, error) {
	if route.stdin {
		return cache.WriteContent(route.src, htmlDoc, fingerprint)
	}
	return cache.Write(route.file, route.src, htmlDoc, fingerprint)
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
		if abs, err := filepath.Abs(sourceDir); err == nil {
			sourceDir = abs
		}
	}
	return render.Options{
		Template:       opts.Template,
		TemplateSource: opts.templateSource,
		FallbackTitle:  fallbackTitle,
		SourceName:     sourceName,
		SourceDir:      sourceDir,
		// Safe mode keeps relative links as written. Goldmark intentionally blocks
		// generated file: URLs, and bypassing that guard would weaken its untrusted-
		// input boundary.
		RebaseLocalLinks: opts.File != "" && opts.Output == "" && !opts.Stdout && !opts.Safe,
		Lang:             opts.Lang,
		CodeTheme:        opts.CodeTheme,
		Safe:             opts.Safe,
		MaxWidth:         opts.MaxWidth,
		Theme:            opts.Theme,
		Palette:          opts.Palette,
		TOC:              opts.TOC,
		Plain:            plain,
		Frame:            opts.Frame,
	}
}

func addImageFingerprint(src []byte, opts *render.Options) {
	if opts.Safe {
		return
	}
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
