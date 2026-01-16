# SANtricity Go Client

A Go client library for the NetApp SANtricity Web Services API, initially extracted from NetApp Trident and then improved.

## Features

- Supports direct connection to E-Series arrays (no Web Services Proxy required).
- Supports SANtricity API 11.90+ (manages Volume creation with new size/unit parameters).
- Handles JWT/Bearer Token authentication.
- TLS options: load trusted TLS certificate chain, enable TLS certificate verification, disable certificate verification.

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
- **Pools**: `GetVolumePools`
- **Hosts**: `CreateHost`, `GetHostForIQN`

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
```

### Environment Variables

The CLI also supports setting credentials and endpoint via environment variables:

- `SANTRICITY_ENDPOINT`: The API endpoint IP or hostname (e.g. `10.0.0.1`).
- `SANTRICITY_USERNAME`: The username.
- `SANTRICITY_PASSWORD`: The password.
- `SANTRICITY_TOKEN`: The bearer token (if using token auth).
- `SANTRICITY_INSECURE`: Set to "true" to disable TLS verification.
- `SANTRICITY_CA_CERT`: Set to "/path/to/chain.pem" to use own certificate chain.

## License

Apache 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE.md) for attribution.

