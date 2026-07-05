<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
# tofu-nextcloud — Agent Operating Guide

> **⛔ NO DIRECT APPLIES TO ANY DEVICE — EVER.**
>
> Direct changes to **any** device — router, firewall, switch, access point, hypervisor, mail gateway, or any other appliance — are **NEVER** permitted, by anyone, for any reason. This bans hand-run `tofu apply`, hand-run `ansible-playbook`, SSH/serial/CLI config writes, REST/API mutations, and web-GUI/console edits.
>
> **Every change MUST flow through the sanctioned pipeline:** declare intent in **prod-netbox** (the single source of truth), then realize it **only** through **prod-semaphore** (the sanctioned runner). A change that did not go **prod-netbox → prod-semaphore** must never reach a device.
>
> **Sole exception:** a specific direct action is permitted *only* when the operator authorizes that exact action in advance by answering an explicit, **alarm-flavored `AskUserQuestion`** — one that names the device, the precise action, and the risk — **in the affirmative**. No standing grants, no inferred permission, no carrying one approval to another action or device. Absent that in-the-moment "yes," the answer is no.
>
> **Never offload the work onto the operator.** When you are blocked, ask for the break-glass authorization that lets *you* do the job — never ask the operator to run a command, SSH in, or make the change on your behalf. The operator grants permission; they do not perform your labour.

Native OpenTofu/Terraform provider for **Nextcloud** — installed state (`occ`
core install/upgrade), `config.php` system config, app install/enable state, and
per-app config, over an SSH + occ transport. Sibling of `../tofu-wordpress` (same
generic-typed-resource philosophy, same toolchain — occ is the exact parallel to
WP-CLI). The workspace-root `../CLAUDE.md` applies; this adds specifics.

## What this is / isn't

- **Is:** a provider that drives Nextcloud hosts entirely through **occ over SSH**
  (`sudo -u www-data php occ …`).
- **Isn't:** an HTTP/REST provider (Nextcloud has no management API), and not a
  files/users/groups data manager. It manages install-state, `config:system`
  config, app install/enable state, and `config:app`.

## Design tenets

- **Transport/logic layer is framework-free.** `internal/nextcloud/` imports no
  terraform-plugin-framework — SSH client, occ wrapper, and the pure builders
  (config typing/name-splitting, app-list parsing, version parsing) live here and
  are unit-tested through an **injected `Executor`**. The provider glue in
  `internal/provider/` wires it to the framework.
- **occ runs as the web user.** Nextcloud's data/config are owned by the
  web-server user; every occ call is `sudo -u <web_user> php <docroot>/occ …`.
  Running occ as root would leave files occ can't later read.
- **Manage-declared-only diff** on `nextcloud_config`: only declared keys are
  reconciled (`ReconcileConfigValue`); unmanaged config is never clobbered. Fix a
  spurious diff in the subset logic, never by widening stored state.
- **Import to 0-diff is the bar** for every stateful resource.
- **Config values are typed;** nested/dotted keys split into positional occ args.

## Toolchain

- Go 1.26.4 (`/home/jameson/.local/go`), `terraform-plugin-framework` v1.19.0.
  Do **not** add or bump deps — `go.mod`/`go.sum` mirror `../tofu-wordpress`.
- Provider address: `registry.terraform.io/jamesonrgrieve/nextcloud`; TypeName
  `nextcloud` (resources `nextcloud_*`).
- General Go / Terraform-provider standards are canonical at
  `/home/jameson/source/ai-prompts/go.md` and `.../tofu.md` — read them first;
  this file holds only repo-specific facts.
- `make check` (tidy + fmt + vet + test + build) is the gate; `.githooks/pre-commit`
  re-runs it. Enable with `git config core.hooksPath .githooks`. Never `--no-verify`.

## Hard rules

- **No secrets in the repo or state.** Nextcloud admin password and DB password
  are **write-only** attributes injected from OpenBao at apply. SSH auth is
  **key/cert only** — never a password, never `sshpass`.
- **Never apply against a production Nextcloud host by hand.** A bad
  `occ config:system:set` or a botched `occ upgrade` takes a live instance down.
  Validate only against a lab/clone install, and drive live changes via Semaphore.
- **NetBox is the source of truth** at the consumer layer — expressed **entirely
  as `netbox-services` `ServiceInstance` params** (there is **no dedicated NetBox
  plugin** for Nextcloud). DB creds resolve to OpenBao. Never hand-maintain
  competing tfvars.
