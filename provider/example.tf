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

resource "santricity_host_group" "pg_cluster" {
  name = "pg-cluster"
}

resource "santricity_host" "pg_host_01" {
  name = "pg-01"
  type = "linux_dm_mp"
  host_group_id = santricity_host_group.pg_cluster.id
  ports {
    type  = "iscsi"
    port  = "iqn.1993-08.org.debian:01:postgres01"
    label = "pg01-iscsi"
  }
}

resource "santricity_host" "pg_host_02" {
  name = "pg-02"
  type = "linux_dm_mp"
  host_group_id = santricity_host_group.pg_cluster.id
  ports {
    type  = "iscsi"
    port  = "iqn.1993-08.org.debian:01:postgres02"
    label = "pg02-iscsi"
  }
}

resource "santricity_mapping" "pg_data_map" {
  volume_id = santricity_volume.pg_data.id
  # Because pg-01 is in a group, this will automatically map to the Group (Cluster).
  host_id   = santricity_host.pg_host_01.id
  lun       = 10 # Arbitrary LUN
}

resource "santricity_mapping" "pg_log_map" {
  volume_id = santricity_volume.pg_log.id
  # We can also map explicitly to the Group ID
  host_group_id = santricity_host_group.pg_cluster.id
  lun           = 11
}

