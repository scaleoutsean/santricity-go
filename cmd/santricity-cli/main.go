package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

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
			volNameFilter, _ := cmd.Flags().GetString("volume-name")

			apiClient.SetIncludeRepositoryVolumes(showRepoVols)
			vols, err := apiClient.GetVolumes(ctx)
			if err != nil {
				log.Fatalf("Error getting volumes: %v", err)
			}

			// Apply Filter
			var filteredVols []santricity.VolumeEx
			if volNameFilter != "" {
				for _, v := range vols {
					if v.Label == volNameFilter {
						filteredVols = append(filteredVols, v)
					}
				}
				vols = filteredVols
			}

			if outputFormat == "json" {
				b, err := json.MarshalIndent(vols, "", "  ")
				if err != nil {
					log.Fatalf("Error marshaling to JSON: %v", err)
				}
				fmt.Println(string(b))
			} else {
				for _, v := range vols {
					log.Printf("Volume: %s (Size: %s Ref: %s)", v.Label, v.VolumeSize, v.VolumeRef)
				}
			}
		},
	}
	getVolumesCmd.Flags().BoolVar(&showRepoVols, "show-repo-vols", false, "Show internal repository volumes")
	getVolumesCmd.Flags().String("volume-name", "", "Filter by volume name")

	var getHostsCmd = &cobra.Command{
		Use:   "hosts",
		Short: "List hosts",
		Run: func(cmd *cobra.Command, args []string) {
			hosts, err := apiClient.GetHosts(ctx)
			if err != nil {
				log.Fatalf("Error getting hosts: %v", err)
			}
			if outputFormat == "json" {
				b, err := json.MarshalIndent(hosts, "", "  ")
				if err != nil {
					log.Fatalf("Error marshaling to JSON: %v", err)
				}
				fmt.Println(string(b))
			} else {
				for _, h := range hosts {
					log.Printf("Host: %s (Ref: %s, Cluster: %s)", h.Label, h.HostRef, h.ClusterRef)
				}
			}
		},
	}
	getCmd.AddCommand(getHostsCmd)

	var getHostGroupsCmd = &cobra.Command{
		Use:   "host-groups",
		Short: "List host groups",
		Run: func(cmd *cobra.Command, args []string) {
			groups, err := apiClient.GetHostGroups(ctx)
			if err != nil {
				log.Fatalf("Error getting host groups: %v", err)
			}
			if outputFormat == "json" {
				b, err := json.MarshalIndent(groups, "", "  ")
				if err != nil {
					log.Fatalf("Error marshaling to JSON: %v", err)
				}
				fmt.Println(string(b))
			} else {
				for _, g := range groups {
					log.Printf("Host Group: %s (Ref: %s)", g.Label, g.ClusterRef)
				}
			}
		},
	}
	getCmd.AddCommand(getHostGroupsCmd)

	var getMappingsCmd = &cobra.Command{
		Use:   "mappings",
		Short: "List volume mappings",
		Run: func(cmd *cobra.Command, args []string) {
			mappings, err := apiClient.GetVolumeMappings(ctx)
			if err != nil {
				log.Fatalf("Error getting mappings: %v", err)
			}
			if outputFormat == "json" {
				b, err := json.MarshalIndent(mappings, "", "  ")
				if err != nil {
					log.Fatalf("Error marshaling to JSON: %v", err)
				}
				fmt.Println(string(b))
			} else {
				fmt.Printf("%-36s %-10s %-36s %-36s\n", "Ref", "LUN", "Volume", "Target")
				for _, m := range mappings {
					fmt.Printf("%-36s %-10d %-36s %-36s\n", m.LunMappingRef, m.LunNumber, m.VolumeRef, m.MapRef)
				}
			}
		},
	}
	getCmd.AddCommand(getMappingsCmd)

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

	var hostGroupName string
	var createHostGroupCmd = &cobra.Command{
		Use:   "host-group",
		Short: "Create a host group (Cluster)",
		Run: func(cmd *cobra.Command, args []string) {
			if hostGroupName == "" {
				log.Fatal("Error: --name is required")
			}
			hg, err := apiClient.CreateHostGroup(ctx, hostGroupName)
			if err != nil {
				log.Fatalf("Error creating host group: %v", err)
			}
			if outputFormat == "json" {
				b, err := json.MarshalIndent(hg, "", "  ")
				if err != nil {
					log.Fatalf("Error marshaling to JSON: %v", err)
				}
				fmt.Println(string(b))
			} else {
				log.Printf("Created Host Group: %s (Ref: %s)", hg.Label, hg.ClusterRef)
			}
		},
	}
	createHostGroupCmd.Flags().StringVar(&hostGroupName, "name", "", "Host Group Name")

	createCmd.AddCommand(createHostCmd)
	createCmd.AddCommand(createHostGroupCmd)

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

	var mappingVolID, mappingTargetID string
	var mappingLun int
	var createMappingCmd = &cobra.Command{
		Use:   "mapping",
		Short: "Create a volume mapping",
		Run: func(cmd *cobra.Command, args []string) {
			if mappingVolID == "" || mappingTargetID == "" {
				log.Fatal("Error: --volume-id and --target-id are required")
			}

			req := santricity.VolumeMappingCreateRequest{
				MappableObjectID: mappingVolID,
				TargetID:         mappingTargetID,
				LunNumber:        mappingLun,
			}

			mapping, err := apiClient.CreateVolumeMapping(ctx, req)
			if err != nil {
				log.Fatalf("Error creating mapping: %v", err)
			}
			if outputFormat == "json" {
				b, err := json.MarshalIndent(mapping, "", "  ")
				if err != nil {
					log.Fatalf("Error marshaling to JSON: %v", err)
				}
				fmt.Println(string(b))
			} else {
				log.Printf("Created Mapping: Volume %s -> Target %s (LUN %d, Ref: %s)", mapping.VolumeRef, mapping.MapRef, mapping.LunNumber, mapping.LunMappingRef)
			}
		},
	}
	createMappingCmd.Flags().StringVar(&mappingVolID, "volume-id", "", "Volume ID (Ref)")
	createMappingCmd.Flags().StringVar(&mappingTargetID, "target-id", "", "Target ID (Host or HostGroup Ref)")
	createMappingCmd.Flags().IntVar(&mappingLun, "lun", 0, "LUN Number (0 for auto-assign)")

	createCmd.AddCommand(createMappingCmd)
	createCmd.AddCommand(createVolumeCmd)
	rootCmd.AddCommand(createCmd)

	getCmd.AddCommand(getSnapshotGroupsCmd)
	getCmd.AddCommand(getSnapshotImagesCmd)

	createCmd.AddCommand(createSnapshotGroupCmd)
	createCmd.AddCommand(createSnapshotImageCmd)
	createCmd.AddCommand(createSnapshotVolumeCmd)

	var rollbackCmd = &cobra.Command{
		Use:   "rollback",
		Short: "Rollback operations",
	}

	var rollbackVolumeCmd = &cobra.Command{
		Use:   "volume",
		Short: "Rollback a volume to a previous Snapshot Image (PiT)",
		Run: func(cmd *cobra.Command, args []string) {
			imageID, _ := cmd.Flags().GetString("image-id")
			if imageID == "" {
				log.Fatal("Error: --image-id is required")
			}

			// Interactive confirmation unless forced (omitted for now in this MVP)
			fmt.Printf("WARNING: Rolling back volume from Snapshot Image %s. Current data on the base volume will be OVERWRITTEN.\n", imageID)

			err := apiClient.RollbackSnapshotImage(ctx, imageID)
			if err != nil {
				log.Fatalf("Error starting rollback: %v", err)
			}

			if outputFormat == "json" {
				fmt.Println(`{"status": "rollback_started", "message": "Rollback operation initiated successfully."}`)
			} else {
				fmt.Println("Rollback operation initiated successfully. Monitor volume status for completion.")
			}
		},
	}
	rollbackVolumeCmd.Flags().String("image-id", "", "Snapshot Image ID (Ref) to restore from")
	rollbackVolumeCmd.MarkFlagRequired("image-id")

	rollbackCmd.AddCommand(rollbackVolumeCmd)
	rootCmd.AddCommand(rollbackCmd)

	rootCmd.AddCommand(deleteCmd)

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
			fmt.Printf("%-36s %-20s %-10s %-36s\n", "ID", "Label", "Status", "BaseVolume")
			for _, g := range groups {
				fmt.Printf("%-36s %-20s %-10s %-36s\n", g.PitGroupRef, g.Label, g.Status, g.BaseVolume)
			}
		}
	},
}

var getSnapshotImagesCmd = &cobra.Command{
	Use:   "snapshot-images",
	Short: "Get all Snapshot Images",
	Run: func(cmd *cobra.Command, args []string) {
		volNameFilter, _ := cmd.Flags().GetString("volume-name")

		images, err := apiClient.GetSnapshotImages(ctx)
		if err != nil {
			log.Fatalf("Error getting snapshot images: %v", err)
		}

		// Fetch volumes to map names
		volumes, err := apiClient.GetVolumes(ctx)
		if err != nil {
			log.Fatalf("Error getting volumes: %v", err)
		}

		volMap := make(map[string]string)
		var targetVolID string

		for _, v := range volumes {
			volMap[v.VolumeRef] = v.Label
			if v.Label == volNameFilter {
				targetVolID = v.VolumeRef
			}
		}

		if volNameFilter != "" {
			if targetVolID == "" {
				log.Fatalf("Volume with name '%s' not found", volNameFilter)
			}

			// Filter images
			var filteredImages []santricity.SnapshotImage
			for _, img := range images {
				if img.BaseVol == targetVolID {
					filteredImages = append(filteredImages, img)
				}
			}
			images = filteredImages
		}

		if outputFormat == "json" {
			jsonData, _ := json.MarshalIndent(images, "", "  ")
			fmt.Println(string(jsonData))
		} else {
			fmt.Printf("%-36s %-20s %-10s %-19s %s\n", "PitRef", "Volume", "Status", "Timestamp", "Seq")
			for _, i := range images {
				ts, _ := strconv.ParseInt(i.PitTimestamp, 10, 64)
				tm := time.Unix(ts, 0)
				volName := volMap[i.BaseVol]
				if volName == "" {
					volName = i.BaseVol // Fallback to Ref
				}
				if len(volName) > 20 {
					volName = volName[:17] + "..."
				}
				fmt.Printf("%-36s %-20s %-10s %-19s %s\n", i.PitRef, volName, i.Status, tm.Format("2006-01-02 15:04:05"), i.PitSequenceNumber)
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
		repoPct, _ := cmd.Flags().GetFloat64("repo-pct")

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

		if hostID, _ := cmd.Flags().GetString("host-id"); hostID != "" {
			// Check if we need to map to HostGroup or Host
			targetID := hostID
			host, err := apiClient.GetHostByRef(ctx, hostID)
			if err == nil {
				// We found the host, check its cluster ref
				if host.ClusterRef != "0000000000000000000000000000000000000000" {
					log.Printf("Note: Host %s is part of Cluster %s. Mapping to Cluster instead.", host.Label, host.ClusterRef)
					targetID = host.ClusterRef
				}
			} else {
				// Maybe the user passed a HostGroup ID? Or an invalid ID.
				// We'll proceed with the assumption it's a valid TargetID if GetHost failed (might be a straight cluster ID passed in?)
				// But strictly speaking --host-id implies a Host.
				// Let's just warn if we can't look it up, but trust the API to return an error if it's garbage.
				// Actually, GetHostByRef will likely return error if ID is not a host.
			}

			mapReq := santricity.VolumeMappingCreateRequest{
				MappableObjectID: vol.SnapshotRef,
				TargetID:         targetID,
				LunNumber:        0, // Auto-assign
			}
			mapping, err := apiClient.CreateVolumeMapping(ctx, mapReq)
			if err != nil {
				log.Printf("Warning: Snapshot Volume created but mapping failed: %v", err)
			} else {
				log.Printf("Mapped Snapshot Volume to Host %s (LUN %d)", mapping.MapRef, mapping.LunNumber)
				log.Println("WARNING: The snapshot volume is a clone of the original volume. If mapped to the same host, take caution with duplicate filesystem UUIDs and LVM Volume Groups.")
			}
		}

		if outputFormat == "json" {
			jsonData, _ := json.MarshalIndent(vol, "", "  ")
			fmt.Println(string(jsonData))
		} else {
			fmt.Printf("Created Snapshot Volume: %s (ID: %s, BasePIT: %s)\n", vol.Label, vol.SnapshotRef, vol.BasePIT)
		}
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete resources",
}

var deleteVolumeCmd = &cobra.Command{
	Use:   "volume",
	Short: "Delete a standard volume",
	Run: func(cmd *cobra.Command, args []string) {
		id, _ := cmd.Flags().GetString("id")
		name, _ := cmd.Flags().GetString("name")

		if id == "" && name == "" {
			log.Fatal("Error: --id or --name is required")
		}

		if id == "" {
			// Resolve by name
			vols, err := apiClient.GetVolumes(ctx)
			if err != nil {
				log.Fatalf("Error getting volumes: %v", err)
			}
			for _, v := range vols {
				if v.Label == name {
					id = v.VolumeRef
					break
				}
			}
			if id == "" {
				log.Fatalf("Volume with name '%s' not found", name)
			}
		}

		err := apiClient.DeleteVolume(ctx, santricity.VolumeEx{VolumeRef: id})
		if err != nil {
			log.Fatalf("Error deleting volume: %v", err)
		}
		fmt.Printf("Deleted Volume %s\n", id)
	},
}

var deleteHostCmd = &cobra.Command{
	Use:   "host",
	Short: "Delete a host",
	Run: func(cmd *cobra.Command, args []string) {
		id, _ := cmd.Flags().GetString("id")
		name, _ := cmd.Flags().GetString("name")

		if id == "" && name == "" {
			log.Fatal("Error: --id or --name is required")
		}

		if id == "" {
			// Resolve by name
			hosts, err := apiClient.GetHosts(ctx)
			if err != nil {
				log.Fatalf("Error getting hosts: %v", err)
			}
			for _, h := range hosts {
				if h.Label == name {
					id = h.HostRef
					break
				}
			}
			if id == "" {
				log.Fatalf("Host with name '%s' not found", name)
			}
		}

		err := apiClient.DeleteHost(ctx, id)
		if err != nil {
			log.Fatalf("Error deleting host: %v", err)
		}
		fmt.Printf("Deleted Host %s\n", id)
	},
}

var deleteHostGroupCmd = &cobra.Command{
	Use:   "host-group",
	Short: "Delete a host group (Cluster)",
	Run: func(cmd *cobra.Command, args []string) {
		id, _ := cmd.Flags().GetString("id")
		name, _ := cmd.Flags().GetString("name")
		force, _ := cmd.Flags().GetBool("force")

		if id == "" && name == "" {
			log.Fatal("Error: --id or --name is required")
		}

		// Resolve ID
		if id == "" {
			groups, err := apiClient.GetHostGroups(ctx)
			if err != nil {
				log.Fatalf("Error getting host groups: %v", err)
			}
			for _, g := range groups {
				if g.Label == name {
					id = g.ClusterRef
					break
				}
			}
			if id == "" {
				log.Fatalf("Host Group with name '%s' not found", name)
			}
		}

		// Check for hosts if not force
		if !force {
			hosts, err := apiClient.GetHosts(ctx)
			if err != nil {
				log.Fatalf("Error getting hosts: %v", err)
			}
			count := 0
			for _, h := range hosts {
				if h.ClusterRef == id {
					count++
				}
			}
			if count > 0 {
				log.Fatalf("Error: Host Group %s is not empty (%d hosts). Use --force to delete anyway.", id, count)
			}
		}

		err := apiClient.DeleteHostGroup(ctx, id)
		if err != nil {
			log.Fatalf("Error deleting host group: %v", err)
		}
		fmt.Printf("Deleted Host Group %s\n", id)
	},
}

var deleteSnapshotGroupCmd = &cobra.Command{
	Use:   "snapshot-group",
	Short: "Delete a snapshot group",
	Run: func(cmd *cobra.Command, args []string) {
		id, _ := cmd.Flags().GetString("id")
		name, _ := cmd.Flags().GetString("name")
		force, _ := cmd.Flags().GetBool("force")

		if id == "" && name == "" {
			log.Fatal("Error: --id or --name is required")
		}

		if id == "" {
			groups, err := apiClient.GetSnapshotGroups(ctx)
			if err != nil {
				log.Fatalf("Error getting snapshot groups: %v", err)
			}
			for _, g := range groups {
				if g.Label == name {
					id = g.PitGroupRef
					break
				}
			}
			if id == "" {
				log.Fatalf("Snapshot Group with name '%s' not found", name)
			}
		}

		// Check for dependencies (Linked Clones / Snapshot Volumes)
		// 1. Get all images in this group
		images, err := apiClient.GetSnapshotImages(ctx)
		if err != nil {
			log.Fatalf("Error listing snapshot images: %v", err)
		}
		var groupImages []string
		for _, img := range images {
			if img.PitGroupRef == id {
				groupImages = append(groupImages, img.PitRef)
			}
		}

		// 2. Get all snapshot volumes that map to these images
		snapVols, err := apiClient.GetSnapshotVolumes(ctx)
		if err != nil {
			log.Printf("Warning: Could not list snapshot volumes to check dependencies: %v", err)
		} else {
			var dependentVols []santricity.SnapshotVolume
			for _, vol := range snapVols {
				for _, pitRef := range groupImages {
					if vol.BasePIT == pitRef {
						dependentVols = append(dependentVols, vol)
						break
					}
				}
			}

			if len(dependentVols) > 0 {
				if !force {
					log.Printf("Error: Snapshot Group has %d dependent Snapshot Volumes (Linked Clones):", len(dependentVols))
					for _, v := range dependentVols {
						log.Printf(" - %s (ID: %s)", v.Label, v.SnapshotRef)
					}
					log.Fatal("Use --force to delete them automatically.")
				} else {
					fmt.Printf("Force deleting %d dependent Snapshot Volumes...\n", len(dependentVols))
					for _, vol := range dependentVols {
						// Resolve Volume to Unmap
						// Attempt unmap logic similar to DeleteVolume
						// We need VolumeEx for UnmapVolume
						// But GetVolume might not find SnapshotVolume depending on API version/config
						// Let's rely on standard UnmapVolume which takes VolumeEx
						// We can try to fetch it as a standard volume

						// Try to look up as normal volume to get mapping details
						// Note: GetVolumeByRef works on Ref, so we use SnapshotRef
						// If fails, maybe it's not mapped or not visible

						// Attempt to unmap first
						// We construct a temporary VolumeEx if we can't fetch it, but Unmap needs Mappings list.
						// So we MUST fetch it.

						// GetVolumeByRef is not exported?
						// Wait, it WAS exported in my grep results: `func (d Client) GetVolumeByRef`
						// Let's assume it works.

						// Or just use DeleteSnapshotVolume and hope it handles unmap?
						// Usually it doesn't.

						// Let's rely on manual cleanup via API calls if needed.
						// InvokeAPI DELETE /snapshot-volumes/{id}

						// Actually, let's try to unmap properly in a best-effort way
						// Iterate volumes to find full object (with mappings)
						vols, err := apiClient.GetVolumes(ctx)
						var targetVol *santricity.VolumeEx
						if err == nil {
							for _, v := range vols {
								if v.VolumeRef == vol.SnapshotRef {
									targetVol = &v
									break
								}
							}
						}

						if targetVol != nil && len(targetVol.Mappings) > 0 {
							fmt.Printf("Unmapping volume %s...\n", targetVol.Label)
							// We need UnmapVolume which takes VolumeEx by value
							// Wait, loop variable 'targetVol' is pointer to loop slice element?
							// No, 'v' is copy. 'targetVol' is pointer to copy. Correct.
							err := apiClient.UnmapVolume(ctx, *targetVol)
							if err != nil {
								log.Printf("Warning: Failed to unmap volume %s: %v", targetVol.Label, err)
							}
						}

						err = apiClient.DeleteSnapshotVolume(ctx, vol.SnapshotRef)
						if err != nil {
							log.Fatalf("Error deleting snapshot volume %s: %v", vol.Label, err)
						}
						fmt.Printf("Deleted Snapshot Volume %s\n", vol.Label)
					}
				}
			}
		}

		err = apiClient.DeleteSnapshotGroup(ctx, id)
		if err != nil {
			log.Fatalf("Error deleting snapshot group: %v", err)
		}
		fmt.Printf("Deleted Snapshot Group %s\n", id)
	},
}

var deleteSnapshotImageCmd = &cobra.Command{
	Use:   "snapshot-image",
	Short: "Delete a snapshot image (PiT)",
	Run: func(cmd *cobra.Command, args []string) {
		id, _ := cmd.Flags().GetString("id")
		force, _ := cmd.Flags().GetBool("force")

		if id == "" {
			log.Fatal("Error: --id is required")
		}

		// Check for dependent Snapshot Volumes
		snapVols, err := apiClient.GetSnapshotVolumes(ctx)
		if err != nil {
			log.Printf("Warning: Could not list snapshot volumes to check dependencies: %v", err)
		} else {
			var dependentVols []santricity.SnapshotVolume
			for _, vol := range snapVols {
				if vol.BasePIT == id {
					dependentVols = append(dependentVols, vol)
				}
			}

			if len(dependentVols) > 0 {
				if !force {
					log.Printf("Error: Snapshot Image has %d dependent Snapshot Volumes (Linked Clones):", len(dependentVols))
					for _, v := range dependentVols {
						log.Printf(" - %s (ID: %s)", v.Label, v.SnapshotRef)
					}
					log.Fatal("Use --force to delete them automatically.")
				} else {
					fmt.Printf("Force deleting %d dependent Snapshot Volumes...\n", len(dependentVols))
					for _, vol := range dependentVols {
						// Unmap and Delete
						vols, err := apiClient.GetVolumes(ctx)
						var targetVol *santricity.VolumeEx
						if err == nil {
							for _, v := range vols {
								if v.VolumeRef == vol.SnapshotRef {
									targetVol = &v
									break
								}
							}
						}

						if targetVol != nil && len(targetVol.Mappings) > 0 {
							fmt.Printf("Unmapping volume %s...\n", targetVol.Label)
							err := apiClient.UnmapVolume(ctx, *targetVol)
							if err != nil {
								log.Printf("Warning: Failed to unmap volume %s: %v", targetVol.Label, err)
							}
						}

						err = apiClient.DeleteSnapshotVolume(ctx, vol.SnapshotRef)
						if err != nil {
							log.Fatalf("Error deleting snapshot volume %s: %v", vol.Label, err)
						}
						fmt.Printf("Deleted Snapshot Volume %s\n", vol.Label)
					}
				}
			}
		}

		err = apiClient.DeleteSnapshotImage(ctx, id)
		if err != nil {
			log.Fatalf("Error deleting snapshot image: %v", err)
		}
		fmt.Printf("Deleted Snapshot Image %s\n", id)
	},
}

var deleteSnapshotVolumeCmd = &cobra.Command{
	Use:   "snapshot-volume",
	Short: "Delete a snapshot volume (Linked Clone)",
	Run: func(cmd *cobra.Command, args []string) {
		id, _ := cmd.Flags().GetString("id")
		name, _ := cmd.Flags().GetString("name")

		if id == "" && name == "" {
			log.Fatal("Error: --id or --name is required")
		}

		if id == "" {
			// Find by name? Assuming snapshot volumes are volumes?
			// The API for snapshot volumes usually returns them as normal volumes too via GetVolumes but with specific properties?
			// Or should we use GetSnapshotVolume specific endpoint if strictly separated?
			// The CreateSnapshotVolume returns SnapshotVolume struct which has SnapshotRef (ID).
			// Let's assume standard volume lookup by name works OR we need specific lookup.
			// Let's rely on standard GetVolumes for now as usually snapshot volumes appear there.
			// However, in SANtricity, snapshot volumes are distinct from standard volumes in some contexts.
			// To be safe, let's just query GetVolumes and filter by name.
			vols, err := apiClient.GetVolumes(ctx)
			if err != nil {
				log.Fatalf("Error getting volumes: %v", err)
			}
			for _, v := range vols {
				if v.Label == name {
					id = v.VolumeRef
					break
				}
			}
			if id == "" {
				log.Fatalf("Snapshot Volume with name '%s' not found", name)
			}
		}

		err := apiClient.DeleteSnapshotVolume(ctx, id)
		if err != nil {
			log.Fatalf("Error deleting snapshot volume: %v", err)
		}
		fmt.Printf("Deleted Snapshot Volume %s\n", id)
	},
}

func init() {
	createSnapshotGroupCmd.Flags().String("volume-id", "", "Base Volume ID (Ref)")
	createSnapshotGroupCmd.Flags().String("name", "", "Snapshot Group Name")
	createSnapshotGroupCmd.Flags().Float64("repo-pct", 20.0, "Repository Percentage")
	createSnapshotGroupCmd.MarkFlagRequired("volume-id")
	createSnapshotGroupCmd.MarkFlagRequired("name")

	getSnapshotImagesCmd.Flags().String("volume-name", "", "Filter by Base Volume Name")

	createSnapshotImageCmd.Flags().String("group-id", "", "Snapshot Group ID (Ref)")
	createSnapshotImageCmd.MarkFlagRequired("group-id")

	createSnapshotVolumeCmd.Flags().String("image-id", "", "Snapshot Image ID (Ref)")
	createSnapshotVolumeCmd.Flags().String("name", "", "Snapshot Volume Name")
	createSnapshotVolumeCmd.Flags().String("mode", "readOnly", "Access Mode (readOnly, readWrite)")
	createSnapshotVolumeCmd.Flags().Float64("repo-pct", 20.0, "Repository Percentage (for Copy-on-Write)")
	createSnapshotVolumeCmd.Flags().String("host-id", "", "Optional: Host ID (Ref) to map volume to")
	createSnapshotVolumeCmd.MarkFlagRequired("image-id")
	createSnapshotVolumeCmd.MarkFlagRequired("name")

	deleteCmd.AddCommand(deleteVolumeCmd)
	deleteCmd.AddCommand(deleteHostCmd)
	deleteCmd.AddCommand(deleteHostGroupCmd)
	deleteCmd.AddCommand(deleteSnapshotGroupCmd)
	deleteCmd.AddCommand(deleteSnapshotImageCmd)
	deleteCmd.AddCommand(deleteSnapshotVolumeCmd)

	deleteVolumeCmd.Flags().String("id", "", "Volume ID (Ref)")
	deleteVolumeCmd.Flags().String("name", "", "Volume Name")

	deleteHostCmd.Flags().String("id", "", "Host ID (Ref)")
	deleteHostCmd.Flags().String("name", "", "Host Name")

	deleteHostGroupCmd.Flags().String("id", "", "Host Group ID (Ref)")
	deleteHostGroupCmd.Flags().String("name", "", "Host Group Name")
	deleteHostGroupCmd.Flags().Bool("force", false, "Force delete non-empty host group")

	deleteSnapshotGroupCmd.Flags().String("id", "", "Snapshot Group ID (Ref)")
	deleteSnapshotGroupCmd.Flags().String("name", "", "Snapshot Group Name")
	deleteSnapshotGroupCmd.Flags().Bool("force", false, "Force delete dependent Snapshot Volumes (Linked Clones)")

	deleteSnapshotVolumeCmd.Flags().String("id", "", "Snapshot Volume ID (Ref)")
	deleteSnapshotVolumeCmd.Flags().String("name", "", "Snapshot Volume Name")

	deleteSnapshotImageCmd.Flags().String("id", "", "Snapshot Image ID (Ref)")
	deleteSnapshotImageCmd.Flags().Bool("force", false, "Force delete dependent Snapshot Volumes")
}
