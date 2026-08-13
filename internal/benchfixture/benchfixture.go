// Package benchfixture generates synthetic skill trees for performance
// validation against docs/superpowers/specs/2026-08-12-skillbrowse-design.md
// §10 (NFR-01 through NFR-04), per
// docs/skillbrowse-implementation-plan.md Phase 5 ("Synthetic fixture
// generator (committed) for 1,000 and 10,000-skill trees").
package benchfixture

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const loremWord = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. "

// Generate creates n synthetic skill directories under root, each
// containing a valid SKILL.md with front matter and a body of
// approximately bodySize bytes. Directories are named so the resulting
// catalog sorts deterministically (skill-000000, skill-000001, ...).
func Generate(root string, n, bodySize int) error {
	body := loremWord
	for len(body) < bodySize {
		body += loremWord
	}
	body = body[:bodySize]

	for i := 0; i < n; i++ {
		dir := filepath.Join(root, fmt.Sprintf("skill-%06d", i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}

		var b strings.Builder
		fmt.Fprintf(&b, "---\nname: Skill %06d\ndescription: Synthetic benchmark skill number %06d.\n---\n", i, i)
		b.WriteString(body)
		b.WriteString("\n")

		path := filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}
