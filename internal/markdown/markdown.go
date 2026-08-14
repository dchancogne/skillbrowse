// Package markdown wraps Glamour rendering with sanitization and a
// width-aware cache, per
// docs/superpowers/specs/2026-08-12-skillbrowse-design.md §11 and §13.
package markdown

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
)

// Render sanitizes content and renders it as Markdown at the given
// terminal width. noColor uses Glamour's stock "notty" style, which
// emits no ANSI codes at all (honoring NO_COLOR/--no-color) and so must
// keep "#"-prefixed headings as the only way to convey heading level in
// plain text; otherwise dark or light selects a heading-decluttered
// variant of Glamour's standard dark/light styles (see headingStyle), and
// fenced code blocks are boxed with a light border (see renderCodeBox).
func Render(content string, width int, dark, noColor bool) (string, error) {
	sanitized := sanitize(content)

	if noColor {
		r, err := glamour.NewTermRenderer(glamour.WithStandardStyle("notty"), glamour.WithWordWrap(width))
		if err != nil {
			return "", err
		}
		defer func() { _ = r.Close() }()
		return r.Render(sanitized)
	}

	blocks, rewritten := extractCodeBlocks(sanitized)

	r, err := glamour.NewTermRenderer(glamour.WithStyles(headingStyle(dark)), glamour.WithWordWrap(width))
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()
	out, err := r.Render(rewritten)
	if err != nil {
		return "", err
	}
	return injectCodeBoxes(out, blocks, dark, width)
}

// codeFenceRe matches a top-level (unindented) fenced code block, per
// commonmark's "```lang\n...\n```" form.
var codeFenceRe = regexp.MustCompile("(?ms)^```([^\n]*)\n(.*?)\n```[ \t]*$")

type codeBlock struct {
	lang, code string
}

// codeBlockSentinel is an invisible, syntax-inert marker (Unicode
// U+2063 INVISIBLE SEPARATOR, well above sanitize's stripped control-char
// range) that stands in for a fenced code block through the main
// Markdown render, so it comes back out as its own paragraph line that
// injectCodeBoxes can find and replace with the boxed rendering.
const codeBlockSentinel = "\u2063CODEBLOCK%d\u2063"

// extractCodeBlocks pulls every top-level fenced code block out of
// content, returning them in order alongside content with each one
// replaced by its sentinel.
func extractCodeBlocks(content string) ([]codeBlock, string) {
	var blocks []codeBlock
	rewritten := codeFenceRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := codeFenceRe.FindStringSubmatch(match)
		blocks = append(blocks, codeBlock{lang: strings.TrimSpace(sub[1]), code: sub[2]})
		return fmt.Sprintf(codeBlockSentinel, len(blocks)-1)
	})
	return blocks, rewritten
}

// codeBoxOverhead is how many columns a boxed code block adds beyond its
// content width: 1 each for the border's left/right edges, 1 each for
// Padding's left/right cells.
const codeBoxOverhead = 4

// injectCodeBoxes replaces each sentinel paragraph left behind by
// extractCodeBlocks with that code block rendered on its own (so it
// keeps Glamour's syntax highlighting) and wrapped in a light border box.
func injectCodeBoxes(rendered string, blocks []codeBlock, dark bool, width int) (string, error) {
	if len(blocks) == 0 {
		return rendered, nil
	}

	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		for idx, blk := range blocks {
			sentinel := fmt.Sprintf(codeBlockSentinel, idx)
			if !strings.Contains(line, sentinel) {
				continue
			}
			boxed, err := renderCodeBox(blk, dark, width)
			if err != nil {
				return "", err
			}
			lines[i] = boxed
			break
		}
	}
	return strings.Join(lines, "\n"), nil
}

// ansiEscapeRe matches SGR escape sequences, used only to test whether a
// line is visually blank (Glamour pads styled lines with trailing
// ANSI-wrapped spaces up to the wrap width, so a plain "" check misses
// them).
var ansiEscapeRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

// trimBlankLines drops leading/trailing lines that are blank once ANSI
// styling is discounted, without disturbing blank lines in the middle.
func trimBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	isBlank := func(l string) bool { return strings.TrimSpace(ansiEscapeRe.ReplaceAllString(l, "")) == "" }

	start := 0
	for start < len(lines) && isBlank(lines[start]) {
		start++
	}
	end := len(lines)
	for end > start && isBlank(lines[end-1]) {
		end--
	}
	return strings.Join(lines[start:end], "\n")
}

var codeBoxBorderColor = lipgloss.Color("240") // dim gray: a "light" (subtle, low-contrast) border

func renderCodeBox(blk codeBlock, dark bool, width int) (string, error) {
	innerWidth := width - codeBoxOverhead
	if innerWidth < 10 {
		innerWidth = 10
	}

	// Margin 0: the lipgloss border box below supplies its own inset, so
	// Glamour's own code-block margin would just double it up.
	cfg := headingStyle(dark)
	cfg.CodeBlock.Margin = uintPtr(0)

	r, err := glamour.NewTermRenderer(glamour.WithStyles(cfg), glamour.WithWordWrap(innerWidth))
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()

	out, err := r.Render("```" + blk.lang + "\n" + blk.code + "\n```\n")
	if err != nil {
		return "", err
	}
	out = trimBlankLines(out)

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(codeBoxBorderColor).
		Padding(0, 1).
		Render(out), nil
}

// headingStyle returns Glamour's standard dark/light style with the H2-H6
// "##"-style literal prefixes removed. Glamour's stock styles keep those
// markers even in color mode (only H1 gets a distinct badge treatment),
// which reads as unrendered Markdown in a terminal that already shows
// color; here heading level is conveyed by color/weight instead, same as
// H1's badge already does.
func headingStyle(dark bool) ansi.StyleConfig {
	cfg := styles.LightStyleConfig
	if dark {
		cfg = styles.DarkStyleConfig
	}

	h2Color, h3Color := "39", "35" // dark: cyan, teal
	if !dark {
		h2Color, h3Color = "27", "25" // light: darker blue shades
	}

	cfg.H2.Prefix = ""
	cfg.H2.Bold = boolPtr(true)
	cfg.H2.Color = stringPtr(h2Color)

	cfg.H3.Prefix = ""
	cfg.H3.Bold = boolPtr(true)
	cfg.H3.Color = stringPtr(h3Color)

	cfg.H4.Prefix = ""
	cfg.H4.Bold = boolPtr(true)

	cfg.H5.Prefix = ""
	cfg.H5.Italic = boolPtr(true)

	cfg.H6.Prefix = ""
	cfg.H6.Italic = boolPtr(true)

	return cfg
}

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }
func uintPtr(u uint) *uint       { return &u }

// sanitize strips control characters that could manipulate the terminal
// (cursor movement, screen clears, hidden escape sequences) while
// retaining tabs and newlines needed for readable Markdown, per design
// doc §11 ("Rendered output strips unsafe control characters...").
func sanitize(content string) string {
	out := make([]rune, 0, len(content))
	for _, r := range content {
		switch {
		case r == '\t' || r == '\n':
			out = append(out, r)
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
			// drop C0/C1 control characters and DEL
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

// Cache renders Markdown content, memoizing by content hash, width, and
// theme so that a terminal resize or a repeated view doesn't re-render
// unchanged content.
type Cache struct {
	mu      sync.Mutex
	entries map[string]string
}

// NewCache returns an empty Cache.
func NewCache() *Cache {
	return &Cache{entries: make(map[string]string)}
}

// Render returns the cached rendering for (content, width, dark, noColor)
// if present, otherwise renders, caches, and returns it.
func (c *Cache) Render(content string, width int, dark, noColor bool) (string, error) {
	key := cacheKey(content, width, dark, noColor)

	c.mu.Lock()
	if v, ok := c.entries[key]; ok {
		c.mu.Unlock()
		return v, nil
	}
	c.mu.Unlock()

	out, err := Render(content, width, dark, noColor)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	c.entries[key] = out
	c.mu.Unlock()
	return out, nil
}

func cacheKey(content string, width int, dark, noColor bool) string {
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%s-%d-%t-%t", hex.EncodeToString(sum[:]), width, dark, noColor)
}
