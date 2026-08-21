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

	// maxDescLines caps how many lines a list item's description wraps
	// onto, so every item still contributes a fixed, predictable height
	// (bubbles/list sizes its viewport from delegate.Height(), not from
	// per-item content).
	maxDescLines = 2
)

// skillDelegate renders each catalog entry as name/version plus install
// path, then agents/source, then a word-wrapped description — closer to
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

func (d skillDelegate) Height() int  { return 2 + maxDescLines }
func (d skillDelegate) Spacing() int { return 1 }

func (d skillDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d skillDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	si, ok := item.(skillItem)
	if !ok || m.Width() <= 0 {
		return
	}

	textWidth := m.Width() - d.styles.normalPrimary.GetPaddingLeft() - d.styles.normalPrimary.GetPaddingRight()

	primary := si.skill.Name
	if si.skill.ProjectScoped {
		primary += "  [project]"
	}
	if si.skill.Version != "" {
		primary += "  v" + si.skill.Version
	}
	if len(si.skill.Diagnostics) > 0 {
		primary += " !"
	}
	path := shortenPath(si.skill.CanonicalPath, si.home)
	if len(si.skill.ObservedPaths) > 1 {
		path = fmt.Sprintf("(%d linked sources)", len(si.skill.ObservedPaths))
	}
	primary += "    " + path
	if si.skill.DuplicateNameCount > 0 {
		if si.skill.DuplicateContentDiffers {
			primary += fmt.Sprintf("  ⚠ (diverges in %d other places)", si.skill.DuplicateNameCount)
		} else {
			primary += fmt.Sprintf("  (identical in %d other places)", si.skill.DuplicateNameCount)
		}
	}

	// Source labels aren't shown here: for every built-in source they're
	// identical to Agents (see internal/sources.Registry), so a parallel
	// "Source: ..." field was pure duplication in the common case; the
	// detail header's "Sources:" section still surfaces the underlying
	// discovered paths for the cases where it isn't (custom sources).
	secondary := "Agents: " + joinWithMore(si.skill.Agents, maxAgentsShown)

	primary = ansi.Truncate(primary, textWidth, "…")
	secondary = ansi.Truncate(secondary, textWidth, "…")
	descLines := wrapText(si.skill.Description, textWidth, maxDescLines)
	for len(descLines) < maxDescLines {
		// Pad so every item renders exactly Height() lines: bubbles/list
		// paginates assuming a fixed per-item height, so a short
		// description can't be allowed to shrink this item's row count.
		descLines = append(descLines, "")
	}

	ps, ss, ds := d.styles.normalPrimary, d.styles.normalSecondary, d.styles.normalDesc
	if index == m.Index() {
		ps, ss, ds = d.styles.selectedPrimary, d.styles.selectedSecondary, d.styles.selectedDesc
	}

	lines := []string{ps.Render(primary), ss.Render(secondary)}
	for _, l := range descLines {
		lines = append(lines, ds.Render(l))
	}

	fmt.Fprint(w, strings.Join(lines, "\n")) //nolint:errcheck
}

// wrapText greedily word-wraps s to width, returning at most maxLines
// lines. If s doesn't fit in maxLines, the last line is truncated with an
// ellipsis rather than silently dropping the remainder off the edge of
// the pane. maxLines is a parameter (not hardcoded to maxDescLines) so
// wrap behavior is testable independent of the delegate's own layout
// constant.
//
//nolint:unparam // see above; the single call site always passes maxDescLines today.
func wrapText(s string, width, maxLines int) []string {
	if width <= 0 || maxLines <= 0 {
		return nil
	}

	var lines []string
	var cur strings.Builder
	flush := func() {
		lines = append(lines, cur.String())
		cur.Reset()
	}

	for _, word := range strings.Fields(s) {
		switch {
		case cur.Len() == 0:
			cur.WriteString(word)
		case lipgloss.Width(cur.String()+" "+word) <= width:
			cur.WriteString(" " + word)
		default:
			flush()
			cur.WriteString(word)
		}
	}
	if cur.Len() > 0 {
		flush()
	}

	if len(lines) <= maxLines {
		return lines
	}
	lines = lines[:maxLines]

	// Force a trailing ellipsis so a hidden remainder is never silently
	// dropped: ansi.Truncate only adds one when the line itself exceeds
	// width, but here the *last visible line* can easily be short even
	// though more text followed it before wrapText cut off at maxLines.
	const ellipsis = "…"
	avail := width - lipgloss.Width(ellipsis)
	if avail < 0 {
		avail = 0
	}
	lines[maxLines-1] = ansi.Truncate(lines[maxLines-1], avail, "") + ellipsis
	return lines
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
