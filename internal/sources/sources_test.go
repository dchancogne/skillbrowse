package sources

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dchancogne/skillbrowse/internal/config"
)

func TestRegistry_RootsUnderHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	reg, err := Registry()
	if err != nil {
		t.Fatal(err)
	}
	if len(reg) != 5 {
		t.Fatalf("expected 5 built-in sources, got %d", len(reg))
	}
	for _, s := range reg {
		if !strings.HasPrefix(s.Root, home) {
			t.Errorf("source %q root %q not under home %q", s.Label, s.Root, home)
		}
		if s.Origin != OriginBuiltin {
			t.Errorf("source %q origin = %q, want built-in", s.Label, s.Origin)
		}
	}
}

func TestRegistry_AllBuiltinsUseDirectMaxDepth(t *testing.T) {
	reg, err := Registry()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range reg {
		if s.MaxDepth != directMaxDepth {
			t.Errorf("%q MaxDepth = %d, want %d", s.Label, s.MaxDepth, directMaxDepth)
		}
	}
}

func TestRegistry_ExcludesPluginCaches(t *testing.T) {
	reg, err := Registry()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range reg {
		if strings.Contains(s.Label, "plugin") {
			t.Errorf("expected no plugin-cache sources in the built-in registry, found %q", s.Label)
		}
	}
}

func TestProjectRoot_FindsNearestGitAncestor(t *testing.T) {
	top := t.TempDir()
	if err := os.Mkdir(filepath.Join(top, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(top, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	if got := projectRoot(nested); got != top {
		t.Errorf("projectRoot(%q) = %q, want %q", nested, got, top)
	}
}

func TestProjectRoot_FallsBackToCwdWhenNoGitFound(t *testing.T) {
	dir := t.TempDir()
	if got := projectRoot(dir); got != dir {
		t.Errorf("projectRoot(%q) = %q, want %q (no .git found)", dir, got, dir)
	}
}

func TestProjectRegistry_RootedUnderProjectDir(t *testing.T) {
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()

	got := ProjectRegistry(project, home)
	if len(got) != 5 {
		t.Fatalf("expected 5 project sources, got %d", len(got))
	}
	for _, s := range got {
		if !strings.HasPrefix(s.Root, project) {
			t.Errorf("source %q root %q not under project %q", s.Label, s.Root, project)
		}
		if s.Origin != OriginProject {
			t.Errorf("source %q origin = %q, want project", s.Label, s.Origin)
		}
	}
}

func TestProjectRegistry_NilWhenRootEqualsHome(t *testing.T) {
	home := t.TempDir()
	if got := ProjectRegistry(home, home); got != nil {
		t.Errorf("expected nil when project root equals home, got %+v", got)
	}
}

func TestLoad_NoDefaultsExcludesRegistry(t *testing.T) {
	got, err := Load(&config.Config{}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no sources, got %d", len(got))
	}
}

func TestLoad_IncludesEnabledConfigSourcesOnly(t *testing.T) {
	cfg := &config.Config{
		Sources: []config.Source{
			{Path: "/opt/a", Label: "A", Agents: []string{"Custom"}, MaxDepth: 4, Enabled: true},
			{Path: "/opt/b", Label: "B", Agents: []string{"Custom"}, MaxDepth: 4, Enabled: false},
		},
	}
	got, err := Load(cfg, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Label != "A" {
		t.Fatalf("expected only enabled source A, got %+v", got)
	}
	if got[0].Origin != OriginConfig {
		t.Errorf("Origin = %q, want config", got[0].Origin)
	}
}

func TestLoad_CLIPathsBecomeCustomSources(t *testing.T) {
	dir := t.TempDir()
	got, err := Load(&config.Config{}, []string{dir}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 source, got %d", len(got))
	}
	if got[0].Root != dir {
		t.Errorf("Root = %q, want %q", got[0].Root, dir)
	}
	if len(got[0].Agents) != 1 || got[0].Agents[0] != "Custom" {
		t.Errorf("Agents = %v, want [Custom]", got[0].Agents)
	}
	if got[0].Origin != OriginCLI {
		t.Errorf("Origin = %q, want cli", got[0].Origin)
	}
}

func TestLoad_CombinesAllThree(t *testing.T) {
	// Pin HOME and cwd to the same directory (with no .git ancestor) so
	// ProjectRegistry resolves to nil and doesn't add extra sources on top
	// of the 5 built-ins this test is counting.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(home)

	cfg := &config.Config{
		Sources: []config.Source{
			{Path: "/opt/a", Label: "A", Agents: []string{"Custom"}, MaxDepth: 4, Enabled: true},
		},
	}
	got, err := Load(cfg, []string{t.TempDir()}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5+1+1 {
		t.Fatalf("expected 7 combined sources, got %d", len(got))
	}
}

func TestLoad_IncludesProjectSourcesWhenRunFromInsideAProject(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)

	got, err := Load(&config.Config{}, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	projectCount := 0
	for _, s := range got {
		if s.Origin == OriginProject {
			projectCount++
		}
	}
	if projectCount != 5 {
		t.Fatalf("expected 5 project-origin sources, got %d (of %d total)", projectCount, len(got))
	}
}
