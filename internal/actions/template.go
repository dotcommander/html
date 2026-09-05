package actions

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// maxTemplateBytes bounds a caller-supplied page template before parsing. The
// renderer separately bounds the assembled output, so neither phase can turn a
// local template into an unbounded allocation.
const maxTemplateBytes = 1 << 20

func prepareTemplate(opts *Options) error {
	switch opts.Template {
	case "", "default", "reader", "notebook":
		return nil
	}

	info, err := os.Stat(opts.Template)
	if err != nil {
		return fmt.Errorf("template file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("template file: %s is a directory", opts.Template)
	}
	f, err := os.Open(opts.Template)
	if err != nil {
		return fmt.Errorf("template file: %w", err)
	}
	source, readErr := readTemplateCapped(f, opts.Template)
	closeErr := f.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return fmt.Errorf("template file %s: %w", opts.Template, closeErr)
	}
	abs, err := filepath.Abs(opts.Template)
	if err != nil {
		return fmt.Errorf("template path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	opts.templateSource = string(source)
	opts.templatePath = abs
	opts.templateInfo = info
	return nil
}

func readTemplateCapped(r io.Reader, path string) ([]byte, error) {
	source, err := io.ReadAll(io.LimitReader(r, maxTemplateBytes+1))
	if err != nil {
		return nil, fmt.Errorf("template file %s: %w", path, err)
	}
	if len(source) > maxTemplateBytes {
		return nil, fmt.Errorf("template file %s is too large; cap is %d KiB", path, maxTemplateBytes>>10)
	}
	return source, nil
}
