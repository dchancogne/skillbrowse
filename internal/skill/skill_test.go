package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, content string) (canonicalPath, skillFilePath string) {
	t.Helper()
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return skillDir, path
}

func TestParse_FrontMatterNameAndDescription(t *testing.T) {
	dir, path := writeSkill(t, "---\nname: My Skill\ndescription: Does the thing.\n---\nBody text.\n")
	p := Parse(dir, path)
	if p.Name != "My Skill" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Description != "Does the thing." {
		t.Errorf("Description = %q", p.Description)
	}
	if len(p.Diagnostics) != 0 {
		t.Errorf("unexpected diagnostics: %v", p.Diagnostics)
	}
}

func TestParse_BodyStripsFrontMatter(t *testing.T) {
	dir, path := writeSkill(t, "---\nname: My Skill\ndescription: Does the thing.\n---\n## Heading\n\nBody text.\n")
	p := Parse(dir, path)
	if strings.Contains(p.Body, "name: My Skill") || strings.Contains(p.Body, "---") {
		t.Errorf("Body still contains front matter: %q", p.Body)
	}
	if !strings.Contains(p.Body, "## Heading") || !strings.Contains(p.Body, "Body text.") {
		t.Errorf("Body missing expected content: %q", p.Body)
	}
	if !strings.HasPrefix(p.Content, "---\nname: My Skill") {
		t.Errorf("Content should retain the raw front matter for the raw view, got %q", p.Content)
	}
}

func TestParse_BodyWithoutFrontMatterEqualsContent(t *testing.T) {
	dir, path := writeSkill(t, "## Heading\n\nBody text.\n")
	p := Parse(dir, path)
	if p.Body != p.Content {
		t.Errorf("Body = %q, want equal to Content %q", p.Body, p.Content)
	}
}

func TestParse_Version(t *testing.T) {
	dir, path := writeSkill(t, "---\nname: My Skill\nversion: 1.2.3\n---\nBody.\n")
	p := Parse(dir, path)
	if p.Version != "1.2.3" {
		t.Errorf("Version = %q", p.Version)
	}
}

func TestParse_VersionAsUnquotedNumberDoesNotBreakParsing(t *testing.T) {
	dir, path := writeSkill(t, "---\nname: My Skill\nversion: 1.0\n---\nBody.\n")
	p := Parse(dir, path)
	if p.Version != "1.0" {
		t.Errorf("Version = %q", p.Version)
	}
	if len(p.Diagnostics) != 0 {
		t.Errorf("unexpected diagnostics: %v", p.Diagnostics)
	}
}

func TestParse_MissingVersionIsEmpty(t *testing.T) {
	dir, path := writeSkill(t, "---\nname: My Skill\n---\nBody.\n")
	p := Parse(dir, path)
	if p.Version != "" {
		t.Errorf("Version = %q, want empty", p.Version)
	}
}

func TestParse_WhitespaceNormalized(t *testing.T) {
	dir, path := writeSkill(t, "---\nname: \"  My   Skill  \"\ndescription: \"  Does   the\n  thing.  \"\n---\nBody.\n")
	p := Parse(dir, path)
	if p.Name != "My Skill" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Description != "Does the thing." {
		t.Errorf("Description = %q", p.Description)
	}
}

func TestParse_MissingNameFallsBackToDirName(t *testing.T) {
	dir, path := writeSkill(t, "---\ndescription: Something.\n---\nBody.\n")
	p := Parse(dir, path)
	if p.Name != "my-skill" {
		t.Errorf("Name = %q, want directory name fallback", p.Name)
	}
}

func TestParse_MissingDescriptionFallsBackToFirstParagraph(t *testing.T) {
	dir, path := writeSkill(t, "---\nname: X\n---\n# Heading\n\nThis is the **first** paragraph with `code` and a [link](https://example.com).\n\nSecond paragraph ignored.\n")
	p := Parse(dir, path)
	want := "This is the first paragraph with code and a link."
	if p.Description != want {
		t.Errorf("Description = %q, want %q", p.Description, want)
	}
}

func TestParse_NoFrontMatterNoParagraphUsesNoDescription(t *testing.T) {
	dir, path := writeSkill(t, "\n\n")
	p := Parse(dir, path)
	if p.Name != "my-skill" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Description != noDescription {
		t.Errorf("Description = %q, want %q", p.Description, noDescription)
	}
}

func TestParse_NoFrontMatterUsesBodyParagraph(t *testing.T) {
	dir, path := writeSkill(t, "Just a plain paragraph, no front matter.\n")
	p := Parse(dir, path)
	if p.Name != "my-skill" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Description != "Just a plain paragraph, no front matter." {
		t.Errorf("Description = %q", p.Description)
	}
}

func TestParse_DescriptionTruncatedTo280Runes(t *testing.T) {
	long := strings.Repeat("a", 400)
	dir, path := writeSkill(t, "---\nname: X\ndescription: "+long+"\n---\nBody.\n")
	p := Parse(dir, path)
	if len([]rune(p.Description)) != MaxDescriptionRunes {
		t.Errorf("Description length = %d, want %d", len([]rune(p.Description)), MaxDescriptionRunes)
	}
}

func TestParse_InvalidFrontMatterStillProducesSkill(t *testing.T) {
	dir, path := writeSkill(t, "---\nname: [unterminated\n---\nFallback paragraph.\n")
	p := Parse(dir, path)
	if len(p.Diagnostics) == 0 {
		t.Fatal("expected a diagnostic for invalid front matter")
	}
	if p.Name != "my-skill" {
		t.Errorf("Name = %q, want directory fallback since front matter was invalid", p.Name)
	}
	if p.Description != "Fallback paragraph." {
		t.Errorf("Description = %q", p.Description)
	}
}

func TestParse_UnclosedFrontMatterDelimiter(t *testing.T) {
	dir, path := writeSkill(t, "---\nname: X\nno closing delimiter\n")
	p := Parse(dir, path)
	if len(p.Diagnostics) == 0 {
		t.Fatal("expected a diagnostic for unclosed front matter")
	}
}

func TestParse_UnknownFrontMatterFieldsIgnored(t *testing.T) {
	dir, path := writeSkill(t, "---\nname: X\nunknown_field: whatever\nlist: [1, 2, 3]\n---\nBody.\n")
	p := Parse(dir, path)
	if len(p.Diagnostics) != 0 {
		t.Errorf("unexpected diagnostics for unknown fields: %v", p.Diagnostics)
	}
	if p.Name != "X" {
		t.Errorf("Name = %q", p.Name)
	}
}

func TestParse_OversizedFileNotLoaded(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "big-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	big := make([]byte, MaxContentSize+1)
	if err := os.WriteFile(path, big, 0o644); err != nil {
		t.Fatal(err)
	}

	p := Parse(skillDir, path)
	if !p.Oversized {
		t.Error("expected Oversized = true")
	}
	if p.Content != "" {
		t.Error("expected Content not to be loaded")
	}
	if p.Name != "big-skill" {
		t.Errorf("Name = %q", p.Name)
	}
	if len(p.Diagnostics) == 0 {
		t.Error("expected a diagnostic for oversized content")
	}
}

func TestParse_MissingFileProducesDiagnostic(t *testing.T) {
	dir := t.TempDir()
	p := Parse(dir, filepath.Join(dir, "SKILL.md"))
	if len(p.Diagnostics) == 0 {
		t.Fatal("expected a diagnostic for a missing file")
	}
	if p.Description != noDescription {
		t.Errorf("Description = %q", p.Description)
	}
	if p.ContentHash != "" {
		t.Errorf("ContentHash = %q, want empty for an unreadable file", p.ContentHash)
	}
}

func TestParse_ContentHashMatchesForIdenticalBody(t *testing.T) {
	dirA, pathA := writeSkill(t, "---\nname: Foo\n---\nSame body text.\n")
	dirB, pathB := writeSkill(t, "---\nname: Foo\nversion: 2.0.0\n---\nSame body text.\n")

	a := Parse(dirA, pathA)
	b := Parse(dirB, pathB)

	if a.ContentHash == "" || b.ContentHash == "" {
		t.Fatalf("expected non-empty hashes, got %q and %q", a.ContentHash, b.ContentHash)
	}
	if a.ContentHash != b.ContentHash {
		t.Errorf("ContentHash differs despite identical bodies: %q vs %q", a.ContentHash, b.ContentHash)
	}
}

func TestParse_ContentHashDiffersForDifferentBody(t *testing.T) {
	dirA, pathA := writeSkill(t, "---\nname: Foo\n---\nBody one.\n")
	dirB, pathB := writeSkill(t, "---\nname: Foo\n---\nBody two.\n")

	a := Parse(dirA, pathA)
	b := Parse(dirB, pathB)

	if a.ContentHash == b.ContentHash {
		t.Errorf("ContentHash matches despite different bodies: %q", a.ContentHash)
	}
}
