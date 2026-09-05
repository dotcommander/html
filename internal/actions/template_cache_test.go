package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dotcommander/html/internal/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateCannotAliasCachePublication(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"file", "stdin"} {
		for _, reportMode := range []bool{false, true} {
			for _, artifact := range []string{"page", "fingerprint"} {
				dir := t.TempDir()
				src := []byte("# Unique " + dir + "\n")
				opts := Options{NoOpen: true, Report: reportMode, Force: true}
				if input == "file" {
					opts.File = filepath.Join(dir, "source.md")
					require.NoError(t, os.WriteFile(opts.File, src, 0o600))
				} else {
					opts.Stdin = strings.NewReader(string(src))
					opts.Markdown = true
				}
				path, err := cacheRouteFor(opts, src).path()
				require.NoError(t, err)
				t.Cleanup(func() {
					for _, p := range cache.PublicationPaths(path) {
						_ = os.Remove(p)
					}
				})
				if artifact == "fingerprint" {
					path = cache.PublicationPaths(path)[1]
				}
				source := `page {{.Content}}`
				require.NoError(t, os.WriteFile(path, []byte(source), 0o600))
				opts.Template = path
				res, err := RunWithResult(opts)
				require.ErrorContains(t, err, "aliases template file")
				assert.Empty(t, res.Path)
				got, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Equal(t, source, string(got))
			}
		}
	}
}
