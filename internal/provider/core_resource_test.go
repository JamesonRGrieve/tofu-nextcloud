// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestMaintenanceInstallArgs(t *testing.T) {
	m := coreModel{
		AdminUser:    types.StringValue("admin"),
		DatabaseType: types.StringValue("mysql"),
		DatabaseName: types.StringValue("nextcloud"),
		DatabaseUser: types.StringValue("nc"),
		DatabaseHost: types.StringValue("localhost"),
		DataDir:      types.StringValue("/var/www/nextcloud/data"),
	}
	got := strings.Join(maintenanceInstallArgs(m, "s3cret", "dbpw"), " ")
	for _, want := range []string{
		"maintenance:install",
		"--database=mysql",
		"--database-name=nextcloud",
		"--database-user=nc",
		"--database-host=localhost",
		"--database-pass=dbpw",
		"--admin-user=admin",
		"--admin-pass=s3cret",
		"--data-dir=/var/www/nextcloud/data",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("maintenanceInstallArgs missing %q in %q", want, got)
		}
	}
	// Omitted optional flags are absent.
	bare := strings.Join(maintenanceInstallArgs(coreModel{}, "", ""), " ")
	if strings.Contains(bare, "--admin-pass=") || strings.Contains(bare, "--database=") {
		t.Errorf("empty model should omit optional flags: %q", bare)
	}
}

func TestCoreMetadata(t *testing.T) {
	resp := &resource.MetadataResponse{}
	NewCoreResource().Metadata(context.Background(),
		resource.MetadataRequest{ProviderTypeName: "nextcloud"}, resp)
	if resp.TypeName != "nextcloud_core" {
		t.Fatalf("TypeName = %q, want nextcloud_core", resp.TypeName)
	}
}
