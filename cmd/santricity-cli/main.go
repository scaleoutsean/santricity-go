package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	santricity "github.com/scaleoutsean/santricity-go"
	"github.com/spf13/cobra"
)

var (
	endpoint     string
	username     string
	password     string
	token        string
	caCert       string
	insecure     bool
	debug        bool
	outputFormat string
	apiClient    *santricity.Client
	ctx          context.Context
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "santricity-cli",
		Short: "CLI for NetApp SANtricity",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Check environment variables if flags are not set
			if endpoint == "" {
				endpoint = os.Getenv("SANTRICITY_ENDPOINT")
			}
			if username == "admin" && os.Getenv("SANTRICITY_USERNAME") != "" {
				// Only override default "admin" if env var is set.
				// If user explicitly set --username, cobra handles that via pflags priority usually,
				// but here we are checking the bound variable.
				// Optimally we'd check cmd.Flags().Changed("username") but "admin" is default.
				// Let's assume if Changed is false, use Env.
				if !cmd.Flags().Changed("username") {
					username = os.Getenv("SANTRICITY_USERNAME")
				}
			}
			if password == "" {
				password = os.Getenv("SANTRICITY_PASSWORD")
			}
			if token == "" {
				token = os.Getenv("SANTRICITY_TOKEN")
			}
			if !cmd.Flags().Changed("insecure") && os.Getenv("SANTRICITY_INSECURE") == "true" {
				insecure = true
			}
			if caCert == "" {
				caCert = os.Getenv("SANTRICITY_CA_CERT")
			}
			if endpoint == "" {
				log.Fatal("Error: --endpoint or SANTRICITY_ENDPOINT is required.")
			}

			// Validate flags
			if caCert != "" && insecure {
				log.Fatal("Error: --ca-cert and --insecure are mutually exclusive. Please choose one.")
			}

			if token != "" && (cmd.Flags().Changed("username") || cmd.Flags().Changed("password")) {
				log.Fatal("Error: --token is mutually exclusive with --username/--password.")
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
				BearerToken:     token,
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
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "Bearer Token")
	rootCmd.PersistentFlags().BoolVar(&insecure, "insecure", false, "Skip TLS verification")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging")
	// rootCmd.MarkPersistentFlagRequired("endpoint")

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

	var showRepoVols bool
	var getVolumesCmd = &cobra.Command{
		Use:   "volumes",
		Short: "List volumes",
		Run: func(cmd *cobra.Command, args []string) {
			apiClient.SetIncludeRepositoryVolumes(showRepoVols)
			vols, err := apiClient.GetVolumes(ctx)
			if err != nil {
				log.Fatalf("Error getting volumes: %v", err)
			}
			if outputFormat == "json" {
				b, err := json.MarshalIndent(vols, "", "  ")
				if err != nil {
					log.Fatalf("Error marshaling to JSON: %v", err)
				}
				fmt.Println(string(b))
			} else {
				for _, v := range vols {
					log.Printf("Volume: %s (Size: %s)", v.Label, v.VolumeSize)
				}
			}
		},
	}
	getVolumesCmd.Flags().BoolVar(&showRepoVols, "show-repo-vols", false, "Show internal repository volumes")

	var getPoolsCmd = &cobra.Command{
		Use:   "pools",
		Short: "List storage pools",
		Run: func(cmd *cobra.Command, args []string) {
			pools, err := apiClient.GetVolumePools(ctx, "", 0, "")
			if err != nil {
				log.Fatalf("Error getting pools: %v", err)
			}
			if outputFormat == "json" {
				b, err := json.MarshalIndent(pools, "", "  ")
				if err != nil {
					log.Fatalf("Error marshaling to JSON: %v", err)
				}
				fmt.Println(string(b))
			} else {
				for _, p := range pools {
					log.Printf("Pool: %s", p.Label)
					log.Printf("  ID: %s", p.VolumeGroupRef)
					log.Printf("  Media: %s", p.DriveMediaType)
					log.Printf("  PhyType: %s", p.DrivePhysicalType)
					log.Printf("  RAID: %s", p.RaidLevel)
					log.Printf("  Free: %s", p.FreeSpace)
				}
			}
		},
	}

	getCmd.AddCommand(getSystemCmd)
	getCmd.AddCommand(getVolumesCmd)
	getCmd.AddCommand(getPoolsCmd)
	rootCmd.AddCommand(getCmd)

	var createCmd = &cobra.Command{
		Use:   "create",
		Short: "Create resources",
	}

	var hostName, portID, portType, hostType, authSecret, groupID string
	var createHostCmd = &cobra.Command{
		Use:   "host",
		Short: "Create a host",
		Run: func(cmd *cobra.Command, args []string) {
			if hostName == "" || portID == "" || portType == "" || hostType == "" {
				log.Fatal("Error: --name, --port, --type, and --host-type are required")
			}
			hg := santricity.HostGroup{}
			if groupID != "" {
				hg.ClusterRef = groupID
				hg.Label = "group-" + groupID
			}
			h, err := apiClient.CreateHost(ctx, hostName, portID, portType, hostType, authSecret, hg)
			if err != nil {
				log.Fatalf("Error creating host: %v", err)
			}
			if outputFormat == "json" {
				b, err := json.MarshalIndent(h, "", "  ")
				if err != nil {
					log.Fatalf("Error marshaling to JSON: %v", err)
				}
				fmt.Println(string(b))
			} else {
				log.Printf("Created Host: %s (Ref: %s)", h.Label, h.HostRef)
			}
		},
	}
	createHostCmd.Flags().StringVar(&hostName, "name", "", "Host name")
	createHostCmd.Flags().StringVar(&portID, "port", "", "Host Port ID (IQN, NQN, WWN)")
	createHostCmd.Flags().StringVar(&portType, "type", "", "Port type (iscsi, nvmeof, fc, sas)")
	createHostCmd.Flags().StringVar(&hostType, "host-type", "", "Host type index (e.g., 28 for Linux_DM_MP)")
	createHostCmd.Flags().StringVar(&authSecret, "auth-secret", "", "CHAP Secret (iSCSI only)")
	createHostCmd.Flags().StringVar(&groupID, "group-id", "", "Host Group ID (optional)")

	createCmd.AddCommand(createHostCmd)
	rootCmd.AddCommand(createCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
