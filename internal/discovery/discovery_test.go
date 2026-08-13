package discovery

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dchancogne/skillbrowse/internal/sources"
)

func mkSkill(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, SkillFileName), []byte("---\nname: x\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func source(root string, maxDepth int) sources.Source {
	return sources.Source{Label: "Test", Agents: []string{"Test"}, Root: root, MaxDepth: maxDepth}
}

func TestScan_MissingRootIsSilent(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist")
	res := Scan(context.Background(), []sources.Source{source(root, 4)})
	if len(res.Candidates) != 0 || len(res.Diagnostics) != 0 {
		t.Fatalf("expected no candidates or diagnostics, got %+v", res)
	}
}

func TestScan_FindsDirectSkillDirectories(t *testing.T) {
	root := t.TempDir()
	mkSkill(t, filepath.Join(root, "skill-a"))
	mkSkill(t, filepath.Join(root, "skill-b"))

	res := Scan(context.Background(), []sources.Source{source(root, 1)})
	if len(res.Candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d: %+v", len(res.Candidates), res.Candidates)
	}
}

func TestScan_RespectsMaxDepth(t *testing.T) {
	root := t.TempDir()
	mkSkill(t, filepath.Join(root, "a", "b", "c"))

	shallow := Scan(context.Background(), []sources.Source{source(root, 2)})
	if len(shallow.Candidates) != 0 {
		t.Fatalf("depth 2 should not reach a skill 3 levels deep, got %+v", shallow.Candidates)
	}

	deep := Scan(context.Background(), []sources.Source{source(root, 3)})
	if len(deep.Candidates) != 1 {
		t.Fatalf("depth 3 should reach the skill, got %+v", deep.Candidates)
	}
}

func TestScan_IgnoresGitNodeModulesVendor(t *testing.T) {
	root := t.TempDir()
	mkSkill(t, filepath.Join(root, ".git", "skill"))
	mkSkill(t, filepath.Join(root, "node_modules", "skill"))
	mkSkill(t, filepath.Join(root, "vendor", "skill"))
	mkSkill(t, filepath.Join(root, "real-skill"))

	res := Scan(context.Background(), []sources.Source{source(root, 4)})
	if len(res.Candidates) != 1 {
		t.Fatalf("expected only the non-ignored skill, got %+v", res.Candidates)
	}
	if filepath.Base(res.Candidates[0].CanonicalPath) != "real-skill" {
		t.Errorf("got %q", res.Candidates[0].CanonicalPath)
	}
}

func TestScan_AcceptsShallowSymlinkButDoesNotRecurseIntoIt(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	mkSkill(t, target)
	mkSkill(t, filepath.Join(target, "nested-skill")) // should NOT be found: no recursion into symlink targets

	if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}

	res := Scan(context.Background(), []sources.Source{source(root, 1)})
	if len(res.Candidates) != 1 {
		t.Fatalf("expected exactly 1 candidate (the symlink target itself), got %+v", res.Candidates)
	}
	if res.Candidates[0].CanonicalPath != resolvedTarget {
		t.Errorf("CanonicalPath = %q, want %q", res.Candidates[0].CanonicalPath, resolvedTarget)
	}
	if want := filepath.Join(resolvedRoot, "link"); res.Candidates[0].ObservedPath != want {
		t.Errorf("ObservedPath = %q, want %q", res.Candidates[0].ObservedPath, want)
	}
}

func TestScan_ResolvesSymlinkRootAndRecursesNormally(t *testing.T) {
	real := t.TempDir()
	mkSkill(t, filepath.Join(real, "skill"))

	linkRoot := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(real, linkRoot); err != nil {
		t.Fatal(err)
	}

	res := Scan(context.Background(), []sources.Source{source(linkRoot, 1)})
	if len(res.Candidates) != 1 {
		t.Fatalf("expected 1 candidate through resolved root, got %+v", res.Candidates)
	}
}

func TestScan_UnreadableDirectoryProducesDiagnostic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits not meaningful on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission checks")
	}

	root := t.TempDir()
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

	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(resolvedRoot, "blocked")

	res := Scan(context.Background(), []sources.Source{source(root, 4)})
	if len(res.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %+v", res.Diagnostics)
	}
	if res.Diagnostics[0].Path != wantPath {
		t.Errorf("diagnostic path = %q, want %q", res.Diagnostics[0].Path, wantPath)
	}
}

func TestScan_CancelledContextReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	mkSkill(t, root)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := Scan(ctx, []sources.Source{source(root, 4)})
	if len(res.Candidates) != 0 {
		t.Fatalf("expected no candidates once cancelled, got %+v", res.Candidates)
	}
}

func TestScan_MultipleSourcesRunConcurrentlyAndAggregate(t *testing.T) {
	var srcs []sources.Source
	for i := 0; i < 5; i++ {
		root := t.TempDir()
		mkSkill(t, root)
		srcs = append(srcs, source(root, 1))
	}

	done := make(chan Result, 1)
	go func() { done <- Scan(context.Background(), srcs) }()

	select {
	case res := <-done:
		if len(res.Candidates) != 5 {
			t.Fatalf("expected 5 candidates across sources, got %d", len(res.Candidates))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Scan did not complete in time")
	}
}
