package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dotcommander/html/internal/atomicfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicationRejectsInterleavedTemplateWrites(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "page.html")
	// Deterministically model A-html, B-html, B-metadata, A-metadata. Each
	// write is atomic but the final pair belongs to different render options.
	require.NoError(t, atomicfile.Write(path, []byte("A page"), 0o600))
	require.NoError(t, writeAt(path, "B page", "template-B"))
	require.NoError(t, atomicfile.Write(fpPath(path), []byte(publicationFingerprint([]byte("A page"), "template-A")), 0o600))
	for _, fp := range []string{"template-A", "template-B"} {
		fresh, err := matchesPublication(path, fp)
		require.NoError(t, err)
		assert.False(t, fresh)
	}
	require.NoError(t, writeAt(path, "A page", "template-A"))
	fresh, err := matchesPublication(path, "template-A")
	require.NoError(t, err)
	assert.True(t, fresh)
}

func TestPublicationRejectsIncompleteOrLegacyPair(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "page.html")
	require.NoError(t, os.WriteFile(path, []byte("page"), 0o600))
	for _, metadata := range []string{"", "legacy", "html-sha256-v1:bad:fp", publicationFingerprint([]byte("other"), "fp")} {
		require.NoError(t, os.WriteFile(fpPath(path), []byte(metadata), 0o600))
		fresh, err := matchesPublication(path, "fp")
		require.NoError(t, err)
		assert.False(t, fresh)
	}
}
