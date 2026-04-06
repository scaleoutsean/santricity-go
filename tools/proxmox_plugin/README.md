# SANtricity Proxmox Plugin (LVM over SAN)

This plugin enables Proxmox VE to natively interact with SANtricity E-Series arrays in ways suitable for PVE 9.

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

Admins pre-provision the base SANtricity volume and configure the local Volume Group (`vgcreate`). The plugin then attaches to this VG via Proxmox's native LVM engine, adding SANtricity-specific monitoring and lifecycle hooks where appropriate. 

This is the lowest-risk approach and prevents PVE upgrades from breaking custom UI patches. Terraform recipes for creating these foundational LUNs (for both shared LVM and single-host ZFS/Btrfs) are provided alongside the plugin.

```sh
pvesm add santricity_lvm my_pve_disk1 \
  --vgname my_pve_lvm1 \
  --api_endpoint SANTRICITY_CONTROLLER_IP_ADDRESS \
  --username "monitor" \
  --password "fakePass" \
  --insecure 0 \
  --shared 1 \
  --saferemove 1 \
  --content "images,rootdir" \
  --snapshot-as-volume-chain 1

```

There are several basic decisions to make on how you want to use LUNs:

- `insecure` - if enabled, it ignores failed SANtricity TLS certificate validation (not recommended)
- `shared` - mandatory for LVMs on shared storage that need to be able to failover (i.e. be used from another host)
- `saferemove` - whether to perform VM/CT disk wipe after un-allocation, it's recommended to enable
- `content` - images and rootdir are common choices, both are recommended 
- `snapshot-as-volume-chain` is a new PVE tech preview feature, recommended to enable as it enables VM snapshots and improved backup

### Workflow: single-host filesystems (ZFS, Btrfs...)

Instead of mapping the same LUN to multiple hosts (HA pairs or entire PVE datacenter) and creating LVMs on shared storage, this approach creates LUNs in same-sized pairs and maps each to dedicated individual PVE host. Filesystem-level replication must be used to provide HA for data access.

These hosts essentially use E-Series as DAS. If filesystem compression is enabled (ZFS, Btrfs), replication can be set up between pairs of hosts, and overall space utilization will still be lower thanks to savings from filesystem compression.

This is useful for high-churn data stores, especially where VMs or CTs are cloned or deleted many times, and application workload is lighter than VM/CT workload.

This workflow works the same way (with the difference that each LUN is mapped to just one host) and can be easily set up with any SANtricity client or Terraform Provider, so shared LVM plugin does not need be used for that. Map E-Series devices to the host, and ensure the host accesses it on boot. All SANtricity clients (Terraform Provider, Go, PowerShell, Python) can automate this.

In the case of unplanned host failure, its LUN(s) can be mapped to a new (replacement) host, which can be done in seconds from SANtricity UI or in a Terraform Provider SANtricity workflow which has host replacement documented.

## Security Profile and Least-Privilege Design

Because this plugin intentionally leverages a "Bring Your Own LUN" architecture rather than aggressively provisioning blocks via Proxmox automatically, it adheres to extremely strict least-privilege security paradigms.

When the SANtricity system API endpoints are provided to Proxmox via the `--api_username` and `--api_password` flags, it only requires **Monitor (Read-Only)** role credentials from SANtricity! 

Because the storage lifecycle is offloaded to your pre-configured Proxmox Logical Volume loops or Terraform CI/CD pipelines, the Proxmox environment never needs dangerous Admin or Storage provisioning bindings sent across the hypervisor management plane.

Proxmox simply utilizes the SANtricity system APIs for heartbeat metrics and "Current Failure" visibility statuses to rapidly identify array-side issues before they impact Datacenter VMs.

## Possible feature enhancements

Using a more powerful SANtricity role, `storage` (Storage Administrator), we could do the entire workflow (`santricity-cli` already can do all of it), but storage admin can not just create, extend and map LUNs, but also delete them. There's no value in using that powerful role when the two-three steps (create, delete, map) can be easily done from a secure workstation, completely out-of-band and without storing (encrypted) storage administrator password on PVE.

My "Proxmox TUI" approach (used in [Firemox](https://github.com/scaleoutsean/firemox), a Promox/SolidFire TUI) originally created for PVE 8, is still vastly superior to what PVE 9 storage plugins do. The only ones that work in semi-meaningful ways are built-in storage plugins, but those also tend to have security as weak as the weakest PVE host. 

Until Proxmox improves their storage plugin approach and adds NVMe/RoCE support, TUIs remain the way to go. Get its SANtricity cousin SANmox [here](https://github.com/scaleoutsean/santricity-powershell/tree/master/sanmox).

This plugin could work in tandem with SANmox:

- SANmox currently creates plain shared LVM datastores, but it could create `santricity_lvm`-type datastores (it currently does not because that would burden the PVE administrator)
- SANmox could then know which datastores are SANtricity-based, and perform maintenance, management and monitoring actions without prompting user to identify SANtricity-based LVM datastores

This option will be evaluated once PVE adds support for NVMe/RoCE because without it end-to-end provisioning remains possible only for E-Series iSCSI storage.
