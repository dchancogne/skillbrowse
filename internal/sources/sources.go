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

// Depth bound applied to built-in registry roots: each is a direct
// "skills" directory whose immediate children are skill directories.
//
// The built-in registry previously also scanned each agent's plugin
// cache (e.g. ~/.claude/plugins/cache, ~/.codex/plugins/cache) at a
// deeper bound to reach nested "skills" directories, per design doc
// §5.1. That's deliberately disabled for now at the user's request —
// plugin-cache skills were noisy/unwanted in the default catalog. To
// reinstate a source like that, add a registry entry with a deeper
// MaxDepth (6 was previously used) pointed at the plugin cache root.
const (
	directMaxDepth    = 1
	cliSourceMaxDepth = config.DefaultMaxDepth
)

// Origin identifies where a Source came from, for diagnostics.
type Origin string

const (
	OriginBuiltin Origin = "built-in"
	OriginProject Origin = "project"
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
	return registryFor(home, OriginBuiltin), nil
}

// registryFor builds the same set of per-agent skill-directory sources
// Registry describes, rooted at root instead of the home directory. Shared
// by Registry (root = home) and ProjectRegistry (root = a project
// directory).
func registryFor(root string, origin Origin) []Source {
	j := func(parts ...string) string {
		return filepath.Join(append([]string{root}, parts...)...)
	}

	return []Source{
		{Label: "Agent Skills", Agents: []string{"Agent Skills"}, Root: j(".agents", "skills"), MaxDepth: directMaxDepth, Origin: origin},
		{Label: "Claude Code", Agents: []string{"Claude Code"}, Root: j(".claude", "skills"), MaxDepth: directMaxDepth, Origin: origin},
		{Label: "Cursor", Agents: []string{"Cursor"}, Root: j(".cursor", "skills"), MaxDepth: directMaxDepth, Origin: origin},
		{Label: "Codex", Agents: []string{"Codex"}, Root: j(".codex", "skills"), MaxDepth: directMaxDepth, Origin: origin},
		{Label: "Hermes", Agents: []string{"Hermes"}, Root: j(".hermes", "skills"), MaxDepth: directMaxDepth, Origin: origin},
	}
}

// projectRoot walks up from cwd looking for a ".git" entry (a directory
// for a normal checkout, or a file for a worktree/submodule), returning
// the first ancestor that has one. If none is found before reaching the
// filesystem root, cwd itself is returned unchanged.
func projectRoot(cwd string) string {
	dir := cwd
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return cwd
		}
		dir = parent
	}
}

// ProjectRegistry returns the same per-agent skill-directory sources as
// Registry, but rooted at the project containing cwd (see projectRoot)
// instead of the home directory, so a repo's own committed skills are
// discovered alongside the user's global ones. It returns nil when the
// resolved project root is the home directory itself, to avoid scanning
// the same directories twice under a different Origin.
func ProjectRegistry(cwd, home string) []Source {
	root := projectRoot(cwd)
	if root == home {
		return nil
	}
	return registryFor(root, OriginProject)
}

// Load combines the built-in registry (unless noDefaults is set), any
// project-local sources for the current working directory, enabled
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

		// Project-source detection is best-effort: unlike Registry's home
		// directory (a fatal environment error per design doc §9), failing
		// to resolve cwd here just means no project sources are added.
		if cwd, err := os.Getwd(); err == nil {
			if home, err := os.UserHomeDir(); err == nil {
				result = append(result, ProjectRegistry(cwd, home)...)
			}
		}
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
