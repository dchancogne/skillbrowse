package sources

import (
	"os"
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
	if len(reg) != 7 {
		t.Fatalf("expected 7 built-in sources, got %d", len(reg))
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

func TestRegistry_PluginCachesHaveDeeperMaxDepth(t *testing.T) {
	reg, err := Registry()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range reg {
		wantDirect := s.Label == "Claude Code plugin cache" || s.Label == "Codex plugin cache"
		if wantDirect && s.MaxDepth != pluginCacheMaxDepth {
			t.Errorf("%q MaxDepth = %d, want %d", s.Label, s.MaxDepth, pluginCacheMaxDepth)
		}
		if !wantDirect && s.MaxDepth != directMaxDepth {
			t.Errorf("%q MaxDepth = %d, want %d", s.Label, s.MaxDepth, directMaxDepth)
		}
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
	cfg := &config.Config{
		Sources: []config.Source{
			{Path: "/opt/a", Label: "A", Agents: []string{"Custom"}, MaxDepth: 4, Enabled: true},
		},
	}
	got, err := Load(cfg, []string{t.TempDir()}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 7+1+1 {
		t.Fatalf("expected 9 combined sources, got %d", len(got))
	}
}
