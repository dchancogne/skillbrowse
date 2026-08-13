// Package skill parses SKILL.md front matter into a normalized model with
// deterministic fallbacks and diagnostics, per
// docs/superpowers/specs/2026-08-12-skillbrowse-design.md §5.4.
package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// MaxContentSize is the size cap above which a SKILL.md file's content is
// not loaded; the record remains listed with a diagnostic instead.
const MaxContentSize = 2 * 1024 * 1024 // 2 MiB

// MaxDescriptionRunes bounds the fallback description derived from the
// first Markdown paragraph.
const MaxDescriptionRunes = 280

const (
	skillFileName    = "SKILL.md"
	noDescription    = "No description provided"
	frontMatterDelim = "---"
)

// Parsed is the normalized result of parsing one SKILL.md file.
type Parsed struct {
	Name        string
	Description string
	Version     string // optional; "" when the front matter has no version field
	Content     string
	ModifiedAt  time.Time
	Oversized   bool
	Diagnostics []string
}

// Parse reads and parses the SKILL.md file at skillFilePath. canonicalPath
// is the containing skill directory, used for the directory-name name
// fallback. Parse never returns an error: any problem becomes a
// diagnostic on the returned Parsed value, and fallbacks keep the skill
// visible per design doc §5.4 ("does not hide the skill").
func Parse(canonicalPath, skillFilePath string) Parsed {
	fallbackName := filepath.Base(canonicalPath)

	info, err := os.Stat(skillFilePath)
	if err != nil {
		return Parsed{
			Name:        fallbackName,
			Description: noDescription,
			Diagnostics: []string{fmt.Sprintf("cannot stat %s: %s", skillFileName, err)},
		}
	}

	if info.Size() > MaxContentSize {
		return Parsed{
			Name:        fallbackName,
			Description: noDescription,
			ModifiedAt:  info.ModTime(),
			Oversized:   true,
			Diagnostics: []string{fmt.Sprintf("%s exceeds 2 MiB and was not loaded", skillFileName)},
		}
	}

	data, err := os.ReadFile(skillFilePath)
	if err != nil {
		return Parsed{
			Name:        fallbackName,
			Description: noDescription,
			ModifiedAt:  info.ModTime(),
			Diagnostics: []string{fmt.Sprintf("cannot read %s: %s", skillFileName, err)},
		}
	}

	content := string(data)
	fm, body, hasFrontMatter, fmErr := extractFrontMatter(content)

	var diagnostics []string
	name := fallbackName
	description := ""
	version := ""

	if hasFrontMatter {
		if fmErr != nil {
			diagnostics = append(diagnostics, fmt.Sprintf("invalid front matter: %s", fmErr))
		} else {
			if n := normalizeWhitespace(fm.Name); n != "" {
				name = n
			}
			if d := normalizeWhitespace(fm.Description); d != "" {
				description = truncateRunes(d, MaxDescriptionRunes)
			}
			version = normalizeWhitespace(fm.Version)
		}
	}

	if description == "" {
		description = firstParagraph(body)
	}
	if description == "" {
		description = noDescription
	}

	return Parsed{
		Name:        name,
		Description: description,
		Version:     version,
		Content:     content,
		ModifiedAt:  info.ModTime(),
		Diagnostics: diagnostics,
	}
}

type frontMatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
}

// extractFrontMatter splits a leading "---" ... "---" YAML block from the
// rest of the document. hasFrontMatter is false when the content doesn't
// begin with a front-matter delimiter at all (not itself an error).
func extractFrontMatter(content string) (fm frontMatter, body string, hasFrontMatter bool, err error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != frontMatterDelim {
		return frontMatter{}, content, false, nil
	}

	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == frontMatterDelim {
			closeIdx = i
			break
		}
	}
	if closeIdx == -1 {
		return frontMatter{}, content, true, fmt.Errorf("no closing %q delimiter found", frontMatterDelim)
	}

	yamlBlock := strings.Join(lines[1:closeIdx], "\n")
	body = strings.Join(lines[closeIdx+1:], "\n")

	var raw frontMatter
	if err := yaml.Unmarshal([]byte(yamlBlock), &raw); err != nil {
		return frontMatter{}, body, true, err
	}
	return raw, body, true, nil
}

var (
	markdownLink   = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	markdownInline = regexp.MustCompile("[*_`]")
)

// firstParagraph returns the first run of non-blank, non-heading lines in
// body, stripped of basic Markdown formatting and truncated to
// MaxDescriptionRunes. It returns "" if no such paragraph exists.
func firstParagraph(body string) string {
	var para []string
	collecting := false

	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		isHeading := strings.HasPrefix(trimmed, "#")

		if trimmed == "" || isHeading {
			if collecting {
				break
			}
			continue
		}

		collecting = true
		para = append(para, trimmed)
	}

	if len(para) == 0 {
		return ""
	}

	joined := strings.Join(para, " ")
	joined = markdownLink.ReplaceAllString(joined, "$1")
	joined = markdownInline.ReplaceAllString(joined, "")
	return truncateRunes(normalizeWhitespace(joined), MaxDescriptionRunes)
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
