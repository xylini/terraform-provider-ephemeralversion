# terraform-provider-ephemeralversion

[![Tests](https://github.com/xylini/terraform-provider-ephemeralversion/actions/workflows/test.yml/badge.svg)](https://github.com/xylini/terraform-provider-ephemeralversion/actions/workflows/test.yml)
[![Release](https://github.com/xylini/terraform-provider-ephemeralversion/actions/workflows/release.yml/badge.svg)](https://github.com/xylini/terraform-provider-ephemeralversion/actions/workflows/release.yml)

A Terraform provider for deriving stable, deterministic version strings from secret values — without ever storing those secrets in state.

### Why does this exist?

Terraform's [`ephemeral` variables](https://developer.hashicorp.com/terraform/language/values/variables#sensitive-values-in-variables) 
are never persisted to state, which is great for secrets — but it also means Terraform has no way to detect when their value has changed between applies. 
This provider solves that problem: it takes an ephemeral value and produces a **non-ephemeral `version` string** (the MD5 hex digest of the input) 
that Terraform *can* track in state. When the secret rotates, the version changes, and any resources that depend on 
it will be updated automatically — no secrets ever touch the state file.

- **Registry:** [registry.terraform.io/providers/xylini/ephemeralversion](https://registry.terraform.io/providers/xylini/ephemeralversion)
- **Source:** [github.com/xylini/terraform-provider-ephemeralversion](https://github.com/xylini/terraform-provider-ephemeralversion)

---

## Requirements

| Tool | Minimum version |
|------|----------------|
| [Terraform](https://developer.hashicorp.com/terraform/downloads) | 1.11 |
| [Go](https://golang.org/) *(only for building from source)* | 1.25 |

---

## Quick start

```hcl
terraform {
  required_providers {
    ephemeralversion = {
      source  = "xylini/ephemeralversion"
      version = "~> 0.1"
    }
  }
}

provider "ephemeralversion" {}

# Single secret → single version string
resource "ephemeralversion_from" "app" {
  name  = "db_password"
  value = var.db_password   # write-only, never stored in state
}

output "app_version" {
  value = ephemeralversion_from.app.version
  # e.g. "fd93bfd1b5de7e5ee5e7e06f2ad28f7d"
}

# Multiple secrets → map of version strings
resource "ephemeralversion_from_map" "secrets" {
  values = {
    db_password = var.db_password
    api_key     = var.api_key
  }
}

output "secret_versions" {
  value = ephemeralversion_from_map.secrets.versions
  # e.g. { db_password = "fd93...", api_key = "1a2b..." }
}
```

See [`examples/basic`](examples/basic) for a complete working configuration.

---

## Resources

### `ephemeralversion_from`

Computes an MD5 hex digest from a single secret string.

#### Schema

| Attribute | Type   | Required | Computed | Write-only | Description |
|-----------|--------|:--------:|:--------:|:----------:|-------------|
| `id`      | string |          | ✓        |            | UUID generated on creation, never changed. |
| `name`    | string | ✓        |          |            | A human-readable name for this resource. Stored in state. |
| `value`   | string | ✓        |          | ✓          | Secret input. Never stored in state. |
| `version` | string |          | ✓        |            | MD5 hex digest of `value`. Recalculated on every apply. |

#### Example

```hcl
resource "ephemeralversion_from" "app" {
  name  = "db_password"
  value = var.secret_value
}

output "version" {
  value = ephemeralversion_from.app.version
}
```

---

### `ephemeralversion_from_map`

Computes MD5 hex digests for a map of secret strings. Useful when you have several secrets and want to track each one independently.

> [!WARNING]
> **Map keys are NOT secret.** The `versions` map (which mirrors the keys of `values`) is stored in Terraform state and visible in plan output. Always use a **non-sensitive name** as the key (e.g. `"db_password"`, `"api_key"`) and the **actual secret** as the value. If you accidentally put a secret as a key, it will be written to state in plain text and exposed in logs.
>
> ✅ **Correct**
> ```hcl
> values = {
>   db_password = var.db_password   # key = name, value = secret
>   api_key     = var.api_key
> }
> ```
> ❌ **Dangerous — secret exposed in state**
> ```hcl
> values = {
>   "${var.db_password}" = "some_label"   # key IS the secret — never do this
> }
> ```

#### Schema

| Attribute  | Type        | Required | Computed | Write-only | Description |
|------------|-------------|:--------:|:--------:|:----------:|-------------|
| `id`       | string      |          | ✓        |            | UUID generated on creation, never changed. |
| `values`   | map(string) | ✓        |          | ✓          | Map of secret names to values. Never stored in state. |
| `versions` | map(string) |          | ✓        |            | Map of secret names to MD5 hex digests. Recalculated on every apply. |

#### Example

```hcl
resource "ephemeralversion_from_map" "secrets" {
  values = {
    db_password = var.db_password
    api_key     = var.api_key
  }
}

output "versions" {
  value = ephemeralversion_from_map.secrets.versions
}
```

---

## How it works

1. `value` / `values` are declared with `WriteOnly: true` (Terraform 1.11+). Terraform passes the value during apply but **never writes it to state**.
2. During plan, the provider reads the config value and computes the MD5 digest, so Terraform can show you exactly what `version` will be before you apply.
3. `id` is a UUID generated once on creation and preserved on every subsequent update.

> **Why MD5?** MD5 is used purely as a fingerprinting mechanism — not for cryptographic security. It produces a short, stable, deterministic string that changes whenever the input changes.

---

## Local development

### Build

```zsh
go build -o terraform-provider-ephemeralversion .
```

### Install for local Terraform use

```zsh
PLUGIN_DIR=~/.terraform.d/plugins/registry.terraform.io/xylini/ephemeralversion/0.1.0/$(go env GOOS)_$(go env GOARCH)
mkdir -p "$PLUGIN_DIR"
cp terraform-provider-ephemeralversion "$PLUGIN_DIR/"
```

### Run the example

The example uses `dev_overrides` so Terraform skips the registry lookup entirely.

```zsh
cd examples/basic
export TF_CLI_CONFIG_FILE=./dev.tfrc
TF_VAR_secret_value="hello" terraform apply -auto-approve
```

### Run tests

```zsh
go test ./...
```

### Regenerate docs

Docs in [`docs/`](docs/) are generated by [tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs). Re-run this after any schema change:

```zsh
go build -o terraform-provider-ephemeralversion .
tfplugindocs generate \
  --provider-name "ephemeralversion" \
  --rendered-provider-name "ephemeralversion"
```

---

## Releasing

Releases are fully automated via [GoReleaser](https://goreleaser.com) and GitHub Actions. To cut a new release, tag and push:

```zsh
git tag v0.1.0
git push origin v0.1.0
```

The [`release.yml`](.github/workflows/release.yml) workflow builds binaries for all platforms, signs the `SHA256SUMS` file with GPG key, and creates a GitHub Release. 
The Terraform Registry picks up the release via webhook within minutes.
