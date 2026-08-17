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
)
