package ui

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dchancogne/skillbrowse/internal/catalog"
	"github.com/dchancogne/skillbrowse/internal/discovery"
	"github.com/dchancogne/skillbrowse/internal/sources"
	"github.com/dchancogne/skillbrowse/internal/update"
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

func TestModel_ListShowsVersionPathAgentsAndSource(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "alpha"), "---\nname: Alpha\ndescription: First skill.\nversion: 1.2.3\n---\nBody.\n")
	srcs := []sources.Source{{Label: "My Source", Agents: []string{"Claude Code"}, Root: root, MaxDepth: 1}}

	// Wide enough that the list pane doesn't truncate before "Source:
	// My Source" and the tail of the (deep, OS-temp-dir-based) path.
	m := newTestModel(t, 400, 40, srcs)
	view := stripANSI(m.View().Content)

	for _, want := range []string{"Alpha", "v1.2.3", "First skill.", "Agents: Claude Code", "Source: My Source", "alpha"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q: %q", want, view)
		}
	}
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

func TestModel_HelpOverlayShowsSourceDiagnostics(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires enforceable Unix permission bits")
	}

	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "alpha"), "---\nname: Alpha\n---\nBody.\n")
	blocked := filepath.Join(root, "blocked")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(blocked, 0o755); err != nil {
			t.Logf("cleanup: chmod %s: %s", blocked, err)
		}
	})

	// MaxDepth 2, not 1: the walker only attempts to read a directory's
	// contents when it intends to recurse further, so an unreadable
	// directory sitting exactly at MaxDepth would never be listed at all.
	srcs := []sources.Source{{Label: "Test", Agents: []string{"Test"}, Root: root, MaxDepth: 2}}
	m := newTestModel(t, 120, 40, srcs)

	if len(m.sourceDiagnostics) != 1 {
		t.Fatalf("expected 1 source diagnostic, got %+v", m.sourceDiagnostics)
	}

	pressKey(m, "?")
	help := stripANSI(m.View().Content)
	if !strings.Contains(help, "Source diagnostics:") {
		t.Errorf("help view missing diagnostics section: %q", help)
	}
	if !strings.Contains(help, "blocked") {
		t.Errorf("help view missing diagnostic path: %q", help)
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

// updateFixtureServer is a minimal fake GitHub release server, just
// enough to exercise the "u" key's check/confirm/install state machine.
// internal/update/apply_test.go covers the full verification matrix
// (bad signature, bad checksum, etc.) in detail; this only needs to
// prove the UI wiring reacts correctly to CheckForUpdate/Apply results.
func updateFixtureServer(t *testing.T, tag, version string) *httptest.Server {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	assetName := update.AssetName(runtime.GOOS, runtime.GOARCH)

	var archiveBuf bytes.Buffer
	gz := gzip.NewWriter(&archiveBuf)
	tw := tar.NewWriter(gz)
	content := []byte("#!/bin/sh\necho \"skillbrowse " + version + "\"\n")
	if err := tw.WriteHeader(&tar.Header{Name: "skillbrowse", Size: int64(len(content)), Mode: 0o755}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(archiveBuf.Bytes())
	checksums := []byte(hex.EncodeToString(sum[:]) + "  " + assetName + "\n")
	sig := ed25519.Sign(priv, checksums)

	assets := map[string][]byte{
		assetName:                 archiveBuf.Bytes(),
		update.ChecksumsAssetName: checksums,
		update.SignatureAssetName: sig,
	}

	var srv *httptest.Server // referenced by the closure below; assigned once the server exists

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/dchancogne/skillbrowse/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		release := update.Release{TagName: tag, HTMLURL: "https://example.invalid/" + tag}
		for name, data := range assets {
			release.Assets = append(release.Assets, update.Asset{Name: name, BrowserDownloadURL: srv.URL + "/assets/" + name, Size: int64(len(data))})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release)
	})
	mux.HandleFunc("/assets/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(assets[r.URL.Path[len("/assets/"):]])
	})

	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv
}

func TestModel_UpdateKey_NoUpdateAvailable(t *testing.T) {
	srcs := twoSkillSources(t)
	m := newTestModel(t, 120, 40, srcs)
	srv := updateFixtureServer(t, "v1.0.0", "1.0.0")
	m.updateClient.APIBaseURL = srv.URL
	m.currentVersion = "1.0.0"

	drain(m, pressKey(m, "u"))
	if !strings.Contains(m.statusMsg, "up to date") {
		t.Errorf("statusMsg = %q, want an up-to-date notice", m.statusMsg)
	}
}

func TestModel_UpdateKey_FindsUpdateAndConfirms(t *testing.T) {
	srcs := twoSkillSources(t)
	m := newTestModel(t, 120, 40, srcs)
	srv := updateFixtureServer(t, "v1.2.0", "1.2.0")
	m.updateClient.APIBaseURL = srv.URL
	m.currentVersion = "1.0.0"

	drain(m, pressKey(m, "u"))
	if !m.updateConfirm {
		t.Fatalf("expected updateConfirm = true after finding an update, statusMsg = %q", m.statusMsg)
	}
	if !strings.Contains(m.statusMsg, "1.0.0") || !strings.Contains(m.statusMsg, "1.2.0") {
		t.Errorf("statusMsg = %q, want it to mention both versions", m.statusMsg)
	}

	// Dismissing with any key other than "y" cancels without installing.
	pressKey(m, "n")
	if m.updateConfirm {
		t.Error("expected updateConfirm = false after dismissing")
	}
	if !strings.Contains(m.statusMsg, "cancelled") {
		t.Errorf("statusMsg = %q, want a cancellation notice", m.statusMsg)
	}
}

func TestModel_UpdateKey_ConfirmInstallSurfacesVerificationFailure(t *testing.T) {
	srcs := twoSkillSources(t)
	m := newTestModel(t, 120, 40, srcs)
	srv := updateFixtureServer(t, "v1.2.0", "1.2.0")
	m.updateClient.APIBaseURL = srv.URL
	m.currentVersion = "1.0.0"

	drain(m, pressKey(m, "u"))
	if !m.updateConfirm {
		t.Fatalf("expected updateConfirm = true, statusMsg = %q", m.statusMsg)
	}

	// Pressing "y" drives the real applyUpdateCmd, which uses
	// update.DefaultVerifier() — empty until Phase 6 embeds a real
	// signing key. That failure should surface as a status message, not
	// a crash: this confirms the "y" confirmation path is wired to a
	// real install attempt.
	drain(m, pressKey(m, "y"))
	if m.updateInstalling {
		t.Error("expected updateInstalling = false once the (failed) install completed")
	}
	if !strings.Contains(m.statusMsg, "Update failed") {
		t.Errorf("statusMsg = %q, want an install failure notice", m.statusMsg)
	}
}
