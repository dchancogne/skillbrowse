package benchfixture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCreatesExpectedTree(t *testing.T) {
	root := t.TempDir()

	const n = 5
	const bodySize = 200
	if err := Generate(root, n, bodySize); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != n {
		t.Fatalf("got %d skill directories, want %d", len(entries), n)
	}

	wantDir := "skill-000003"
	found := false
	for _, e := range entries {
		if e.Name() == wantDir {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected directory %q not found among %v", wantDir, entries)
	}

	skillPath := filepath.Join(root, wantDir, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", skillPath, err)
	}
	content := string(data)

	if !strings.HasPrefix(content, "---\n") {
		t.Errorf("SKILL.md does not start with front matter delimiter: %q", content[:20])
	}
	if !strings.Contains(content, "name: Skill 000003") {
		t.Errorf("SKILL.md missing expected name field, got: %q", content)
	}
	if !strings.Contains(content, "description: Synthetic benchmark skill number 000003.") {
		t.Errorf("SKILL.md missing expected description field, got: %q", content)
	}

	frontMatterEnd := strings.Index(content[4:], "---\n")
	if frontMatterEnd == -1 {
		t.Fatalf("could not find closing front matter delimiter")
	}
	body := content[4+frontMatterEnd+4:]
	if len(strings.TrimSuffix(body, "\n")) != bodySize {
		t.Errorf("body length = %d, want %d", len(strings.TrimSuffix(body, "\n")), bodySize)
	}
}

func TestGenerateZeroSkills(t *testing.T) {
	root := t.TempDir()

	if err := Generate(root, 0, 100); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries, want 0", len(entries))
	}
}

func TestGenerateInvalidRoot(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "not-a-dir")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup WriteFile() error = %v", err)
	}

	if err := Generate(filepath.Join(blocked, "child"), 1, 10); err == nil {
		t.Fatal("Generate() error = nil, want error for unwritable root")
	}
}
