package main

import (
	"context"
	"fmt"

	santricity "github.com/scaleoutsean/santricity-go"
)

func main() {
	// Example usage
	config := santricity.ClientConfig{
		ApiControllers: []string{"10.0.0.1", "10.0.0.2"},
		ApiPort:        8443,
		Username:       "admin",
		Password:       "password",
		// BearerToken: "your_token_here", // Optional
		VerifyTLS: false,
	}

	ctx := context.Background()
	client := santricity.NewAPIClient(ctx, config)

	fmt.Println("Client created:", client)

	// Create a generic VolumeCreateRequest to demonstrate new fields
	req := santricity.VolumeCreateRequest{
		VolumeGroupRef: "pool_ref",
		Name:           "my-volume",
		SizeUnit:       "gb",
		Size:           "10",
		RaidLevel:      "raid1",
		BlockSize:      512,
	}

	fmt.Printf("Request struct: %+v\n", req)
}
