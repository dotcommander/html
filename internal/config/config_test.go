package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFrom_Missing(t *testing.T) {
	t.Parallel()

	cfg, err := loadFrom(filepath.Join(t.TempDir(), "nope.json"))
	require.NoError(t, err)
	assert.Equal(t, Config{}, cfg, "missing config must yield the zero Config (current behavior)")
}

func TestLoadFrom_Valid(t *testing.T) {
	t.Parallel()

	p := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(p,
		[]byte(`{"open_command":"firefox","max_width":"48rem","default_theme":"dark","default_palette":"catppuccin","toc":true}`), 0o644))

	cfg, err := loadFrom(p)
	require.NoError(t, err)
	assert.Equal(t, "firefox", cfg.OpenCommand)
	assert.Equal(t, "48rem", cfg.MaxWidth)
	assert.Equal(t, "dark", cfg.DefaultTheme)
	assert.Equal(t, "catppuccin", cfg.DefaultPalette)
	require.NotNil(t, cfg.TOC)
	assert.True(t, *cfg.TOC)
}

func TestLoadFrom_MalformedJSON(t *testing.T) {
	t.Parallel()

	p := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(p, []byte(`{not valid json`), 0o644))

	_, err := loadFrom(p)
	assert.Error(t, err)
}

func TestLoadFrom_InvalidTheme(t *testing.T) {
	t.Parallel()

	p := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(p, []byte(`{"default_theme":"chartreuse"}`), 0o644))

	_, err := loadFrom(p)
	assert.Error(t, err)
}

func TestLoadFrom_InvalidPalette(t *testing.T) {
	t.Parallel()

	p := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(p, []byte(`{"default_palette":"chartreuse"}`), 0o644))

	_, err := loadFrom(p)
	assert.Error(t, err)
}

func TestLoadFrom_InvalidMaxWidth(t *testing.T) {
	t.Parallel()

	p := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(p, []byte(`{"max_width":"100"}`), 0o644)) // missing unit
	_, err := loadFrom(p)
	assert.Error(t, err)
}
