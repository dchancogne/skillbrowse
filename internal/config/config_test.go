package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_MissingFileIsValidEmpty(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Version != SupportedVersion || len(cfg.Sources) != 0 {
		t.Fatalf("expected empty valid config, got %+v", cfg)
	}
}

func TestLoad_Defaults(t *testing.T) {
	path := writeConfig(t, `
version = 1

[[sources]]
path = "/opt/company/skills"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(cfg.Sources))
	}
	src := cfg.Sources[0]
	if src.Path != "/opt/company/skills" {
		t.Errorf("Path = %q", src.Path)
	}
	if src.Label != "skills" {
		t.Errorf("Label = %q, want default of final path component", src.Label)
	}
	if len(src.Agents) != 1 || src.Agents[0] != "Custom" {
		t.Errorf("Agents = %v, want [Custom]", src.Agents)
	}
	if src.MaxDepth != DefaultMaxDepth {
		t.Errorf("MaxDepth = %d, want %d", src.MaxDepth, DefaultMaxDepth)
	}
	if !src.Enabled {
		t.Errorf("Enabled = false, want true by default")
	}
}

func TestLoad_ExplicitFields(t *testing.T) {
	path := writeConfig(t, `
version = 1

[[sources]]
path = "~/work/shared-agent-skills"
label = "Team skills"
agents = ["Claude Code", "Codex"]
max_depth = 6
enabled = false
`)

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	src := cfg.Sources[0]
	want := filepath.Join(home, "work/shared-agent-skills")
	if src.Path != want {
		t.Errorf("Path = %q, want %q", src.Path, want)
	}
	if src.Label != "Team skills" {
		t.Errorf("Label = %q", src.Label)
	}
	if len(src.Agents) != 2 || src.Agents[0] != "Claude Code" || src.Agents[1] != "Codex" {
		t.Errorf("Agents = %v", src.Agents)
	}
	if src.MaxDepth != 6 {
		t.Errorf("MaxDepth = %d", src.MaxDepth)
	}
	if src.Enabled {
		t.Errorf("Enabled = true, want false")
	}
}

func TestLoad_RejectsWrongVersion(t *testing.T) {
	path := writeConfig(t, `version = 2`)
	_, err := Load(path)
	assertConfigError(t, err, "version")
}

func TestLoad_RejectsRelativePath(t *testing.T) {
	path := writeConfig(t, `
version = 1

[[sources]]
path = "relative/dir"
`)
	_, err := Load(path)
	assertConfigError(t, err, "sources[0].path")
}

func TestLoad_RejectsTildeNotFirstComponent(t *testing.T) {
	path := writeConfig(t, `
version = 1

[[sources]]
path = "/opt/~weird"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Sources[0].Path != "/opt/~weird" {
		t.Errorf("expected literal path preserved, got %q", cfg.Sources[0].Path)
	}
}

func TestLoad_RejectsMaxDepthOutOfRange(t *testing.T) {
	for _, depth := range []string{"0", "13"} {
		path := writeConfig(t, `
version = 1

[[sources]]
path = "/opt/skills"
max_depth = `+depth)
		_, err := Load(path)
		assertConfigError(t, err, "sources[0].max_depth")
	}
}

func TestLoad_RejectsEmptyPath(t *testing.T) {
	path := writeConfig(t, `
version = 1

[[sources]]
path = ""
`)
	_, err := Load(path)
	assertConfigError(t, err, "sources[0].path")
}

func TestLoad_InvalidTOML(t *testing.T) {
	path := writeConfig(t, `this is not toml =`)
	_, err := Load(path)
	var cfgErr *Error
	if !asConfigError(err, &cfgErr) {
		t.Fatalf("expected *config.Error, got %T: %v", err, err)
	}
}

func TestDefaultPath_UsesXDGWhenAbsolute(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	want := "/xdg/config/skillbrowse/config.toml"
	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestDefaultPath_FallsBackToHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "relative/not/absolute")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", "skillbrowse", "config.toml")
	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

func assertConfigError(t *testing.T, err error, wantField string) {
	t.Helper()
	var cfgErr *Error
	if !asConfigError(err, &cfgErr) {
		t.Fatalf("expected *config.Error, got %T: %v", err, err)
	}
	if cfgErr.Field != wantField {
		t.Errorf("Field = %q, want %q", cfgErr.Field, wantField)
	}
}

func asConfigError(err error, target **Error) bool {
	e, ok := err.(*Error)
	if !ok {
		return false
	}
	*target = e
	return true
}
