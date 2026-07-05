# SPDX-License-Identifier: AGPL-3.0-or-later
# nextcloud_core — install/upgrade Nextcloud via occ.
# Import an existing install to 0-diff:  tofu import nextcloud_core.site /var/www/nextcloud

variable "nc_admin_password" {
  description = "Nextcloud admin password. Injected from OpenBao at apply; write-only, never stored in state."
  type        = string
  sensitive   = true
}

variable "nc_db_password" {
  description = "Database password for install. Injected from OpenBao; write-only, never stored in state."
  type        = string
  sensitive   = true
}

resource "nextcloud_core" "site" {
  path    = "/var/www/nextcloud"
  version = "28.0.1"

  admin_user    = "admin"
  database_type = "mysql"
  database_name = "nextcloud"
  database_user = "nextcloud"
  database_host = "localhost"
  data_dir      = "/var/www/nextcloud/data"

  # Write-only: read from config at apply, never persisted.
  admin_password    = var.nc_admin_password
  database_password = var.nc_db_password
}
