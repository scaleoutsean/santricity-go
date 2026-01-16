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

## Moving Volumes (Remapping)

If you need to move a volume from one host to another (e.g., from `host-a` to `host-b`):

1.  Locate the `santricity_mapping` resource for that volume in your Terraform configuration.
2.  Change the `host_id` parameter from `santricity_host.host_a.id` to `santricity_host.host_b.id`.
3.  Run `terraform apply`.
4.  Terraform will detect the change to the immutable `host_id` field. It will destroy the existing mapping (Unmap from A) and immediately create a new mapping (Map to B).

## Known Limitations

- iSCSI only
- Hosts must have valid IQN or NQN (CHAP-only iSCSI is not supported)

## Live Test

This does stuff to your box (creates and removes small volumes and hosts/host groups) to confirm it works. Volumes, hosts, IQNs/NQNs, and host groups have randomized names to avoid naming conflicts.

Find a DDP that has 10 GiB of spare capacity and run these tests to try it. 

```sh
cd tests/live_test
go build -o live_tester main.go

export SANTRICITY_ENDPOINT="10.x.x.x"
export SANTRICITY_USERNAME="admin"
export SANTRICITY_PASSWORD=""
# how to find DDP pool ID: Swagger > Volumes > GET storage-pools
export SANTRICITY_POOL_ID="<your-pool-id>"

./live_tester
```

## Development

- The provider logic relies on the `santricity-go` client library.
- Use `make build` or `go build` in the root to build the project.
