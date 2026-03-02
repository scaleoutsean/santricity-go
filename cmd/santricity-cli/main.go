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

	var volName, volPoolID, volSizeStr, volMediaType, volFSType, volRaidLevel string
	var volBlockSize int
	var createVolumeCmd = &cobra.Command{
		Use:   "volume",
		Short: "Create a volume",
		Run: func(cmd *cobra.Command, args []string) {
			if volName == "" || volPoolID == "" || volSizeStr == "" {
				log.Fatal("Error: --name, --pool-id, and --size are required")
			}
			// Parse size (simple GB assumption for CLI or parse bytes? Client usage implies bytes)
			// Let's assume input is GB for CLI convenience, or bytes?
			// The snippet in README says "size" but doesn't specify unit.
			// Let's assume GB for simplicity in CLI.
			// Wait, client.CreateVolume takes bytes.
			var sizeGB uint64
			fmt.Sscanf(volSizeStr, "%d", &sizeGB) // Simple parsing
			sizeBytes := sizeGB * 1024 * 1024 * 1024

			vol, err := apiClient.CreateVolume(ctx, volName, volPoolID, sizeBytes, volMediaType, volFSType, volRaidLevel, volBlockSize, 0, nil)
			if err != nil {
				log.Fatalf("Error creating volume: %v", err)
			}
			if outputFormat == "json" {
				b, err := json.MarshalIndent(vol, "", "  ")
				if err != nil {
					log.Fatalf("Error marshaling to JSON: %v", err)
				}
				fmt.Println(string(b))
			} else {
				log.Printf("Created Volume: %s (Ref: %s, BlockSize: %d)", vol.Label, vol.VolumeRef, vol.BlockSize)
			}
		},
	}
	createVolumeCmd.Flags().StringVar(&volName, "name", "", "Volume name")
	createVolumeCmd.Flags().StringVar(&volPoolID, "pool-id", "", "Pool ID (Volume Group Ref)")
	createVolumeCmd.Flags().StringVar(&volSizeStr, "size", "", "Size in GB")
	createVolumeCmd.Flags().StringVar(&volMediaType, "media-type", "hdd", "Media Type (hdd, ssd, nvme)")
	createVolumeCmd.Flags().StringVar(&volFSType, "fstype", "xfs", "Filesystem Type")
	createVolumeCmd.Flags().StringVar(&volRaidLevel, "raid-level", "raid6", "RAID Level")
	createVolumeCmd.Flags().IntVar(&volBlockSize, "block-size", 0, "Block Size (e.g. 512, 4096)")

	createCmd.AddCommand(createVolumeCmd)
	rootCmd.AddCommand(createCmd)

	getCmd.AddCommand(getSnapshotGroupsCmd)
	getCmd.AddCommand(getSnapshotImagesCmd)

	createCmd.AddCommand(createSnapshotGroupCmd)
	createCmd.AddCommand(createSnapshotImageCmd)
	createCmd.AddCommand(createSnapshotVolumeCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

var getSnapshotGroupsCmd = &cobra.Command{
	Use:   "snapshot-groups",
	Short: "Get all Snapshot Groups",
	Run: func(cmd *cobra.Command, args []string) {
		groups, err := apiClient.GetSnapshotGroups(ctx)
		if err != nil {
			log.Fatalf("Error getting snapshot groups: %v", err)
		}
		if outputFormat == "json" {
			jsonData, _ := json.MarshalIndent(groups, "", "  ")
			fmt.Println(string(jsonData))
		} else {
			fmt.Printf("%-36s %-20s %-10s\n", "ID", "Label", "Status")
			for _, g := range groups {
				fmt.Printf("%-36s %-20s %-10s\n", g.PitGroupRef, g.Label, g.Status)
			}
		}
	},
}

var getSnapshotImagesCmd = &cobra.Command{
	Use:   "snapshot-images",
	Short: "Get all Snapshot Images",
	Run: func(cmd *cobra.Command, args []string) {
		images, err := apiClient.GetSnapshotImages(ctx)
		if err != nil {
			log.Fatalf("Error getting snapshot images: %v", err)
		}
		if outputFormat == "json" {
			jsonData, _ := json.MarshalIndent(images, "", "  ")
			fmt.Println(string(jsonData))
		} else {
			fmt.Printf("%-36s %-36s %-10s\n", "PitRef", "PitGroupRef", "Status")
			for _, i := range images {
				fmt.Printf("%-36s %-36s %-10s\n", i.PitRef, i.PitGroupRef, i.Status)
			}
		}
	},
}

var createSnapshotGroupCmd = &cobra.Command{
	Use:   "snapshot-group",
	Short: "Create a Snapshot Group",
	Run: func(cmd *cobra.Command, args []string) {
		volID, _ := cmd.Flags().GetString("volume-id")
		name, _ := cmd.Flags().GetString("name")
		repoPct, _ := cmd.Flags().GetInt("repo-pct")

		req := santricity.SnapshotGroupCreateRequest{
			BaseMappableObjectId: volID,
			Name:                 name,
			RepositoryPercentage: repoPct,
			WarningThreshold:     80,
			AutoDeleteLimit:      30,
			FullPolicy:           "purgepit",
		}
		group, err := apiClient.CreateSnapshotGroup(ctx, req)
		if err != nil {
			log.Fatalf("Error creating snapshot group: %v", err)
		}
		if outputFormat == "json" {
			jsonData, _ := json.MarshalIndent(group, "", "  ")
			fmt.Println(string(jsonData))
		} else {
			fmt.Printf("Created Snapshot Group: %s (ID: %s)\n", group.Label, group.PitGroupRef)
		}
	},
}

var createSnapshotImageCmd = &cobra.Command{
	Use:   "snapshot-image",
	Short: "Create a Snapshot Image (Instant Snapshot)",
	Run: func(cmd *cobra.Command, args []string) {
		groupID, _ := cmd.Flags().GetString("group-id")

		req := santricity.SnapshotImageCreateRequest{
			GroupId: groupID,
		}
		image, err := apiClient.CreateSnapshotImage(ctx, req)
		if err != nil {
			log.Fatalf("Error creating snapshot image: %v", err)
		}
		if outputFormat == "json" {
			jsonData, _ := json.MarshalIndent(image, "", "  ")
			fmt.Println(string(jsonData))
		} else {
			fmt.Printf("Created Snapshot Image: %s (Group: %s)\n", image.PitRef, image.PitGroupRef)
		}
	},
}

var createSnapshotVolumeCmd = &cobra.Command{
	Use:   "snapshot-volume",
	Short: "Create a Snapshot Volume (Linked Clone)",
	Run: func(cmd *cobra.Command, args []string) {
		snapImageID, _ := cmd.Flags().GetString("image-id")
		name, _ := cmd.Flags().GetString("name")
		accessMode, _ := cmd.Flags().GetString("mode")
		repoPct, _ := cmd.Flags().GetFloat64("repo-pct")

		req := santricity.SnapshotVolumeCreateRequest{
			SnapshotImageId:      snapImageID,
			Name:                 name,
			ViewMode:             accessMode,
			RepositoryPercentage: repoPct,
		}
		vol, err := apiClient.CreateSnapshotVolume(ctx, req)
		if err != nil {
			log.Fatalf("Error creating snapshot volume: %v", err)
		}
		if outputFormat == "json" {
			jsonData, _ := json.MarshalIndent(vol, "", "  ")
			fmt.Println(string(jsonData))
		} else {
			fmt.Printf("Created Snapshot Volume: %s (ID: %s, BasePIT: %s)\n", vol.Label, vol.SnapshotRef, vol.BasePIT)
		}
	},
}

func init() {
	createSnapshotGroupCmd.Flags().String("volume-id", "", "Base Volume ID (Ref)")
	createSnapshotGroupCmd.Flags().String("name", "", "Snapshot Group Name")
	createSnapshotGroupCmd.Flags().Int("repo-pct", 20, "Repository Percentage")
	createSnapshotGroupCmd.MarkFlagRequired("volume-id")
	createSnapshotGroupCmd.MarkFlagRequired("name")

	createSnapshotImageCmd.Flags().String("group-id", "", "Snapshot Group ID (Ref)")
	createSnapshotImageCmd.MarkFlagRequired("group-id")

	createSnapshotVolumeCmd.Flags().String("image-id", "", "Snapshot Image ID (Ref)")
	createSnapshotVolumeCmd.Flags().String("name", "", "Snapshot Volume Name")
	createSnapshotVolumeCmd.Flags().String("mode", "readOnly", "Access Mode (readOnly, readWrite)")
	createSnapshotVolumeCmd.Flags().Float64("repo-pct", 20.0, "Repository Percentage (for Copy-on-Write)")
	createSnapshotVolumeCmd.MarkFlagRequired("image-id")
	createSnapshotVolumeCmd.MarkFlagRequired("name")
}
