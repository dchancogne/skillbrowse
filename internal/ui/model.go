// Package ui implements the Bubble Tea terminal interface: the catalog
// list, detail reader, search, rescan, and help overlay, per
// docs/superpowers/specs/2026-08-12-skillbrowse-design.md §3.
package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dchancogne/skillbrowse/internal/catalog"
	"github.com/dchancogne/skillbrowse/internal/markdown"
	"github.com/dchancogne/skillbrowse/internal/sources"
)

// WideBreakpoint is the terminal width at or above which the two-pane
// layout is used, per design doc §3.2.
const WideBreakpoint = 100

// MinWidth and MinHeight are the smallest terminal dimensions the UI will
// render normally in; below this, View shows a "too small" message
// instead of a corrupted layout (design doc §3.3). These specific values
// aren't dictated by the spec and are an implementation choice.
const (
	MinWidth  = 40
	MinHeight = 10
)

type focus int

const (
	focusList focus = iota
	focusDetail
)

// Model is the root Bubble Tea model for skillbrowse.
type Model struct {
	sources []sources.Source
	home    string
	dark    bool
	noColor bool

	list     list.Model
	viewport viewport.Model
	spinner  spinner.Model
	help     help.Model
	keys     appKeyMap
	mdCache  *markdown.Cache

	width, height int
	ready         bool
	narrow        bool

	focus     focus
	showRaw   bool
	showHelp  bool
	statusMsg string

	scanning     bool
	scanCancel   context.CancelFunc
	totalSkills  int
	sourceCount  int
	warningCount int
}

// Option customizes a Model returned by New.
type Option func(*Model)

// WithNoColor disables ANSI styling, honoring NO_COLOR/--no-color.
func WithNoColor(v bool) Option { return func(m *Model) { m.noColor = v } }

// WithDark selects the dark or light Markdown theme.
func WithDark(v bool) Option { return func(m *Model) { m.dark = v } }

// New builds the root model. srcs is the fully resolved source list
// (built-in registry + config + CLI paths) that rescans will walk.
func New(srcs []sources.Source, opts ...Option) *Model {
	m := &Model{
		sources:     srcs,
		dark:        true,
		sourceCount: len(srcs),
		mdCache:     markdown.NewCache(),
		keys:        newAppKeyMap(),
		help:        help.New(),
		spinner:     spinner.New(spinner.WithSpinner(spinner.Line)),
	}

	if home, err := os.UserHomeDir(); err == nil {
		m.home = home
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Skills"
	l.SetShowTitle(true)
	l.SetShowStatusBar(true)
	l.SetShowHelp(false)
	l.SetStatusBarItemName("skill", "skills")
	l.DisableQuitKeybindings()
	m.list = l

	m.viewport = viewport.New()

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.startScan())
}

// startScan cancels any in-flight scan and launches a fresh one.
func (m *Model) startScan() tea.Cmd {
	if m.scanCancel != nil {
		m.scanCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.scanCancel = cancel
	m.scanning = true
	return tea.Batch(m.spinner.Tick, scanCmd(ctx, m.sources))
}

func (m *Model) cancelScan() {
	if m.scanCancel != nil {
		m.scanCancel()
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.narrow = m.width < WideBreakpoint
		m.ready = true
		m.applySizes()
		return m, nil

	case spinner.TickMsg:
		if !m.scanning {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case scanResultMsg:
		return m, m.applyScanResult(msg)

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	// Anything else (e.g. list.FilterMatchesMsg, produced asynchronously
	// by the list component's own Cmds) belongs to the list.
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *Model) applyScanResult(msg scanResultMsg) tea.Cmd {
	m.scanning = false

	selectedID := ""
	if s := m.selectedSkill(); s != nil {
		selectedID = s.ID
	}

	m.totalSkills = len(msg.catalog.Skills)
	m.warningCount = len(msg.result.Diagnostics)
	for _, s := range msg.catalog.Skills {
		m.warningCount += len(s.Diagnostics)
	}

	cmd := m.list.SetItems(itemsFromSkills(msg.catalog.Skills, m.home))

	if selectedID != "" {
		for i, s := range msg.catalog.Skills {
			if s.ID == selectedID {
				m.list.Select(i)
				break
			}
		}
	}

	m.refreshDetailContent()
	return cmd
}

func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	if !m.ready || m.tooSmall() {
		if k == "q" || k == "ctrl+c" {
			m.cancelScan()
			return m, tea.Quit
		}
		return m, nil
	}

	if m.showHelp {
		switch k {
		case "?", "esc":
			m.showHelp = false
		case "q", "ctrl+c":
			m.cancelScan()
			return m, tea.Quit
		}
		return m, nil
	}

	// While the user is actively typing a filter query, only global quit
	// keys are intercepted; everything else (including "r", "v", "u",
	// "?") is text for the filter box.
	if m.list.SettingFilter() {
		if k == "ctrl+c" {
			m.cancelScan()
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	switch k {
	case "q", "ctrl+c":
		m.cancelScan()
		return m, tea.Quit
	case "?":
		m.showHelp = true
		return m, nil
	case "r":
		m.statusMsg = ""
		return m, m.startScan()
	case "v":
		if m.selectedSkill() != nil {
			m.showRaw = !m.showRaw
			m.refreshDetailContent()
		}
		return m, nil
	case "u":
		m.statusMsg = "Checking for updates is not implemented yet (Phase 3)."
		return m, nil
	}

	if m.focus == focusDetail {
		switch k {
		case "esc":
			m.focus = focusList
			return m, nil
		default:
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	switch k {
	case "enter":
		if m.selectedSkill() != nil {
			m.focus = focusDetail
			m.refreshDetailContent()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.refreshDetailContent()
	return m, cmd
}

func (m *Model) applySizes() {
	if m.tooSmall() {
		return
	}

	const footerHeight = 1
	contentHeight := m.height - footerHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	if m.narrow {
		m.list.SetSize(m.width, contentHeight)
		m.viewport.SetWidth(m.width)
		m.viewport.SetHeight(contentHeight)
	} else {
		listWidth := m.width / 3
		if listWidth < 24 {
			listWidth = 24
		}
		detailWidth := m.width - listWidth
		if detailWidth < 1 {
			detailWidth = 1
		}
		m.list.SetSize(listWidth, contentHeight)
		m.viewport.SetWidth(detailWidth)
		m.viewport.SetHeight(contentHeight)
	}

	m.help.SetWidth(m.width)
	m.refreshDetailContent()
}

func (m *Model) tooSmall() bool {
	return m.width < MinWidth || m.height < MinHeight
}

func (m *Model) selectedSkill() *catalog.Skill {
	item, ok := m.list.SelectedItem().(skillItem)
	if !ok {
		return nil
	}
	return &item.skill
}

func (m *Model) refreshDetailContent() {
	s := m.selectedSkill()
	if s == nil {
		m.viewport.SetContent("No skill selected.")
		return
	}

	header := m.renderHeader(*s)

	var body string
	switch {
	case m.showRaw || s.Content == "":
		body = s.Content
		if body == "" {
			body = "(content not available)"
		}
	default:
		rendered, err := m.mdCache.Render(s.Content, m.viewport.Width(), m.dark, m.noColor)
		if err != nil {
			body = s.Content
		} else {
			body = rendered
		}
	}

	m.viewport.SetContent(header + "\n" + body)
}

func (m *Model) renderHeader(s catalog.Skill) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s\n", m.style(s.Name, headerNameStyle))
	fmt.Fprintf(&b, "%s\n\n", m.style(s.Description, headerDescStyle))

	fmt.Fprintf(&b, "%s %s\n", m.style("Agents:", sectionStyle), strings.Join(s.Agents, ", "))
	fmt.Fprintf(&b, "%s %s\n", m.style("Sources:", sectionStyle), strings.Join(s.SourceLabels, ", "))
	for _, p := range s.ObservedPaths {
		fmt.Fprintf(&b, "  %s\n", shortenPath(p, m.home))
	}
	fmt.Fprintf(&b, "%s %s\n", m.style("Path:", sectionStyle), shortenPath(s.CanonicalPath, m.home))
	if !s.ModifiedAt.IsZero() {
		fmt.Fprintf(&b, "%s %s\n", m.style("Modified:", sectionStyle), s.ModifiedAt.Format(time.RFC3339))
	}

	if len(s.Diagnostics) > 0 {
		b.WriteString("\n" + m.style("Diagnostics:", sectionStyle) + "\n")
		for _, d := range s.Diagnostics {
			fmt.Fprintf(&b, "  %s\n", m.style("! "+d, diagnosticStyle))
		}
	}

	return b.String()
}

func (m *Model) style(s string, style lipgloss.Style) string {
	if m.noColor {
		return s
	}
	return style.Render(s)
}

func (m *Model) renderFooter() string {
	parts := []string{
		fmt.Sprintf("%d skills", m.totalSkills),
		fmt.Sprintf("%d sources", m.sourceCount),
		fmt.Sprintf("%d warnings", m.warningCount),
	}
	if m.scanning {
		parts = append(parts, m.spinner.View()+" scanning")
	}
	if m.statusMsg != "" {
		parts = append(parts, m.statusMsg)
	}
	return m.style(strings.Join(parts, "  ·  "), footerStyle)
}

func (m *Model) View() tea.View {
	if !m.ready {
		return tea.NewView("Loading…")
	}
	if m.tooSmall() {
		return tea.NewView(fmt.Sprintf(
			"Terminal too small. Need at least %dx%d, have %dx%d.",
			MinWidth, MinHeight, m.width, m.height,
		))
	}
	if m.showHelp {
		v := tea.NewView(m.help.View(m.keys))
		v.AltScreen = true
		return v
	}

	var body string
	if m.narrow {
		if m.focus == focusDetail {
			body = m.viewport.View()
		} else {
			body = m.list.View()
		}
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.list.View(), m.viewport.View())
	}

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, body, m.renderFooter()))
	v.AltScreen = true
	return v
}
