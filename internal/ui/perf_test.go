package ui

import (
	"context"
	"runtime"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dchancogne/skillbrowse/internal/benchfixture"
	"github.com/dchancogne/skillbrowse/internal/catalog"
	"github.com/dchancogne/skillbrowse/internal/discovery"
	"github.com/dchancogne/skillbrowse/internal/sources"
)

// loadedModel builds a Model with n synthetic skills already scanned in,
// for benchmarking against a realistically sized catalog rather than the
// two-item fixtures model_test.go uses for correctness tests.
func loadedModel(tb testing.TB, n int) *Model {
	tb.Helper()
	root := tb.TempDir()
	if err := benchfixture.Generate(root, n, 200); err != nil {
		tb.Fatal(err)
	}
	srcs := []sources.Source{{Label: "Bench", Agents: []string{"Bench"}, Root: root, MaxDepth: 1}}

	m := New(srcs, WithNoColor(true))
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	result := discovery.Scan(context.Background(), srcs)
	cat := catalog.BuildFromCandidates(result.Candidates)
	if len(cat.Skills) != n {
		tb.Fatalf("expected %d skills, got %d", n, len(cat.Skills))
	}
	if _, cmd := m.Update(scanResultMsg{result: result, catalog: cat}); cmd != nil {
		cmd()
	}
	return m
}

// BenchmarkModel_View_BeforeScan measures rendering the initial
// scanning-indicator frame, per design doc §10 NFR-01 ("Render the frame
// or scan indicator within 100 ms at p95") — this is what's on screen
// before the async scan Cmd completes, so it must be cheap regardless of
// how large the eventual catalog is.
func BenchmarkModel_View_BeforeScan(b *testing.B) {
	m := New(nil, WithNoColor(true))
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	b.Logf("env: go=%s os=%s arch=%s terminal=120x40", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

// BenchmarkModel_View_Loaded measures steady-state rendering (both list
// and detail panes) with a realistically sized catalog loaded.
func BenchmarkModel_View_Loaded(b *testing.B) {
	m := loadedModel(b, 1000)

	b.Logf("env: go=%s os=%s arch=%s terminal=120x40 skills=1000", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

// BenchmarkModel_Navigation measures per-keystroke cost of moving the
// list selection, per design doc §10 NFR-03 ("Navigation... render
// within 50 ms at p95").
func BenchmarkModel_Navigation(b *testing.B) {
	m := loadedModel(b, 1000)
	down := tea.KeyPressMsg{Text: "j", Code: 'j'}

	b.Logf("env: go=%s os=%s arch=%s terminal=120x40 skills=1000", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Update(down)
		_ = m.View()
	}
}

// BenchmarkModel_Filter measures per-keystroke cost while typing into
// the fuzzy search box, per design doc §10 NFR-03 ("filtering render
// within 50 ms at p95").
func BenchmarkModel_Filter(b *testing.B) {
	m := loadedModel(b, 1000)
	m.Update(tea.KeyPressMsg{Text: "/", Code: '/'})

	b.Logf("env: go=%s os=%s arch=%s terminal=120x40 skills=1000", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, cmd := m.Update(tea.KeyPressMsg{Text: "5", Code: '5'})
		if cmd != nil {
			m.Update(cmd())
		}
		_ = m.View()
	}
}
