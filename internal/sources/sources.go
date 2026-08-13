// Package sources defines the built-in skill source registry and merges it
// with validated custom sources, per
// docs/superpowers/specs/2026-08-12-skillbrowse-design.md §5.1.
package sources

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dchancogne/skillbrowse/internal/config"
)

// Depth bounds applied to built-in registry roots. Direct roots contain
// skill directories as immediate children; plugin-cache roots need a
// deeper, still-bounded scan to reach nested "skills" directories.
const (
	directMaxDepth      = 1
	pluginCacheMaxDepth = 6
	cliSourceMaxDepth   = config.DefaultMaxDepth
)

// Origin identifies where a Source came from, for diagnostics.
type Origin string

const (
	OriginBuiltin Origin = "built-in"
	OriginConfig  Origin = "config"
	OriginCLI     Origin = "cli"
)

// Source is a fully resolved scan root: an absolute path plus the
// attribution (label, agents) discovery.Scan should attach to any
// candidates found under it.
type Source struct {
	Label    string
	Agents   []string
	Root     string
	MaxDepth int
	Origin   Origin
}

// Registry returns the built-in source descriptors from design doc §5.1,
// with roots resolved against the current user's home directory. It
// returns an error only if the home directory cannot be determined, which
// design doc §9 treats as a fatal environment error.
func Registry() ([]Source, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("determine home directory: %w", err)
	}

	root := func(parts ...string) string {
		return filepath.Join(append([]string{home}, parts...)...)
	}

	return []Source{
		{Label: "Agent Skills", Agents: []string{"Agent Skills"}, Root: root(".agents", "skills"), MaxDepth: directMaxDepth, Origin: OriginBuiltin},
		{Label: "Claude Code", Agents: []string{"Claude Code"}, Root: root(".claude", "skills"), MaxDepth: directMaxDepth, Origin: OriginBuiltin},
		{Label: "Claude Code plugin cache", Agents: []string{"Claude Code"}, Root: root(".claude", "plugins", "cache"), MaxDepth: pluginCacheMaxDepth, Origin: OriginBuiltin},
		{Label: "Cursor", Agents: []string{"Cursor"}, Root: root(".cursor", "skills"), MaxDepth: directMaxDepth, Origin: OriginBuiltin},
		{Label: "Codex", Agents: []string{"Codex"}, Root: root(".codex", "skills"), MaxDepth: directMaxDepth, Origin: OriginBuiltin},
		{Label: "Codex plugin cache", Agents: []string{"Codex"}, Root: root(".codex", "plugins", "cache"), MaxDepth: pluginCacheMaxDepth, Origin: OriginBuiltin},
		{Label: "Hermes", Agents: []string{"Hermes"}, Root: root(".hermes", "skills"), MaxDepth: directMaxDepth, Origin: OriginBuiltin},
	}, nil
}

// Load combines the built-in registry (unless noDefaults is set), enabled
// sources from cfg, and unlabeled command-line --path sources into the
// final list of sources to scan.
func Load(cfg *config.Config, cliPaths []string, noDefaults bool) ([]Source, error) {
	var result []Source

	if !noDefaults {
		builtins, err := Registry()
		if err != nil {
			return nil, err
		}
		result = append(result, builtins...)
	}

	if cfg != nil {
		for _, s := range cfg.Sources {
			if !s.Enabled {
				continue
			}
			result = append(result, Source{
				Label:    s.Label,
				Agents:   s.Agents,
				Root:     s.Path,
				MaxDepth: s.MaxDepth,
				Origin:   OriginConfig,
			})
		}
	}

	for _, p := range cliPaths {
		abs, err := resolveCLIPath(p)
		if err != nil {
			return nil, fmt.Errorf("--path %q: %w", p, err)
		}
		result = append(result, Source{
			Label:    filepath.Base(abs),
			Agents:   []string{"Custom"},
			Root:     abs,
			MaxDepth: cliSourceMaxDepth,
			Origin:   OriginCLI,
		})
	}

	return result, nil
}

func resolveCLIPath(p string) (string, error) {
	if p == "~" || len(p) >= 2 && p[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			p = home
		} else {
			p = filepath.Join(home, p[2:])
		}
	}
	return filepath.Abs(p)
}
