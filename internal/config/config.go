// Package config loads optional user preferences from ~/.config/html/config.json.
// The tool reads config; it does not contain config. Every field is optional, so
// a missing file reproduces the built-in behavior exactly.
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// Config holds optional user preferences. A zero value (or absent key) means
// "use the built-in default".
type Config struct {
	OpenCommand    string `json:"open_command"`    // launcher command (e.g. "firefox"); "" = OS default
	MaxWidth       string `json:"max_width"`       // reader column CSS max-width (e.g. "48rem"); "" = default
	DefaultTheme   string `json:"default_theme"`   // "light" | "dark" | "auto"; "" = auto (system)
	DefaultPalette string `json:"default_palette"` // "sepia" | "blue" | "green" | "rose" | "catppuccin"; "" = sepia
	TOC            *bool  `json:"toc"`             // override automatic TOC; nil = automatic
}

// maxConfigBytes caps the config read; a config file is tiny, so anything past
// this is treated as malformed rather than read unbounded.
const maxConfigBytes = 64 << 10

// maxWidthRe constrains max_width to a simple CSS length. This both rejects
// typos with a clear error and prevents the value from breaking out of the
// generated CSS rule it is injected into.
var maxWidthRe = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?(px|rem|em|ch|vw|vh|%)$`)

// path returns ~/.config/html/config.json.
func path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: home dir: %w", err)
	}
	return filepath.Join(home, ".config", "html", "config.json"), nil
}

// Load reads and parses the user config. A missing file returns a zero Config
// and no error (current behavior); a present-but-unreadable or malformed file
// returns a clear error.
func Load() (Config, error) {
	p, err := path()
	if err != nil {
		return Config{}, err
	}
	return loadFrom(p)
}

// loadFrom is the path-injected core of Load, kept separate so tests can point
// at a temp file without mutating the environment.
func loadFrom(p string) (Config, error) {
	f, err := os.Open(p)
	if os.IsNotExist(err) {
		return Config{}, nil // missing config => defaults
	}
	if err != nil {
		return Config{}, fmt.Errorf("config: open %s: %w", p, err)
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxConfigBytes+1))
	if err != nil {
		return Config{}, fmt.Errorf("config: read %s: %w", p, err)
	}
	if len(data) > maxConfigBytes {
		return Config{}, fmt.Errorf("config: %s is too large (cap %d bytes)", p, maxConfigBytes)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("config: invalid JSON in %s: %w", p, err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("config: %s: %w", p, err)
	}
	return cfg, nil
}

// validate rejects values the renderer/launcher cannot honor, so the user gets a
// clear error instead of a silently ignored typo.
func (c Config) validate() error {
	switch c.DefaultTheme {
	case "", "light", "dark", "auto":
	default:
		return fmt.Errorf("default_theme must be \"light\", \"dark\", or \"auto\" (got %q)", c.DefaultTheme)
	}
	switch c.DefaultPalette {
	case "", "sepia", "blue", "green", "rose", "catppuccin":
	default:
		return fmt.Errorf("default_palette must be \"sepia\", \"blue\", \"green\", \"rose\", or \"catppuccin\" (got %q)", c.DefaultPalette)
	}
	if c.MaxWidth != "" && !maxWidthRe.MatchString(c.MaxWidth) {
		return fmt.Errorf("max_width must be a CSS length like \"48rem\" (got %q)", c.MaxWidth)
	}
	return nil
}
