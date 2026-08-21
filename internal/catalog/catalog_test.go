package catalog

import (
	"testing"
	"time"

	"github.com/dchancogne/skillbrowse/internal/discovery"
	"github.com/dchancogne/skillbrowse/internal/skill"
	"github.com/dchancogne/skillbrowse/internal/sources"
)

func candidate(label string, agents []string, canonical, observed string) discovery.Candidate {
	return discovery.Candidate{
		Source:        sources.Source{Label: label, Agents: agents},
		ObservedPath:  observed,
		CanonicalPath: canonical,
		SkillFilePath: canonical + "/SKILL.md",
	}
}

func fakeParse(names map[string]skill.Parsed) ParseFunc {
	return func(canonicalPath, skillFilePath string) skill.Parsed {
		return names[canonicalPath]
	}
}

func TestBuild_MergesDuplicateCanonicalPaths(t *testing.T) {
	candidates := []discovery.Candidate{
		candidate("Claude Code", []string{"Claude Code"}, "/skills/a", "/claude/a"),
		candidate("Claude Code plugin cache", []string{"Claude Code"}, "/skills/a", "/claude/plugins/a"),
		candidate("Cursor", []string{"Cursor"}, "/skills/a", "/cursor/a"),
	}
	parse := fakeParse(map[string]skill.Parsed{
		"/skills/a": {Name: "A", Description: "desc"},
	})

	cat := Build(candidates, parse)
	if len(cat.Skills) != 1 {
		t.Fatalf("expected 1 merged skill, got %d", len(cat.Skills))
	}
	s := cat.Skills[0]
	if len(s.Agents) != 2 || s.Agents[0] != "Claude Code" || s.Agents[1] != "Cursor" {
		t.Errorf("Agents = %v, want [Claude Code Cursor]", s.Agents)
	}
	if len(s.SourceLabels) != 3 {
		t.Errorf("SourceLabels = %v, want 3 distinct labels", s.SourceLabels)
	}
	if len(s.ObservedPaths) != 3 {
		t.Errorf("ObservedPaths = %v, want 3", s.ObservedPaths)
	}
}

func TestBuild_SameNameDifferentPathsStaySeparate(t *testing.T) {
	candidates := []discovery.Candidate{
		candidate("Claude Code", []string{"Claude Code"}, "/skills/a", "/claude/a"),
		candidate("Cursor", []string{"Cursor"}, "/skills/b", "/cursor/b"),
	}
	parse := fakeParse(map[string]skill.Parsed{
		"/skills/a": {Name: "Same Name"},
		"/skills/b": {Name: "Same Name"},
	})

	cat := Build(candidates, parse)
	if len(cat.Skills) != 2 {
		t.Fatalf("expected 2 separate skills, got %d", len(cat.Skills))
	}
	for _, s := range cat.Skills {
		if s.DuplicateNameCount != 1 {
			t.Errorf("skill %q: DuplicateNameCount = %d, want 1", s.CanonicalPath, s.DuplicateNameCount)
		}
	}
}

func TestBuild_DuplicateNameIdenticalContentNotFlaggedAsDiverged(t *testing.T) {
	candidates := []discovery.Candidate{
		candidate("Claude Code", []string{"Claude Code"}, "/skills/a", "/claude/a"),
		candidate("Cursor", []string{"Cursor"}, "/skills/b", "/cursor/b"),
	}
	parse := fakeParse(map[string]skill.Parsed{
		"/skills/a": {Name: "Same Name", ContentHash: "hash1"},
		"/skills/b": {Name: "Same Name", ContentHash: "hash1"},
	})

	cat := Build(candidates, parse)
	for _, s := range cat.Skills {
		if s.DuplicateContentDiffers {
			t.Errorf("skill %q: DuplicateContentDiffers = true, want false for matching hashes", s.CanonicalPath)
		}
	}
}

func TestBuild_DuplicateNameDivergedContentFlagged(t *testing.T) {
	candidates := []discovery.Candidate{
		candidate("Claude Code", []string{"Claude Code"}, "/skills/a", "/claude/a"),
		candidate("Cursor", []string{"Cursor"}, "/skills/b", "/cursor/b"),
	}
	parse := fakeParse(map[string]skill.Parsed{
		"/skills/a": {Name: "Same Name", ContentHash: "hash1"},
		"/skills/b": {Name: "Same Name", ContentHash: "hash2"},
	})

	cat := Build(candidates, parse)
	for _, s := range cat.Skills {
		if !s.DuplicateContentDiffers {
			t.Errorf("skill %q: DuplicateContentDiffers = false, want true for differing hashes", s.CanonicalPath)
		}
	}
}

func TestBuild_NoDuplicateNameLeavesFieldsZero(t *testing.T) {
	candidates := []discovery.Candidate{
		candidate("Claude Code", []string{"Claude Code"}, "/skills/a", "/claude/a"),
	}
	parse := fakeParse(map[string]skill.Parsed{
		"/skills/a": {Name: "Unique"},
	})

	cat := Build(candidates, parse)
	s := cat.Skills[0]
	if s.DuplicateNameCount != 0 || s.DuplicateContentDiffers {
		t.Errorf("got DuplicateNameCount=%d DuplicateContentDiffers=%v, want zero values", s.DuplicateNameCount, s.DuplicateContentDiffers)
	}
}

func TestBuild_ParsesEachCanonicalPathOnce(t *testing.T) {
	candidates := []discovery.Candidate{
		candidate("A", []string{"A"}, "/skills/a", "/a"),
		candidate("B", []string{"B"}, "/skills/a", "/b"),
	}
	calls := 0
	parse := func(canonicalPath, skillFilePath string) skill.Parsed {
		calls++
		return skill.Parsed{Name: "X"}
	}

	Build(candidates, parse)
	if calls != 1 {
		t.Errorf("parse called %d times, want 1", calls)
	}
}

func TestBuild_DeterministicDefaultSort(t *testing.T) {
	candidates := []discovery.Candidate{
		candidate("A", []string{"A"}, "/skills/zebra", "/zebra"),
		candidate("A", []string{"A"}, "/skills/apple-2", "/apple-2"),
		candidate("A", []string{"A"}, "/skills/apple-1", "/apple-1"),
	}
	parse := fakeParse(map[string]skill.Parsed{
		"/skills/zebra":   {Name: "Zebra"},
		"/skills/apple-2": {Name: "apple"},
		"/skills/apple-1": {Name: "Apple"},
	})

	cat := Build(candidates, parse)
	got := []string{cat.Skills[0].CanonicalPath, cat.Skills[1].CanonicalPath, cat.Skills[2].CanonicalPath}
	want := []string{"/skills/apple-1", "/skills/apple-2", "/skills/zebra"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
			break
		}
	}
}

func TestBuild_CarriesDiagnostics(t *testing.T) {
	candidates := []discovery.Candidate{candidate("A", []string{"A"}, "/skills/a", "/a")}
	parse := fakeParse(map[string]skill.Parsed{
		"/skills/a": {Name: "A", Diagnostics: []string{"invalid front matter: boom"}},
	})

	cat := Build(candidates, parse)
	if len(cat.Skills[0].Diagnostics) != 1 {
		t.Fatalf("expected diagnostics to carry through, got %v", cat.Skills[0].Diagnostics)
	}
}

func TestSearch_EmptyQueryReturnsDefaultOrder(t *testing.T) {
	skills := []Skill{{Name: "B"}, {Name: "A"}}
	got := Search(skills, "")
	if len(got) != 2 || got[0].Name != "B" {
		t.Errorf("expected unchanged order, got %+v", got)
	}
}

func TestSearch_MatchesAcrossFields(t *testing.T) {
	skills := []Skill{
		{Name: "Alpha", Description: "unrelated", Agents: []string{"Claude Code"}, CanonicalPath: "/a"},
		{Name: "Beta", Description: "mentions the target keyword", CanonicalPath: "/b"},
		{Name: "Gamma", SourceLabels: []string{"Team skills"}, CanonicalPath: "/c"},
		{Name: "Delta", ObservedPaths: []string{"/observed/keyword-path"}, CanonicalPath: "/d"},
	}

	if got := Search(skills, "claude"); len(got) != 1 || got[0].Name != "Alpha" {
		t.Errorf("agent search: got %+v", got)
	}
	if got := Search(skills, "keyword"); len(got) != 2 {
		t.Errorf("description/path search: got %+v", got)
	}
	if got := Search(skills, "team"); len(got) != 1 || got[0].Name != "Gamma" {
		t.Errorf("source label search: got %+v", got)
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	skills := []Skill{{Name: "SkillBrowse", CanonicalPath: "/x"}}
	got := Search(skills, "skillbrowse")
	if len(got) != 1 {
		t.Errorf("expected case-insensitive match, got %+v", got)
	}
}

func TestBuild_PreservesVersion(t *testing.T) {
	candidates := []discovery.Candidate{candidate("A", []string{"A"}, "/skills/a", "/a")}
	parse := fakeParse(map[string]skill.Parsed{
		"/skills/a": {Name: "A", Version: "1.2.3"},
	})

	cat := Build(candidates, parse)
	if cat.Skills[0].Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", cat.Skills[0].Version, "1.2.3")
	}
}

func TestBuild_PreservesModifiedAtAndContent(t *testing.T) {
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candidates := []discovery.Candidate{candidate("A", []string{"A"}, "/skills/a", "/a")}
	parse := fakeParse(map[string]skill.Parsed{
		"/skills/a": {Name: "A", Content: "---\nname: A\n---\nbody", Body: "body", ModifiedAt: when},
	})

	cat := Build(candidates, parse)
	if cat.Skills[0].Content != "---\nname: A\n---\nbody" {
		t.Errorf("Content = %q", cat.Skills[0].Content)
	}
	if cat.Skills[0].Body != "body" {
		t.Errorf("Body = %q, want %q", cat.Skills[0].Body, "body")
	}
	if !cat.Skills[0].ModifiedAt.Equal(when) {
		t.Errorf("ModifiedAt = %v, want %v", cat.Skills[0].ModifiedAt, when)
	}
}
