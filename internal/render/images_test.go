package render

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInlineImage_LocalPNG(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	imgPath := filepath.Join(tmp, "img.png")
	imgData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9C, 0x63, 0x64, 0x00, 0x00, 0x00,
		0x02, 0x00, 0x01, 0xE2, 0x21, 0xBC, 0x33, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
		0x42, 0x60, 0x82,
	}
	require.NoError(t, os.WriteFile(imgPath, imgData, 0o644))

	got, err := Render([]byte("![alt](img.png)\n"), Options{SourceDir: tmp, FallbackTitle: "t"})
	require.NoError(t, err)
	assert.Contains(t, got, "data:image/png;base64,")
	assert.NotContains(t, got, `src="img.png"`)
}

func TestInlineImage_PercentEncodedLocalPath(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	imgData := []byte("png bytes")
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "my dot.png"), imgData, 0o644))

	src := []byte("![alt](my%20dot.png)\n")
	fpBefore := ImageDependencyFingerprint(src, tmp)
	got, err := Render(src, Options{SourceDir: tmp, FallbackTitle: "t"})
	require.NoError(t, err)
	assert.Contains(t, got, "data:image/png;base64,")
	assert.NotContains(t, got, `src="my%20dot.png"`)

	require.NoError(t, os.WriteFile(filepath.Join(tmp, "my dot.png"), []byte("new png bytes"), 0o644))
	assert.NotEqual(t, fpBefore, ImageDependencyFingerprint(src, tmp))
}

func TestInlineImage_LocalSVG(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="20"><rect width="40" height="20" fill="#2563eb"/></svg>`
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "badge.svg"), []byte(svg), 0o644))

	got, err := Render([]byte("![badge](badge.svg)\n"), Options{SourceDir: tmp, FallbackTitle: "t"})
	require.NoError(t, err)
	assert.Contains(t, got, "data:image/svg+xml;base64,")
	assert.NotContains(t, got, `src="badge.svg"`)
}

func TestInlineImage_RemoteUnchanged(t *testing.T) {
	t.Parallel()

	got, err := Render([]byte("![alt](https://example.com/x.png)\n"), Options{SourceDir: t.TempDir(), FallbackTitle: "t"})
	require.NoError(t, err)
	assert.Contains(t, got, `src="https://example.com/x.png"`)
	assert.NotContains(t, got, "base64")
}
