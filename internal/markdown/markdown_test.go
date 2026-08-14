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

func TestRender_SubheadingsDropLiteralHashPrefixInColor(t *testing.T) {
	for _, dark := range []bool{true, false} {
		out, err := Render("## Subheading\n\nBody.\n", 80, dark, false)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, "##") {
			t.Errorf("dark=%v: expected no literal '##' in colored output, got %q", dark, out)
		}
		if !strings.Contains(out, "Subheading") {
			t.Errorf("dark=%v: expected heading text preserved, got %q", dark, out)
		}
	}
}

func TestRender_NoColorKeepsHashPrefixForSubheadings(t *testing.T) {
	out, err := Render("## Subheading\n\nBody.\n", 80, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## Subheading") {
		t.Errorf("expected literal '## ' prefix preserved with no color (only way to convey heading level as plain text), got %q", out)
	}
}

func TestRender_FencedCodeBlockGetsABorderedBox(t *testing.T) {
	content := "Intro.\n\n```go\nfunc main() {}\n```\n\nOutro.\n"
	out, err := Render(content, 60, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "┌") || !strings.Contains(out, "└") {
		t.Errorf("expected a border box around the code block, got %q", out)
	}
	if !strings.Contains(out, "main") {
		t.Errorf("expected code content preserved, got %q", out)
	}
	if strings.Contains(out, "```") {
		t.Errorf("expected fence markers consumed by rendering, got %q", out)
	}
	if strings.Contains(out, "CODEBLOCK") {
		t.Errorf("expected no leaked sentinel placeholder, got %q", out)
	}
	if !strings.Contains(out, "Intro.") || !strings.Contains(out, "Outro.") {
		t.Errorf("expected surrounding prose preserved, got %q", out)
	}
}

func TestRender_MultipleCodeBlocksEachGetOwnBox(t *testing.T) {
	content := "```go\nfirst\n```\n\ntext\n\n```python\nsecond\n```\n"
	out, err := Render(content, 60, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "first") || !strings.Contains(out, "second") {
		t.Errorf("expected both code blocks preserved, got %q", out)
	}
	if strings.Count(out, "┌") != 2 {
		t.Errorf("expected 2 boxes, got %d in %q", strings.Count(out, "┌"), out)
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
