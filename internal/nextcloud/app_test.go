// SPDX-License-Identifier: AGPL-3.0-or-later

package nextcloud

import (
	"strings"
	"testing"
)

func TestAppArgBuilders(t *testing.T) {
	cases := []struct {
		got  []string
		want string
	}{
		{appInstallArgs("calendar"), "app:install calendar"},
		{appEnableArgs("calendar"), "app:enable calendar"},
		{appDisableArgs("calendar"), "app:disable calendar"},
		{appRemoveArgs("calendar"), "app:remove calendar"},
		{appUpdateArgs("calendar"), "app:update calendar"},
		{appListArgs(), "app:list --output=json"},
	}
	for _, c := range cases {
		if got := strings.Join(c.got, " "); got != c.want {
			t.Errorf("args = %q, want %q", got, c.want)
		}
	}
}

func TestParseAppList(t *testing.T) {
	j := `{"enabled":{"files":"1.19.0","dav":"1.28.0"},"disabled":{"encryption":false,"calendar":"4.5.0"}}`
	apps, err := parseAppList([]byte(j))
	if err != nil {
		t.Fatalf("parseAppList err: %v", err)
	}
	if len(apps) != 4 {
		t.Fatalf("expected 4 apps, got %d", len(apps))
	}
	if s := apps["files"]; !s.Enabled || s.Version != "1.19.0" {
		t.Errorf("files = %+v, want enabled 1.19.0", s)
	}
	// A disabled app whose value is a version string keeps the version.
	if s := apps["calendar"]; s.Enabled || s.Version != "4.5.0" {
		t.Errorf("calendar = %+v, want disabled 4.5.0", s)
	}
	// A disabled app whose value is `false` yields an empty version.
	if s := apps["encryption"]; s.Enabled || s.Version != "" {
		t.Errorf("encryption = %+v, want disabled empty-version", s)
	}
	if _, err := parseAppList([]byte("nope")); err == nil {
		t.Fatal("parseAppList should error on invalid json")
	}
}

func TestAppEnabledPresent(t *testing.T) {
	apps := map[string]AppState{
		"files":      {Enabled: true, Version: "1.19.0"},
		"encryption": {Enabled: false},
	}
	if !AppEnabled(apps, "files") {
		t.Error("files should be enabled")
	}
	if AppEnabled(apps, "encryption") {
		t.Error("encryption should not be enabled")
	}
	if AppEnabled(apps, "missing") {
		t.Error("missing app should not be enabled")
	}
	if !AppPresent(apps, "encryption") {
		t.Error("encryption should be present")
	}
	if AppPresent(apps, "missing") {
		t.Error("missing app should not be present")
	}
}
