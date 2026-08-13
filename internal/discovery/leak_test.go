package discovery

import (
	"context"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/dchancogne/skillbrowse/internal/sources"
)

// TestScan_NoGoroutineLeakOnCancellation is the NFR-07 check from design
// doc §10 ("Quit and rescan cancel obsolete work without goroutine
// leaks"): Scan's worker pool must not leave any goroutine running after
// its context is cancelled mid-scan.
func TestScan_NoGoroutineLeakOnCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Enough sources, each deep/wide enough, that cancellation is very
	// likely to land mid-walk rather than after Scan has already
	// finished on its own.
	var srcs []sources.Source
	for i := 0; i < maxWorkers*2; i++ {
		root := t.TempDir()
		for d := 0; d < 6; d++ {
			mkSkill(t, joinDepth(root, d))
		}
		srcs = append(srcs, sources.Source{Label: "Leak", Agents: []string{"Leak"}, Root: root, MaxDepth: 8})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Millisecond)
	defer cancel()

	Scan(ctx, srcs) // may return partial results; that's fine, only the leak matters here
}

// TestScan_NoGoroutineLeakOnNormalCompletion is the same check for the
// ordinary (uncancelled) path, so a regression that only cleans up on
// the cancellation branch would still be caught.
func TestScan_NoGoroutineLeakOnNormalCompletion(t *testing.T) {
	defer goleak.VerifyNone(t)

	var srcs []sources.Source
	for i := 0; i < maxWorkers*2; i++ {
		root := t.TempDir()
		mkSkill(t, root)
		srcs = append(srcs, sources.Source{Label: "Leak", Agents: []string{"Leak"}, Root: root, MaxDepth: 1})
	}

	Scan(context.Background(), srcs)
}

func joinDepth(root string, depth int) string {
	path := root
	for i := 0; i < depth; i++ {
		path += "/level"
	}
	return path
}
