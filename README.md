[![Go](https://github.com/scaleoutsean/santricity-go/actions/workflows/go.yml/badge.svg)](https://github.com/scaleoutsean/santricity-go/actions/workflows/go.yml)

# SANtricity Go Client

A Go client library for the NetApp SANtricity API, initially extracted from NetApp Trident and subsequently substantially improved.

Sub-projects:

- [SANtricity Provider](./provider/) for Terraform
- [SANtricity CSI driver](./csi/) for Kubernetes

## Features

- Supports direct connection to E-Series arrays (no Web Services Proxy required).
- Handles JWT/Bearer Token authentication.
- Supports SANtricity API 11.90+ with iSCSI and NVMe/RoCE host-side interfaces.
- TLS options: load trusted TLS certificate chain, enable TLS certificate verification, disable certificate verification.
- Reporting-friendly CLI feature for show-back or charge-back.

For Terraform SANtricity Provider and SANtricity CSI, check their respective folders.

## Usage

### CLI

```bash
# Using Basic Auth
./santricity-cli --endpoint 10.0.0.1 --username admin --password "password" get system --insecure

# Using Bearer Token
./santricity-cli --endpoint 10.0.0.1 --token "eyJ..." get system --insecure
```

### Library Usage

```go
package main

import (
	"context"
	"fmt"
	santricity "github.com/scaleoutsean/santricity-go"
)

func main() {
	config := santricity.ClientConfig{
		ApiControllers: []string{"10.0.0.1", "10.0.0.2"}, // IPs of the controllers
		ApiPort:        8443,                             // Default HTTPS port
		Username:       "admin",
		Password:       "password",
		BearerToken:    "",                               // Optional Bearer token
		VerifyTLS:      false,                            // Set to true in production
	}

	ctx := context.Background()
	client := santricity.NewAPIClient(ctx, config)
    
    // Check connection
    sys, err := client.AboutInfo(ctx)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Connected to system: %s\n", sys.SystemID)
}
```

## Supported Operations

The library supports common storage management operations:

- **System**: `AboutInfo`, `GetStorageSystem`
- **Volumes**: `GetVolumes`, `CreateVolume`, `ResizeVolume`, `DeleteVolume`, `MapVolume`, `UnmapVolume`
- **Snapshots**: WIP
- **Pools**: `GetVolumePools`
- **Hosts**: `CreateHost`, `GetHostForPort`

## CLI

A simple CLI is provided to interact with the API. This can be built from the `cmd` directory.

```sh
# Get help
go run cmd/santricity-cli/main.go --help

# Example: Get system info (from /utils/about)
go run cmd/santricity-cli/main.go --endpoint 10.0.0.1 --insecure --password mypassword get system

# Example: Get system info with custom CA certificate and debug logging
go run cmd/santricity-cli/main.go --endpoint 10.0.0.1 --ca-cert /path/to/chain.pem --password mypassword --debug get system

# Example: List volumes
go run cmd/santricity-cli/main.go --endpoint 10.0.0.1 --insecure --password mypassword get volumes

# Example: List volumes' metadata JSON output 
santricity-cli get volumes -o json | jq '.[] | select(.metadata != null) | {label: .label, k8s_meta: .metadata}'

# Example: Create host (Linux, NVMe-oF (RoCE))
santricity-cli create host --name h3 --type nvmeof --port "nqn.2014-08.org.nvmexpress:uuid:b6087fac-aef6-4e75-85c1-abd7078c94f9" --host-type 28 --insecure

# Example: Create volume for a legacy application that needs 512 byte sector sizes on NVMe pool
santricity-cli create volume --name my-512-vol --size 10 --pool-id "040000006D039EA000493A26000004FD6996CBC0" --block-size 512 --insecure

# create a snapshot group; note that DDP allocates repository files in 8GiB increments while classic RAID volume groups are precise
santricity-cli create snapshot-group --name "backup-group" --volume-id "<VOLUME_REF>" --repo-pct 20

# create a snapshot image (instant point-in-time)
santricity-cli create snapshot-image --group-id "<GROUP_REF>"

# list snapshot images (shows timestamp and sequence number)
santricity-cli get snapshot-images --volume-name "my-data-vol"

# create a linked clone (writable volume) from a snapshot image and map it to a host (or cluster)
santricity-cli create snapshot-volume \
  --name "clone-for-dev" \
  --image-id "<PIT_REF>" \
  --host-id "<HOST_REF>"

# Example: Get volume by name and output as JSON
santricity-cli get volumes --volume-name "snap-vol-1" -o json

# Example: List hosts (to get hostRef)
santricity-cli get hosts

# Example: Map the snapshot volume to the host (using IDs from above)
santricity-cli create mapping --volume-id <VOLUME_REF> --target-id <HOST_REF> --lun 0
```

### Snapshot Management

The CLI supports creating and managing snapshots (PiT) and snapshot volumes (Linked Clones).

Related concepts ([official FAQs](https://docs.netapp.com/us-en/e-series-santricity/sm-storage/faq-snapshots.html#what-is-a-snapshot-group)):
- snapshot group - due to Copy-on-Write (CoW), when a snapshot for a volume is created, modified data blocks are evacuated to "snapshot reserve" volume(s) that store Point-in-Time (PiT) data.
- snapshot image - that's a read-only snapshot 
- snapshot volume - that's a clone linked to its base volume via snapshots, elsewhere known as "linked clone". SANtricity supports read-only and read-write (these need own reserve on top of snapshot group, which is used by base volumes) linked clones.

**NOTES:** 
- when presenting linked clones (confusingly called "snapshot volumes") to a host, you are responsible for "re-signaturing" them and dealing with any host "confusion" that may result from that. It is generally safer to map a clone to a host that cannot see the base volume at the same time.
- DDP allocates snapshot group capacity in 8GiB units, so array snapshots on volumes smaller than 16GiB are space-inefficient (16G x 25pct = 4GiB)
- SANtricity snapshots are space-hungry. It is recommended to only create ephemeral snapshots (that are kept long enough to be used for temporary protection, backup workflows or quick testing, and then deleted)

1. **Create a Snapshot Group** (required for the first snapshot of a volume):
   The `jq` utility can help you find the volumeRef if your system has many volumes.
   ```bash
   santricity-cli get volumes -o json | jq -r '.[] | select(.label == "my-vol") | .volumeRef'
   santricity-cli create snapshot-group --volume-id "0200000060080E500043C0B80000062C5D6C963B" --name group-vol1 --repo-pct 20
   ```

2. **Create a Snapshot Image** (Instant Snapshot):
   ```bash
   # Get the Group ID first
   santricity-cli get snapshot-groups
   
   # Create a snapshot in that group
   santricity-cli create snapshot-image --group-id "0400000060080E500043C0B80000062D5D6C963E"
   ```

3. **Create a Snapshot Volume** (Linked Clone from a specific Snapshot Image):
   ```bash
   # Get the Snapshot Image ID
   santricity-cli get snapshot-images
   
   # Create a read-only clone
   santricity-cli create snapshot-volume --image-id "4200000060080E500043C0B80000062E5D6C9641" --name clone-vol1-test --mode readOnly
   ```
4. **Rollback a Volume from Snapshot Image**
   ```bash
   # unmap base volume or stop client access (unmount) during rollback
   santricity-cli rollback volume --image-id "4200000060080E500043C0B80000062E5D6C9641"
   ```

### Wrap Go CLI in Python scripts

```python
import subprocess, json

output = subprocess.check_output(["santricity-cli", "get", "volumes", "-o", "json"])
data = json.loads(output)
# data now contains a list of dicts with all fields, including 'metadata' (if available)
```

### Environment Variables

The CLI also supports setting credentials and endpoint via environment variables:

- `SANTRICITY_ENDPOINT`: The API endpoint IP or hostname (e.g. `10.0.0.1`).
- `SANTRICITY_USERNAME`: The username.
- `SANTRICITY_PASSWORD`: The password.
- `SANTRICITY_TOKEN`: The bearer token (if using token auth).
- `SANTRICITY_INSECURE`: Set to "true" to disable TLS verification.
- `SANTRICITY_CA_CERT`: Set to "/path/to/chain.pem" to use own certificate chain.

## Implementation Notes

### Terminology 

The SANtricity UI avoids exposing Repo Groups and doesn't even have a name for them. Other terms may be confusing. Some examples:

| Concept | Single Vol | Consistency Group |
| --------| -----------| ------------------|
| Base Snapshot Engine (Logic & Settings) | SnapshotGroup | ConsistencyGroup |
| The Point-in-Time (Frozen) Snapshot Image | SnapshotImage | ConsistencyGroupSnapshot |
| The Mountable Snapshot-linked Clone | SnapshotVolume | ConsistencyGroupView |
| Hidden Evacuation Storage (CoW reserve) | ConcatRepositoryVolume | ConcatRepositoryVolume |

The above may be expanded with Repo Group terms later.

See the official documentation (including [TR-4747](https://www.netapp.com/media/17167-tr4747.pdf)) and my blog for more on SANtricity snapshots.

## License

Apache 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE.md) for attribution.

