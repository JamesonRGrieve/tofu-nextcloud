// SPDX-License-Identifier: AGPL-3.0-or-later

package nextcloud

import (
	"strings"
	"testing"
)

func TestSystemConfigType(t *testing.T) {
	cases := map[string]string{
		"maintenance_window_start": TypeInteger,
		"loglevel":                 TypeInteger,
		"maintenance":              TypeBoolean,
		"filelocking.enabled":      TypeBoolean,
		"default_phone_region":     TypeString,
		"overwrite.cli.url":        TypeString,
		"trusted_domains":          TypeString,
	}
	for key, want := range cases {
		if got := SystemConfigType(key); got != want {
			t.Errorf("SystemConfigType(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestSplitConfigName(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"default_phone_region", []string{"default_phone_region"}},
		{"memcache.local", []string{"memcache", "local"}},
		{"overwrite.cli.url", []string{"overwrite", "cli", "url"}},
		{"trusted_domains.1", []string{"trusted_domains", "1"}},
	}
	for _, c := range cases {
		if got := splitConfigName(c.in); strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("splitConfigName(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestConfigSystemSetArgs(t *testing.T) {
	// String value: no --type flag.
	str := configSystemSetArgs("overwrite.cli.url", "https://cloud.example.com", TypeString)
	wantStr := "config:system:set overwrite cli url --value=https://cloud.example.com"
	if strings.Join(str, " ") != wantStr {
		t.Fatalf("string args = %q, want %q", strings.Join(str, " "), wantStr)
	}
	// Boolean value: --type=boolean appended.
	b := configSystemSetArgs("maintenance", "true", TypeBoolean)
	wantB := "config:system:set maintenance --value=true --type=boolean"
	if strings.Join(b, " ") != wantB {
		t.Fatalf("boolean args = %q, want %q", strings.Join(b, " "), wantB)
	}
	// Array index element.
	arr := configSystemSetArgs("trusted_domains.1", "cloud.example.com", TypeString)
	wantArr := "config:system:set trusted_domains 1 --value=cloud.example.com"
	if strings.Join(arr, " ") != wantArr {
		t.Fatalf("array args = %q, want %q", strings.Join(arr, " "), wantArr)
	}
}

func TestConfigSystemGetDeleteArgs(t *testing.T) {
	if got := strings.Join(configSystemGetArgs("memcache.local"), " "); got != "config:system:get memcache local" {
		t.Fatalf("get args = %q", got)
	}
	if got := strings.Join(configSystemDeleteArgs("debug"), " "); got != "config:system:delete debug" {
		t.Fatalf("delete args = %q", got)
	}
}

func TestConfigAppArgs(t *testing.T) {
	if got := strings.Join(configAppSetArgs("theming", "color", "#0082c9"), " "); got != "config:app:set theming color --value=#0082c9" {
		t.Fatalf("app set args = %q", got)
	}
	if got := strings.Join(configAppGetArgs("theming", "color"), " "); got != "config:app:get theming color" {
		t.Fatalf("app get args = %q", got)
	}
	if got := strings.Join(configAppDeleteArgs("theming", "color"), " "); got != "config:app:delete theming color" {
		t.Fatalf("app delete args = %q", got)
	}
}

func TestNormalizeConfigValue(t *testing.T) {
	cases := []struct {
		key, in, want string
	}{
		{"maintenance", "true", "true"},
		{"maintenance", "1", "true"},
		{"maintenance", "false", "false"},
		{"maintenance", "", "false"},
		{"loglevel", " 2 ", "2"},
		{"default_phone_region", "US", "US"},
		{"overwrite.cli.url", "https://x", "https://x"},
	}
	for _, c := range cases {
		if got := NormalizeConfigValue(c.key, c.in); got != c.want {
			t.Errorf("NormalizeConfigValue(%q, %q) = %q, want %q", c.key, c.in, got, c.want)
		}
	}
}

func TestReconcileConfigValue(t *testing.T) {
	// Semantically equal boolean → keep declared form (no spurious diff).
	if got := ReconcileConfigValue("maintenance", "false", ""); got != "false" {
		t.Errorf("declared false vs read-back empty should keep %q, got %q", "false", got)
	}
	if got := ReconcileConfigValue("maintenance", "true", "1"); got != "true" {
		t.Errorf("declared true vs read-back 1 should keep %q, got %q", "true", got)
	}
	// Equal string → keep declared.
	if got := ReconcileConfigValue("default_phone_region", "US", "US"); got != "US" {
		t.Errorf("equal string should keep declared, got %q", got)
	}
	// Genuine drift → surface the device value.
	if got := ReconcileConfigValue("default_phone_region", "US", "GB"); got != "GB" {
		t.Errorf("drift should surface live GB, got %q", got)
	}
	if got := ReconcileConfigValue("maintenance", "false", "true"); got != "true" {
		t.Errorf("drift (off→on) should surface live true, got %q", got)
	}
}
