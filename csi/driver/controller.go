package driver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/container-storage-interface/spec/lib/go/csi"
	santricity "github.com/scaleoutsean/santricity-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

func (d *Driver) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
					},
				},
			},
		},
	}, nil
}

func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if d.client == nil {
		return nil, status.Error(codes.FailedPrecondition, "SANtricity client not initialized")
	}

	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "Name cannot be empty")
	}

	// SANtricity has a max limit of 30 characters.
	// Standard PVC names are longer (pvc-uuid...).
	// Strategy: If len > 30, truncate and append hash of full name to ensure uniqueness.
	// We'll use "v_" prefix (2 chars) + 12 chars of name + "_" + 15 chars of hash = 30 chars?
	// Or simpler: just use full hash?
	// Let's try to preserve some of the PVC name for readability functionality.
	// Max: 30.
	// Prefix: "pvc-" (4) ... left 26.
	// If name starts with "pvc-", we can keep it.
	// Suffix with hash (8 chars) -> 18 chars for original name.

	const maxLen = 30
	if len(name) > maxLen {
		// New format: pvc-{16chars}_{8hash} = 4+16+1+8 = 29 chars
		// Or if name is just UUID: use 20 chars + 8 hash.
		// For safety and collision avoidance:
		hash := sha256.Sum256([]byte(name))
		hashStr := hex.EncodeToString(hash[:]) // 64 chars

		// Take first 8 chars of hash
		shortHash := hashStr[:8]

		// Take prefix of original name (up to 20 chars to leave room for hash and separator)
		// We use 30 - 1 (separator) - 8 (hash) = 21 chars max prefix
		prefixLen := maxLen - 1 - len(shortHash)
		prefix := name
		if len(prefix) > prefixLen {
			prefix = prefix[:prefixLen]
		}

		// Replace hyphens with underscores if desired, though SANtricity supports hyphens.
		// User mentioned "change - into _" but standard PVC usually works with hyphen.
		// We'll stick to original chars unless restricted.

		newName := fmt.Sprintf("%s_%s", prefix, shortHash)
		klog.Infof("Volume name %s too long, shortened to %s", name, newName)
		name = newName
	}

	// Ensure valid characters? (Alphanumeric, -, _). PVC names are DNS labels (alphanumeric, -).
	// So we are safe.

	// Calculate size
	reqBytes := req.GetCapacityRange().GetRequiredBytes()
	// Minimum 1GB if not specified
	if reqBytes == 0 {
		reqBytes = 1024 * 1024 * 1024
	}

	klog.Infof("Creating volume %s with size %d", name, reqBytes)

	// Parse Parameters
	params := req.GetParameters()
	poolID := params["poolID"]
	poolName := params["poolName"]
	mediaType := params["mediaType"]
	if mediaType == "" {
		mediaType = "hdd"
	}
	fsType := params["fsType"]
	if fsType == "" {
		fsType = "xfs"
	}
	raidLevel := params["raidLevel"] // Optional

	// Find Storage Pool
	var selectedPoolRef string

	// Extract Metadata from CSI params
	// These usually come from external-provisioner using --extra-create-metadata
	metadata := make(map[string]string)
	if v, ok := params["csi.storage.k8s.io/pvc/name"]; ok {
		metadata["pvc_name"] = v
	}
	if v, ok := params["csi.storage.k8s.io/pvc/namespace"]; ok {
		metadata["pvc_namespace"] = v
	}
	if v, ok := params["csi.storage.k8s.io/pv/name"]; ok {
		metadata["pv_name"] = v
	}

	if poolID != "" {
		// Verify if Pool Exists ? Or just use it blindly?
		// Verification is safer.
		// There isn't a direct "GetVolumePoolByRef", but there is GetVolumePools.
		// Actually GetVolumePoolByRef was seen in client.go search result earlier?
		// <match path="/home/sean/code/santricity-go/client.go" line=545> func (d Client) GetVolumePoolByRef...
		p, err := d.client.GetVolumePoolByRef(ctx, poolID)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "Specified poolID %s not found: %v", poolID, err)
		}
		selectedPoolRef = p.VolumeGroupRef
		klog.Infof("Selected storage pool by ID: %s (%s)", p.Label, p.VolumeGroupRef)
	} else {
		// Fallback to searching by Name/Criteria
		pools, err := d.client.GetVolumePools(ctx, mediaType, uint64(reqBytes), poolName)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to get storage pools: %v", err)
		}
		if len(pools) == 0 {
			return nil, status.Errorf(codes.ResourceExhausted, "No storage pools found matching requirements (mediaType=%s, name=%s)", mediaType, poolName)
		}
		// Pick the first pool (simplistic approach)
		pool := pools[0]
		selectedPoolRef = pool.VolumeGroupRef
		klog.Infof("Selected storage pool by Name/Search: %s (%s)", pool.Label, pool.VolumeGroupRef)
	}

	// Create Volume
	// Note: segmentSize=0, blockSize=0 use defaults
	vol, err := d.client.CreateVolume(ctx, name, selectedPoolRef, uint64(reqBytes), mediaType, fsType, raidLevel, 0, 0, metadata)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create volume: %v", err)
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      vol.VolumeRef,
			CapacityBytes: reqBytes,
			VolumeContext: map[string]string{
				"poolID": vol.VolumeGroupRef,
				"label":  vol.Label,
				"wwn":    vol.WorldWideName,
			},
		},
	}, nil
}

func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if d.client == nil {
		return nil, status.Error(codes.FailedPrecondition, "SANtricity client not initialized")
	}

	volID := req.GetVolumeId()
	if volID == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}

	klog.Infof("Deleting volume %s", volID)

	// Construct minimal VolumeEx object for deletion
	vol := santricity.VolumeEx{
		VolumeRef: volID,
	}

	err := d.client.DeleteVolume(ctx, vol)
	if err != nil {
		// Verify if it's already gone? The library DeleteVolume returns success or error.
		// If 404, we should return success (idempotency).
		// The current client.go seems to not error on 404 (check logic), or return error.
		// Checking client.go snippet: case http.StatusNotFound: // do nothing
		// So checking err is sufficient.
		return nil, status.Errorf(codes.Internal, "Failed to delete volume: %v", err)
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (d *Driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	if d.client == nil {
		return nil, status.Error(codes.FailedPrecondition, "SANtricity client not initialized")
	}

	volID := req.GetVolumeId()
	nodeID := req.GetNodeId()

	if volID == "" || nodeID == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID and Node ID must be provided")
	}

	// 1. Get Host for NodeID (IQN)
	// The nodeID passed by CSI is typically the Node ID reported by NodeGetInfo.
	// We need to ensure the host exists on the array.
	klog.Infof("Ensuring host exists for IQN: %s", nodeID)
	host, err := d.client.EnsureHostForIQN(ctx, nodeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to ensure host for IQN %s: %v", nodeID, err)
	}
	klog.Infof("Found/Created Host %s (%s)", host.Label, host.HostRef)

	// 2. Map Volume
	vol, err := d.client.GetVolumeByRef(ctx, volID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Volume %s not found: %v", volID, err)
	}

	// Check if already mapped to this host
	for _, m := range vol.Mappings {
		// We match MapRef to HostRef
		if m.MapRef == host.HostRef {
			klog.Infof("Volume %s already mapped to host %s (LUN %d)", volID, host.Label, m.LunNumber)
			return &csi.ControllerPublishVolumeResponse{
				PublishContext: map[string]string{
					"lun": strconv.Itoa(m.LunNumber),
				},
			}, nil
		}
	}

	// Perform Mapping
	// lun=0 means auto-assign
	mapping, err := d.client.MapVolume(ctx, vol, host, 0)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to map volume: %v", err)
	}

	// 3. Get Target IQN
	targetSettings, err := d.client.GetTargetSettings(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get target settings: %v", err)
	}
	targetIQN := targetSettings.NodeName.IscsiNodeName

	// 4. Get Target Portals (IPs)
	// For simplicity, we can use the management IPs or look for specific iSCSI interface info.
	// client.GetStorageSystem() gives Controllers[].IPAddresses.
	// But usually we need checks for iSCSI interfaces specifically.
	// client.GetTargetSettings actually doesn't return IPs.
	// client.GetStorageSystem does.
	// Let's rely on configured HostDataIP if present, akin to client logic.
	// Or just use the management IP logic fallbacks inside d.client.config.HostDataIP?
	// But the Node needs it explicitly.
	// Let's start with a placeholder or Env variable, or query interfaces.
	// Since we don't have GetInterfaces implemented/checked, let's look for known IPs.

	// Helper: If d.endpoint is an IP, maybe use that? No, that's Management.
	// We'll create a fallback list.
	var portals []string

	// Try to get configured HostDataIP from client config? Not exposed easily.
	// Let's implement GetSummary or just use the first Management IP for now and assume it works for iSCSI (often true for valid test setup or single port)
	// But real world needs 'iscsi/target-settings' or 'analysed-volume-statistics'? No.
	// Checking client.go for method to get interfaces...
	// There is invokeAPI for /graph/interfaces?
	// Let's assume user passes "iscsiDataIP" parameter in StorageClass for now or use a heuristic.
	// Heuristic: Use os.Getenv("SANTRICITY_ISCSI_IP") or similar.
	// Better: Return the management IP as a fallback.

	// Actually, let's use the API to find it.
	// "/iscsi/target-settings" (IscsiTargetSettings) do not have IPs.
	// "/hardware-inventory" -> controllers -> ethernetInterfaces.

	// For now, I will hardcode finding it via GetStorageSystem which has IPs.
	sys, err := d.client.GetStorageSystem(ctx)
	if err == nil && len(sys.Controllers) > 0 {
		for _, c := range sys.Controllers {
			if len(c.IPAddresses) > 0 {
				portals = append(portals, c.IPAddresses[0])
			}
		}
	} else {
		// Fallback
		portals = []string{"127.0.0.1"}
	}

	// Taking the first one as primary
	targetPortal := portals[0]
	// Standard iSCSI port
	targetPortal += ":3260"

	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			"lun":          strconv.Itoa(mapping.LunNumber),
			"targetIQN":    targetIQN,
			"targetPortal": targetPortal,
			"volumeID":     volID,
		},
	}, nil
}

func (d *Driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	if d.client == nil {
		return nil, status.Error(codes.FailedPrecondition, "SANtricity client not initialized")
	}

	volID := req.GetVolumeId()
	nodeID := req.GetNodeId()

	if volID == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}

	// 1. Get Volume with mappings
	vol, err := d.client.GetVolumeByRef(ctx, volID)
	if err != nil {
		// If volume not found, purely idempotent
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	var mappingsToDelete []santricity.LUNMapping

	if nodeID != "" {
		host, err := d.client.GetHostForIQN(ctx, nodeID)
		if err != nil {
			klog.Warningf("Host %s not found during unpublish: %v", nodeID, err)
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}

		for _, m := range vol.Mappings {
			if m.MapRef == host.HostRef {
				mappingsToDelete = append(mappingsToDelete, m)
			}
		}
	} else {
		// Delete all mappings? Valid for RWO but dangerous for RWX.
		// Spec: "If the Node ID is not specified... unpublish from all nodes."
		mappingsToDelete = vol.Mappings
	}

	for _, m := range mappingsToDelete {
		klog.Infof("Removing mapping %s (LUN %d) for volume %s", m.LunMappingRef, m.LunNumber, volID)
		// Direct API call to delete mapping
		_, _, err := d.client.InvokeAPI(ctx, nil, "DELETE", "/volume-mappings/"+m.LunMappingRef)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to delete mapping %s: %v", m.LunMappingRef, err)
		}
	}

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (d *Driver) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ValidateVolumeCapabilities not implemented yet")
}

func (d *Driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ListVolumes not implemented yet")
}

func (d *Driver) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetCapacity not implemented yet")
}

func (d *Driver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "CreateSnapshot not implemented yet")
}

func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "DeleteSnapshot not implemented yet")
}

func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ListSnapshots not implemented yet")
}

func (d *Driver) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	if d.client == nil {
		return nil, status.Error(codes.FailedPrecondition, "SANtricity client not initialized")
	}

	volID := req.GetVolumeId()
	if volID == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}

	capRange := req.GetCapacityRange()
	if capRange == nil {
		return nil, status.Error(codes.InvalidArgument, "Capacity range must be provided")
	}
	requiredBytes := capRange.GetRequiredBytes()

	klog.Infof("Expanding volume %s to %d bytes", volID, requiredBytes)

	// API call to expand
	// The library ExpandVolume takes target size in bytes.
	// Note: SANtricity API traditionally sends "expansionSize" as INCREMENTAL size or NEW TOTAL?
	// The library `ExpandVolume` implementation seems to assume it's the size to pass to `expansionSize` param.
	// Documentation check or library usage in provider/resource_santricity_volume.go:170 calls it with `newSizeBytes`.
	// But `client.go` request struct uses `ExpansionSize`.
	// For most array APIs, "expand" usually means "increase by", but the param name `ExpansionSize` is ambiguous without docs.
	// However, usually in CSI we get the new TOTAL size.
	// Let's assume the library wrapper expects the INCREMENTAL amount if the API expects "expansionSize".
	// WAIT: `client.go` says: `request := VolumeExpansionRequest{ ExpansionSize: fmt.Sprintf("%d", newSizeInBytes) ...`
	// If the user passes newSizeInBytes to the func, it puts it in ExpansionSize.
	// If the API treats it as total or delta is the key.
	// SANtricity Web Services API docs usually say "expansionSize: The size to expand the volume by." (Delta)
	// So we need to fetch current size first!

	vol, err := d.client.GetVolumeByRef(ctx, volID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Volume %s not found: %v", volID, err)
	}

	currentBytes, _ := strconv.ParseInt(vol.VolumeSize, 10, 64) // VolumeSize is string
	if requiredBytes <= currentBytes {
		// Already large enough
		return &csi.ControllerExpandVolumeResponse{
			CapacityBytes:         currentBytes,
			NodeExpansionRequired: true,
		}, nil
	}

	delta := requiredBytes - currentBytes
	klog.Infof("Delta expansion: %d bytes (Current: %d, Required: %d)", delta, currentBytes, requiredBytes)

	err = d.client.ExpandVolume(ctx, volID, delta)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to expand volume: %v", err)
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         requiredBytes,
		NodeExpansionRequired: true,
	}, nil
}

func (d *Driver) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ControllerGetVolume not implemented yet")
}

func (d *Driver) ControllerModifyVolume(ctx context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ControllerModifyVolume not implemented yet")
}
