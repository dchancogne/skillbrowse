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
}
