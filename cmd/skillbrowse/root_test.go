package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dchancogne/skillbrowse/internal/buildinfo"
)

func TestExitCodeFor(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil-shaped usage error", &usageError{err: errors.New("bad config")}, 2},
		{"wrapped usage error", fmt.Errorf("context: %w", &usageError{err: errors.New("bad config")}), 2},
		{"cobra unknown command", errors.New(`unknown command "foo" for "skillbrowse"`), 2},
		{"cobra unknown flag", errors.New("unknown flag: --bogus"), 2},
		{"cobra wrong arg count", errors.New("accepts at most 1 arg(s), received 2"), 2},
		{"plain operational error", errors.New("network unreachable"), 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := exitCodeFor(c.err); got != c.want {
				t.Errorf("exitCodeFor(%v) = %d, want %d", c.err, got, c.want)
			}
		})
	}
}

func TestVersionCommand_PrintsAllFields(t *testing.T) {
	cmd := newVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	info := buildinfo.Get()
	for _, want := range []string{info.Version, info.Commit, info.Date, info.GoVersion, info.OS, info.Arch} {
		if !strings.Contains(out, want) {
			t.Errorf("version output missing %q: %q", want, out)
		}
	}
	if !strings.HasPrefix(out, "skillbrowse "+info.Version) {
		t.Errorf("version output should start with %q, got %q", "skillbrowse "+info.Version, out)
	}
}

func TestRun_UnknownCommandExitsTwo(t *testing.T) {
	if got := run([]string{"notacommand"}); got != 2 {
		t.Errorf("run([notacommand]) = %d, want 2", got)
	}
}

func TestRun_UnknownFlagExitsTwo(t *testing.T) {
	if got := run([]string{"--bogus-flag"}); got != 2 {
		t.Errorf("run([--bogus-flag]) = %d, want 2", got)
	}
}

func TestRun_MalformedConfigExitsTwo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("version = 2"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Fails during config.Load, before the TUI would ever start.
	if got := run([]string{"--config", path, "--no-defaults"}); got != 2 {
		t.Errorf("run with malformed config = %d, want 2", got)
	}
}

func TestRun_VersionExitsZero(t *testing.T) {
	if got := run([]string{"version"}); got != 0 {
		t.Errorf("run([version]) = %d, want 0", got)
	}
}
