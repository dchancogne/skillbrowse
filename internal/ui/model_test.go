package ui

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dchancogne/skillbrowse/internal/catalog"
	"github.com/dchancogne/skillbrowse/internal/discovery"
	"github.com/dchancogne/skillbrowse/internal/sources"
)

// ansiSeq strips ANSI escape sequences from rendered views. Outside a real
// tea.Program (which honors WithColorProfile), lipgloss falls back to
// environment-based color detection, so plain string assertions on
// m.View() need this regardless of WithNoColor.
var ansiSeq = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string { return ansiSeq.ReplaceAllString(s, "") }

func writeSkill(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// twoSkillSources builds a two-skill fixture tree and returns the sources
// pointing at it, sorted so "Alpha" sorts before "Beta" in the
// deterministic default order.
func twoSkillSources(t *testing.T) []sources.Source {
	t.Helper()
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "alpha"), "---\nname: Alpha\ndescription: First skill.\n---\nAlpha body.\n")
	writeSkill(t, filepath.Join(root, "beta"), "---\nname: Beta\ndescription: Second skill.\n---\nBeta body.\n")
	return []sources.Source{{Label: "Test", Agents: []string{"Test"}, Root: root, MaxDepth: 1}}
}

// runScan performs a real scan synchronously (bypassing the async Cmd
// plumbing, which is thin glue already covered by manual smoke testing)
// and feeds the result straight into Update, the way the real program
// would once the Cmd returned by Init/startScan completes.
func runScan(t *testing.T, m *Model, srcs []sources.Source) {
	t.Helper()
	result := discovery.Scan(context.Background(), srcs)
	cat := catalog.BuildFromCandidates(result.Candidates)
	if _, cmd := m.Update(scanResultMsg{result: result, catalog: cat}); cmd != nil {
		cmd() // drain list.SetItems' returned Cmd; its message isn't needed by these tests
	}
}

func newTestModel(t *testing.T, width, height int, srcs []sources.Source) *Model {
	t.Helper()
	m := New(srcs, WithNoColor(true))
	if _, cmd := m.Update(tea.WindowSizeMsg{Width: width, Height: height}); cmd != nil {
		cmd()
	}
	runScan(t, m, srcs)
	return m
}

func pressKey(m *Model, s string) tea.Cmd {
	_, cmd := m.Update(tea.KeyPressMsg{Text: s, Code: rune(s[0])})
	return cmd
}

// drain runs cmd once and feeds its resulting message back into Update,
// the way tea.Program's real event loop would for one hop. Some list
// operations (e.g. filtering) compute their result as a Cmd rather than
// synchronously, so tests that depend on that result need to drive it at
// least one step. This deliberately does NOT recurse into whatever Cmd
// that next Update call returns: some of bubbles' own components
// (spinner ticks, status-message auto-hide) schedule real tea.Tick
// timers, and blindly chasing every subsequent Cmd would mean actually
// sleeping through them.
func drain(m *Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			m.Update(c())
		}
		return
	}
	m.Update(msg)
}

func TestModel_ScanPopulatesListAndFooterCounts(t *testing.T) {
	srcs := twoSkillSources(t)
	m := newTestModel(t, 120, 40, srcs)

	if m.totalSkills != 2 {
		t.Fatalf("totalSkills = %d, want 2", m.totalSkills)
	}
	if len(m.list.Items()) != 2 {
		t.Fatalf("list has %d items, want 2", len(m.list.Items()))
	}

	view := stripANSI(m.View().Content)
	if !strings.Contains(view, "Alpha") {
		t.Errorf("view does not contain first skill name: %q", view)
	}
	if !strings.Contains(view, "2 skills") {
		t.Errorf("footer missing skill count: %q", view)
	}
	if !strings.Contains(view, "1 sources") {
		t.Errorf("footer missing source count: %q", view)
	}
}

func TestModel_WideLayoutShowsListAndDetailTogether(t *testing.T) {
	srcs := twoSkillSources(t)
	m := newTestModel(t, 120, 40, srcs)

	view := stripANSI(m.View().Content)
	if !strings.Contains(view, "Alpha") {
		t.Errorf("wide view missing list content: %q", view)
	}
	if !strings.Contains(view, "First skill.") {
		t.Errorf("wide view missing detail content for the selected skill: %q", view)
	}
}

func TestModel_NarrowLayoutOpensAndClosesDetailWithEnterAndEsc(t *testing.T) {
	srcs := twoSkillSources(t)
	m := newTestModel(t, 60, 30, srcs)

	if !m.narrow {
		t.Fatal("expected narrow layout at width 60")
	}

	listView := stripANSI(m.View().Content)
	if !strings.Contains(listView, "Alpha") {
		t.Errorf("narrow list view missing skill name: %q", listView)
	}
	if strings.Contains(listView, "Alpha body.") {
		t.Errorf("narrow list view should not show the full detail body before Enter: %q", listView)
	}

	pressKey(m, "enter")
	if m.focus != focusDetail {
		t.Fatal("expected focus to move to detail after Enter")
	}
	detailView := stripANSI(m.View().Content)
	if !strings.Contains(detailView, "First skill.") {
		t.Errorf("narrow detail view missing body: %q", detailView)
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.focus != focusList {
		t.Fatal("expected Esc to return focus to the catalog")
	}
}

func TestModel_RawToggleChangesDetailContent(t *testing.T) {
	srcs := twoSkillSources(t)
	m := newTestModel(t, 120, 40, srcs)

	rendered := m.viewport.GetContent()
	pressKey(m, "v")
	if !m.showRaw {
		t.Fatal("expected showRaw = true after pressing v")
	}
	raw := m.viewport.GetContent()

	if rendered == raw {
		t.Error("expected raw and rendered content to differ")
	}
	if !strings.Contains(raw, "---") {
		t.Errorf("raw content should include front matter delimiters, got %q", raw)
	}
}

func TestModel_RescanPreservesSelection(t *testing.T) {
	srcs := twoSkillSources(t)
	m := newTestModel(t, 120, 40, srcs)

	m.list.Select(1) // select "Beta"
	beta := m.selectedSkill()
	if beta == nil || beta.Name != "Beta" {
		t.Fatalf("expected Beta selected, got %+v", beta)
	}

	runScan(t, m, srcs) // simulate a rescan completing with the same fixture

	again := m.selectedSkill()
	if again == nil || again.Name != "Beta" {
		t.Fatalf("expected Beta to remain selected after rescan, got %+v", again)
	}
}

func TestModel_HelpTogglesOverlay(t *testing.T) {
	srcs := twoSkillSources(t)
	m := newTestModel(t, 120, 40, srcs)

	pressKey(m, "?")
	if !m.showHelp {
		t.Fatal("expected showHelp = true after ?")
	}
	help := stripANSI(m.View().Content)
	if !strings.Contains(help, "quit") {
		t.Errorf("help view missing quit binding: %q", help)
	}

	pressKey(m, "?")
	if m.showHelp {
		t.Fatal("expected ? to close the help overlay")
	}
}

func TestModel_TooSmallShowsFallbackMessage(t *testing.T) {
	srcs := twoSkillSources(t)
	m := New(srcs, WithNoColor(true))
	m.Update(tea.WindowSizeMsg{Width: 10, Height: 5})

	view := stripANSI(m.View().Content)
	if !strings.Contains(view, "too small") {
		t.Errorf("expected too-small fallback message, got %q", view)
	}
}

func TestModel_QuitKeyReturnsTeaQuit(t *testing.T) {
	srcs := twoSkillSources(t)
	m := newTestModel(t, 120, 40, srcs)

	cmd := pressKey(m, "q")
	if cmd == nil {
		t.Fatal("expected a Cmd from pressing q")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestModel_SearchFiltersList(t *testing.T) {
	srcs := twoSkillSources(t)
	m := newTestModel(t, 120, 40, srcs)

	drain(m, pressKey(m, "/"))
	if !m.list.SettingFilter() {
		t.Fatal("expected / to open the filter input")
	}

	for _, r := range "Alpha" {
		_, cmd := m.Update(tea.KeyPressMsg{Text: string(r), Code: r})
		drain(m, cmd)
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	drain(m, cmd)

	visible := m.list.VisibleItems()
	if len(visible) != 1 {
		t.Fatalf("expected 1 visible item after filtering to \"Alpha\", got %d", len(visible))
	}
	item, ok := visible[0].(skillItem)
	if !ok || item.skill.Name != "Alpha" {
		t.Errorf("expected Alpha to remain, got %+v", visible)
	}
}

func TestModel_UpdateKeyShowsNotImplementedStatus(t *testing.T) {
	srcs := twoSkillSources(t)
	m := newTestModel(t, 120, 40, srcs)

	pressKey(m, "u")
	if !strings.Contains(m.statusMsg, "not implemented") {
		t.Errorf("statusMsg = %q, want a not-implemented notice", m.statusMsg)
	}
}
