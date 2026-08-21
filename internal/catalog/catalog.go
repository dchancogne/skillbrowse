// Package catalog merges discovered skill candidates into deduplicated
// records, sorts them deterministically, and provides fuzzy search, per
// docs/superpowers/specs/2026-08-12-skillbrowse-design.md §5.5 and §6.
package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/sahilm/fuzzy"

	"github.com/dchancogne/skillbrowse/internal/discovery"
	"github.com/dchancogne/skillbrowse/internal/skill"
)

// Skill is one normalized, deduplicated catalog record.
type Skill struct {
	ID            string
	Name          string
	Description   string
	Version       string // optional; "" when SKILL.md's front matter has no version field
	CanonicalPath string
	SkillFilePath string
	ObservedPaths []string
	Agents        []string
	SourceLabels  []string
	ModifiedAt    time.Time
	Content       string // full raw SKILL.md file, for the raw-view toggle
	Body          string // Content with front matter stripped, for Markdown rendering
	ContentHash   string
	Diagnostics   []string

	// DuplicateNameCount is how many other records (at different canonical
	// paths) share this Skill's Name, case-insensitively. It is 0 unless a
	// same-named skill exists at a genuinely separate path - per design
	// doc §5.5, canonical-path duplicates already merge above this point,
	// so this only fires on real, unrelated same-name collisions.
	DuplicateNameCount int
	// DuplicateContentDiffers is true when at least one same-named sibling
	// has a different ContentHash, meaning the copies have diverged rather
	// than being independently-placed copies of identical content.
	DuplicateContentDiffers bool
}

// Catalog is an immutable snapshot of every discovered skill, sorted in
// the deterministic default order (case-insensitive name, then canonical
// path).
type Catalog struct {
	Skills []Skill
}

// ParseFunc parses one SKILL.md file into normalized metadata. skill.Parse
// satisfies this signature; tests may substitute a fake.
type ParseFunc func(canonicalPath, skillFilePath string) skill.Parsed

// Build merges discovery candidates that share a canonical path into a
// single Skill record, parsing each unique canonical path exactly once,
// and returns them in the deterministic default sort order.
func Build(candidates []discovery.Candidate, parse ParseFunc) Catalog {
	groups := make(map[string][]discovery.Candidate)
	for _, c := range candidates {
		groups[c.CanonicalPath] = append(groups[c.CanonicalPath], c)
	}

	skills := make([]Skill, 0, len(groups))
	for canonicalPath, group := range groups {
		first := group[0]
		parsed := parse(first.CanonicalPath, first.SkillFilePath)

		var observed, agents, labels []string
		for _, c := range group {
			observed = append(observed, c.ObservedPath)
			agents = append(agents, c.Source.Agents...)
			labels = append(labels, c.Source.Label)
		}

		skills = append(skills, Skill{
			ID:            skillID(canonicalPath),
			Name:          parsed.Name,
			Description:   parsed.Description,
			Version:       parsed.Version,
			CanonicalPath: canonicalPath,
			SkillFilePath: first.SkillFilePath,
			ObservedPaths: sortedUnique(observed),
			Agents:        sortedUnique(agents),
			SourceLabels:  sortedUnique(labels),
			ModifiedAt:    parsed.ModifiedAt,
			Content:       parsed.Content,
			Body:          parsed.Body,
			ContentHash:   parsed.ContentHash,
			Diagnostics:   parsed.Diagnostics,
		})
	}

	markNameCollisions(skills)
	sortDefault(skills)
	return Catalog{Skills: skills}
}

// markNameCollisions sets DuplicateNameCount and DuplicateContentDiffers on
// every record that shares its Name (case-insensitively) with a record at a
// different CanonicalPath. Canonical-path duplicates are already merged
// into one record by Build, so this only detects genuinely separate
// same-name installations - an advisory signal, not a merge: per design
// doc §5.5, same-named skills at different paths remain distinct records.
func markNameCollisions(skills []Skill) {
	byName := make(map[string][]int, len(skills))
	for i, s := range skills {
		key := strings.ToLower(s.Name)
		byName[key] = append(byName[key], i)
	}

	for _, idxs := range byName {
		if len(idxs) < 2 {
			continue
		}
		for _, i := range idxs {
			skills[i].DuplicateNameCount = len(idxs) - 1
			for _, j := range idxs {
				if j != i && skills[j].ContentHash != skills[i].ContentHash {
					skills[i].DuplicateContentDiffers = true
					break
				}
			}
		}
	}
}

// BuildFromCandidates is Build using skill.Parse directly.
func BuildFromCandidates(candidates []discovery.Candidate) Catalog {
	return Build(candidates, skill.Parse)
}

func sortDefault(skills []Skill) {
	sort.Slice(skills, func(i, j int) bool {
		ni, nj := strings.ToLower(skills[i].Name), strings.ToLower(skills[j].Name)
		if ni != nj {
			return ni < nj
		}
		return skills[i].CanonicalPath < skills[j].CanonicalPath
	})
}

// Search performs case-insensitive fuzzy search over name, description,
// agents, source labels, observed paths, and canonical path, ranked by
// match score. An empty query returns skills unchanged (the deterministic
// default order). Equal-score matches retain skills' relative input
// order, so callers should pass an already-default-sorted slice for
// deterministic path tie-breaking.
func Search(skills []Skill, query string) []Skill {
	query = strings.TrimSpace(query)
	if query == "" {
		return skills
	}

	searchable := make([]string, len(skills))
	for i, s := range skills {
		searchable[i] = SearchableText(s)
	}

	matches := fuzzy.Find(query, searchable)
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	result := make([]Skill, len(matches))
	for i, m := range matches {
		result[i] = skills[m.Index]
	}
	return result
}

// SearchableText concatenates the fields Search matches against, per
// design doc §6: name, description, agents, source labels, observed
// paths, and canonical path.
func SearchableText(s Skill) string {
	return strings.Join([]string{
		s.Name,
		s.Description,
		strings.Join(s.Agents, " "),
		strings.Join(s.SourceLabels, " "),
		strings.Join(s.ObservedPaths, " "),
		s.CanonicalPath,
	}, " ")
}

func skillID(canonicalPath string) string {
	sum := sha256.Sum256([]byte(canonicalPath))
	return hex.EncodeToString(sum[:])[:16]
}

func sortedUnique(values []string) []string {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
