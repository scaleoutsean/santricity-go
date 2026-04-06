terraform {
  required_providers {
    santricity = {
      source  = "scaleoutsean/santricity"
      version = "~> 0.1.0"
    }
  }
}

provider "santricity" {
  # Configure this via SANTRICITY_URL, SANTRICITY_USERNAME, SANTRICITY_PASSWORD
}

# 1. Add the standalone Proxmox host
resource "santricity_host" "pve_standalone" {
  name          = "pve_standalone"
  host_type     = "linux" # Use appropriate host type
  
  # Ensure ports are defined correctly for your iSCSI or NVMe-oF initiators
  # port {
  #   type  = "iscsi"
  #   label = "iqn.1993-08.org.debian:01:pvestandalone"
  # }
}

# 2. Create the LUN for ZFS/Btrfs usage
resource "santricity_volume" "pve_local_lun" {
  name     = "PVE_Local_ZFS"
  pool_id  = "YOUR_DDP_POOL_ID" # Replace with actual pool ID
  size     = 5000               # Size in MB (5TB)
  segment_size = 128
}

# 3. Map the Volume directly to the single Host
resource "santricity_mapping" "pve_das_mapping" {
  lun        = 10
  volume_id  = santricity_volume.pve_local_lun.id
  target_id  = santricity_host.pve_standalone.id
}

output "volume_id" {
  value = santricity_volume.pve_local_lun.id
}
