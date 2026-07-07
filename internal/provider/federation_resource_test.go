// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"testing"

	"github.com/JamesonRGrieve/tofu-nextcloud/internal/nextcloud"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFederationMetadata(t *testing.T) {
	resp := &resource.MetadataResponse{}
	NewTrustedServerResource().Metadata(context.Background(),
		resource.MetadataRequest{ProviderTypeName: "nextcloud"}, resp)
	if resp.TypeName != "nextcloud_trusted_server" {
		t.Fatalf("TypeName = %q, want nextcloud_trusted_server", resp.TypeName)
	}
	resp = &resource.MetadataResponse{}
	NewFederationConfigResource().Metadata(context.Background(),
		resource.MetadataRequest{ProviderTypeName: "nextcloud"}, resp)
	if resp.TypeName != "nextcloud_federation_config" {
		t.Fatalf("TypeName = %q, want nextcloud_federation_config", resp.TypeName)
	}
}

// TestFedBoolAttrsTable verifies the toggle table maps each distinct
// files_sharing key onto a round-tripping model field.
func TestFedBoolAttrsTable(t *testing.T) {
	wantKeys := map[string]bool{
		nextcloud.KeyOutgoingS2S:       true,
		nextcloud.KeyIncomingS2S:       true,
		nextcloud.KeyOutgoingGroupS2S:  true,
		nextcloud.KeyAutoAcceptTrusted: true,
	}
	if len(fedBoolAttrs) != len(wantKeys) {
		t.Fatalf("fedBoolAttrs has %d entries, want %d", len(fedBoolAttrs), len(wantKeys))
	}
	seen := map[string]bool{}
	for _, a := range fedBoolAttrs {
		if !wantKeys[a.key] {
			t.Errorf("unexpected key %q", a.key)
		}
		if seen[a.key] {
			t.Errorf("duplicate key %q", a.key)
		}
		seen[a.key] = true
		// get/set round-trip: set true, read it back through the accessor.
		var m federationConfigModel
		a.set(&m, types.BoolValue(true))
		if !a.get(&m).ValueBool() {
			t.Errorf("key %q get/set does not round-trip", a.key)
		}
	}
}
