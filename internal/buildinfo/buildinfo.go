// Package buildinfo exposes version and build metadata for skillbrowse.
package buildinfo

import "runtime"

// Version, Commit, and Date are set via -ldflags at build time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Info is the full set of version and build metadata reported by
// `skillbrowse version`.
type Info struct {
	Version   string
	Commit    string
	Date      string
	GoVersion string
	OS        string
	Arch      string
}

// Get returns the current build's metadata.
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}
