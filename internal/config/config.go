// Package config loads and validates skillbrowse's TOML configuration file
// and resolves custom source paths, per
// docs/superpowers/specs/2026-08-12-skillbrowse-design.md §5.2.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	// SupportedVersion is the only accepted value of the config file's
	// top-level "version" field.
	SupportedVersion = 1

	// DefaultMaxDepth is applied to a source when max_depth is omitted.
	DefaultMaxDepth = 4

	// MinMaxDepth and MaxMaxDepth bound the accepted max_depth range.
	MinMaxDepth = 1
	MaxMaxDepth = 12
)

// Source is one validated, normalized [[sources]] entry.
type Source struct {
	Path     string
	Label    string
	Agents   []string
	MaxDepth int
	Enabled  bool
}

// Config is the normalized, validated content of config.toml.
type Config struct {
	Version int
	Sources []Source
}

// Error reports a problem with a specific config file and, where
// applicable, a specific field, along with correction guidance.
type Error struct {
	Path    string
	Field   string
	Message string
}

func (e *Error) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("%s: %s", e.Path, e.Message)
	}
	return fmt.Sprintf("%s: %s: %s", e.Path, e.Field, e.Message)
}

// rawConfig and rawSource mirror the TOML shape before defaults and
// validation are applied. Pointer fields distinguish "omitted" from
// "explicit zero value".
type rawConfig struct {
	Version int         `toml:"version"`
	Sources []rawSource `toml:"sources"`
}

type rawSource struct {
	Path     string   `toml:"path"`
	Label    *string  `toml:"label"`
	Agents   []string `toml:"agents"`
	MaxDepth *int     `toml:"max_depth"`
	Enabled  *bool    `toml:"enabled"`
}

// DefaultPath resolves the default config file location:
// $XDG_CONFIG_HOME/skillbrowse/config.toml when XDG_CONFIG_HOME is an
// absolute path, otherwise ~/.config/skillbrowse/config.toml.
func DefaultPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "skillbrowse", "config.toml"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "skillbrowse", "config.toml"), nil
}

// Load reads and validates the config file at path. A missing file is not
// an error: it returns an empty, valid Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Version: SupportedVersion}, nil
		}
		return nil, &Error{Path: path, Message: fmt.Sprintf("cannot read config file: %s", err)}
	}

	var raw rawConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, &Error{Path: path, Message: fmt.Sprintf("invalid TOML: %s", err)}
	}

	if raw.Version != SupportedVersion {
		return nil, &Error{
			Path:    path,
			Field:   "version",
			Message: fmt.Sprintf("must equal %d, got %d", SupportedVersion, raw.Version),
		}
	}

	sources := make([]Source, 0, len(raw.Sources))
	for i, rs := range raw.Sources {
		source, err := normalizeSource(path, i, rs)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}

	return &Config{Version: raw.Version, Sources: sources}, nil
}

func normalizeSource(configPath string, index int, rs rawSource) (Source, error) {
	field := func(name string) string { return fmt.Sprintf("sources[%d].%s", index, name) }

	if rs.Path == "" {
		return Source{}, &Error{Path: configPath, Field: field("path"), Message: "must not be empty"}
	}

	expanded, err := expandPath(rs.Path)
	if err != nil {
		return Source{}, &Error{Path: configPath, Field: field("path"), Message: err.Error()}
	}
	if !filepath.IsAbs(expanded) {
		return Source{}, &Error{
			Path:    configPath,
			Field:   field("path"),
			Message: fmt.Sprintf("relative paths are not allowed (%q); use an absolute path or a leading ~ for your home directory", rs.Path),
		}
	}

	label := rs.Label
	if label == nil || *label == "" {
		base := filepath.Base(expanded)
		label = &base
	}

	agents := rs.Agents
	if len(agents) == 0 {
		agents = []string{"Custom"}
	}

	maxDepth := DefaultMaxDepth
	if rs.MaxDepth != nil {
		maxDepth = *rs.MaxDepth
		if maxDepth < MinMaxDepth || maxDepth > MaxMaxDepth {
			return Source{}, &Error{
				Path:    configPath,
				Field:   field("max_depth"),
				Message: fmt.Sprintf("must be between %d and %d, got %d", MinMaxDepth, MaxMaxDepth, maxDepth),
			}
		}
	}

	enabled := true
	if rs.Enabled != nil {
		enabled = *rs.Enabled
	}

	return Source{
		Path:     expanded,
		Label:    *label,
		Agents:   agents,
		MaxDepth: maxDepth,
		Enabled:  enabled,
	}, nil
}

// expandPath accepts "~" only as the first path component and expands it
// to the current user's home directory. Any other use of "~" is left
// untouched (and will subsequently fail the absolute-path check).
func expandPath(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot expand ~: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}
