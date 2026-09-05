package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateSourceLimitBoundary(t *testing.T) {
	t.Parallel()
	source, err := readTemplateCapped(strings.NewReader(strings.Repeat("x", maxTemplateBytes)), "boundary")
	require.NoError(t, err)
	assert.Len(t, source, maxTemplateBytes)
	_, err = readTemplateCapped(strings.NewReader(strings.Repeat("x", maxTemplateBytes+1)), "oversize")
	require.ErrorContains(t, err, "cap is 1024 KiB")
}

func TestTemplateExecutionFailurePreservesOutput(t *testing.T) {
	t.Parallel()
	for _, source := range []string{`prefix{{.Data.missing}}`, `prefix{{index .Data "missing"}}`} {
		dir := t.TempDir()
		path := filepath.Join(dir, "page.tmpl")
		output := filepath.Join(dir, "existing.html")
		require.NoError(t, os.WriteFile(path, []byte(source), 0o600))
		require.NoError(t, os.WriteFile(output, []byte("keep existing"), 0o600))
		res, err := RunWithResult(Options{Stdin: strings.NewReader(`{}`), Template: path, Output: output, NoOpen: true})
		require.ErrorContains(t, err, "execute page template")
		assert.Empty(t, res.Stdout)
		assert.Empty(t, res.Path)
		content, err := os.ReadFile(output)
		require.NoError(t, err)
		assert.Equal(t, "keep existing", string(content))
	}
}

func TestTemplate64MiBOutputLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "page.tmpl")
	output := filepath.Join(dir, "existing.html")
	// A small source template expands beyond the real production output cap.
	source := `{{range .Data}}` + strings.Repeat("x", 64<<10) + `{{end}}`
	require.NoError(t, os.WriteFile(path, []byte(source), 0o600))
	require.NoError(t, os.WriteFile(output, []byte("keep existing"), 0o600))
	res, err := RunWithResult(Options{
		Stdin: strings.NewReader("[" + strings.Repeat("true,", 1024) + "true]"),
		Plain: true, Lang: "text", Template: path, Output: output, NoOpen: true,
	})
	require.ErrorContains(t, err, "64 MiB")
	assert.Empty(t, res.Path)
	assert.Empty(t, res.Stdout)
	content, err := os.ReadFile(output)
	require.NoError(t, err)
	assert.Equal(t, "keep existing", string(content))
}
