package actions

import (
	"os"
	"strings"
	"testing"

	"github.com/dotcommander/html/internal/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_NoOpen(t *testing.T) {
	t.Parallel()

	// Write a minimal markdown source into a temp file.
	f, err := os.CreateTemp(t.TempDir(), "src*.md")
	require.NoError(t, err)
	_, err = f.WriteString("# Test Heading\n\nA paragraph.\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// Cleanup the cache entry this test creates.
	t.Cleanup(func() {
		p, cerr := cache.PathFor(f.Name())
		if cerr == nil {
			_ = os.Remove(p)
			_ = os.Remove(strings.TrimSuffix(p, ".html") + ".fp")
		}
	})

	path, err := Run(Options{File: f.Name(), NoOpen: true})
	require.NoError(t, err)
	assert.NotEmpty(t, path)

	// Cache file must exist and contain rendered HTML.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(data), "<!DOCTYPE html>"),
		"expected rendered HTML in cache file")
}
