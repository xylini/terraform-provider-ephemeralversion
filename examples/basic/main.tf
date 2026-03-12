terraform {
  required_providers {
    ephemeralversion = {
      source  = "xylini/ephemeralversion"
      version = "~> 0.1"
    }
  }
}

variable "secret_value" {
  type      = string
  sensitive = true
}

resource "ephemeralversion_from" "single" {
  provider = ephemeralversion
  name     = "db_password"
  value    = var.secret_value
}

resource "ephemeralversion_from_map" "multi" {
  provider = ephemeralversion
  values = {
    db_password  = var.secret_value
    api_key      = "static-api-key"
  }
}

output "single_id"       { value = ephemeralversion_from.single.id }
output "single_version"  { value = ephemeralversion_from.single.version }
output "map_id"          { value = ephemeralversion_from_map.multi.id }
output "map_versions"    { value = ephemeralversion_from_map.multi.versions }
