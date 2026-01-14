package main

import (
	"log"

	santricity "github.com/scaleoutsean/santricity-go"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

var (
	endpoint  string
	username  string
	password  string
	insecure  bool
	apiClient *santricity.Client
	ctx       context.Context
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "santricity-cli",
		Short: "CLI for NetApp SANtricity",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Initialize client
			config := santricity.ClientConfig{
				ApiControllers: []string{endpoint},
				ApiPort:        8443,
				Username:       username,
				Password:       password,
				VerifyTLS:      !insecure,
			}
			ctx = context.Background()
			apiClient = santricity.NewAPIClient(ctx, config)
		},
	}

	rootCmd.PersistentFlags().StringVar(&endpoint, "endpoint", "", "Controller IP/Hostname (required)")
	rootCmd.PersistentFlags().StringVarP(&username, "username", "u", "admin", "Username")
	rootCmd.PersistentFlags().StringVarP(&password, "password", "p", "", "Password")
	rootCmd.PersistentFlags().BoolVar(&insecure, "insecure", false, "Skip TLS verification")
	rootCmd.MarkPersistentFlagRequired("endpoint")

	var getCmd = &cobra.Command{
		Use:   "get",
		Short: "Get resources",
	}

	var getSystemCmd = &cobra.Command{
		Use:   "system",
		Short: "Get system info",
		Run: func(cmd *cobra.Command, args []string) {
			sys, err := apiClient.AboutInfo(ctx)
			if err != nil {
				log.Fatalf("Error getting system info: %v", err)
			}
			log.Printf("System ID: %s, Version: %s", sys.SystemID, sys.Version)
		},
	}

	var getVolumesCmd = &cobra.Command{
		Use:   "volumes",
		Short: "List volumes",
		Run: func(cmd *cobra.Command, args []string) {
			vols, err := apiClient.GetVolumes(ctx)
			if err != nil {
				log.Fatalf("Error getting volumes: %v", err)
			}
			for _, v := range vols {
				log.Printf("Volume: %s (Size: %s)", v.Label, v.VolumeSize)
			}
		},
	}

	getCmd.AddCommand(getSystemCmd)
	getCmd.AddCommand(getVolumesCmd)
	rootCmd.AddCommand(getCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
