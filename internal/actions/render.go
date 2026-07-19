package actions

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dotcommander/html/internal/cache"
	"github.com/dotcommander/html/internal/render"
)

func renderFile(opts Options) (string, []render.ImageDiagnostic, error) {
	info, err := os.Stat(opts.File)
	if err != nil {
		return "", nil, fmt.Errorf("source file: %w", err)
	}
	if info.IsDir() {
		return "", nil, fmt.Errorf("source file: %s is a directory", opts.File)
	}

	fallbackTitle := strings.TrimSuffix(filepath.Base(opts.File), filepath.Ext(opts.File))
	plain := resolveMode(opts.Plain, opts.Markdown, !isMarkdownExt(opts.File))
	renderOpts := buildRenderOpts(opts, fallbackTitle, filepath.Base(opts.File), plain)

	f, err := os.Open(opts.File)
	if err != nil {
		return "", nil, fmt.Errorf("source file: %w", err)
	}
	src, err := readCapped(f, "source file "+opts.File)
	f.Close()
	if err != nil {
		return "", nil, err
	}
	if render.Detect(src) == render.KindBinary {
		return "", nil, errBinaryInput
	}
	addImageFingerprint(src, &renderOpts)
	fp := render.Fingerprint(renderOpts)

	fresh, err := cache.Fresh(opts.File, src, fp)
	if err != nil {
		return "", nil, err
	}
	if fresh && !opts.Force {
		path, err := cache.PathFor(opts.File)
		return path, render.ImageDiagnostics(src, renderOpts), err
	}

	htmlDoc, diagnostics, err := render.RenderWithDiagnostics(src, renderOpts)
	if err != nil {
		return "", nil, err
	}
	path, err := cache.Write(opts.File, src, htmlDoc, fp)
	return path, diagnostics, err
}

// renderStdin renders piped data. The bytes must be read up front (to auto-detect
// the mode and to key the cache by content), so there is no mtime fast-path: the
// content hash is the cache key.
func renderStdin(opts Options) (string, []render.ImageDiagnostic, error) {
	src, err := readCapped(opts.Stdin, "stdin")
	if err != nil {
		return "", nil, err
	}
	if len(src) == 0 {
		return "", nil, errors.New("no input on stdin")
	}

	kind := render.Detect(src)
	if kind == render.KindBinary {
		return "", nil, errBinaryInput
	}
	plain := resolveMode(opts.Plain, opts.Markdown, kind != render.KindMarkdown)
	renderOpts := buildRenderOpts(opts, stdinTitle(opts.Title), "", plain)
	addImageFingerprint(src, &renderOpts)
	fp := render.Fingerprint(renderOpts)

	fresh, err := cache.FreshContent(src, fp)
	if err != nil {
		return "", nil, err
	}
	if fresh && !opts.Force {
		path, err := cache.PathForContent(src)
		return path, render.ImageDiagnostics(src, renderOpts), err
	}

	htmlDoc, diagnostics, err := render.RenderWithDiagnostics(src, renderOpts)
	if err != nil {
		return "", nil, err
	}
	path, err := cache.WriteContent(src, htmlDoc, fp)
	return path, diagnostics, err
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
		FallbackTitle: fallbackTitle,
		SourceName:    sourceName,
		SourceDir:     sourceDir,
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
