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
)

// Render sanitizes content and renders it as Markdown at the given
// terminal width. noColor selects Glamour's "notty" style, which emits no
// ANSI codes at all (honoring NO_COLOR/--no-color); otherwise dark or
// light selects between Glamour's standard dark/light styles.
func Render(content string, width int, dark, noColor bool) (string, error) {
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(styleName(dark, noColor)),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()
	return r.Render(sanitize(content))
}

func styleName(dark, noColor bool) string {
	switch {
	case noColor:
		return "notty"
	case dark:
		return "dark"
	default:
		return "light"
	}
}

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
