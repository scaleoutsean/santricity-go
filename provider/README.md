# Terraform Provider for NetApp SANtricity

This directory contains the source code for the SANtricity Terraform Provider.

## Volume Expansion Behavior

When expanding a volume resource (`santricity_volume`), the provider sends an expansion request to the storage array. The array processes this request asynchronously.

1.  **Immediate State Update**: The Terraform provider updates its state to reflect the new requested size immediately after the API accepts the request. It does **not** wait for the background expansion job to complete, which can take several minutes.
2.  **Background Process**: On the storage array, the volume enters an expansion state (initially "initializing", then progressing). 
3.  **Host Visibility**: Most modern operating systems will detect the capacity change automatically once the expansion process initializes, but there may be a brief delay while the job starts. If you need to verify completion programmatically, you can query the API for the volume's expansion job status, though Terraform considers the operation "complete" once the request is sent.

## Host Replacement Approach

Assuming a host dies or is due for hardware refresh:

- Make sure the host that's going to be replaced is offline
- Name the replacement host the same (e.g. `server2`)
- Update the replaced host's IQN or NQN in your Terraform plan
- `apply` will force removal of the offlined `server2`, and create `server2` with the new IQN/NQN. 
  - In the case of iSCSI clients where CHAP was set before, also update `chap_secret` with the new CHAP secret, or update your `iscsid.conf` with the old host's CHAP secret (if you have it)

## Host Update Approach

The provider supports in-place updates for the following Host attributes:

- **Name**: You can rename a host without disrupting connectivity.
- **Host Group**: You can move a host between host groups (or remove it from one).
- **Host Type**: You can change the operating system type (e.g., from `linux_dm_mp` to `vmware`).

**Note**: Changing the `ports` (IQN, NQN, or WWN) is considered a structural identity change and will force the destruction and recreation of the host resource to ensure integrity.

## Snapshots (Groups, Images, Volumes)

In terms of dependencies, linked clones ("snapshot volumes") and snapshots ("snapshot images") sit atop of snapshot groups.

The provider currently treats Snapshot Groups as immutable resources regarding their capacity settings.

- **Capacity**: Use the `repository_percentage` argument to set the initial capacity of the snapshot repository relative to the base volume.
- **Resizing**: There is currently no support for resizing an existing Snapshot Group's repository capacity in-place. Because that resource is immutable, changing `repository_percentage` will force the destruction and recreation of the Snapshot Group—which includes **deleting all contained snapshots and linked clones**. If you need to regularly grow snapshot repository capacity, create an enhancement request in Issues.
- **Best Practice**: Provision snapshot group capacity generously (e.g., 20-40% or more depending on change rate) to avoid running out of space, as expanding it later requires wiping your snapshot history and SANtricity snapshots are capacity-hungry.

Snapshot resource:

```hcl
resource "santricity_snapshot_image" "my_snapshot" {
  group_id = santricity_snapshot_group.my_group.id
}
```

Linked clone resource:

```hcl
resource "santricity_snapshot_volume" "my_clone" {
  name              = "my-clone-vol"
  snapshot_image_id = santricity_snapshot_image.my_snapshot.id
  view_mode         = "readWrite"
  repository_percentage = 20
}
```

See `example.tf` for an example on how to use these. Note that in practice we'd have to use consistency group snapshots for multi-volume applications, or else ensure that application and host I/O on volumes being snapshot are quiesced or stopped.

## Moving Volumes (Remapping)

If you need to move a volume from one host to another (e.g., from `host-a` to `host-b`):

1.  Locate the `santricity_mapping` resource for that volume in your Terraform configuration.
2.  Change the `host_id` parameter from `santricity_host.host_a.id` to `santricity_host.host_b.id`.
3.  Run `terraform apply`.
4.  Terraform will detect the change to the immutable `host_id` field. It will destroy the existing mapping (Unmap from A) and immediately create a new mapping (Map to B).

## Known Limitations

- NVMe/RoCE works, iSCSI should work, and FC might (needs testing with real hardware, but this provider does not aim for FC support)
- Hosts must have valid IQN (CHAP-only iSCSI is not supported) or NQN
- Snapshot implementation supports single volumes and consistency group snapshots that needs testing

## Install

```sh
# 1. Create the plugin directory structure
mkdir -p ~/.terraform.d/plugins/local/scaleoutsean/santricity/1.0.0/linux_amd64

# 2. Build the provider binary verify the path to your main.go
# Assuming you are in the root of the repo:
SANTRICITY_PLUGIN="$(whoami)/.terraform.d/plugins/local/scaleoutsean/santricity/1.0.0/linux_amd64/terraform-provider-santricity_v1.0.0"
go build -o ${SANTRICITY_PLUGIN} ./cmd/terraform-provider-santricity/

# 3. Initialize/Re-initialize Terraform in your example directory
cd provider
rm -rf .terraform .terraform.lock.hcl  # Optional: Clean up old init state
terraform init
```

To run the example, change variables in `vars.tf` or pass them to `terraform apply` using other methods supported by Terraform.

## Development

- The provider logic relies on the `santricity-go` client library.
- Use `make build` or `go build` **in the root** to build the entire project.
