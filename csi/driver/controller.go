package driver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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

	// Parse optional blockSize parameter
	var blockSize int // 0 means default
	if blockSizeStr, ok := params["blockSize"]; ok {
		bs, err := strconv.Atoi(blockSizeStr)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid blockSize parameter: %s", blockSizeStr)
		}
		if bs != 512 && bs != 4096 {
			return nil, status.Errorf(codes.InvalidArgument, "blockSize must be 512 or 4096, got %d", bs)
		}
		blockSize = bs
	}

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
		// Verify if Pool Exists
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
	// Note: segmentSize=0 uses default, blockSize=0 uses array default (unless specified)
	vol, err := d.client.CreateVolume(ctx, name, selectedPoolRef, uint64(reqBytes), mediaType, fsType, raidLevel, blockSize, 0, metadata)
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

	// Check for dependencies (Guardrails) before cascading delete
	if err := d.client.CheckVolumeDependencies(ctx, volID); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "volume has active snapshots or clones: %v", err)
	}

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

	// 1. Get Host for NodeID (IQN or NQN)
	// The nodeID passed by CSI is typically the Node ID reported by NodeGetInfo.
	// We need to ensure the host exists on the array.
	var host santricity.HostEx
	var hostErr error
	isISCSI := strings.HasPrefix(nodeID, "iqn.")
	isNVMe := strings.HasPrefix(nodeID, "nqn.")

	if isISCSI {
		klog.Infof("Looking up host for IQN: %s", nodeID)
		host, hostErr = d.client.GetHostForPort(ctx, nodeID)
		if hostErr == nil && host.HostRef == "" {
			hostErr = status.Errorf(codes.NotFound, "Host for IQN %s not found on array. Please create it manually.", nodeID)
		}
	} else if isNVMe {
		klog.Infof("Looking up host for NQN: %s", nodeID)
		host, hostErr = d.client.GetHostForPort(ctx, nodeID)
		if hostErr == nil && host.HostRef == "" {
			hostErr = status.Errorf(codes.NotFound, "Host for NQN %s not found on array. Please create it manually.", nodeID)
		}
	} else {
		return nil, status.Errorf(codes.InvalidArgument, "Node ID %s format not recognized (must start with iqn. or nqn.)", nodeID)
	}

	if hostErr != nil {
		return nil, status.Errorf(codes.Internal, "Failed to ensure host for %s: %v", nodeID, hostErr)
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
		// Also check if mapped to HostGroup (ClusterRef) of which this Host is a member
		if m.MapRef == host.HostRef || (host.ClusterRef != "" && m.MapRef == host.ClusterRef) {
			klog.Infof("Volume %s already mapped to host %s (LUN %d)", volID, host.Label, m.LunNumber)
			return &csi.ControllerPublishVolumeResponse{
				PublishContext: map[string]string{
					"lun": strconv.Itoa(m.LunNumber),
					"wwn": vol.WorldWideName,
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

	// 3. Get Target Information and Populate Publish Context
	publishContext := map[string]string{
		"lun":      strconv.Itoa(mapping.LunNumber),
		"volumeID": volID,
		"wwn":      vol.WorldWideName, // Add WWN to PublishContext for Node stage to construct /dev/disk/by-id/wwn-...
	}

	// Determine Target Portal (IP)
	// TODO: Replace with robust interface discovery (e.g. filter by "iscsi" or "nvme-roce" interface type)
	// For now, use the first management IP found on controllers as a fallback.
	var portals []string
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
	targetPortalIP := portals[0]

	if isISCSI {
		targetSettings, err := d.client.GetTargetSettings(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to get iSCSI target settings: %v", err)
		}
		// Log detailed settings for debugging
		klog.Infof("iSCSI Target Settings: NodeName=%s, Portals=%v", targetSettings.NodeName.IscsiNodeName, targetSettings.Portals)

		publishContext["targetIQN"] = targetSettings.NodeName.IscsiNodeName
		publishContext["targetPortal"] = targetPortalIP + ":3260"
	} else if isNVMe {
		nvmeSettings, err := d.client.GetNVMeoFSettings(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to get NVMeoF target settings: %v", err)
		}
		// Log detailed settings for debugging
		klog.Infof("NVMeoF Target Settings: NvmeNodeName=%s, IscsiNodeName=%v, RemoteNodeWWN=%v",
			nvmeSettings.NodeName.NvmeNodeName, nvmeSettings.NodeName.IscsiNodeName, nvmeSettings.NodeName.RemoteNodeWWN)

		publishContext["targetNQN"] = nvmeSettings.NodeName.NvmeNodeName
		publishContext["targetPortal"] = targetPortalIP + ":4420" // Default NVMe port
		// Note: NVMe protocols (RoCE vs TCP) might require "transport" field or different ports.
		// Assuming TCP or RoCE v2 with default port.
	}

	return &csi.ControllerPublishVolumeResponse{
		PublishContext: publishContext,
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
		host, err := d.client.GetHostForPort(ctx, nodeID)
		if err != nil {
			klog.Warningf("Host %s not found during unpublish: %v", nodeID, err)
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}

		for _, m := range vol.Mappings {
			if m.MapRef == host.HostRef || (host.ClusterRef != "" && m.MapRef == host.ClusterRef) {
				mappingsToDelete = append(mappingsToDelete, m)
			}
		}
	} else {
		// If NodeID is not specified, CSI spec says to unpublish from ALL nodes.
		// We will remove ALL LUN mappings for THIS volume.
		// Use with caution: effective for completely detaching a volume.
		mappingsToDelete = vol.Mappings
	}

	for _, m := range mappingsToDelete {
		klog.Infof("Removing mapping %s (LUN %d) for volume %s", m.LunMappingRef, m.LunNumber, volID)
		// Direct API call to delete mapping
		resp, _, err := d.client.InvokeAPI(ctx, nil, "DELETE", "/volume-mappings/"+m.LunMappingRef)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to delete mapping %s: %v", m.LunMappingRef, err)
		}
		// InvokeAPI returns success even for 404/500 if the request completed. We must check the status code.
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {

			// Verification Logic:
			// If the DELETE failed with an error (e.g. 400 Bad Request, 500 Internal Error),
			// we double-check if the Volume or Mapping actually still exists.
			// It is possible the mapping was removed by another thread/process or the error is spurious.
			klog.Warningf("Delete mapping %s failed with status %d; verifying if resource still exists...", m.LunMappingRef, resp.StatusCode)

			// 1. Re-fetch volume to see if it or its mappings are gone
			volCheck, errCheck := d.client.GetVolumeByRef(ctx, volID)
			if errCheck != nil {
				// If getting the volume fails, we need to determine if it's because it wasn't found.
				// d.client.GetVolumeByRef usually errors if not found.
				// Since we don't have typed errors from the client yet, we check the string or rely on the fact that if we can't read it, we probably can't delete it either.
				// But specifically for 404 on GET, we assume success (volume gone).
				if strings.Contains(strings.ToLower(errCheck.Error()), "not found") || strings.Contains(errCheck.Error(), "404") {
					klog.Infof("Volume %s not found during verification (err: %v); assuming mapping %s is removed.", volID, errCheck, m.LunMappingRef)
					continue // Success, move to next mapping
				}

				// If we can't verify due to some other error, we must return the original delete error (or the verification error)
				return nil, status.Errorf(codes.Internal, "Failed to delete mapping %s (status %d) and failed to verify volume state: %v", m.LunMappingRef, resp.StatusCode, errCheck)
			}

			// 2. Volume exists from double-check. Check if the specific mapping is present.
			mappingStillExists := false
			for _, mc := range volCheck.Mappings {
				if mc.LunMappingRef == m.LunMappingRef {
					mappingStillExists = true
					break
				}
			}

			if !mappingStillExists {
				klog.Infof("Mapping %s confirmed absent during verification; ignoring delete failure.", m.LunMappingRef)
				continue
			}

			// Mapping still exists, and DELETE failed.
			return nil, status.Errorf(codes.Internal, "Failed to delete mapping %s: API returned status %d and mapping persists.", m.LunMappingRef, resp.StatusCode)
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
	// SANtricity API VolumeExpansionRequest takes 'expansionSize' which must be greater than current capacity.
	// This implies it expects the New Total Size (Target Size).
	// We verify current capacity to ensure idempotency and valid request.

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

	klog.Infof("Expanding volume %s from %d to %d bytes", volID, currentBytes, requiredBytes)

	err = d.client.ExpandVolume(ctx, volID, requiredBytes)
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
