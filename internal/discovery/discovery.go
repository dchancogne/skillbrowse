// Package discovery walks source roots to find skill candidates under
// bounded depth and concurrency, with cancellation support, per
// docs/superpowers/specs/2026-08-12-skillbrowse-design.md §5.3.
package discovery

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/dchancogne/skillbrowse/internal/sources"
)

// SkillFileName is the exact, case-sensitive file name that marks a
// directory as a skill candidate.
const SkillFileName = "SKILL.md"

// maxWorkers bounds how many source roots are scanned concurrently.
const maxWorkers = 8

var ignoredDirNames = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
}

// Candidate is a directory found to contain a SKILL.md file, attributed to
// the source it was discovered under.
type Candidate struct {
	Source        sources.Source
	ObservedPath  string
	CanonicalPath string
	SkillFilePath string
}

// Diagnostic reports a source root, or a directory within it, that could
// not be scanned.
type Diagnostic struct {
	SourceLabel string
	Path        string
	Message     string
}

// Result is the aggregate output of scanning every requested source.
type Result struct {
	Candidates  []Candidate
	Diagnostics []Diagnostic
}

// Scan walks every source concurrently (bounded by maxWorkers) and
// aggregates their candidates and diagnostics. It respects ctx
// cancellation: an in-progress scan stops issuing new filesystem reads
// once ctx is done, returning whatever was collected so far.
func Scan(ctx context.Context, srcs []sources.Source) Result {
	type partial struct {
		candidates  []Candidate
		diagnostics []Diagnostic
	}

	jobs := make(chan sources.Source)
	results := make(chan partial)

	var wg sync.WaitGroup
	workers := maxWorkers
	if workers > len(srcs) {
		workers = len(srcs)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for src := range jobs {
				c, d := scanSource(ctx, src)
				results <- partial{candidates: c, diagnostics: d}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, src := range srcs {
			select {
			case jobs <- src:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var out Result
	for p := range results {
		out.Candidates = append(out.Candidates, p.candidates...)
		out.Diagnostics = append(out.Diagnostics, p.diagnostics...)
	}

	sort.Slice(out.Candidates, func(i, j int) bool {
		return out.Candidates[i].CanonicalPath < out.Candidates[j].CanonicalPath
	})
	sort.Slice(out.Diagnostics, func(i, j int) bool {
		return out.Diagnostics[i].Path < out.Diagnostics[j].Path
	})

	return out
}

func scanSource(ctx context.Context, src sources.Source) ([]Candidate, []Diagnostic) {
	if _, err := os.Lstat(src.Root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []Diagnostic{{SourceLabel: src.Label, Path: src.Root, Message: err.Error()}}
	}

	// Always resolve through EvalSymlinks, not just when the root itself
	// is a symlink: an ancestor path component can be a symlink too
	// (e.g. macOS's /var -> /private/var), which must be canonicalized
	// for candidates reached via this root to dedup correctly against
	// candidates reached via a symlinked path elsewhere.
	root, err := filepath.EvalSymlinks(src.Root)
	if err != nil {
		return nil, []Diagnostic{{SourceLabel: src.Label, Path: src.Root, Message: err.Error()}}
	}

	w := &walker{ctx: ctx, src: src, visited: map[string]bool{}}
	w.walk(root, 0)
	return w.candidates, w.diagnostics
}

type walker struct {
	ctx         context.Context
	src         sources.Source
	visited     map[string]bool
	candidates  []Candidate
	diagnostics []Diagnostic
}

func (w *walker) walk(dir string, depth int) {
	select {
	case <-w.ctx.Done():
		return
	default:
	}

	if w.visited[dir] {
		return
	}
	w.visited[dir] = true

	if hasSkillFile(dir) {
		w.candidates = append(w.candidates, Candidate{
			Source:        w.src,
			ObservedPath:  dir,
			CanonicalPath: dir,
			SkillFilePath: filepath.Join(dir, SkillFileName),
		})
	}

	if depth >= w.src.MaxDepth {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		w.diagnostics = append(w.diagnostics, Diagnostic{SourceLabel: w.src.Label, Path: dir, Message: err.Error()})
		return
	}

	names := make([]string, len(entries))
	byName := make(map[string]os.DirEntry, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
		byName[e.Name()] = e
	}
	sort.Strings(names)

	for _, name := range names {
		select {
		case <-w.ctx.Done():
			return
		default:
		}

		entry := byName[name]
		full := filepath.Join(dir, name)

		if entry.Type()&os.ModeSymlink != 0 {
			w.acceptShallowSymlink(full)
			continue
		}

		if !entry.IsDir() || ignoredDirNames[name] {
			continue
		}

		w.walk(full, depth+1)
	}
}

// acceptShallowSymlink implements the "explicit symlink targets" rule:
// a symlink whose immediate target is a directory containing SKILL.md is
// accepted as a candidate, but its target is never itself walked.
func (w *walker) acceptShallowSymlink(link string) {
	target, err := filepath.EvalSymlinks(link)
	if err != nil {
		return
	}
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		return
	}
	if !hasSkillFile(target) {
		return
	}
	w.candidates = append(w.candidates, Candidate{
		Source:        w.src,
		ObservedPath:  link,
		CanonicalPath: target,
		SkillFilePath: filepath.Join(target, SkillFileName),
	})
}

func hasSkillFile(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, SkillFileName))
	return err == nil && !info.IsDir()
}
