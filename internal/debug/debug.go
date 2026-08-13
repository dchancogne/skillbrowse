// Package debug provides an opt-in stderr diagnostic log, gated by the
// SKILLBROWSE_DEBUG=1 environment variable, per
// docs/superpowers/specs/2026-08-12-skillbrowse-design.md §9 ("A
// SKILLBROWSE_DEBUG=1 environment variable enables structured diagnostic
// details on stderr for development without writing a log file").
package debug

import (
	"fmt"
	"os"
)

// Enabled reports whether SKILLBROWSE_DEBUG=1 was set at process start.
var Enabled = os.Getenv("SKILLBROWSE_DEBUG") == "1"

// Log writes a debug line to stderr when Enabled. It is a no-op
// otherwise, so callers can log freely without checking Enabled
// themselves. Never pass skill content here — diagnostics must not leak
// file contents (design doc §9).
func Log(format string, args ...any) {
	if !Enabled {
		return
	}
	fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
}
