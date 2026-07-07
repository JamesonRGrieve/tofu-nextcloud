// SPDX-License-Identifier: AGPL-3.0-or-later

package nextcloud

import (
	"errors"
	"strings"
	"testing"
)

func TestFederationBoolValue(t *testing.T) {
	cases := []struct {
		key  string
		on   bool
		want string
	}{
		{KeyOutgoingS2S, true, "yes"},
		{KeyOutgoingS2S, false, "no"},
		{KeyIncomingS2S, true, "yes"},
		{KeyOutgoingGroupS2S, false, "no"},
		{KeyLookupServerUpload, true, "yes"},
		{KeyAutoAcceptTrusted, true, "1"},
		{KeyAutoAcceptTrusted, false, "0"},
	}
	for _, c := range cases {
		if got := FederationBoolValue(c.key, c.on); got != c.want {
			t.Errorf("FederationBoolValue(%q, %v) = %q, want %q", c.key, c.on, got, c.want)
		}
	}
}

func TestParseFederationBool(t *testing.T) {
	cases := map[string]bool{
		"yes":   true,
		"1":     true,
		"true":  true,
		"ON":    true,
		" yes ": true,
		"no":    false,
		"0":     false,
		"":      false,
		"maybe": false,
	}
	for in, want := range cases {
		if got := ParseFederationBool(in); got != want {
			t.Errorf("ParseFederationBool(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestNormalizeServerURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://cloud.example.com", "https://cloud.example.com"},
		{"https://cloud.example.com/", "https://cloud.example.com"},
		{" https://cloud.example.com/ ", "https://cloud.example.com"},
		{"https://cloud.example.com///", "https://cloud.example.com"},
	}
	for _, c := range cases {
		if got := NormalizeServerURL(c.in); got != c.want {
			t.Errorf("NormalizeServerURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTrustedServerArgs(t *testing.T) {
	if got := strings.Join(trustedServerListArgs(), " "); got != "federation:trusted-servers:list --output=json" {
		t.Fatalf("list args = %q", got)
	}
	if got := strings.Join(trustedServerAddArgs("https://peer.example.com"), " "); got != "federation:trusted-servers:add https://peer.example.com" {
		t.Fatalf("add args = %q", got)
	}
	if got := strings.Join(trustedServerRemoveArgs("https://peer.example.com"), " "); got != "federation:trusted-servers:remove https://peer.example.com" {
		t.Fatalf("remove args = %q", got)
	}
}

func TestParseTrustedServers(t *testing.T) {
	// Shape 1: array of objects (trailing slash normalized).
	objs, err := parseTrustedServers([]byte(`[{"id":1,"url":"https://a.example.com/","status":1},{"id":2,"url":"https://b.example.com","status":3}]`))
	if err != nil {
		t.Fatalf("objects err: %v", err)
	}
	if len(objs) != 2 || objs[0].URL != "https://a.example.com" || objs[0].ID != 1 || objs[1].URL != "https://b.example.com" {
		t.Fatalf("objects = %+v", objs)
	}
	// Shape 2: array of strings.
	strs, err := parseTrustedServers([]byte(`["https://c.example.com/","https://d.example.com"]`))
	if err != nil {
		t.Fatalf("strings err: %v", err)
	}
	if len(strs) != 2 || strs[0].URL != "https://c.example.com" {
		t.Fatalf("strings = %+v", strs)
	}
	// Shape 3: map id → url.
	m, err := parseTrustedServers([]byte(`{"1":"https://e.example.com/"}`))
	if err != nil {
		t.Fatalf("map err: %v", err)
	}
	if len(m) != 1 || m[0].URL != "https://e.example.com" {
		t.Fatalf("map = %+v", m)
	}
	// Empty forms yield no servers.
	for _, in := range []string{"", "  ", "[]", "null"} {
		got, err := parseTrustedServers([]byte(in))
		if err != nil || len(got) != 0 {
			t.Fatalf("empty %q → %+v, %v", in, got, err)
		}
	}
	// Garbage errors.
	if _, err := parseTrustedServers([]byte("not json")); err == nil {
		t.Fatal("garbage should error")
	}
}

func TestTrustedServerPresent(t *testing.T) {
	list := []TrustedServer{{URL: "https://a.example.com"}, {URL: "https://b.example.com"}}
	if !TrustedServerPresent(list, "https://a.example.com/") {
		t.Fatal("trailing-slash URL should match after normalization")
	}
	if TrustedServerPresent(list, "https://z.example.com") {
		t.Fatal("absent URL must not match")
	}
}

func TestOCC_TrustedServersList(t *testing.T) {
	f := &fakeExec{responses: []fakeResp{{out: `[{"id":1,"url":"https://peer.example.com","status":1}]`}}}
	list, err := NewOCC(f, "/nc", "www-data").TrustedServersList()
	if err != nil {
		t.Fatalf("TrustedServersList err: %v", err)
	}
	if len(list) != 1 || list[0].URL != "https://peer.example.com" {
		t.Fatalf("list = %+v", list)
	}
	want := "sudo -u 'www-data' php '/nc/occ' 'federation:trusted-servers:list' '--output=json'"
	if f.calls[0] != want {
		t.Fatalf("unexpected command: %q", f.calls[0])
	}
}

func TestOCC_TrustedServerAddRemove(t *testing.T) {
	f := &fakeExec{responses: []fakeResp{{out: ""}, {out: ""}}}
	o := NewOCC(f, "/nc", "www-data")
	if err := o.TrustedServerAdd("https://peer.example.com"); err != nil {
		t.Fatalf("add err: %v", err)
	}
	if err := o.TrustedServerRemove("https://peer.example.com"); err != nil {
		t.Fatalf("remove err: %v", err)
	}
	if !strings.Contains(f.calls[0], "'federation:trusted-servers:add' 'https://peer.example.com'") {
		t.Fatalf("add command = %q", f.calls[0])
	}
	if !strings.Contains(f.calls[1], "'federation:trusted-servers:remove' 'https://peer.example.com'") {
		t.Fatalf("remove command = %q", f.calls[1])
	}
}

func TestOCC_TrustedServerAddPropagatesError(t *testing.T) {
	f := &fakeExec{responses: []fakeResp{{err: errors.New("occ: boom")}}}
	if err := NewOCC(f, "/nc", "").TrustedServerAdd("https://peer.example.com"); err == nil {
		t.Fatal("TrustedServerAdd must surface the executor error")
	}
}
