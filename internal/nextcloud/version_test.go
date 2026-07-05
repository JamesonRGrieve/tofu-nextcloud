// SPDX-License-Identifier: AGPL-3.0-or-later

package nextcloud

import "testing"

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in            string
		maj, min, pat int
		wantErr       bool
	}{
		{"28.0.1", 28, 0, 1, false},
		{"28.0", 28, 0, 0, false},
		{"27", 27, 0, 0, false},
		{"v26.0.5", 26, 0, 5, false},
		{" 28.0.1 ", 28, 0, 1, false},
		{"", 0, 0, 0, true},
		{"28.x", 0, 0, 0, true},
	}
	for _, c := range cases {
		maj, min, pat, err := ParseVersion(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseVersion(%q) expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseVersion(%q) unexpected error: %v", c.in, err)
			continue
		}
		if maj != c.maj || min != c.min || pat != c.pat {
			t.Errorf("ParseVersion(%q) = %d.%d.%d, want %d.%d.%d", c.in, maj, min, pat, c.maj, c.min, c.pat)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"28.0.1", "28.0.1", 0},
		{"28.0.0", "28.0.1", -1},
		{"28.1.0", "28.0.9", 1},
		{"28.0", "28.0.0", 0},
		{"29.0", "28.9.9", 1},
	}
	for _, c := range cases {
		got, err := CompareVersions(c.a, c.b)
		if err != nil {
			t.Errorf("CompareVersions(%q,%q) error: %v", c.a, c.b, err)
			continue
		}
		if got != c.want {
			t.Errorf("CompareVersions(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
	if _, err := CompareVersions("bad", "28.0"); err == nil {
		t.Error("CompareVersions with unparseable input should error")
	}
}
