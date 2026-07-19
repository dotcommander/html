package render

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// ImageDependencyFingerprint returns a digest of local image references whose
// current filesystem state can affect rendered Markdown. It follows the same
// eligibility rules as inlineImage and records missing/oversize states too, so
// a later file appearance or size-threshold crossing invalidates the cache.
func ImageDependencyFingerprint(src []byte, baseDir string) string {
	if baseDir == "" {
		return ""
	}
	node := mdUnsafe.Parser().Parse(text.NewReader(src))
	seen := map[string]bool{}
	var deps []string
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		img, ok := n.(*ast.Image)
		if !ok {
			return ast.WalkContinue, nil
		}
		dep, ok := imageDependencyState(baseDir, string(img.Destination))
		if ok && !seen[dep] {
			seen[dep] = true
			deps = append(deps, dep)
		}
		return ast.WalkContinue, nil
	})
	if len(deps) == 0 {
		return ""
	}
	slices.Sort(deps)
	h := sha256.New()
	for _, dep := range deps {
		h.Write([]byte(dep))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func imageDependencyState(baseDir, dest string) (string, bool) {
	if dest == "" || strings.HasPrefix(dest, "data:") ||
		strings.HasPrefix(dest, "//") || strings.Contains(dest, "://") {
		return "", false
	}
	clean := dest
	if i := strings.IndexAny(clean, "?#"); i >= 0 {
		clean = clean[:i]
	}
	if clean == "" {
		return "", false
	}
	clean = imageFilesystemPath(clean)
	if _, ok := mimeByExt[strings.ToLower(filepath.Ext(clean))]; !ok {
		return "", false
	}
	abs, contained, err := containedImagePath(baseDir, clean)
	if err != nil || !contained {
		return "", false
	}
	resolved := abs
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		resolved = real
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "missing:" + resolved, true
	}
	if info.IsDir() {
		return "dir:" + resolved, true
	}
	if info.Size() > maxInlineImage {
		return "oversize:" + resolved + ":" + info.ModTime().UTC().Format(time.RFC3339Nano) + ":" + info.Mode().String(), true
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return "unreadable:" + resolved + ":" + info.ModTime().UTC().Format(time.RFC3339Nano), true
	}
	sum := sha256.Sum256(b)
	return "inline:" + resolved + ":" + hex.EncodeToString(sum[:]), true
}

func imageFilesystemPath(path string) string {
	decoded, err := url.PathUnescape(path)
	if err != nil {
		return path
	}
	return decoded
}
