# SPDX-License-Identifier: AGPL-3.0-or-later
# nextcloud_app / nextcloud_appconfig — WP-CLI-style app management via occ.
# Import:  tofu import nextcloud_app.calendar /var/www/nextcloud/calendar

resource "nextcloud_app" "calendar" {
  path    = "/var/www/nextcloud"
  app_id  = "calendar"
  enabled = true
}

# An installed-but-disabled app.
resource "nextcloud_app" "encryption" {
  path    = "/var/www/nextcloud"
  app_id  = "encryption"
  enabled = false
}

# Per-app configuration (occ config:app:set theming color '#0082c9').
# Import:  tofu import nextcloud_appconfig.brand theming/color
resource "nextcloud_appconfig" "brand" {
  path   = "/var/www/nextcloud"
  app_id = "theming"
  key    = "color"
  value  = "#0082c9"
}
