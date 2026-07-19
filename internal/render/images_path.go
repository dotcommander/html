package render

import (
	"os"
	"path/filepath"
	"strings"
)

// containedImagePath resolves path against baseDir and verifies both its lexical
// path and its symlink-resolved path remain below baseDir. Resolving the deepest
// existing ancestor also covers missing dependency targets beneath symlinked
// directories, keeping rendering and cache fingerprinting on the same boundary.
func containedImagePath(baseDir, path string) (string, bool, error) {
	if baseDir == "" {
		return "", false, nil
	}
	base, err := filepath.Abs(baseDir)
	if err != nil {
		return "", false, err
	}
	resolvedBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", false, err
	}
	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(base, candidate)
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return "", false, err
	}
	if !pathWithin(base, candidate) {
		return "", false, nil
	}
	resolvedCandidate, err := resolveExistingAncestor(candidate)
	if err != nil {
		return "", false, err
	}
	return candidate, pathWithin(resolvedBase, resolvedCandidate), nil
}

func resolveExistingAncestor(path string) (string, error) {
	ancestor := path
	var suffix []string
	for {
		if _, err := os.Lstat(ancestor); err == nil {
			resolved, err := filepath.EvalSymlinks(ancestor)
			if err != nil {
				return "", err
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", os.ErrNotExist
		}
		suffix = append(suffix, filepath.Base(ancestor))
		ancestor = parent
	}
}

func pathWithin(base, candidate string) bool {
	rel, err := filepath.Rel(base, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
