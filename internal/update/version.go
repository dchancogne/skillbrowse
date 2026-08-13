package update

import (
	"strconv"
	"strings"
)

// semver is a minimal major.minor.patch parse — good enough to order
// release tags without pulling in a full semver dependency. Any
// pre-release/build metadata suffix (after "-" or "+") is ignored.
type semver struct {
	major, minor, patch int
	ok                  bool
}

func parseVersion(s string) semver {
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}

	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return semver{}
	}

	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return semver{}
		}
		nums[i] = n
	}

	return semver{major: nums[0], minor: nums[1], patch: nums[2], ok: true}
}

// compareVersions returns -1, 0, or 1 as a compares before, equal to, or
// after b. Unparseable versions sort before every parseable one, so an
// unparseable current version (e.g. a "dev" build) is always treated as
// updatable.
func compareVersions(a, b string) int {
	va, vb := parseVersion(a), parseVersion(b)

	switch {
	case !va.ok && !vb.ok:
		return strings.Compare(a, b)
	case !va.ok:
		return -1
	case !vb.ok:
		return 1
	}

	for _, pair := range [][2]int{{va.major, vb.major}, {va.minor, vb.minor}, {va.patch, vb.patch}} {
		if pair[0] != pair[1] {
			if pair[0] < pair[1] {
				return -1
			}
			return 1
		}
	}
	return 0
}
