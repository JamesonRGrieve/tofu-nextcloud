# SPDX-License-Identifier: AGPL-3.0-or-later
# nextcloud_config — manage config.php system config (manage-declared-only).
# Import:  tofu import nextcloud_config.site /var/www/nextcloud

resource "nextcloud_config" "site" {
  path = "/var/www/nextcloud"

  # Only the declared keys are managed; unmanaged config is never touched.
  # Dotted keys address nested config; known bool/int keys get the correct type.
  values = {
    "trusted_domains.1"        = "cloud.example.com"
    "overwrite.cli.url"        = "https://cloud.example.com"
    "default_phone_region"     = "US"
    "maintenance_window_start" = "1"       # integer
    "maintenance"              = "false"   # boolean
    "loglevel"                 = "2"       # integer
    "memcache.local"           = "\\OC\\Memcache\\APCu"
  }
}
