# SPDX-License-Identifier: AGPL-3.0-or-later
# nextcloud_trusted_server / nextcloud_federation_config — federated sharing.
# These realize netbox-federation intent (FederationRealm / FederationPeer,
# protocol nextcloud_federated_sharing) at the consumer layer.

# A federated Nextcloud peer (federation app trusted-server list).
# Import:  tofu import nextcloud_trusted_server.partner https://cloud.partner.org
resource "nextcloud_trusted_server" "partner" {
  path = "/var/www/nextcloud"
  url  = "https://cloud.partner.org"
}

# Federated-sharing feature toggles (manage-declared-only; unset attrs untouched).
# Import:  tofu import nextcloud_federation_config.site /var/www/nextcloud
resource "nextcloud_federation_config" "site" {
  path                    = "/var/www/nextcloud"
  outgoing_enabled        = true
  incoming_enabled        = true
  federated_group_sharing = false
  auto_accept_trusted     = true
  lookup_server           = "https://lookup.nextcloud.com"
}
