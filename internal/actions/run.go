package actions

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dotcommander/html/internal/cache"
	"github.com/dotcommander/html/internal/open"
	"github.com/dotcommander/html/internal/render"
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
	File     string    // path to the source file (empty when reading Stdin)
	Stdin    io.Reader // piped source; non-nil selects stdin mode (injectable for tests)
	NoOpen   bool      // render only; do not launch the browser
	Force    bool      // rebuild even if the cache is fresh
	Safe     bool      // disable raw HTML passthrough (safe for untrusted Markdown)
	Plain    bool      // force preformatted plain-text rendering
	Markdown bool      // force Markdown rendering (overrides auto-detection)
	Title    string    // page title for stdin input (file input uses the basename)
	Lang     string    // force a syntax-highlight language for plain mode ("" = auto; "text" = raw)
	OpenCmd  string    // launcher command (config open_command); "" = OS default
	MaxWidth string    // reader column CSS max-width (config max_width); "" = default
	Theme    string    // initial theme (config default_theme): "light"|"dark"|"auto"|""
	TOC      *bool     // TOC override (config toc): nil = automatic
	Output   string    // -o: write rendered HTML to this path ("-" = stdout) instead of caching+opening; "" = default
}

// Run renders the configured source (Options.File or Options.Stdin) to its cache
// file (reusing a fresh cache unless Force), optionally opens it in the browser,
// and returns the cache file path. The path is returned as soon as it is known
// so callers can print it even if opening the browser later fails.
func Run(opts Options) (path string, err error) {
	if opts.Stdin != nil {
		path, err = renderStdin(opts)
	} else {
		path, err = renderFile(opts)
	}
	if err != nil {
		return path, err
	}
	if !opts.NoOpen && opts.Output == "" {
		if err := open.Open(path, opts.OpenCmd); err != nil {
			return path, fmt.Errorf("open browser: %w", err)
		}
	}
	return path, nil
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
	fp := render.Fingerprint(renderOpts)

	fresh, err := cache.Fresh(opts.File, fp)
	if err != nil {
		return "", err
	}
	if fresh && !opts.Force {
		return cache.PathFor(opts.File)
	}

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
	return render.Options{
		FallbackTitle: fallbackTitle,
		SourceName:    sourceName,
		Lang:          opts.Lang,
		Safe:          opts.Safe,
		MaxWidth:      opts.MaxWidth,
		Theme:         opts.Theme,
		TOC:           opts.TOC,
		Plain:         plain,
	}
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
