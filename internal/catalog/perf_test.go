package catalog_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/dchancogne/skillbrowse/internal/benchfixture"
	"github.com/dchancogne/skillbrowse/internal/catalog"
	"github.com/dchancogne/skillbrowse/internal/discovery"
	"github.com/dchancogne/skillbrowse/internal/sources"
)

// benchBodySize approximates a realistic SKILL.md body; chosen so that
// 10,000 skills total roughly 2 MB of Markdown content, well inside the
// NFR-04 budget (10,000 skills, <=50 MiB of loaded Markdown, <100 MiB
// total memory).
const benchBodySize = 200

func benchmarkPipeline(b *testing.B, n int) {
	b.Helper()
	root := b.TempDir()
	if err := benchfixture.Generate(root, n, benchBodySize); err != nil {
		b.Fatal(err)
	}
	srcs := []sources.Source{{Label: "Bench", Agents: []string{"Bench"}, Root: root, MaxDepth: 1}}

	b.Logf("env: go=%s os=%s arch=%s skills=%d", runtime.Version(), runtime.GOOS, runtime.GOARCH, n)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result := discovery.Scan(context.Background(), srcs)
		cat := catalog.BuildFromCandidates(result.Candidates)
		if len(cat.Skills) != n {
			b.Fatalf("expected %d skills, got %d", n, len(cat.Skills))
		}
	}
}

// BenchmarkPipeline_1000Skills and BenchmarkPipeline_10000Skills measure
// the discovery -> catalog pipeline against design doc §10's NFR-02
// ("Make a 1,000-skill local SSD fixture usable within 500 ms at p95")
// scale points. Run with `go test ./internal/catalog/ -bench Pipeline
// -benchmem` and record the reported ns/op alongside the environment
// line each benchmark logs (Go version, OS, arch, filesystem type,
// terminal dimensions aren't applicable here — those apply to the UI
// benchmarks in internal/ui).
func BenchmarkPipeline_1000Skills(b *testing.B)  { benchmarkPipeline(b, 1000) }
func BenchmarkPipeline_10000Skills(b *testing.B) { benchmarkPipeline(b, 10000) }

// TestPipeline_1000SkillsRegressionGuard is a coarse smoke test, not a
// strict measurement of NFR-02's 500 ms/p95 target: shared CI hardware
// is noisier than "a 2022-era laptop with an SSD," so this asserts a
// generously loose bound (5s) purely to catch gross regressions (e.g. an
// accidentally quadratic algorithm), while BenchmarkPipeline_1000Skills
// above is the actual instrument for tracking the real NFR-02 number
// over time on controlled hardware.
func TestPipeline_1000SkillsRegressionGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in -short mode")
	}

	root := t.TempDir()
	if err := benchfixture.Generate(root, 1000, benchBodySize); err != nil {
		t.Fatal(err)
	}
	srcs := []sources.Source{{Label: "Bench", Agents: []string{"Bench"}, Root: root, MaxDepth: 1}}

	start := time.Now()
	result := discovery.Scan(context.Background(), srcs)
	cat := catalog.BuildFromCandidates(result.Candidates)
	elapsed := time.Since(start)

	if len(cat.Skills) != 1000 {
		t.Fatalf("expected 1000 skills, got %d", len(cat.Skills))
	}
	t.Logf("1000-skill pipeline: %s", elapsed)
	if elapsed > 5*time.Second {
		t.Errorf("pipeline took %s, want well under 500ms on real hardware (regression guard threshold: 5s)", elapsed)
	}
}

// TestCatalog_MemoryBudget checks NFR-04: "Stay below 100 MiB for
// 10,000 skills whose loaded Markdown totals no more than 50 MiB." It
// measures retained heap after a full GC, not cumulative allocation
// during the build (which is a different, larger number dominated by
// short-lived garbage — see BenchmarkPipeline_10000Skills' B/op for
// that figure instead).
func TestCatalog_MemoryBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in -short mode")
	}

	const n = 10000
	const bodySize = 5000 // 10,000 * 5,000 B = ~50 MiB of Markdown content

	root := t.TempDir()
	if err := benchfixture.Generate(root, n, bodySize); err != nil {
		t.Fatal(err)
	}
	srcs := []sources.Source{{Label: "Bench", Agents: []string{"Bench"}, Root: root, MaxDepth: 1}}

	result := discovery.Scan(context.Background(), srcs)

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	cat := catalog.BuildFromCandidates(result.Candidates)
	if len(cat.Skills) != n {
		t.Fatalf("expected %d skills, got %d", n, len(cat.Skills))
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	retainedMiB := float64(after.HeapAlloc-before.HeapAlloc) / (1 << 20)
	t.Logf("retained heap for %d skills (~%d MiB Markdown): %.1f MiB", n, (n*bodySize)>>20, retainedMiB)

	const budgetMiB = 100
	if retainedMiB > budgetMiB {
		t.Errorf("retained %.1f MiB, want under %d MiB (NFR-04)", retainedMiB, budgetMiB)
	}

	runtime.KeepAlive(cat)
}
