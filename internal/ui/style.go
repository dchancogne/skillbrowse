package ui

import "charm.land/lipgloss/v2"

var (
	headerNameStyle = lipgloss.NewStyle().Bold(true)
	headerDescStyle = lipgloss.NewStyle().Faint(true)
	sectionStyle    = lipgloss.NewStyle().Bold(true)
	diagnosticStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	footerStyle     = lipgloss.NewStyle().Faint(true)

	// metadataBoxStyle sets off a skill's metadata (name, description,
	// agents, sources, paths, dates) as a distinct block from its
	// Markdown-rendered body below it. MarginLeft indents the whole box;
	// Width is set per-render in renderHeader so long fields (notably
	// Description) wrap instead of overflowing the pane.
	metadataBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")).
				Padding(0, 1).
				MarginLeft(2)

	// metadataBoxStyleNoColor keeps the same box structure (plain Unicode
	// border glyphs, not ANSI) but skips BorderForeground, matching how
	// m.style already strips all styling under --no-color/NO_COLOR.
	metadataBoxStyleNoColor = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				Padding(0, 1).
				MarginLeft(2)

	// helpTitleStyle labels the "?" help overlay's box.
	helpTitleStyle = lipgloss.NewStyle().Bold(true)

	// helpKeyStyle sets off each binding's key label from its description
	// in the "?" help overlay.
	helpKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))

	// helpBoxStyle frames the full keybinding list shown by the "?"
	// overlay, so it reads as a distinct panel rather than a stray line of
	// text pinned to the corner of the screen.
	helpBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2)

	// helpBoxStyleNoColor mirrors helpBoxStyle without ANSI color, matching
	// how m.style strips all styling under --no-color/NO_COLOR.
	helpBoxStyleNoColor = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				Padding(1, 2)

	// activePaneStyle frames whichever pane (list or detail) currently
	// receives keyboard input, so focus is visible at a glance rather than
	// inferred from where the cursor last was.
	activePaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("6"))

	// inactivePaneStyle frames the pane that isn't receiving input, using
	// the same dim border color as the rest of the UI's boxes so the active
	// pane's brighter border reads as the sole highlight.
	inactivePaneStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))

	// activePaneStyleNoColor and inactivePaneStyleNoColor differentiate
	// focus without relying on color (--no-color/NO_COLOR): the active pane
	// gets a heavier border weight instead of a brighter color.
	activePaneStyleNoColor = lipgloss.NewStyle().
				Border(lipgloss.ThickBorder())

	inactivePaneStyleNoColor = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder())
)

// paneBorderOverhead is how many columns/rows a pane's border style adds
// beyond its content size: 1 on each side for the top/bottom/left/right
// border edges.
const paneBorderOverhead = 2
