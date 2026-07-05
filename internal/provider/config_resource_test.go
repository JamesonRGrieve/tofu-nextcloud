// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestSortedKeys(t *testing.T) {
	in := map[string]string{"maintenance": "false", "loglevel": "2", "default_phone_region": "US"}
	got := sortedKeys(in)
	want := []string{"default_phone_region", "loglevel", "maintenance"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortedKeys = %v, want %v", got, want)
	}
	if got := sortedKeys(map[string]string{}); len(got) != 0 {
		t.Fatalf("empty map should yield no keys, got %v", got)
	}
}

func TestConfigMetadata(t *testing.T) {
	resp := &resource.MetadataResponse{}
	NewConfigResource().Metadata(context.Background(),
		resource.MetadataRequest{ProviderTypeName: "nextcloud"}, resp)
	if resp.TypeName != "nextcloud_config" {
		t.Fatalf("TypeName = %q, want nextcloud_config", resp.TypeName)
	}
}
