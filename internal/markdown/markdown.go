// Package markdown wraps Glamour rendering with sanitization and a
// width-aware cache, per
// docs/superpowers/specs/2026-08-12-skillbrowse-design.md §11 and §13.
package markdown

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
)

// Render sanitizes content and renders it as Markdown at the given
// terminal width. noColor uses Glamour's stock "notty" style, which
// emits no ANSI codes at all (honoring NO_COLOR/--no-color) and so must
// keep "#"-prefixed headings as the only way to convey heading level in
// plain text; otherwise dark or light selects a heading-decluttered
// variant of Glamour's standard dark/light styles (see headingStyle).
func Render(content string, width int, dark, noColor bool) (string, error) {
	var opt glamour.TermRendererOption
	if noColor {
		opt = glamour.WithStandardStyle("notty")
	} else {
		opt = glamour.WithStyles(headingStyle(dark))
	}

	r, err := glamour.NewTermRenderer(opt, glamour.WithWordWrap(width))
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()
	return r.Render(sanitize(content))
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
