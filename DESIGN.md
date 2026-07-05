<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
# tofu-nextcloud — Design

Native OpenTofu/Terraform provider for Nextcloud installed state, `config.php`
system config, app install/enable state, and per-app config, over an SSH + occ
transport. This document is the architectural summary; it describes the target
end state.

## Architecture

Two layers, mirroring `tofu-wordpress`:

- **`internal/nextcloud/`** — the transport and pure logic, **free of any
  terraform-plugin-framework import** so it is unit-testable in isolation:
  - `ssh.go` — an `os/exec`-based SSH client (no in-process SSH library, so
    `go.mod` stays identical to the siblings). Key/cert auth only; a `key_pem` is
    materialized to a temp `0600` file per call and removed after. Transient
    connection resets are retried.
  - `occ.go` — an `OCC` wrapper over an injected `Executor`. `occCommand` renders
    a shell-safe `sudo -u <web_user> php <docroot>/occ …` string (pure); the run
    methods go through the `Executor` so apply logic is exercised in tests with a
    fake — **no unit test ever contacts a device**.
  - `config.go` — `config:system` key typing (`SystemConfigType`), dotted-name
    splitting, occ argument builders, value normalization, and
    `ReconcileConfigValue` (manage-declared-only diff).
  - `app.go` — `app:*` argument builders, `app:list --output=json` parsing
    (`parseAppList` → per-app enabled/version), and the `OCC` app-lifecycle
    methods.
  - `version.go` — version parse/compare for the core upgrade decision.
- **`internal/provider/`** — the framework glue: `provider.go`
  (host/port/user + SSH auth + docroot + web_user) and one `*_resource.go` per
  resource. Resources wrap the transport in an `OCC` and are configured with a
  shared `*providerClient` (SSH client + default docroot + web user).

`main.go` serves the provider at `registry.terraform.io/jamesonrgrieve/nextcloud`.

## Transport: occ over SSH

Nextcloud exposes no management API. All reads and writes are `occ` invocations
executed on the host over SSH. Because Nextcloud's data and config directories
must be owned by the web-server user, occ is run as that user
(`sudo -u www-data php <docroot>/occ …`) — never as root, which would leave files
occ can't later read. The request `ctx` timeout bounds each command.

## Schema conventions

- Every stateful resource implements `ImportState` to **0-diff**; `path` defaults
  to the provider `docroot`, is `Optional+Computed`, and is filled on
  create/read/import so the plan is stable.
- **Manage-declared-only** diff on `nextcloud_config`: only the keys the
  configuration declares are read back and reconciled; `ReconcileConfigValue`
  keeps the declared form when it is semantically equal to the device value (e.g.
  a boolean declared `"false"` vs read back `""`), and surfaces the live value on
  real drift. Unmanaged config keys are never touched.
- `config:system` values are **typed**: integer/boolean keys are written with the
  correct occ `--type`; nested keys (`memcache.local`, `overwrite.cli.url`,
  `trusted_domains.1`) are addressed by splitting the dotted key into positional
  occ arguments.
- Singleton-ish resources (`core`, `config`) use a no-op `Delete` — the underlying
  install/config persists; the resource just stops managing it. `nextcloud_app`
  and `nextcloud_appconfig` have real deletes (`app:remove`, `config:app:delete`).

## Secrets

`admin_password` and `database_password` (core) are **write-only** attributes:
read from config at apply via `req.Config.GetAttribute`, never written to plan or
state. They are injected from OpenBao at apply. SSH auth is key/cert only. State
stores no plaintext credential.

## Source of truth & verification owed

At the `tofu/` consumer layer the intent is expressed **entirely as
`netbox-services` `ServiceInstance` params** — there is **no dedicated NetBox
plugin** for Nextcloud. The consumer module maps those params → these resources;
DB/admin secrets come from OpenBao.

The device-apply paths (`maintenance:install`, `upgrade`, `config:system:set`,
`app:install`, …) are present and exercised **only through an injected fake
executor** in unit tests. The provider **never** applies to a device directly.
Per the workspace change-safety protocol, any real apply is driven **only**
through the sanctioned pipeline (prod-netbox → prod-semaphore), **proven on a lab
twin first** in byte-for-byte identical form, with a snapshot/rollback armed
out-of-band and a real external HTTP health check. A green `tofu plan` is
necessary, not sufficient — the lab-twin drill (see `todo.json`) is the
verification still owed before any production apply.

## Roadmap parity items

Tracked in `todo.json`:

- Confirm the full production `config.php` define set round-trips 0-diff through
  `nextcloud_config`, extending the integer/boolean key classification as needed.
- Reconcile the app-version pinning story (occ app-store installs track the
  latest available; a differing declared `version` triggers `app:update`).
- Lab-twin drill: prove core install/upgrade + config + app management end-to-end
  through dev-netbox → dev-semaphore at 0-diff on re-apply.
