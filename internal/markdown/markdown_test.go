package markdown

import (
	"strings"
	"testing"
)

func TestRender_BasicMarkdown(t *testing.T) {
	out, err := Render("# Title\n\nBody text.\n", 80, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Title") || !strings.Contains(out, "Body text.") {
		t.Errorf("rendered output missing expected text: %q", out)
	}
}

func TestRender_NoColorProducesNoANSI(t *testing.T) {
	out, err := Render("# Title\n\n**bold** text.\n", 80, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("expected no ANSI escape codes with noColor, got %q", out)
	}
}

func TestSanitize_StripsControlCharsKeepsTabsAndNewlines(t *testing.T) {
	in := "safe\ttext\nwith\x1b[31mescape\x07and\x00nulls"
	out := sanitize(in)
	if strings.ContainsAny(out, "\x1b\x07\x00") {
		t.Errorf("expected control characters stripped, got %q", out)
	}
	if !strings.Contains(out, "\t") || !strings.Contains(out, "\n") {
		t.Errorf("expected tabs and newlines preserved, got %q", out)
	}
}

func TestCache_ReturnsMemoizedRendering(t *testing.T) {
	c := NewCache()
	content := "# Cached\n\nHello.\n"

	first, err := c.Render(content, 80, true, true)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Render(content, 80, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("expected identical cached output, got %q vs %q", first, second)
	}
}

func TestCache_DifferentWidthsAreCachedSeparately(t *testing.T) {
	c := NewCache()
	content := "A reasonably long paragraph of text that will wrap differently depending on the configured terminal width, so this test can distinguish the two renderings.\n"

	narrow, err := c.Render(content, 20, true, true)
	if err != nil {
		t.Fatal(err)
	}
	wide, err := c.Render(content, 100, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if narrow == wide {
		t.Error("expected different renderings for different widths")
	}
}
