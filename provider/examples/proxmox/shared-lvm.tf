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

# 1. Define the Proxmox Cluster as a Host Group
resource "santricity_host_group" "pve_cluster" {
  name = "pve_cluster_hg"
}

# 2. Add individual Proxmox hosts to the Host Group
resource "santricity_host" "pve_node1" {
  name          = "pve_node1"
  host_group_id = santricity_host_group.pve_cluster.id
  host_type     = "linux" # Use appropriate host type
  
  # Ensure ports are defined correctly for your iSCSI or NVMe-oF initiators
  # port {
  #   type  = "iscsi"
  #   label = "iqn.1993-08.org.debian:01:pvenode1"
  # }
}

resource "santricity_host" "pve_node2" {
  name          = "pve_node2"
  host_group_id = santricity_host_group.pve_cluster.id
  host_type     = "linux" 
  
  # port {
  #   type  = "iscsi"
  #   label = "iqn.1993-08.org.debian:01:pvenode2"
  # }
}

# 3. Create the "Big LUN" for the Shared LVM
resource "santricity_volume" "pve_lvm_lun" {
  name     = "PVE_Shared_LVM"
  pool_id  = "YOUR_DDP_POOL_ID" # Replace with actual pool ID
  size     = 10000              # Size in MB (10TB)
  segment_size = 128
}

# 4. Map the Volume to the entire Proxmox Cluster Host Group
resource "santricity_mapping" "pve_lvm_mapping" {
  lun        = 10
  volume_id  = santricity_volume.pve_lvm_lun.id
  target_id  = santricity_host_group.pve_cluster.id
}

output "volume_id" {
  value = santricity_volume.pve_lvm_lun.id
}
