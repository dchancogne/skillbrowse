package buildinfo

import (
	"runtime"
	"testing"
)

func TestGetDefaults(t *testing.T) {
	info := Get()

	if info.Version != Version || info.Commit != Commit || info.Date != Date {
		t.Errorf("Get() = %+v, want Version/Commit/Date to match package vars %q/%q/%q",
			info, Version, Commit, Date)
	}
	if info.GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %q, want %q", info.GoVersion, runtime.Version())
	}
	if info.OS != runtime.GOOS {
		t.Errorf("OS = %q, want %q", info.OS, runtime.GOOS)
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", info.Arch, runtime.GOARCH)
	}
}

func TestGetReflectsOverriddenVars(t *testing.T) {
	origVersion, origCommit, origDate := Version, Commit, Date
	t.Cleanup(func() {
		Version, Commit, Date = origVersion, origCommit, origDate
	})

	Version, Commit, Date = "1.2.3", "abc123", "2026-08-17"

	info := Get()
	if info.Version != "1.2.3" || info.Commit != "abc123" || info.Date != "2026-08-17" {
		t.Errorf("Get() = %+v, want overridden ldflags values reflected", info)
	}
}
