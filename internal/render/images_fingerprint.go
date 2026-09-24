package render

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"slices"
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
	node := MdUnsafe.Parser().Parse(text.NewReader(src))
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
	abs, _, _, ok := classifyInlineImagePath(baseDir, dest)
	if !ok {
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
