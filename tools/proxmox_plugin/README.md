# SANtricity Proxmox Plugin (LVM over SAN)

This plugin enables Proxmox VE to natively interact with SANtricity E-Series arrays in ways suitable for PVE 9.

## About `santricity_lvm`

The initial release used a custom health-check. 

That has been removed to simplify and improve the plugin. The plugin now requires no credentials of any kind. It's merely a metadata store for a TUI, SANmox. If you don't use SANmox (or build a Web UI or TUI of your own) there's no reason to use this plugin - just use PVE LVM Plugin.

Since the initial release of santricity_lvm I've realized that my "Proxmox TUI" approach (first used in [Firemox](https://github.com/scaleoutsean/firemox), a Promox/SolidFire TUI) is still vastly superior to what PVE 9 storage plugins do.

Until Proxmox improves their storage plugin approach and adds NVMe/RoCE support, TUIs remain the way to go. Get its SANtricity cousin SANmox [here](https://github.com/scaleoutsean/santricity-powershell/tree/master/sanmox).

SANmox initially created standard PVE shared LVM datastores while `santricity_lvm`-type datastores were optional. 

Now SANmox requires `santricity_lvm`, and thanks to that it knows *which* SANtricity systems and datastores are being used, which means SANmox can perform maintenance, management and monitoring actions without prompting user to identify SANtricity-based LVM datastores or arrays.

## Architecture: "The Big LUN" Approach

This plugin extends Proxmox's native `LVMPlugin`. 

Instead of creating dozens or hundreds of individual LUNs directly on the array (which can hit scaling limits and complex failover dependencies), this plugin provisions large, high-performance SANtricity volumes (e.g., 16TB - 64TB) backed by DDPs. These "Big LUNs" are mapped to the PVE Host Group (PVE Datacenter). 

Proxmox then automatically formats these volumes as clustered `LVM` groups, allowing you to rapidly provision hundreds of VM disks (`.raw` or differencing `qcow2` snapshot chains) natively via the Proxmox UI, completely offloading the churn from the array controllers to the hypervisor file layer.

At the same time - thanks to experimental snapshot support on LVM in PVE 9 - this also eliminates the need for storage snapshots which, while supported and available, add little to no value to PVE with shared LVM datastores.

## Installation

1. Copy the compiled `santricity-cli` binary to `/usr/local/bin/` on **all** Proxmox nodes and make it executable.
2. Copy `SANtricityPlugin.pm` to `/usr/share/perl5/PVE/Storage/Custom/SANtricityPlugin.pm` on **all** Proxmox nodes and make it executable.

As you consume E-Series LUNs, register them using the plugin via the usual `pvesm` command, which stores LVM configuration in Proxmox Datacenter storage config (`/etc/pve/storage.cfg`).

### Workflow: Pre-provisioned (Bring Your Own LUN) for shared LVM

Rather than aggressively hijacking the Proxmox UI or automatically provisioning storage upon plugin addition, this plugin relies on a **"Bring Your Own LUN"** approach.

Admins pre-provision the base SANtricity volume and configure the local Volume Group (`vgcreate`). The plugin then attaches to this VG via Proxmox's native LVM engine, "tagging" LVM datastore with SANtricity system details. 

This is the lowest-risk approach: it prevents PVE upgrades from breaking custom UI patches and doesn't require any array credentials. Terraform recipes for creating these foundational LUNs (for both shared LVM and single-host ZFS/Btrfs) are provided alongside the plugin.

```sh
pvesm add santricity_lvm my_pve_disk1 \
  --vgname my_pve_lvm1 \
  --array_serial 952103002724 \
  --shared 1 \
  --saferemove 1 \
  --content "images,rootdir" \
  --snapshot-as-volume-chain 1

```

There are several basic decisions to make on how you want to use LUNs:

- `shared` - mandatory for LVMs on shared storage that need to be able to failover (i.e. be used from another host)
- `saferemove` - whether to perform VM/CT disk wipe after un-allocation, it's recommended to enable
- `content` - images and rootdir are common choices, both are recommended 
- `snapshot-as-volume-chain` is a new PVE tech preview feature, recommended to enable as it enables VM snapshots and improved backup

You may get your array's serial number with:

```sh
curl -ks -X GET 'https://USER:PASS@CONTROLLER:8443/devmgr/v2/storage-systems/1' | jq .chassisSerialNumber
```

### Workflow: single-host filesystems (ZFS, Btrfs...)

Instead of mapping the same LUN to multiple hosts (HA pairs or entire PVE datacenter) and creating LVMs on shared storage, this approach creates LUNs in same-sized pairs and maps each to dedicated individual PVE host. Filesystem-level replication must be used to provide HA for data access.

These hosts essentially use E-Series as DAS. If filesystem compression is enabled (ZFS, Btrfs), replication can be set up between pairs of hosts, and overall space utilization will still be lower thanks to savings from filesystem compression.

This is useful for high-churn data stores, especially where VMs or CTs are cloned or deleted many times, and application workload is lighter than VM/CT workload.

This workflow works the same way (with the difference that each LUN is mapped to just one host) and can be easily set up with any SANtricity client or Terraform Provider, so shared LVM plugin does not need be used for that. Map E-Series devices to the host, and ensure the host accesses it on boot. All SANtricity clients (Terraform Provider, Go, PowerShell, Python) can automate this.

In the case of unplanned host failure, its LUN(s) can be mapped to a new (replacement) host, which can be done in seconds from SANtricity UI or in a Terraform Provider SANtricity workflow which has host replacement documented.

## Security Profile and Least-Privilege Design

Because this plugin intentionally leverages a "Bring Your Own LUN" architecture rather than aggressively provisioning blocks via Proxmox automatically, it introduces no additional risks compared to built-in PVE shared LVM plugin.

By design, **zero** API credentials are kept on the Proxmox nodes. The `SANtricityPlugin.pm` module acts purely as a metadata registry, storing only routing attributes like `array_serial`. This fully eliminates the risk of an array-level compromise even if a Proxmox host is completely breached or hit with ransomware.

Because the storage lifecycle is offloaded to your pre-configured Proxmox Logical Volume loops, external TUIs, or Terraform CI/CD pipelines, the Proxmox environment never needs dangerous Admin, Storage, or even Read-Only provisioning bindings sent across the hypervisor management plane.

Proxmox simply utilizes native LVM I/O paths for health checks, offloading rich analytics and management to air-gapped external tooling that can safely utilize the metadata bridges defined in this plugin.
