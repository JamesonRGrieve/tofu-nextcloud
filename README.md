<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
# tofu-nextcloud

A native OpenTofu/Terraform provider for **Nextcloud** — installed state (`occ`
core install/upgrade), `config.php` system config, app install/enable state, and
per-app config — driven over an **SSH + occ** transport.

Sibling of `tofu-wordpress` / `tofu-opnsense` / `tofu-proxmox`: same toolchain
(Go 1.26.4, `terraform-plugin-framework` v1.19.0), same house standards. General
Go / Terraform-provider rules are canonical at
`/home/jameson/source/ai-prompts/go.md` and `.../tofu.md`.

Address: `registry.terraform.io/jamesonrgrieve/nextcloud` · TypeName `nextcloud`.

## Source of truth (consumer layer)

At the `tofu/` consumer layer this provider is driven **entirely from
`netbox-services` `ServiceInstance` params** — there is **no dedicated NetBox
plugin** for Nextcloud. The Nextcloud service's `ServiceInstance` carries the
docroot, version, config keys, and app set as parameters; the consumer module
maps those params → these resources. DB and admin secrets are resolved from
OpenBao at apply. The provider itself is source-of-truth-agnostic (it takes
attributes).

## Why a provider (not a shell module)

Nextcloud has no HTTP management API; everything administrative is done by running
`occ` on the host (as the web-server user). A native provider gives typed schemas,
manage-declared-only diffs, and import-to-0-diff that a `local-exec` script cannot.

## Resources

| Resource | Manages | occ / mechanism |
|---|---|---|
| `nextcloud_core` | Core install/upgrade | `occ status` / `maintenance:install` / `upgrade` |
| `nextcloud_config` | `config.php` system config (manage-declared-only) | `occ config:system:get` / `set` / `delete` (typed, dotted nested keys) |
| `nextcloud_app` | An app (install/enable/disable/version/remove) | `occ app:install/enable/disable/update/remove/list` |
| `nextcloud_appconfig` | A single per-app config value | `occ config:app:get` / `set` / `delete` |
| `nextcloud_trusted_server` | A federated Nextcloud peer (trusted-server list) | `occ federation:trusted-servers:add/remove/list` |
| `nextcloud_federation_config` | Federated-sharing toggles (manage-declared-only) | `occ config:app:set files_sharing …` + `config:system lookup_server` |

### Federation (netbox-federation consumer)

`nextcloud_trusted_server` and `nextcloud_federation_config` realize
[`netbox-federation`](../netbox-federation) intent at the consumer layer: a
`FederationRealm` (a local Nextcloud that federates, protocol
`nextcloud_federated_sharing`) → `nextcloud_federation_config`; each of its
`FederationPeer` rows → a `nextcloud_trusted_server`. `nextcloud_federation_config`
is manage-declared-only — only the toggles the configuration sets are written, so
an unset attribute is never clobbered on the device.

## Import IDs

| Resource | Import ID | Example |
|---|---|---|
| `nextcloud_core` | `<docroot>` | `tofu import nextcloud_core.site /var/www/nextcloud` |
| `nextcloud_config` | `<docroot>` | `tofu import nextcloud_config.site /var/www/nextcloud` |
| `nextcloud_app` | `<docroot>/<app_id>` (or bare `<app_id>`) | `tofu import nextcloud_app.cal /var/www/nextcloud/calendar` |
| `nextcloud_appconfig` | `<app_id>/<key>` | `tofu import nextcloud_appconfig.brand theming/color` |
| `nextcloud_trusted_server` | `<peer_url>` | `tofu import nextcloud_trusted_server.partner https://cloud.partner.org` |
| `nextcloud_federation_config` | `<docroot>` | `tofu import nextcloud_federation_config.site /var/www/nextcloud` |

Every stateful resource imports to **0-diff** — onboard a live install by
importing, then planning to confirm no changes.

## Secrets

Zero secrets in state, ever. The Nextcloud **admin password** and **DB password**
are `WriteOnly` attributes — read from config at apply and never persisted. Supply
them from OpenBao via `TF_VAR_*` (or an ephemeral `vault_kv_secret_v2`). SSH auth
is **key/cert only** (an OpenBao-signed key via `ssh_key_pem`, or an on-disk
`ssh_key_file`) — never a password, never `sshpass`.

## occ over SSH

occ must run as the web-server user (Nextcloud's data/config are owned by it), so
every invocation is `sudo -u <web_user> php <docroot>/occ …`. The `web_user`
(default `www-data`) and `docroot` are provider attributes.

## Usage

See [`examples/`](examples/): `provider.tf`, `core.tf`, `config.tf`, `app.tf`, and
`federation.tf`.

## Development

```
export PATH=$PATH:/home/jameson/.local/go/bin
make check    # go mod tidy + gofmt + vet + test + build
```

`make check` is mirrored by `.githooks/pre-commit` (enable with
`git config core.hooksPath .githooks`). `--no-verify` is forbidden without
explicit authorization. Architecture: see [`DESIGN.md`](DESIGN.md).

## ⛔ No direct applies

This provider drives production Nextcloud hosts. **Never** apply by hand — declare
intent in prod-netbox (`netbox-services` `ServiceInstance`) and realize it only
through prod-semaphore. See [`CLAUDE.md`](CLAUDE.md).

## License

AGPL-3.0-or-later. Every source file carries an SPDX header.
