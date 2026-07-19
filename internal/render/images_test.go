package render

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
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

func TestInlineImage_LocalReferenceWithoutSourceDirectoryHasNoDiagnostic(t *testing.T) {
	t.Parallel()

	got, diagnostics, err := RenderWithDiagnostics(
		[]byte("![local](image.png)\n"),
		Options{FallbackTitle: "stdin"},
	)
	require.NoError(t, err)
	assert.Contains(t, got, `src="image.png"`)
	assert.NotContains(t, got, "base64")
	assert.Empty(t, diagnostics)
}

func TestInlineImage_RejectsPathsOutsideSourceDirectory(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	baseDir := filepath.Join(parent, "docs")
	require.NoError(t, os.Mkdir(baseDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(parent, "outside.png"), []byte("private bytes"), 0o644))

	src := []byte("![outside](../outside.png)\n")
	got, diagnostics, err := RenderWithDiagnostics(src, Options{SourceDir: baseDir, FallbackTitle: "t"})
	require.NoError(t, err)
	assert.Contains(t, got, `src="../outside.png"`)
	assert.NotContains(t, got, "private bytes")
	assert.Equal(t, []ImageDiagnostic{{Code: DiagnosticImageOutsideRoot, Destination: "../outside.png"}}, diagnostics)
	assert.Empty(t, ImageDependencyFingerprint(src, baseDir))
}

func TestInlineImage_RejectsAbsolutePathOutsideSourceDirectory(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	baseDir := filepath.Join(parent, "docs")
	outPath := filepath.Join(parent, "outside.png")
	require.NoError(t, os.Mkdir(baseDir, 0o755))
	require.NoError(t, os.WriteFile(outPath, []byte("private bytes"), 0o644))

	src := []byte("![outside](" + outPath + ")\n")
	got, diagnostics, err := RenderWithDiagnostics(src, Options{SourceDir: baseDir, FallbackTitle: "t"})
	require.NoError(t, err)
	assert.Contains(t, got, `src="`+outPath+`"`)
	assert.NotContains(t, got, "private bytes")
	assert.NotContains(t, got, "base64")
	assert.Equal(t, []ImageDiagnostic{{Code: DiagnosticImageOutsideRoot, Destination: outPath}}, diagnostics)
	assert.Empty(t, ImageDependencyFingerprint(src, baseDir))
}

func TestInlineImage_RejectsSymlinkEscapesWithFingerprintParity(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	baseDir := filepath.Join(parent, "docs")
	outsideDir := filepath.Join(parent, "private")
	require.NoError(t, os.Mkdir(baseDir, 0o755))
	require.NoError(t, os.Mkdir(outsideDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "outside.png"), []byte("private bytes"), 0o644))
	require.NoError(t, os.Symlink(outsideDir, filepath.Join(baseDir, "linked")))

	src := []byte("![outside](linked/outside.png)\n")
	got, diagnostics, err := RenderWithDiagnostics(src, Options{SourceDir: baseDir, FallbackTitle: "t"})
	require.NoError(t, err)
	assert.Contains(t, got, `src="linked/outside.png"`)
	assert.NotContains(t, got, "private bytes")
	assert.Equal(t, []ImageDiagnostic{{Code: DiagnosticImageOutsideRoot, Destination: "linked/outside.png"}}, diagnostics)
	assert.Empty(t, ImageDependencyFingerprint(src, baseDir))
}

func TestSafeImagesBecomeNonFetchingPlaceholders(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	baseDir := filepath.Join(parent, "docs")
	require.NoError(t, os.Mkdir(baseDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(parent, "outside.png"), []byte("private bytes"), 0o644))
	require.NoError(t, os.Symlink(filepath.Join(parent, "outside.png"), filepath.Join(baseDir, "linked.png")))

	src := []byte(strings.Join([]string{
		`![remote](https://example.com/tracker.png)`,
		`![traversal](../outside.png)`,
		`![symlink](linked.png)`,
		`![unsafe <label> & text](local.png)`,
	}, "\n\n"))
	got, err := Render(src, Options{SourceDir: baseDir, FallbackTitle: "safe", Safe: true})
	require.NoError(t, err)

	assert.NotContains(t, got, "<img")
	assert.NotContains(t, got, `src=`)
	assert.NotContains(t, got, "tracker.png")
	assert.NotContains(t, got, "outside.png")
	assert.NotContains(t, got, "linked.png")
	assert.NotContains(t, got, "private bytes")
	assert.Contains(t, got, "[Image: remote]")
	assert.Contains(t, got, "[Image: traversal]")
	assert.Contains(t, got, "[Image: symlink]")
	assert.Contains(t, got, "[Image: unsafe &lt;label&gt; &amp; text]")
}

func TestInlineImage_AggregateBudgetCountsRepeatedReferences(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "repeat.png"), []byte("xx"), 0o644))
	uri, ok := inlineImage(tmp, "repeat.png")
	require.True(t, ok)
	md := newMarkdownWithImageLimit(true, "", int64(len(uri)*2))
	pc := parser.NewContext()
	pc.Set(baseDirKey, tmp)
	src := []byte("![one](repeat.png)\n![two](repeat.png)\n![three](repeat.png)\n")
	doc := md.Parser().Parse(text.NewReader(src), parser.WithContext(pc))
	var buf bytes.Buffer
	require.NoError(t, md.Renderer().Render(&buf, src, doc))

	got := buf.String()
	assert.Equal(t, 2, strings.Count(got, "data:image/png;base64,"))
	assert.Equal(t, 1, strings.Count(got, `src="repeat.png"`))
}

func TestInlineImage_PerImageBudgetLeavesReferenceExternal(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	f, err := os.Create(filepath.Join(tmp, "large.png"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	require.NoError(t, f.Truncate(maxInlineImage+1))

	got, err := Render([]byte("![large](large.png)\n"), Options{SourceDir: tmp, FallbackTitle: "t"})
	require.NoError(t, err)
	assert.Contains(t, got, `src="large.png"`)
	assert.NotContains(t, got, "data:image/png;base64,")
}

func TestRenderWithDiagnostics_ClassifiesAndDeduplicatesImages(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "local.png"), []byte("png"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(tmp, "directory.png"), 0o755))
	large, err := os.Create(filepath.Join(tmp, "large.png"))
	require.NoError(t, err)
	require.NoError(t, large.Truncate(maxInlineImage+1))
	require.NoError(t, large.Close())

	src := []byte(strings.Join([]string{
		`![remote](https://example.com/x.png)`,
		`![missing](missing.png)`,
		`![missing again](missing.png)`,
		`![unsupported](image.tiff)`,
		`![unreadable](directory.png)`,
		`![oversized](large.png)`,
		`![local](local.png)`,
		`![embedded](data:image/png;base64,eA==)`,
	}, "\n\n"))

	_, diagnostics, err := RenderWithDiagnostics(src, Options{SourceDir: tmp, FallbackTitle: "t"})
	require.NoError(t, err)
	assert.Equal(t, []ImageDiagnostic{
		{Code: DiagnosticImageRemote, Destination: "https://example.com/x.png"},
		{Code: DiagnosticImageMissing, Destination: "missing.png"},
		{Code: DiagnosticImageUnsupported, Destination: "image.tiff"},
		{Code: DiagnosticImageUnreadable, Destination: "directory.png"},
		{Code: DiagnosticImageOversized, Destination: "large.png"},
	}, diagnostics)
}

func TestImageDiagnostics_AggregateBudgetAndSafeMode(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "small.png"), []byte("x"), 0o644))
	src := []byte("![small](small.png)\n")

	md := newMarkdownWithImageLimit(true, "", 1)
	pc := parser.NewContext()
	pc.Set(baseDirKey, tmp)
	md.Parser().Parse(text.NewReader(src), parser.WithContext(pc))
	assert.Equal(t, []ImageDiagnostic{{Code: DiagnosticImageOverBudget, Destination: "small.png"}}, imageDiagnostics(pc))

	_, diagnostics, err := RenderWithDiagnostics(
		[]byte("![missing](missing.png)\n"),
		Options{SourceDir: tmp, FallbackTitle: "safe", Safe: true},
	)
	require.NoError(t, err)
	assert.Empty(t, diagnostics)
}
