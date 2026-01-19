package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	santricity "github.com/scaleoutsean/santricity-go"
	log "github.com/sirupsen/logrus"
)

var (
	client     *santricity.Client
	poolID     string
	testPrefix string
	endpoint   string
	username   string
	password   string
	verifyTLS  bool = false
)

func init() {
	rand.Seed(time.Now().UnixNano())
	testPrefix = fmt.Sprintf("live_%d", rand.Intn(10000))
}

func setup() error {
	endpoint = os.Getenv("SANTRICITY_ENDPOINT")
	username = os.Getenv("SANTRICITY_USERNAME")
	password = os.Getenv("SANTRICITY_PASSWORD")
	poolID = os.Getenv("SANTRICITY_POOL_ID")

	if endpoint == "" || poolID == "" {
		return fmt.Errorf("SANTRICITY_ENDPOINT and SANTRICITY_POOL_ID are required")
	}

	config := santricity.ClientConfig{
		ApiControllers: []string{endpoint},
		Username:       username,
		Password:       password,
		VerifyTLS:      verifyTLS,
	}

	if os.Getenv("SANTRICITY_DEBUG") == "true" {
		log.SetLevel(log.DebugLevel)
		config.DebugTraceFlags = map[string]bool{"method": true, "call": true}
	}

	client = santricity.NewAPIClient(context.Background(), config)
	return nil
}

func main() {
	if err := setup(); err != nil {
		fmt.Printf("Setup failed: %v\n", err)
		os.Exit(1)
	}
	ctx := context.Background()

	// 1. Get Host Type Code
	htCode := getLinuxHostTypeCode(ctx)
	fmt.Printf("Using Host Type Code: %s\n", htCode)

	runTestA(ctx, htCode)
	runTestB(ctx, htCode)
	runTestC(ctx, htCode)
}

func getLinuxHostTypeCode(ctx context.Context) string {
	resp, body, err := client.InvokeAPI(ctx, nil, "GET", "/host-types")
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != http.StatusOK {
		panic(fmt.Errorf("GetHostTypes failed: %d", resp.StatusCode))
	}
	var types []santricity.HostType
	json.Unmarshal(body, &types)

	for _, ht := range types {
		if ht.Name == "Linux DM-MP" || ht.Name == "Linux" {
			return ht.Code
		}
	}
	if len(types) > 0 {
		return types[0].Code
	}
	return "LnxALUA"
}

func deleteHostUtil(ctx context.Context, hostRef string) {
	_, _, err := client.InvokeAPI(ctx, nil, "DELETE", "/hosts/"+hostRef)
	if err != nil {
		fmt.Printf("Warning: DeleteHost failed: %v\n", err)
	}
}

func unmapUtil(ctx context.Context, volRef string) {
	// Must refresh volume to get mappings
	vol, err := client.GetVolumeByRef(ctx, volRef)
	if err != nil {
		fmt.Printf("Warning: GetVolumeByRef for unmap failed: %v\n", err)
		return
	}
	client.UnmapVolume(ctx, vol)
}

func runTestA(ctx context.Context, htCode string) {
	fmt.Println("=== Test A: Create 1 volume, map to 1 iSCSI host, destroy ===")

	// Create Volume
	volName := fmt.Sprintf("%s_volA", testPrefix)
	sizeBytes := uint64(4 * 1024 * 1024 * 1024)
	vol, err := client.CreateVolume(ctx, volName, poolID, sizeBytes, "hdd", "xfs", "raid6", 0, 0, nil)
	if err != nil {
		panic(fmt.Errorf("CreateVolume failed: %v", err))
	}
	fmt.Printf("Created volume %s (%s)\n", volName, vol.VolumeRef)

	// Create Host
	hostName := fmt.Sprintf("%s_hostA", testPrefix)
	iqn := fmt.Sprintf("iqn.1998-01.com.test:%s", hostName)
	host, err := client.CreateHost(ctx, hostName, iqn, "iscsi", htCode, "", santricity.HostGroup{})
	if err != nil {
		client.DeleteVolume(ctx, vol)
		panic(fmt.Errorf("CreateHost failed: %v", err))
	}
	fmt.Printf("Created host %s (%s)\n", hostName, host.HostRef)

	// Map
	lun := 0
	mapping, err := client.MapVolume(ctx, vol, host, lun)
	if err != nil {
		deleteHostUtil(ctx, host.HostRef)
		client.DeleteVolume(ctx, vol)
		panic(fmt.Errorf("MapVolume failed: %v", err))
	}
	fmt.Printf("Mapped volume %s to host %s at LUN %d\n", volName, hostName, mapping.LunNumber)

	// Clean up
	fmt.Println("Cleaning up Test A...")
	unmapUtil(ctx, vol.VolumeRef)
	deleteHostUtil(ctx, host.HostRef)
	client.DeleteVolume(ctx, vol)
	fmt.Println("Test A Passed")
}

func runTestB(ctx context.Context, htCode string) {
	fmt.Println("\n=== Test B: Create 2 volumes, map to 2-host cluster, verify, expand 1, sleep 2m, destroy ===")

	// Create Cluster
	clusterName := fmt.Sprintf("%s_cluster", testPrefix)
	hg, err := client.CreateHostGroup(ctx, clusterName)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created Host Group %s\n", clusterName)

	ids := []string{"1", "2"}
	hostRefs := []string{}
	var lastHost santricity.HostEx

	for _, id := range ids {
		hName := fmt.Sprintf("%s_hostB_%s", testPrefix, id)
		iqn := fmt.Sprintf("iqn.1998-01.com.test:%s", hName)

		h, err := client.CreateHost(ctx, hName, iqn, "iscsi", htCode, "", hg)
		if err != nil {
			panic(err)
		}
		hostRefs = append(hostRefs, h.HostRef)
		lastHost = h
		fmt.Printf("Created Host %s in Group %s\n", hName, clusterName)
	}

	volRefs := []string{}
	volObjs := []santricity.VolumeEx{}
	for _, id := range ids {
		vName := fmt.Sprintf("%s_volB_%s", testPrefix, id)
		sizeBytes := uint64(4 * 1024 * 1024 * 1024)
		v, err := client.CreateVolume(ctx, vName, poolID, sizeBytes, "hdd", "xfs", "raid6", 0, 0, nil)
		if err != nil {
			panic(err)
		}
		volRefs = append(volRefs, v.VolumeRef)
		volObjs = append(volObjs, v)
		fmt.Printf("Created Volume %s\n", vName)
	}

	for i, v := range volObjs {
		lun := i + 10
		// Pass lastHost (which has ClusterRef set). MapVolume will use ClusterRef.
		_, err := client.MapVolume(ctx, v, lastHost, lun)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Mapped Vol %s to Cluster\n", v.Label)
	}

	fmt.Println("Expanding Volume 1...")
	targetSize := int64(8 * 1024 * 1024 * 1024)
	vBefore, _ := client.GetVolumeByRef(ctx, volRefs[0])
	fmt.Printf("Vol 1 Size Before: %s\n", vBefore.VolumeSize)

	err = client.ExpandVolume(ctx, volRefs[0], targetSize)
	if err != nil {
		panic(fmt.Errorf("ExpandVolume failed: %v", err))
	}
	fmt.Println("Expand requested. Sleeping 2 minutes to allow expansion...")
	time.Sleep(2 * time.Minute)

	vUpdated, err := client.GetVolumeByRef(ctx, volRefs[0])
	if err != nil {
		panic(err)
	}
	fmt.Printf("Vol 1 Size After: %s\n", vUpdated.VolumeSize)

	fmt.Println("Cleaning up Test B...")
	for _, v := range volObjs {
		unmapUtil(ctx, v.VolumeRef)
		client.DeleteVolume(ctx, v)
	}
	for _, hRef := range hostRefs {
		deleteHostUtil(ctx, hRef)
	}
	client.DeleteHostGroup(ctx, hg.ClusterRef)
	fmt.Println("Test B Passed")
}

func runTestC(ctx context.Context, htCode string) {
	fmt.Println("\n=== Test C: Host Replacement Scenario ===")
	vName := fmt.Sprintf("%s_volC", testPrefix)
	sizeBytes := uint64(4 * 1024 * 1024 * 1024)
	vol, err := client.CreateVolume(ctx, vName, poolID, sizeBytes, "hdd", "xfs", "raid6", 0, 0, nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created Volume %s\n", vName)

	h1Name := fmt.Sprintf("%s_hostC1", testPrefix)
	iqn1 := fmt.Sprintf("iqn.1998-01.com.test:%s", h1Name)
	h1, err := client.CreateHost(ctx, h1Name, iqn1, "iscsi", htCode, "", santricity.HostGroup{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created Host %s\n", h1Name)

	_, err = client.MapVolume(ctx, vol, h1, 0)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Mapped Vol to Host 1\n")

	h2Name := fmt.Sprintf("%s_hostC2", testPrefix)
	iqn2 := fmt.Sprintf("iqn.1998-01.com.test:%s", h2Name)
	h2, err := client.CreateHost(ctx, h2Name, iqn2, "iscsi", htCode, "", santricity.HostGroup{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created Host %s\n", h2Name)

	unmapUtil(ctx, vol.VolumeRef)
	fmt.Printf("Unmapped Vol from Host 1\n")

	// Refresh vol because unmapUtil updated state on array but vol variable is stale?
	// unmapUtil fetched its own copy. 'vol' variable is stale regarding mappings, but MapVolume doesn't check 'vol' mappings for validity locally, it trusts result of mapVolume call.
	// But it's better to refresh.
	vol, _ = client.GetVolumeByRef(ctx, vol.VolumeRef)

	_, err = client.MapVolume(ctx, vol, h2, 0)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Mapped Vol to Host 2\n")

	fmt.Println("Cleaning up Test C...")
	unmapUtil(ctx, vol.VolumeRef)
	deleteHostUtil(ctx, h1.HostRef)
	deleteHostUtil(ctx, h2.HostRef)
	client.DeleteVolume(ctx, vol)
	fmt.Println("Test C Passed")
}
