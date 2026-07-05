// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Nextcloud version parsing/comparison. `occ status` reports a dotted version
// (e.g. "28.0.1"); the nextcloud_core resource compares the installed version
// against the declared one to decide whether an `occ upgrade` is needed. Pure —
// unit-tested.
package nextcloud

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseVersion parses a dotted Nextcloud version into its numeric components.
// Missing trailing components default to 0 (e.g. "28.0" → 28.0.0). Extra
// components beyond patch are ignored. A leading "v" is tolerated.
func ParseVersion(s string) (major, minor, patch int, err error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return 0, 0, 0, fmt.Errorf("empty version string")
	}
	parts := strings.Split(s, ".")
	nums := make([]int, 3)
	for i := 0; i < 3 && i < len(parts); i++ {
		n, perr := strconv.Atoi(strings.TrimSpace(parts[i]))
		if perr != nil {
			return 0, 0, 0, fmt.Errorf("invalid version %q: component %q is not numeric", s, parts[i])
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], nil
}

// CompareVersions returns -1 if a < b, 0 if equal, +1 if a > b. A parse error on
// either side is returned; callers treat that as "cannot compare".
func CompareVersions(a, b string) (int, error) {
	amaj, amin, apat, err := ParseVersion(a)
	if err != nil {
		return 0, err
	}
	bmaj, bmin, bpat, err := ParseVersion(b)
	if err != nil {
		return 0, err
	}
	for _, pair := range [][2]int{{amaj, bmaj}, {amin, bmin}, {apat, bpat}} {
		switch {
		case pair[0] < pair[1]:
			return -1, nil
		case pair[0] > pair[1]:
			return 1, nil
		}
	}
	return 0, nil
}
