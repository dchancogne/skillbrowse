package ui

import (
	"strings"

	"charm.land/bubbles/v2/list"

	"github.com/dchancogne/skillbrowse/internal/catalog"
)

// skillItem adapts a catalog.Skill to bubbles/list's Item interface.
// skillDelegate (delegate.go) renders it; skillItem itself only needs to
// provide the searchable filter text.
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
