package main

import (
	"log"
	"os"

	santricity "github.com/scaleoutsean/santricity-go"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

var (
	endpoint  string
	username  string
	password  string
	caCert    string
	insecure  bool
	debug     bool
	apiClient *santricity.Client
	ctx       context.Context
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "santricity-cli",
		Short: "CLI for NetApp SANtricity",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Validate flags
			if caCert != "" && insecure {
				log.Fatal("Error: --ca-cert and --insecure are mutually exclusive. Please choose one.")
			}

			// Initialize client
			var caCertPEM string
			if caCert != "" {
				certBytes, err := os.ReadFile(caCert)
				if err != nil {
					log.Fatalf("Error reading CA cert: %v", err)
				}
				caCertPEM = string(certBytes)
			}

			debugFlags := make(map[string]bool)
			if debug {
				debugFlags["api"] = true
				debugFlags["method"] = true
			}

			config := santricity.ClientConfig{
				ApiControllers:  []string{endpoint},
				ApiPort:         8443,
				Username:        username,
				Password:        password,
				VerifyTLS:       !insecure,
				CACertPEM:       caCertPEM,
				DebugTraceFlags: debugFlags,
			}
			ctx = context.Background()
			apiClient = santricity.NewAPIClient(ctx, config)

			// Establish connection to find the System ID
			if _, err := apiClient.Connect(ctx); err != nil {
				log.Fatalf("Error connecting to system: %v", err)
			}
		},
	}

	rootCmd.PersistentFlags().StringVar(&endpoint, "endpoint", "", "Controller IP/Hostname (required)")
	rootCmd.PersistentFlags().StringVar(&caCert, "ca-cert", "", "Path to CA Certificate file")
	rootCmd.PersistentFlags().StringVarP(&username, "username", "u", "admin", "Username")
	rootCmd.PersistentFlags().StringVarP(&password, "password", "p", "", "Password")
	rootCmd.PersistentFlags().BoolVar(&insecure, "insecure", false, "Skip TLS verification")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging")
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
