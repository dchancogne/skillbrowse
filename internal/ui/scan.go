package ui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/dchancogne/skillbrowse/internal/catalog"
	"github.com/dchancogne/skillbrowse/internal/discovery"
	"github.com/dchancogne/skillbrowse/internal/sources"
)

// scanResultMsg carries a completed scan's output back into Update.
type scanResultMsg struct {
	result  discovery.Result
	catalog catalog.Catalog
}

// scanCmd runs the discovery -> catalog pipeline in the background. It
// respects ctx cancellation so a quit or a subsequent rescan can abandon
// in-flight filesystem work (design doc NFR-07).
func scanCmd(ctx context.Context, srcs []sources.Source) tea.Cmd {
	return func() tea.Msg {
		result := discovery.Scan(ctx, srcs)
		cat := catalog.BuildFromCandidates(result.Candidates)
		return scanResultMsg{result: result, catalog: cat}
	}
}
