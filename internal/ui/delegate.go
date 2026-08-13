package ui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	maxAgentsShown = 5
	maxLabelsShown = 3
)

// skillDelegate renders each catalog entry as three lines — name/version
// plus install path, then agents/source, then description — closer to
// what a developer scanning an installed-skills list actually wants to
// see than bubbles/list's stock two-line (title/description) delegate.
type skillDelegate struct {
	styles skillDelegateStyles
}

type skillDelegateStyles struct {
	normalPrimary, selectedPrimary     lipgloss.Style
	normalSecondary, selectedSecondary lipgloss.Style
	normalDesc, selectedDesc           lipgloss.Style
}

func newSkillDelegate() skillDelegate {
	// Mirrors bubbles/list's own DefaultItemStyles convention: the
	// selected row gets a left border "bar" plus 1 cell of padding, the
	// normal row gets 2 cells of padding instead so text still lines up.
	primary := lipgloss.NewStyle().Padding(0, 0, 0, 2)
	secondary := primary.Foreground(lipgloss.Color("245"))
	desc := primary.Foreground(lipgloss.Color("241"))

	selectedPrimary := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("212")).
		Foreground(lipgloss.Color("212")).
		Padding(0, 0, 0, 1)
	selectedSecondary := selectedPrimary.Foreground(lipgloss.Color("219"))
	selectedDesc := selectedPrimary.Foreground(lipgloss.Color("183"))

	return skillDelegate{styles: skillDelegateStyles{
		normalPrimary:     primary,
		selectedPrimary:   selectedPrimary,
		normalSecondary:   secondary,
		selectedSecondary: selectedSecondary,
		normalDesc:        desc,
		selectedDesc:      selectedDesc,
	}}
}

func (d skillDelegate) Height() int  { return 3 }
func (d skillDelegate) Spacing() int { return 1 }

func (d skillDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d skillDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	si, ok := item.(skillItem)
	if !ok || m.Width() <= 0 {
		return
	}

	textWidth := m.Width() - d.styles.normalPrimary.GetPaddingLeft() - d.styles.normalPrimary.GetPaddingRight()

	primary := si.skill.Name
	if si.skill.Version != "" {
		primary += "  v" + si.skill.Version
	}
	if len(si.skill.Diagnostics) > 0 {
		primary += " !"
	}
	path := shortenPath(si.skill.CanonicalPath, si.home)
	if len(si.skill.ObservedPaths) > 1 {
		path = fmt.Sprintf("%d sources", len(si.skill.ObservedPaths))
	}
	primary += "    " + path

	secondary := fmt.Sprintf("Agents: %s    Source: %s",
		joinWithMore(si.skill.Agents, maxAgentsShown),
		joinWithMore(si.skill.SourceLabels, maxLabelsShown),
	)

	primary = ansi.Truncate(primary, textWidth, "…")
	secondary = ansi.Truncate(secondary, textWidth, "…")
	desc := ansi.Truncate(si.skill.Description, textWidth, "…")

	ps, ss, ds := d.styles.normalPrimary, d.styles.normalSecondary, d.styles.normalDesc
	if index == m.Index() {
		ps, ss, ds = d.styles.selectedPrimary, d.styles.selectedSecondary, d.styles.selectedDesc
	}

	fmt.Fprintf(w, "%s\n%s\n%s", ps.Render(primary), ss.Render(secondary), ds.Render(desc)) //nolint:errcheck
}

// joinWithMore joins the first max items with ", " and summarizes the
// rest as "+N more", matching the convention of tools that list agent
// compatibility for a skill without letting a long list dominate the row.
func joinWithMore(items []string, max int) string {
	if len(items) <= max {
		return strings.Join(items, ", ")
	}
	return fmt.Sprintf("%s +%d more", strings.Join(items[:max], ", "), len(items)-max)
}
