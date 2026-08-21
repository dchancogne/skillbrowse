package ui

import (
	"bytes"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"

	"github.com/dchancogne/skillbrowse/internal/catalog"
)

func TestWrapText_FitsOnOneLine(t *testing.T) {
	got := wrapText("Short description.", 40, maxDescLines)
	if len(got) != 1 || got[0] != "Short description." {
		t.Errorf("got %q, want a single unchanged line", got)
	}
}

func TestWrapText_WrapsOntoAllowedLines(t *testing.T) {
	got := wrapText("one two three four five six seven eight", 15, 2)
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(got), got)
	}
	for _, l := range got {
		if lipgloss.Width(l) > 15 {
			t.Errorf("line %q exceeds width 15", l)
		}
	}
}

func TestWrapText_OverflowGetsEllipsisNotSilentlyDropped(t *testing.T) {
	// Long enough that a naive wrap produces more than maxDescLines
	// lines, but where the *last visible* wrapped line individually
	// still fits within width (so a bug that only truncates when the
	// line itself is too long would miss this case entirely).
	got := wrapText("aaaa bbbb cccc dddd eeee ffff gggg hhhh", 10, 2)
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(got), got)
	}
	last := got[len(got)-1]
	if !strings.HasSuffix(last, "…") {
		t.Errorf("expected last line to end with an ellipsis marking hidden text, got %q", last)
	}
	if lipgloss.Width(last) > 10 {
		t.Errorf("last line %q exceeds width 10 even with ellipsis", last)
	}
}

func TestWrapText_PaddingKeepsFixedLineCount(t *testing.T) {
	// Render (not exercised directly here) always pads to maxDescLines
	// itself; wrapText's contract is just to never return more lines
	// than requested.
	got := wrapText("", 40, maxDescLines)
	if len(got) != 0 {
		t.Errorf("empty description: got %q, want no lines", got)
	}
}

func renderSkillItem(t *testing.T, s catalog.Skill) string {
	t.Helper()
	d := newSkillDelegate()
	items := itemsFromSkills([]catalog.Skill{s}, "")
	m := list.New(items, d, 80, 10)

	var buf bytes.Buffer
	d.Render(&buf, m, 0, items[0])
	return buf.String()
}

func TestRender_TagsProjectScopedSkills(t *testing.T) {
	got := renderSkillItem(t, catalog.Skill{Name: "demo", ProjectScoped: true})
	if !strings.Contains(got, "[project]") {
		t.Errorf("expected project-scoped skill's row to contain a [project] tag, got %q", got)
	}
}

func TestRender_OmitsProjectTagForGlobalSkills(t *testing.T) {
	got := renderSkillItem(t, catalog.Skill{Name: "demo", ProjectScoped: false})
	if strings.Contains(got, "[project]") {
		t.Errorf("expected global skill's row to omit the [project] tag, got %q", got)
	}
}
