package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cleanupCacheFor registers a t.Cleanup that removes the cache file for srcPath.
// Errors other than not-exist are silently ignored (best-effort cleanup in tests).
func cleanupCacheFor(t *testing.T, srcPath string) {
	t.Helper()
	t.Cleanup(func() {
		p, err := PathFor(srcPath)
		if err != nil {
			return
		}
		_ = os.Remove(p)
		_ = os.Remove(fpPath(p))
	})
}

func TestPathFor_Stable(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "src*.md")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	cleanupCacheFor(t, f.Name())

	p1, err := PathFor(f.Name())
	require.NoError(t, err)
	p2, err := PathFor(f.Name())
	require.NoError(t, err)

	assert.Equal(t, p1, p2)
	assert.True(t, strings.HasSuffix(p1, ".html"), "expected .html suffix, got %s", p1)
	cacheDir, err := dir()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(p1, cacheDir+string(os.PathSeparator)),
		"expected path under active cache dir %s, got %s", cacheDir, p1)
}

func TestPathFor_SymlinkResolves(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	realFile := filepath.Join(dir, "real.md")
	require.NoError(t, os.WriteFile(realFile, []byte("hello"), 0o644))

	symFile := filepath.Join(dir, "link.md")
	require.NoError(t, os.Symlink(realFile, symFile))

	// Both paths share one cache key (EvalSymlinks collapses the symlink).
	cleanupCacheFor(t, realFile)

	pReal, err := PathFor(realFile)
	require.NoError(t, err)
	pSym, err := PathFor(symFile)
	require.NoError(t, err)

	assert.Equal(t, pReal, pSym)
}

func TestFresh_MissingCache(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "src*.md")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// No cache written — expect false, nil.
	fresh, err := Fresh(f.Name(), nil, "")
	require.NoError(t, err)
	assert.False(t, fresh)
}

func TestFresh_AfterWrite(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "src*.md")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	cleanupCacheFor(t, f.Name())

	_, err = Write(f.Name(), nil, "<html>x</html>", "")
	require.NoError(t, err)

	// Advance the cache file's mtime by 1 second beyond the source mtime.
	srcInfo, err := os.Stat(f.Name())
	require.NoError(t, err)
	later := srcInfo.ModTime().Add(time.Second)

	cachePath, err := PathFor(f.Name())
	require.NoError(t, err)
	require.NoError(t, os.Chtimes(cachePath, later, later))

	fresh, err := Fresh(f.Name(), nil, "")
	require.NoError(t, err)
	assert.True(t, fresh)
}

func TestFresh_StaleWhenSourceNewer(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "src*.md")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	cleanupCacheFor(t, f.Name())

	_, err = Write(f.Name(), nil, "<html>y</html>", "")
	require.NoError(t, err)

	// Advance the source file's mtime to be after the cache file's mtime.
	cachePath, err := PathFor(f.Name())
	require.NoError(t, err)
	cacheInfo, err := os.Stat(cachePath)
	require.NoError(t, err)
	laterSrc := cacheInfo.ModTime().Add(time.Second)
	require.NoError(t, os.Chtimes(f.Name(), laterSrc, laterSrc))

	fresh, err := Fresh(f.Name(), nil, "")
	require.NoError(t, err)
	assert.False(t, fresh)
}

func TestWrite_RoundTrip(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "src*.md")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	cleanupCacheFor(t, f.Name())

	const htmlContent = "<html><body>round-trip</body></html>"
	got, err := Write(f.Name(), nil, htmlContent, "")
	require.NoError(t, err)

	data, err := os.ReadFile(got)
	require.NoError(t, err)
	assert.Equal(t, htmlContent, string(data))

	info, err := os.Stat(got)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode()&0o777)

	fingerprintInfo, err := os.Stat(fpPath(got))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fingerprintInfo.Mode()&0o777)

	cacheDirInfo, err := os.Stat(filepath.Dir(got))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), cacheDirInfo.Mode()&0o777)
}

func TestFresh_MissingSource(t *testing.T) {
	t.Parallel()

	_, err := Fresh("/nonexistent/path/xyz.md", nil, "")
	assert.Error(t, err)
}

func TestFresh_FingerprintMismatch(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "src*.md")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	cleanupCacheFor(t, f.Name())

	_, err = Write(f.Name(), nil, "<html>z</html>", "fp-A")
	require.NoError(t, err)

	// Make the cache mtime newer than the source so mtime freshness holds and
	// only the fingerprint governs the result.
	srcInfo, err := os.Stat(f.Name())
	require.NoError(t, err)
	later := srcInfo.ModTime().Add(time.Second)
	cachePath, err := PathFor(f.Name())
	require.NoError(t, err)
	require.NoError(t, os.Chtimes(cachePath, later, later))

	// Matching fingerprint => fresh.
	fresh, err := Fresh(f.Name(), nil, "fp-A")
	require.NoError(t, err)
	assert.True(t, fresh)

	// Changed fingerprint => stale, despite a fresh mtime.
	fresh, err = Fresh(f.Name(), nil, "fp-B")
	require.NoError(t, err)
	assert.False(t, fresh)
}

func TestFresh_StaleWhenSourceBytesChangeWithoutNewerMtime(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source.md")
	original := []byte("first")
	replacement := []byte("other")
	require.Len(t, replacement, len(original))
	require.NoError(t, os.WriteFile(srcPath, original, 0o644))
	cleanupCacheFor(t, srcPath)

	_, err := Write(srcPath, original, "<html>first</html>", "renderer")
	require.NoError(t, err)
	cachePath, err := PathFor(srcPath)
	require.NoError(t, err)
	cacheInfo, err := os.Stat(cachePath)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(srcPath, replacement, 0o644))
	for _, sourceTime := range []time.Time{
		cacheInfo.ModTime(),
		cacheInfo.ModTime().Add(-time.Hour),
	} {
		require.NoError(t, os.Chtimes(srcPath, sourceTime, sourceTime))
		fresh, err := Fresh(srcPath, replacement, "renderer")
		require.NoError(t, err)
		assert.False(t, fresh, "changed bytes must invalidate cache at source mtime %s", sourceTime)
	}
}

func TestFresh_InvalidatesLegacyFingerprint(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source.md")
	source := []byte("source")
	require.NoError(t, os.WriteFile(srcPath, source, 0o644))
	cleanupCacheFor(t, srcPath)

	cachePath, err := PathFor(srcPath)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), 0o700))
	require.NoError(t, os.WriteFile(cachePath, []byte("old html"), 0o600))
	require.NoError(t, os.WriteFile(fpPath(cachePath), []byte("renderer"), 0o600))

	fresh, err := Fresh(srcPath, source, "renderer")
	require.NoError(t, err)
	assert.False(t, fresh)
}

func TestFresh_TightensExistingCachePermissions(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "source.md")
	source := []byte("# private\n")
	require.NoError(t, os.WriteFile(srcPath, source, 0o644))
	cleanupCacheFor(t, srcPath)
	cachePath, err := Write(srcPath, source, "<p>private</p>", "renderer")
	require.NoError(t, err)
	require.NoError(t, os.Chmod(filepath.Dir(cachePath), 0o755))
	require.NoError(t, os.Chmod(cachePath, 0o644))
	require.NoError(t, os.Chmod(fpPath(cachePath), 0o644))

	fresh, err := Fresh(srcPath, source, "renderer")
	require.NoError(t, err)
	require.True(t, fresh)
	for path, want := range map[string]os.FileMode{
		filepath.Dir(cachePath): 0o700,
		cachePath:               0o600,
		fpPath(cachePath):       0o600,
	} {
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, want, info.Mode().Perm(), path)
	}
}
