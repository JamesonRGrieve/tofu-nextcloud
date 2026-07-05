# SPDX-License-Identifier: AGPL-3.0-or-later
# Provider configuration. Credentials are injected at apply from OpenBao via
# TF_VAR_* / ephemeral resources — never hard-coded here.

terraform {
  required_providers {
    nextcloud = {
      source  = "jamesonrgrieve/nextcloud"
      version = "~> 0.1"
    }
  }
}

provider "nextcloud" {
  host     = "nextcloud.example.internal" # Nextcloud host, reached over SSH
  user     = "root"                       # SSH login user
  docroot  = "/var/www/nextcloud"         # default document root (holds occ)
  web_user = "www-data"                   # occ runs as this user via sudo

  # SSH auth is key/cert only. Prefer an OpenBao-signed key materialized at apply:
  ssh_key_pem = var.ssh_signed_key # sensitive; supplied via TF_VAR_ssh_signed_key
  # ssh_key_file = "/home/runner/.ssh/id_nc"   # or an on-disk identity
}

variable "ssh_signed_key" {
  description = "OpenBao-signed SSH private key (PEM). Injected at apply, never committed."
  type        = string
  sensitive   = true
}
