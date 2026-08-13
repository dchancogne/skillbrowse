package catalog_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dchancogne/skillbrowse/internal/catalog"
	"github.com/dchancogne/skillbrowse/internal/discovery"
	"github.com/dchancogne/skillbrowse/internal/sources"
)

// TestPipeline_EndToEnd is the "CLI-less test harness" from
// docs/skillbrowse-implementation-plan.md Phase 1: it proves
// sources -> discovery -> skill -> catalog work together against real
// fixture trees representing multiple source families, a duplicate
// reached via a real path and a symlink, and mixed valid/invalid/
// oversized/unreadable skills, without any terminal UI involved.
func TestPipeline_EndToEnd(t *testing.T) {
	claudeRoot := t.TempDir()
	cursorRoot := t.TempDir()
	pluginCacheRoot := t.TempDir()

	// A normal, well-formed skill discovered directly under one source.
	writeSkill(t, filepath.Join(claudeRoot, "formatter"),
		"---\nname: Formatter\ndescription: Formats code.\n---\nBody.\n")

	// The same skill also exposed through a second source (plugin cache),
	// reached this time via a symlink — must merge into one record with
	// both agents/labels/observed paths unioned.
	if err := os.Symlink(filepath.Join(claudeRoot, "formatter"), filepath.Join(pluginCacheRoot, "formatter-link")); err != nil {
		t.Fatal(err)
	}

	// A skill with invalid front matter: still visible, with a diagnostic
	// and directory-name/paragraph fallbacks.
	writeSkill(t, filepath.Join(cursorRoot, "broken-front-matter"),
		"---\nname: [unterminated\n---\nFallback text here.\n")

	// An oversized skill: visible with a diagnostic, content not loaded.
	bigDir := filepath.Join(cursorRoot, "too-big")
	if err := os.MkdirAll(bigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bigDir, "SKILL.md"), make([]byte, 3*1024*1024), 0o644); err != nil {
		t.Fatal(err)
	}

	// An unreadable directory: produces a source diagnostic, doesn't
	// abort the rest of the scan.
	var blocked string
	if runtime.GOOS != "windows" && os.Geteuid() != 0 {
		blocked = filepath.Join(cursorRoot, "blocked")
		if err := os.MkdirAll(blocked, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(blocked, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(blocked, 0o755); err != nil {
				t.Logf("cleanup: chmod %s: %s", blocked, err)
			}
		})
	}

	srcs := []sources.Source{
		{Label: "Claude Code", Agents: []string{"Claude Code"}, Root: claudeRoot, MaxDepth: 1},
		// MaxDepth 2 (not 1): the walker only attempts to read a
		// directory's contents when it intends to recurse further, so
		// an unreadable directory sitting exactly at MaxDepth would
		// never be listed at all and couldn't produce a diagnostic.
		{Label: "Cursor", Agents: []string{"Cursor"}, Root: cursorRoot, MaxDepth: 2},
		{Label: "Claude Code plugin cache", Agents: []string{"Claude Code"}, Root: pluginCacheRoot, MaxDepth: 6},
		{Label: "Missing", Agents: []string{"Missing"}, Root: filepath.Join(t.TempDir(), "does-not-exist"), MaxDepth: 1},
	}

	result := discovery.Scan(context.Background(), srcs)
	cat := catalog.BuildFromCandidates(result.Candidates)

	if len(cat.Skills) != 3 {
		t.Fatalf("expected 3 merged skills, got %d: %+v", len(cat.Skills), cat.Skills)
	}

	byName := map[string]catalog.Skill{}
	for _, s := range cat.Skills {
		byName[s.Name] = s
	}

	formatter, ok := byName["Formatter"]
	if !ok {
		t.Fatal("expected the merged Formatter skill")
	}
	if len(formatter.Agents) != 1 || formatter.Agents[0] != "Claude Code" {
		t.Errorf("Agents = %v, want [Claude Code]", formatter.Agents)
	}
	if len(formatter.SourceLabels) != 2 {
		t.Errorf("SourceLabels = %v, want 2 (direct + plugin cache)", formatter.SourceLabels)
	}
	if len(formatter.ObservedPaths) != 2 {
		t.Errorf("ObservedPaths = %v, want 2 (real path + symlink)", formatter.ObservedPaths)
	}

	broken, ok := byName["broken-front-matter"]
	if !ok {
		t.Fatal("expected the broken-front-matter skill to remain visible via directory-name fallback")
	}
	if len(broken.Diagnostics) == 0 {
		t.Error("expected a diagnostic for invalid front matter")
	}
	if broken.Description != "Fallback text here." {
		t.Errorf("Description = %q", broken.Description)
	}

	tooBig, ok := byName["too-big"]
	if !ok {
		t.Fatal("expected the oversized skill to remain visible")
	}
	if tooBig.Content != "" {
		t.Error("expected oversized content not to be loaded")
	}
	if len(tooBig.Diagnostics) == 0 {
		t.Error("expected a diagnostic for the oversized file")
	}

	if blocked != "" {
		resolvedParent, err := filepath.EvalSymlinks(filepath.Dir(blocked))
		if err != nil {
			t.Fatal(err)
		}
		wantPath := filepath.Join(resolvedParent, filepath.Base(blocked))

		found := false
		for _, d := range result.Diagnostics {
			if d.Path == wantPath {
				found = true
			}
		}
		if !found {
			t.Errorf("expected a source diagnostic for %q, got %+v", wantPath, result.Diagnostics)
		}
	}

	// Fuzzy search across the merged catalog: real fixture paths are long
	// enough that weak subsequence matches can appear in the results too
	// (expected fuzzy-search behavior), so only the top-ranked match is
	// asserted here.
	if got := catalog.Search(cat.Skills, "format"); len(got) == 0 || got[0].Name != "Formatter" {
		t.Errorf("Search(format) top result = %+v, want Formatter first", got)
	}
}

func writeSkill(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
