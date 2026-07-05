// SPDX-License-Identifier: AGPL-3.0-or-later

// Command tofu-nextcloud is the OpenTofu/Terraform provider plugin entrypoint
// for managing Nextcloud installed state (occ core install/upgrade), system
// config (config.php via config:system), and app install/enable state
// (config:app), driven over an SSH + occ transport.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/JamesonRGrieve/tofu-nextcloud/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// version is overridden at build time via -ldflags.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/jamesonrgrieve/nextcloud",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
