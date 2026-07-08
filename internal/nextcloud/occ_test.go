// SPDX-License-Identifier: AGPL-3.0-or-later

package nextcloud

import (
	"errors"
	"strings"
	"testing"
)

func TestShQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "'plain'"},
		{"/var/www/nextcloud", "'/var/www/nextcloud'"},
		{"a'b", `'a'\''b'`},
	}
	for _, c := range cases {
		if got := shQuote(c.in); got != c.want {
			t.Errorf("shQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestOccPath(t *testing.T) {
	if got := occPath("/var/www/nextcloud"); got != "/var/www/nextcloud/occ" {
		t.Errorf("occPath = %q", got)
	}
	// A trailing slash is normalized (no double slash).
	if got := occPath("/var/www/nextcloud/"); got != "/var/www/nextcloud/occ" {
		t.Errorf("occPath trailing slash = %q", got)
	}
}

func TestOccCommand(t *testing.T) {
	got := occCommand("www-data", "/var/www/nextcloud", "status", "--output=json")
	// Runs as the web user via `su -s /bin/sh … -c` (not sudo — see occCommand).
	for _, want := range []string{"su -s /bin/sh 'www-data' -c ", "/var/www/nextcloud/occ", "status", "--output=json"} {
		if !strings.Contains(got, want) {
			t.Fatalf("occCommand = %q missing %q", got, want)
		}
	}
	if strings.Contains(got, "sudo") {
		t.Fatalf("occCommand must not use sudo (absent on minimal CTs): %q", got)
	}
	// A value containing a space survives intact.
	got = occCommand("nginx", "/srv/nc", "config:system:set", "instanceid", "My Cloud")
	if !strings.Contains(got, "My Cloud") {
		t.Fatalf("spaced value not preserved: %q", got)
	}
	if !strings.Contains(got, "su -s /bin/sh 'nginx'") {
		t.Fatalf("web user not honored: %q", got)
	}
}

func TestNewOCCDefaultsWebUser(t *testing.T) {
	o := NewOCC(&fakeExec{}, "/var/www/nextcloud", "")
	if o.WebUser != DefaultWebUser {
		t.Fatalf("WebUser = %q, want %q", o.WebUser, DefaultWebUser)
	}
}

func TestStatusArgs(t *testing.T) {
	if got := strings.Join(statusArgs(), " "); got != "status --output=json" {
		t.Fatalf("statusArgs = %q", got)
	}
}

func TestParseStatus(t *testing.T) {
	installed := `{"installed":true,"version":"28.0.1.1","versionstring":"28.0.1","edition":""}`
	s, err := parseStatus([]byte(installed))
	if err != nil {
		t.Fatalf("parseStatus err: %v", err)
	}
	if !s.Installed || s.Version != "28.0.1" {
		t.Fatalf("parseStatus = %+v, want installed 28.0.1", s)
	}
	// Falls back to version when versionstring is absent.
	s, err = parseStatus([]byte(`{"installed":false,"version":"27.1.5.1"}`))
	if err != nil {
		t.Fatalf("parseStatus err: %v", err)
	}
	if s.Installed || s.Version != "27.1.5.1" {
		t.Fatalf("parseStatus fallback = %+v", s)
	}
	if _, err := parseStatus([]byte("not json")); err == nil {
		t.Fatal("parseStatus should error on invalid json")
	}
}

func TestOCC_Status(t *testing.T) {
	f := &fakeExec{responses: []fakeResp{{out: `{"installed":true,"versionstring":"28.0.1"}`}}}
	o := NewOCC(f, "/var/www/nextcloud", "www-data")
	s, err := o.Status()
	if err != nil {
		t.Fatalf("Status err: %v", err)
	}
	if !s.Installed || s.Version != "28.0.1" {
		t.Fatalf("Status = %+v", s)
	}
	for _, want := range []string{"su -s /bin/sh 'www-data' -c ", "/var/www/nextcloud/occ", "status", "--output=json"} {
		if !strings.Contains(f.calls[0], want) {
			t.Fatalf("unexpected command %q missing %q", f.calls[0], want)
		}
	}
}

func TestOCC_IsInstalled(t *testing.T) {
	yes := &fakeExec{responses: []fakeResp{{out: `{"installed":true,"versionstring":"28.0.1"}`}}}
	if !NewOCC(yes, "/nc", "").IsInstalled() {
		t.Fatal("installed:true must report installed")
	}
	no := &fakeExec{responses: []fakeResp{{out: `{"installed":false}`}}}
	if NewOCC(no, "/nc", "").IsInstalled() {
		t.Fatal("installed:false must report not installed")
	}
	boom := &fakeExec{responses: []fakeResp{{err: errors.New("occ: boom")}}}
	if NewOCC(boom, "/nc", "").IsInstalled() {
		t.Fatal("an executor error must report not installed")
	}
}

func TestOCC_ConfigSystemSetPropagatesError(t *testing.T) {
	f := &fakeExec{responses: []fakeResp{{err: errors.New("occ: boom")}}}
	if err := NewOCC(f, "/nc", "").ConfigSystemSet("maintenance", "true", TypeBoolean); err == nil {
		t.Fatal("ConfigSystemSet must surface the executor error")
	}
}

func TestOCC_AppList(t *testing.T) {
	f := &fakeExec{responses: []fakeResp{{out: `{"enabled":{"files":"1.19.0"},"disabled":{"encryption":false}}`}}}
	apps, err := NewOCC(f, "/nc", "").AppList()
	if err != nil {
		t.Fatalf("AppList err: %v", err)
	}
	if !AppEnabled(apps, "files") || apps["files"].Version != "1.19.0" {
		t.Fatalf("files app = %+v", apps["files"])
	}
	if AppEnabled(apps, "encryption") || !AppPresent(apps, "encryption") {
		t.Fatalf("encryption should be present but disabled: %+v", apps["encryption"])
	}
}
