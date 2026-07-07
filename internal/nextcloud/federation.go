// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Federation helpers. Nextcloud federated sharing has two moving parts driven by
// occ: the set of trusted Nextcloud peers (the `federation` app's trusted-server
// list) and the federated-sharing feature toggles (`files_sharing` app config
// plus the Global Scale lookup server in `config:system`). These pure helpers
// build the occ argument vectors, parse the trusted-server list JSON, and encode/
// decode the yes-no/1-0 boolean forms Nextcloud stores — so the
// nextcloud_trusted_server and nextcloud_federation_config resources issue the
// correct occ and reconcile to 0-diff. All pure — unit-tested; the OCC methods go
// through the injected Executor and never contact a device in a unit test.
package nextcloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// FederationApp is the occ app id that owns the trusted-server list.
const FederationApp = "federation"

// FilesSharingApp is the occ app id that owns the federated-sharing toggles.
const FilesSharingApp = "files_sharing"

// Federated-sharing app-config keys (files_sharing) and the Global Scale lookup
// server system key. These are the exact occ config keys each federation_config
// attribute maps onto.
const (
	KeyOutgoingS2S        = "outgoing_server2server_share_enabled"
	KeyIncomingS2S        = "incoming_server2server_share_enabled"
	KeyOutgoingGroupS2S   = "outgoing_server2server_group_share_enabled"
	KeyLookupServerUpload = "lookupServerUploadEnabled"
	KeyAutoAcceptTrusted  = "federatedTrustedShareAutoAccept"
	SystemKeyLookupServer = "lookup_server"
)

// yesNoKeys are the files_sharing keys Nextcloud stores as "yes"/"no". The
// remaining boolean key (KeyAutoAcceptTrusted) is stored as "1"/"0".
var yesNoKeys = map[string]bool{
	KeyOutgoingS2S:        true,
	KeyIncomingS2S:        true,
	KeyOutgoingGroupS2S:   true,
	KeyLookupServerUpload: true,
}

// FederationBoolValue encodes a bool in the on-disk form Nextcloud expects for a
// given files_sharing key: "yes"/"no" for the server-to-server toggles, "1"/"0"
// for the auto-accept flag. Pure — unit-tested.
func FederationBoolValue(key string, on bool) string {
	if yesNoKeys[key] {
		if on {
			return "yes"
		}
		return "no"
	}
	if on {
		return "1"
	}
	return "0"
}

// ParseFederationBool decodes a value read back from occ into a bool, accepting
// every truthy form Nextcloud may return ("yes"/"1"/"true"/"on"). Anything else
// (including "no"/"0"/"" for an unset key) is false. Pure — unit-tested.
func ParseFederationBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "yes", "1", "true", "on":
		return true
	default:
		return false
	}
}

// NormalizeServerURL canonicalizes a peer base URL for idempotent matching:
// trimmed of surrounding whitespace and of any trailing slash, so
// "https://cloud.example.com/" and "https://cloud.example.com" compare equal.
// Pure — unit-tested.
func NormalizeServerURL(url string) string {
	return strings.TrimRight(strings.TrimSpace(url), "/")
}

// TrustedServer is a single federated peer as reported by the trusted-server
// list. Only URL is load-bearing for the resource; ID/Status are captured when
// present for diagnostics.
type TrustedServer struct {
	ID     int
	URL    string
	Status int
}

// trustedServerListArgs lists the trusted federated peers as JSON.
func trustedServerListArgs() []string {
	return []string{"federation:trusted-servers:list", "--output=json"}
}

// trustedServerAddArgs adds a trusted federated peer by base URL.
func trustedServerAddArgs(url string) []string {
	return []string{"federation:trusted-servers:add", url}
}

// trustedServerRemoveArgs removes a trusted federated peer by base URL.
func trustedServerRemoveArgs(url string) []string {
	return []string{"federation:trusted-servers:remove", url}
}

// parseTrustedServers parses the trusted-server list JSON. Nextcloud's occ output
// is accepted in three shapes so an import survives version drift: an array of
// objects ({"id","url","status"}), a plain array of URL strings, or a map keyed
// by id → URL. An empty document or empty array yields no servers. Pure —
// unit-tested.
func parseTrustedServers(b []byte) ([]TrustedServer, error) {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		return nil, nil
	}
	// Shape 1: array of objects.
	var objs []struct {
		ID     int    `json:"id"`
		URL    string `json:"url"`
		Status int    `json:"status"`
	}
	if err := json.Unmarshal(b, &objs); err == nil {
		out := make([]TrustedServer, 0, len(objs))
		for _, o := range objs {
			out = append(out, TrustedServer{ID: o.ID, URL: NormalizeServerURL(o.URL), Status: o.Status})
		}
		return out, nil
	}
	// Shape 2: array of URL strings.
	var strs []string
	if err := json.Unmarshal(b, &strs); err == nil {
		out := make([]TrustedServer, 0, len(strs))
		for _, s := range strs {
			out = append(out, TrustedServer{URL: NormalizeServerURL(s)})
		}
		return out, nil
	}
	// Shape 3: map of id → URL.
	var m map[string]string
	if err := json.Unmarshal(b, &m); err == nil {
		out := make([]TrustedServer, 0, len(m))
		for _, s := range m {
			out = append(out, TrustedServer{URL: NormalizeServerURL(s)})
		}
		return out, nil
	}
	return nil, fmt.Errorf("parse occ federation:trusted-servers:list json: unrecognized shape")
}

// TrustedServerPresent reports whether a normalized URL is in the list (matched
// after normalization so a trailing slash never causes a false miss).
func TrustedServerPresent(list []TrustedServer, url string) bool {
	want := NormalizeServerURL(url)
	for _, s := range list {
		if s.URL == want {
			return true
		}
	}
	return false
}

// TrustedServersList returns the parsed trusted-server list.
func (o *OCC) TrustedServersList() ([]TrustedServer, error) {
	out, err := o.run(trustedServerListArgs()...)
	if err != nil {
		return nil, err
	}
	return parseTrustedServers(out)
}

// TrustedServerAdd adds a trusted federated peer by base URL (idempotent on the
// resource layer — callers add only when absent).
func (o *OCC) TrustedServerAdd(url string) error {
	_, err := o.run(trustedServerAddArgs(url)...)
	return err
}

// TrustedServerRemove removes a trusted federated peer by base URL.
func (o *OCC) TrustedServerRemove(url string) error {
	_, err := o.run(trustedServerRemoveArgs(url)...)
	return err
}
