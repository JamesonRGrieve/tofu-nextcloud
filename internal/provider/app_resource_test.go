// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestAppEnableAction(t *testing.T) {
	if got := appEnableAction(true); got != "enable" {
		t.Errorf("appEnableAction(true) = %q, want enable", got)
	}
	if got := appEnableAction(false); got != "disable" {
		t.Errorf("appEnableAction(false) = %q, want disable", got)
	}
}

func TestAppIDAndImport(t *testing.T) {
	if got := appID("/var/www/nextcloud", "calendar"); got != "/var/www/nextcloud/calendar" {
		t.Fatalf("appID = %q", got)
	}
	p, id := parseAppImportID("/var/www/nextcloud/calendar")
	if p != "/var/www/nextcloud" || id != "calendar" {
		t.Fatalf("parseAppImportID = (%q,%q)", p, id)
	}
	// Bare app id → empty path (provider docroot used).
	p, id = parseAppImportID("calendar")
	if p != "" || id != "calendar" {
		t.Fatalf("bare import = (%q,%q)", p, id)
	}
}

func TestAppConfigIDAndImport(t *testing.T) {
	if got := appConfigID("/var/www/nextcloud", "theming", "color"); got != "/var/www/nextcloud/theming/color" {
		t.Fatalf("appConfigID = %q", got)
	}
	app, key := parseAppConfigImportID("theming/color")
	if app != "theming" || key != "color" {
		t.Fatalf("parseAppConfigImportID = (%q,%q)", app, key)
	}
	// Bare app id → empty key.
	app, key = parseAppConfigImportID("theming")
	if app != "theming" || key != "" {
		t.Fatalf("bare appconfig import = (%q,%q)", app, key)
	}
}

func TestAppMetadata(t *testing.T) {
	resp := &resource.MetadataResponse{}
	NewAppResource().Metadata(context.Background(),
		resource.MetadataRequest{ProviderTypeName: "nextcloud"}, resp)
	if resp.TypeName != "nextcloud_app" {
		t.Fatalf("TypeName = %q, want nextcloud_app", resp.TypeName)
	}
	resp = &resource.MetadataResponse{}
	NewAppConfigResource().Metadata(context.Background(),
		resource.MetadataRequest{ProviderTypeName: "nextcloud"}, resp)
	if resp.TypeName != "nextcloud_appconfig" {
		t.Fatalf("TypeName = %q, want nextcloud_appconfig", resp.TypeName)
	}
}
