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
)
