package ui

import "charm.land/bubbles/v2/key"

// appKeyMap is the full set of application-level bindings from
// docs/superpowers/specs/2026-08-12-skillbrowse-design.md §3.6. It is
// rendered by the help overlay ("?") independently of the list
// component's own (unused) help view, so it can describe exactly the
// bindings this application defines rather than bubbles/list's generic
// pagination keys.
type appKeyMap struct {
	Up, Down     key.Binding
	Search       key.Binding
	Enter        key.Binding
	Esc          key.Binding
	ScrollDetail key.Binding
	RawToggle    key.Binding
	Rescan       key.Binding
	Update       key.Binding
	Help         key.Binding
	Quit         key.Binding
}

func newAppKeyMap() appKeyMap {
	return appKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "focus/open details"),
		),
		Esc: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "clear search / return to catalog"),
		),
		ScrollDetail: key.NewBinding(
			key.WithKeys("up", "down", "pgup", "pgdown"),
			key.WithHelp("↑/↓/pgup/pgdn", "scroll details"),
		),
		RawToggle: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "toggle raw/rendered"),
		),
		Rescan: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "rescan"),
		),
		Update: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "check for updates"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// ShortHelp and FullHelp implement help.KeyMap.
func (k appKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Search, k.Enter, k.Help, k.Quit}
}

func (k appKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Search, k.Enter},
		{k.Esc, k.ScrollDetail, k.RawToggle},
		{k.Rescan, k.Update, k.Help, k.Quit},
	}
}
