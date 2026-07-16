package render

import (
	"net/url"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_RebasesLocalLinksWhenEnabled(t *testing.T) {
	t.Parallel()

	baseDir := filepath.Join(t.TempDir(), "source dir")
	src := []byte("[next](../next%20page.md?raw=1#details)\n")
	got, err := Render(src, Options{
		FallbackTitle:    "links",
		SourceDir:        baseDir,
		RebaseLocalLinks: true,
	})
	require.NoError(t, err)

	abs, err := filepath.Abs(filepath.Join(baseDir, "..", "next page.md"))
	require.NoError(t, err)
	want := (&url.URL{
		Scheme:   "file",
		Path:     filepath.ToSlash(abs),
		RawQuery: "raw=1",
		Fragment: "details",
	}).String()
	assert.Contains(t, got, `href="`+want+`"`)
}

func TestRender_LocalLinkRebasingLeavesNonRelativeLinksUnchanged(t *testing.T) {
	t.Parallel()

	src := []byte("[anchor](#part) [root](/docs/a.md) [remote](https://example.com/a) [cdn](//cdn.example.com/a) [mail](mailto:a@example.com)\n")
	got, err := Render(src, Options{
		FallbackTitle:    "links",
		SourceDir:        t.TempDir(),
		RebaseLocalLinks: true,
	})
	require.NoError(t, err)

	for _, destination := range []string{"#part", "/docs/a.md", "https://example.com/a", "//cdn.example.com/a", "mailto:a@example.com"} {
		assert.Contains(t, got, `href="`+destination+`"`)
	}
}

func TestRender_DoesNotRebaseLocalLinksByDefault(t *testing.T) {
	t.Parallel()

	got, err := Render([]byte("[next](next.md#part)\n"), Options{
		FallbackTitle: "links",
		SourceDir:     t.TempDir(),
	})
	require.NoError(t, err)
	assert.Contains(t, got, `href="next.md#part"`)
}

func TestFingerprint_LocalLinkRebasingInvalidatesCache(t *testing.T) {
	t.Parallel()

	withoutRebasing := Fingerprint(Options{})
	withRebasing := Fingerprint(Options{SourceDir: "/one", RebaseLocalLinks: true})
	assert.NotEqual(t, withoutRebasing, withRebasing)
	assert.NotEqual(t,
		withRebasing,
		Fingerprint(Options{SourceDir: "/two", RebaseLocalLinks: true}),
		"the effective link base must invalidate a shared cache entry",
	)
}
