package render

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// maxInlineImage caps a single inlined image; larger files are left as external
// references rather than bloating the document.
const maxInlineImage = 10 << 20 // 10 MiB

// baseDirKey carries the directory that local image paths resolve against. It is
// set per-render from Options.SourceDir; when absent (e.g. the title-extraction
// re-parse), the transformer is a no-op.
var baseDirKey = parser.NewContextKey()

var mimeByExt = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".svg":  "image/svg+xml",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".ico":  "image/x-icon",
}

// imageInliner rewrites local image destinations to base64 data: URIs so the
// rendered page carries its images inline (no external requests).
type imageInliner struct{}

func (imageInliner) Transform(doc *ast.Document, _ text.Reader, pc parser.Context) {
	baseDir, _ := pc.Get(baseDirKey).(string)
	if baseDir == "" {
		return
	}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		img, ok := n.(*ast.Image)
		if !ok {
			return ast.WalkContinue, nil
		}
		if encoded, ok := inlineImage(baseDir, string(img.Destination)); ok {
			img.Destination = []byte(encoded)
		}
		return ast.WalkContinue, nil
	})
}

// inlineImage resolves a local image reference to a data: URI. It returns ok=false
// (leaving the original ref) for remote/data refs, unknown types, missing/oversize
// files, or any read error — image inlining never fails a render.
func inlineImage(baseDir, dest string) (string, bool) {
	if dest == "" || strings.HasPrefix(dest, "data:") ||
		strings.HasPrefix(dest, "//") || strings.Contains(dest, "://") {
		return "", false
	}
	// Drop any #fragment / ?query before touching the filesystem.
	clean := dest
	if i := strings.IndexAny(clean, "?#"); i >= 0 {
		clean = clean[:i]
	}
	if clean == "" {
		return "", false
	}
	mime, ok := mimeByExt[strings.ToLower(filepath.Ext(clean))]
	if !ok {
		return "", false
	}
	p := clean
	if !filepath.IsAbs(p) {
		p = filepath.Join(baseDir, p)
	}
	info, err := os.Stat(p)
	if err != nil || info.IsDir() || info.Size() > maxInlineImage {
		return "", false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b), true
}
