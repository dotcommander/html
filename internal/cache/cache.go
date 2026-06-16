package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// dir returns the cache directory path (~/.config/html/cache).
// It does not create the directory.
func dir() (string, error) {
	// HTML_CACHE_DIR overrides the cache location (used by tests; also a user escape hatch).
	if d := os.Getenv("HTML_CACHE_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cache: home dir: %w", err)
	}
	return filepath.Join(home, ".config", "html", "cache"), nil
}

// keyFromBytes hashes raw bytes into a "<sha256-hex>.html" cache filename.
// Both the path-keyed and content-keyed callers funnel through here so the
// filename shape lives in exactly one place.
func keyFromBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]) + ".html"
}

// keyFromString is keyFromBytes for an identity string (e.g. a resolved path).
func keyFromString(s string) string { return keyFromBytes([]byte(s)) }

// key computes the cache filename for srcPath.
// Uses sha256(realpath(srcPath)) so the key is stable across relative paths
// and symlinks. If EvalSymlinks fails (e.g. on a transient error), falls back
// to the absolute path so callers still get a valid key.
func key(srcPath string) (string, error) {
	abs, err := filepath.Abs(srcPath)
	if err != nil {
		return "", fmt.Errorf("cache: abs path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// Transient or race: fall back to abs so we still have a key.
		resolved = abs
	}
	return keyFromString(resolved), nil
}

// pathForKey joins the cache directory with a precomputed key filename.
func pathForKey(k string) (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, k), nil
}

// PathFor returns the cache file path for the given Markdown source file.
// The key is sha256(realpath(srcPath)) so it is stable regardless of how the
// path was spelled (relative, symlinked, ../). It does NOT create anything.
func PathFor(srcPath string) (string, error) {
	k, err := key(srcPath)
	if err != nil {
		return "", err
	}
	return pathForKey(k)
}

// PathForContent returns the cache file path for an in-memory source (stdin),
// keyed by sha256(content). Identical piped content therefore reuses a single
// cache entry. It does NOT create anything.
func PathForContent(content []byte) (string, error) {
	return pathForKey(keyFromBytes(content))
}

// freshAt reports whether cachePath is a usable cache: it exists, is not older
// than notBefore (a zero notBefore skips the mtime check — used by content-keyed
// callers whose key already guarantees byte identity), AND its fingerprint
// sidecar matches wantFP. Missing cache => false (rebuild). A missing sidecar
// reads as "" and so only matches a wantFP of "".
func freshAt(cachePath string, notBefore time.Time, wantFP string) (bool, error) {
	cacheInfo, err := os.Stat(cachePath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("cache: stat cache: %w", err)
	}
	// Stale if the source is newer than the cache (path-keyed callers only).
	if !notBefore.IsZero() && notBefore.After(cacheInfo.ModTime()) {
		return false, nil
	}
	// Stale if the renderer fingerprint changed (assets/schema/highlight CSS).
	gotFP, _ := os.ReadFile(fpPath(cachePath)) // missing sidecar => "" (mismatch unless wantFP is "")
	return string(gotFP) == wantFP, nil
}

// Fresh reports whether a usable cached HTML already exists for srcPath:
// the cache file exists, its mtime is >= the source's mtime, AND the stored
// renderer fingerprint matches wantFP. Missing cache => false (rebuild).
// A missing/unstattable source => error.
func Fresh(srcPath, wantFP string) (bool, error) {
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return false, fmt.Errorf("cache: stat source: %w", err)
	}
	cachePath, err := PathFor(srcPath)
	if err != nil {
		return false, err
	}
	return freshAt(cachePath, srcInfo.ModTime(), wantFP)
}

// FreshContent reports whether a usable cached HTML already exists for an
// in-memory source: the content-keyed cache file exists AND its fingerprint
// matches wantFP. The content hash is the key, so existence implies the bytes
// are identical — there is no source mtime to compare.
func FreshContent(content []byte, wantFP string) (bool, error) {
	cachePath, err := PathForContent(content)
	if err != nil {
		return false, err
	}
	return freshAt(cachePath, time.Time{}, wantFP)
}

// fpPath returns the fingerprint sidecar path for a cache .html path:
// "<hash>.html" -> "<hash>.fp".
func fpPath(htmlPath string) string {
	return strings.TrimSuffix(htmlPath, ".html") + ".fp"
}

// maxCacheAge bounds how long a rendered cache file is kept. On every Write,
// entries older than this are opportunistically removed (no background GC, no
// manifest). Set <= 0 to disable pruning.
const maxCacheAge = 7 * 24 * time.Hour

// prune removes regular cache entries older than maxAge from d. It is
// best-effort: a single unreadable/locked entry is skipped, and an unreadable
// dir aborts the whole pass silently. Pruning by age (not source freshness)
// also clears orphaned temp files; the in-flight temp is safe because it is
// always newer than the cutoff.
func prune(d string, maxAge time.Duration) {
	if maxAge <= 0 {
		return
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(d, e.Name()))
		}
	}
}

// prepareDir ensures the cache directory exists and opportunistically prunes
// stale entries on the way in, returning the directory path.
func prepareDir() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(d, 0o755); err != nil {
		return "", fmt.Errorf("cache: mkdir: %w", err)
	}
	prune(d, maxCacheAge)
	return d, nil
}

// writeAt atomically writes html and its fingerprint sidecar next to finalPath
// (temp file + rename, so a concurrent reader never observes a partial file).
func writeAt(finalPath, html, fingerprint string) error {
	d := filepath.Dir(finalPath)
	if err := atomicWrite(d, finalPath, html); err != nil {
		return err
	}
	return atomicWrite(d, fpPath(finalPath), fingerprint)
}

// Write atomically writes html to the cache path for srcPath (creating the
// cache directory on first use) plus a fingerprint sidecar, and returns the
// cache file path. Both writes are atomic (temp file + rename) so a concurrent
// reader never observes a half-written file.
func Write(srcPath, html, fingerprint string) (string, error) {
	if _, err := prepareDir(); err != nil {
		return "", err
	}
	finalPath, err := PathFor(srcPath)
	if err != nil {
		return "", err
	}
	if err := writeAt(finalPath, html, fingerprint); err != nil {
		return "", err
	}
	return finalPath, nil
}

// WriteContent atomically writes html to the content-keyed cache path for an
// in-memory source (stdin) plus a fingerprint sidecar, and returns the cache
// file path.
func WriteContent(content []byte, html, fingerprint string) (string, error) {
	if _, err := prepareDir(); err != nil {
		return "", err
	}
	finalPath, err := PathForContent(content)
	if err != nil {
		return "", err
	}
	if err := writeAt(finalPath, html, fingerprint); err != nil {
		return "", err
	}
	return finalPath, nil
}

// atomicWrite writes content to finalPath via a temp file in dir + rename, so a
// concurrent reader never observes a partial file. The temp file is removed on
// any error before the rename lands.
func atomicWrite(dir, finalPath, content string) (err error) {
	tmp, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return fmt.Errorf("cache: create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err = tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("cache: write temp: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("cache: close temp: %w", err)
	}
	if err = os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("cache: chmod temp: %w", err)
	}
	if err = os.Rename(tmpName, finalPath); err != nil {
		return fmt.Errorf("cache: rename to cache: %w", err)
	}
	return nil
}
