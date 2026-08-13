package ui

import "charm.land/lipgloss/v2"

var (
	headerNameStyle = lipgloss.NewStyle().Bold(true)
	headerDescStyle = lipgloss.NewStyle().Faint(true)
	sectionStyle    = lipgloss.NewStyle().Bold(true)
	diagnosticStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	footerStyle     = lipgloss.NewStyle().Faint(true)
)
