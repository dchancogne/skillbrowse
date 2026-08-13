package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"

	"github.com/dchancogne/skillbrowse/internal/catalog"
)

// skillItem adapts a catalog.Skill to bubbles/list's Item and DefaultItem
// interfaces.
type skillItem struct {
	skill catalog.Skill
	home  string
}

// FilterValue deliberately omits paths, unlike catalog.SearchableText:
// bubbles/list's built-in filter requires the query to fuzzy-subsequence-
// match the ENTIRE FilterValue with no relevance threshold, so a long,
// character-rich absolute path makes nearly any short query spuriously
// "match" every item. Name/description/agents/labels are short enough
// that this isn't a problem, and remain fully searchable.
func (i skillItem) FilterValue() string {
	return strings.Join([]string{
		i.skill.Name,
		i.skill.Description,
		strings.Join(i.skill.Agents, " "),
		strings.Join(i.skill.SourceLabels, " "),
	}, " ")
}

func (i skillItem) Title() string {
	if len(i.skill.Diagnostics) > 0 {
		return i.skill.Name + " !"
	}
	return i.skill.Name
}

func (i skillItem) Description() string {
	desc := i.skill.Description

	path := shortenPath(i.skill.CanonicalPath, i.home)
	if len(i.skill.ObservedPaths) > 1 {
		path = fmt.Sprintf("%d sources", len(i.skill.ObservedPaths))
	}

	return fmt.Sprintf("%s  ·  %s  ·  %s", desc, strings.Join(i.skill.Agents, ", "), path)
}

func itemsFromSkills(skills []catalog.Skill, home string) []list.Item {
	items := make([]list.Item, len(skills))
	for i, s := range skills {
		items[i] = skillItem{skill: s, home: home}
	}
	return items
}

// shortenPath replaces a leading home-directory prefix with "~", per
// design doc §9 ("must use home-relative paths in the UI where possible").
func shortenPath(path, home string) string {
	if home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+"/") {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
