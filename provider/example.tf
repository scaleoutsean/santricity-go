terraform {
  required_providers {
    santricity = {
      source  = "local/scaleoutsean/santricity"
      version = "1.0.0"
    }
  }
}

provider "santricity" {
  endpoint = "10.0.0.1"      # Replace with your Controller IP
  # username = "admin"       # Optional if token is used
  # password = "password"    # Optional if token is used
  token    = "eyJ..."        # Optional if username/password is used
  insecure = true
}

resource "santricity_volume" "pg_data" {
  name       = "pg_data_vol"
  pool_id    = var.pool_id
  size_gb    = 50
  raid_level = "raid6"
}

resource "santricity_volume" "pg_log" {
  name       = "pg_log_vol"
  pool_id    = var.pool_id
  size_gb    = 10
  raid_level = "raid1"
}

variable "pool_id" {
  type        = string
  description = "The DDP Pool ID (Ref) to provision volumes in."
  default     = "04000000600A098000E3C1B000002CED62CF874D"
}
