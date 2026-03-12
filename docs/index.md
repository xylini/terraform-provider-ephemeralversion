---
page_title: "ephemeralversion Provider"
description: |-
  The ephemeralversion provider derives stable, deterministic version strings from
  ephemeral (write-only) secret values without ever storing those secrets in Terraform state.
  It solves the problem of Terraform being unable to detect changes to ephemeral variables
  by producing a trackable MD5 version string that updates whenever the secret rotates,
  allowing dependent resources to be automatically updated without leaking secrets into state.
---

# ephemeralversion Provider

The ephemeralversion provider derives stable, deterministic version strings from
ephemeral (write-only) secret values without ever storing those secrets in Terraform state.
It solves the problem of Terraform being unable to detect changes to ephemeral variables
by producing a trackable MD5 version string that updates whenever the secret rotates,
allowing dependent resources to be automatically updated without leaking secrets into state.

Terraform's [ephemeral variables](https://developer.hashicorp.com/terraform/language/values/variables#sensitive-values-in-variables) 
are never persisted to state or plan output, which makes them ideal
for passing secrets. However, this creates a blind spot: because ephemeral values leave no
trace in state, **Terraform has no way to know when they change between applies**. If a
password or API key rotates, any resource that depends on it will not be updated unless you
force a replacement manually.

The **ephemeralversion** provider closes that gap. It accepts an ephemeral value and returns
its **MD5 hex digest** as an ordinary, non-ephemeral `version` string that Terraform _can_
track in state. The secret itself never touches the state file — only its fingerprint does.
When the secret rotates, the fingerprint changes, the `version` attribute is updated on the
next apply, and any resource referencing it is automatically flagged for an update.

~> **Terraform 1.11 required.** This provider relies on write-only attributes, which are
only supported in Terraform 1.11 and later. Using it with an older version will result in
a schema validation error.

-> **MD5 is used for fingerprinting only, not security.** The `version` output is an MD5 hex
digest. MD5 is intentionally chosen here because it produces a short, stable, deterministic
string — it is **not** used as a cryptographic hash and provides no confidentiality guarantee
for the input.

## Example Usage

```terraform
terraform {
  required_providers {
    ephemeralversion = {
      source  = "xylini/ephemeralversion"
      version = "~> 0.2"
    }
  }
}

provider "ephemeralversion" {}

# Declare an ephemeral variable — its value is never written to state or plan output
variable "db_password" {
  type      = string
  ephemeral = true
}

# Derive a version string from the ephemeral secret
resource "ephemeralversion_from" "db_password" {
  name  = "db_password"
  value = var.db_password  # write-only — passed to the provider at apply time, never stored in state
}

# Use both the ephemeral secret and its version in the same resource:
#   - password is write-only: the actual secret reaches the resource but is never stored
#   - password_version is stored in state and triggers an update whenever the secret rotates
resource "example_database_user" "app" {
  username         = "app"
  password         = var.db_password      # write-only field — secret is passed directly, not stored
  password_version = ephemeralversion_from.db_password.version
}

# Declare an ephemeral map variable — values are never written to state or plan output
variable "secrets" {
  type      = map(string)
  ephemeral = true
}

# Track several secrets at once
resource "ephemeralversion_from_map" "secrets" {
  values = var.secrets
}

# Use a single secret from the map alongside its version in another resource:
#   - password is write-only: the actual secret reaches the resource but is never stored
#   - password_version is stored in state and triggers an update whenever db_password rotates
resource "example_database_user" "app" {
  username         = "app"
  password         = var.secrets["db_password"]                    # write-only field — secret is passed directly, not stored
  password_version = ephemeralversion_from_map.secrets.versions["db_password"]
}
```

## Write-only inputs

~> **Write-only values are passed at apply time only.** The `value` and `values` attributes
are declared `WriteOnly`. Terraform passes them to the provider during apply but **never
writes them to state or plan output**. After apply, these attributes read back as `null` in
state — this is expected behaviour, not data loss.

## Using `ephemeralversion_from_map` safely

!> **Map keys are NOT secret and WILL be stored in state.** The `versions` output map mirrors
the keys of `values`. Those keys are written to the Terraform state file and appear in plain
text in plan output. You **must** use a non-sensitive descriptor as the key and the actual
secret as the value. Putting a secret in the key position will permanently expose it.

```terraform
# ✅ Correct — key is a name, value is the secret
resource "ephemeralversion_from_map" "ok" {
  values = {
    db_password = var.db_password
    api_key     = var.api_key
  }
}

# ❌ Dangerous — the secret itself becomes a state key
resource "ephemeralversion_from_map" "bad" {
  values = {
    "${var.db_password}" = "label"  # never do this
  }
}
```

## Schema

The provider has no configuration arguments. Simply declare it in `required_providers` and
call `provider "ephemeralversion" {}` with an empty body.
