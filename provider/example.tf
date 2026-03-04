terraform {
  required_providers {
    santricity = {
      source  = "local/scaleoutsean/santricity"
      version = "1.0.0"
    }
  }
}

provider "santricity" {
  endpoint = var.endpoint
  # username = "admin"       # Optional if token is used
  # password = "password"    # Optional if token is used
  token    = var.token
  insecure = var.insecure
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
  host_group_id = santricity_host_group.pg_cluster.id
  lun           = 10 # Arbitrary LUN
}

resource "santricity_mapping" "pg_log_map" {
  volume_id = santricity_volume.pg_log.id
  # We can also map explicitly to the Group ID
  host_group_id = santricity_host_group.pg_cluster.id
  lun           = 11
}

# --- Snapshot Examples ---

# 1. Create a Snapshot Group for the base volume (pg_data)
resource "santricity_snapshot_group" "pg_data_snap_group" {
  base_volume_id        = santricity_volume.pg_data.id
  name                  = "pg-data-snaps"
  repository_percentage = 20
  warning_threshold     = 80
  auto_delete_limit     = 30
  full_policy           = "purgepit"
  storage_pool_id       = var.pool_id
}

# 2. Create a Snapshot Image (Instant Snapshot) from the Snapshot Group
resource "santricity_snapshot_image" "pg_data_snap_daily" {
  group_id = santricity_snapshot_group.pg_data_snap_group.id
}

# 3. Create a Snapshot Volume (Linked Clone/PiT View) from the Snapshot Image
resource "santricity_snapshot_volume" "pg_data_clone_dev" {
  snapshot_image_id     = santricity_snapshot_image.pg_data_snap_daily.id
  name                  = "pg-data-dev-clone"
  view_mode             = "readWrite"
  repository_percentage = 20
  repository_pool_id    = var.pool_id
  full_threshold        = 90
}

