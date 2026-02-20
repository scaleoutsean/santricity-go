package driver

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/goiscsi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
	"k8s.io/utils/exec"
)

// GetISCSIInitiatorName reads the local initiator name from /etc/iscsi/initiatorname.iscsi
// Exported for main.go
func GetISCSIInitiatorName() (string, error) {
	return getISCSIInitiatorName()
}

// GetNVMeInitiatorName reads the local initiator name from /etc/nvme/hostnqn
// Exported for main.go
func GetNVMeInitiatorName() (string, error) {
	return getNVMeInitiatorName()
}

// getISCSIInitiatorName reads the local initiator name from /etc/iscsi/initiatorname.iscsi
func getISCSIInitiatorName() (string, error) {
	// Standard location
	path := "/etc/iscsi/initiatorname.iscsi"
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %v", path, err)
	}

	// Parse file content: InitiatorName=iqn.xxxx...
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "InitiatorName=") {
			return strings.TrimPrefix(line, "InitiatorName="), nil
		}
	}
	return "", fmt.Errorf("GetISCSIInitiatorName not found in %s", path)
}

// getNVMeInitiatorName reads the local initiator name from /etc/nvme/hostnqn
func getNVMeInitiatorName() (string, error) {
	// Standard location
	path := "/etc/nvme/hostnqn"
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %v", path, err)
	}

	// The file typically contains just the NQN string on the first line
	lines := strings.Split(string(content), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0]), nil
	}

	return "", fmt.Errorf("host NQN not found in %s", path)
}

func (d *Driver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	// 1. Parse Request
	volID := req.GetVolumeId()
	if volID == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}

	stagingTargetPath := req.GetStagingTargetPath()
	if stagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Staging Target Path must be provided")
	}

	pubCtx := req.GetPublishContext()
	lunStr, ok := pubCtx["lun"]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "LUN not found in PublishContext")
	}
	lun, _ := strconv.Atoi(lunStr)

	targetPortal, ok := pubCtx["targetPortal"]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "Target Portal not found in PublishContext")
	}

	var devicePath string
	var err error

	// Check if NVMe or iSCSI
	if targetNQN, isNVMe := pubCtx["targetNQN"]; isNVMe {
		klog.Infof("NodeStageVolume NVMe: Vol=%s, Portal=%s, NQN=%s, LUN=%d", volID, targetPortal, targetNQN, lun)

		// Parse portal
		host, port, err := net.SplitHostPort(targetPortal)
		if err != nil {
			// If no port found, assume default or portal is just IP
			host = targetPortal
			port = NVMeDefaultPort
		}

		if err := ConnectNVMeSubsystem(ctx, targetNQN, host, port); err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to connect NVMe subsystem: %v", err)
		}

		devicePath, err = WaitForNVMeDevice(ctx, targetNQN, 15*time.Second)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to find NVMe device: %v", err)
		}

		klog.Infof("Found NVMe device: %s", devicePath)

	} else if targetIQN, isISCSI := pubCtx["targetIQN"]; isISCSI {
		klog.Infof("NodeStageVolume iSCSI: Vol=%s, Portal=%s, IQN=%s, LUN=%d", volID, targetPortal, targetIQN, lun)

		// 2. iSCSI Connect using goiscsi
		client := goiscsi.NewLinuxISCSI(nil)

		// Discovery logic (simplified)
		// ... existing logic condensed ...

		target := goiscsi.ISCSITarget{
			Target: targetIQN,
			Portal: targetPortal,
		}

		// Check session
		sessions, err := client.GetSessions()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to get sessions: %v", err)
		}

		isLoggedIn := false
		for _, s := range sessions {
			if s.Target == targetIQN && (s.Portal == targetPortal || strings.HasPrefix(s.Portal, targetPortal)) {
				isLoggedIn = true
				break
			}
		}

		if !isLoggedIn {
			klog.Infof("Logging in to %s at %s", targetIQN, targetPortal)
			if err := client.PerformLogin(target); err != nil {
				return nil, status.Errorf(codes.Internal, "iSCSI Login failed: %v", err)
			}
		}

		// 3. Find Device
		// Use /dev/disk/by-id/wwn-0x... prefix for robustness.
		// SANtricity generally exports WWNs that map to Linux disk IDs.
		// The WWN in PublishContext is usually upper case 32 chars hex.
		// Linux udev standardizes on lower case "wwn-0x<WWN>" or "scsi-3<WWN>".
		// We prefer wwn-0x prefix if available, but scsi-3 is also common for SAS/FC/iSCSI.

		wwn, wwnOk := pubCtx["wwn"]
		if wwnOk && len(wwn) > 0 {
			// Convert to lower case
			wwnLower := strings.ToLower(wwn)

			// Try construct path: /dev/disk/by-id/wwn-0x<wwn>
			wwnPath := fmt.Sprintf("/dev/disk/by-id/wwn-0x%s", wwnLower)

			// Try alternative: /dev/disk/by-id/scsi-3%s (SCSI page 83 T10 OUI)
			// SANtricity uses NAA IEEE registered extended (6) so it starts with 6.
			// scsi-3 + WWN is typical.
			scsiPath := fmt.Sprintf("/dev/disk/by-id/scsi-3%s", wwnLower)

			// Wait loop for either path
			timeout := 15 * time.Second
			start := time.Now()
			found := false
			const tick = 500 * time.Millisecond

			for time.Since(start) < timeout {
				if _, err := os.Stat(wwnPath); err == nil {
					devicePath = wwnPath
					found = true
					break
				}
				if _, err := os.Stat(scsiPath); err == nil {
					devicePath = scsiPath
					found = true
					break
				}
				time.Sleep(tick)
			}

			if !found {
				klog.Warningf("Timed out waiting for WWN path %s or %s. Falling back to by-path.", wwnPath, scsiPath)
				// Fallback to by-path if WWN symlinks not yet created
				devicePath = fmt.Sprintf("/dev/disk/by-path/ip-%s-iscsi-%s-lun-%d", targetPortal, targetIQN, lun)
			} else {
				klog.Infof("Found iSCSI device via WWN: %s", devicePath)
			}
		} else {
			// Fallback if no WWN provided
			devicePath = fmt.Sprintf("/dev/disk/by-path/ip-%s-iscsi-%s-lun-%d", targetPortal, targetIQN, lun)
		}

		// Wait for device (simple fallback check)
		if _, err := os.Stat(devicePath); os.IsNotExist(err) {
			time.Sleep(2 * time.Second)
		}
		if _, err := os.Stat(devicePath); os.IsNotExist(err) {
			klog.Warningf("Device %s not found immediately after login", devicePath)
		}

	} else {
		return nil, status.Error(codes.InvalidArgument, "Neither targetNQN nor targetIQN found in PublishContext")
	}

	// Common Mount Logic
	// Resolve symlink to get /dev/sdX if needed
	realDev, err := filepath.EvalSymlinks(devicePath)
	if err != nil {
		realDev = devicePath
	}
	klog.Infof("Processing device %s -> %s", devicePath, realDev)

	// 4. Mount/Format/Resize
	// Use SafeFormatAndMount from k8s.io/mount-utils
	mounter := mount.New("")
	safeMounter := mount.SafeFormatAndMount{
		Interface: mounter,
		Exec:      exec.New(),
	}

	fsType := req.GetVolumeCapability().GetMount().GetFsType()
	if fsType == "" {
		fsType = "ext4" // Default
	}

	mountOptions := req.GetVolumeCapability().GetMount().GetMountFlags()

	// Default to enabling 'discard' (UNMAP/TRIM) unless explicitly disabled in StorageClass options.
	// This is beneficial for SSDs (wear leveling) and thin provisioning, and generally harmless for HDDs/Thick volumes.
	// Modern Linux filesystems (ext4, xfs) handle discard safely.
	hasDiscard := false
	for _, opt := range mountOptions {
		if opt == "discard" || opt == "nodiscard" {
			hasDiscard = true
			break
		}
	}
	if !hasDiscard {
		mountOptions = append(mountOptions, "discard")
	}

	// Check if already mounted
	notMnt, err := mounter.IsLikelyNotMountPoint(stagingTargetPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "Failed to check if %s is mount point: %v", stagingTargetPath, err)
	}

	if !notMnt {
		klog.Infof("Volume %s already mounted at %s", volID, stagingTargetPath)
		return &csi.NodeStageVolumeResponse{}, nil
	}

	// Create staging path if it doesn't exist
	if err := os.MkdirAll(stagingTargetPath, 0750); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create staging path: %v", err)
	}

	klog.Infof("Formatting (if needed) and mounting %s to %s with fs=%s options=%v", realDev, stagingTargetPath, fsType, mountOptions)

	// FormatAndMount triggers 'blkid' checking internally.
	// If the device has an existing filesystem, it will NOT reformat it, unless the filesystem is corrupted.
	// It handles the "check existing FS" logic safely.
	err = safeMounter.FormatAndMount(realDev, stagingTargetPath, fsType, mountOptions)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to format and mount device: %v", err)
	}

	// NOTE: We do not perform "auto-expansion" (resizing FS if underlying device grew) here.
	// K8s ControllerExpandVolume -> NodeExpandVolume workflow handles that explicitly.
	// If a user resizes out-of-band (e.g. Terraform), they must trigger a PVC edit or pod restart
	// might trigger standard Kubelet checks, but we strictly follow CSI spec to avoid race conditions.

	return &csi.NodeStageVolumeResponse{}, nil
}

func (d *Driver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	// Unmount and Logout
	stagingTargetPath := req.GetStagingTargetPath()
	if stagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Staging Target Path must be provided")
	}

	klog.Infof("Unmounting %s", stagingTargetPath)

	// Use mounter to unmount
	mounter := mount.New("")

	// Unmount
	err := mounter.Unmount(stagingTargetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to unmount %s: %v", stagingTargetPath, err)
	}

	// Remove directory
	if err := os.Remove(stagingTargetPath); err != nil && !os.IsNotExist(err) {
		klog.Warningf("Failed to remove staging path %s: %v", stagingTargetPath, err)
	}

	// We assume we leave the session open if other volumes use it, or we rely on a cleaner process.
	// For this level of implementation, just return success.

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (d *Driver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	// Bind Mount from Staging Path to Target Path
	targetPath := req.GetTargetPath()
	stagingPath := req.GetStagingTargetPath()

	if targetPath == "" || stagingPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Start/Target paths missing")
	}

	// Mkdir target
	if err := os.MkdirAll(targetPath, 0750); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create target path %s: %v", targetPath, err)
	}

	mounter := mount.New("")

	// Check if already mounted
	notMnt, err := mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "Failed to check mount point: %v", err)
	}
	if !notMnt {
		klog.Infof("Volume already bind-mounted to %s", targetPath)
		return &csi.NodePublishVolumeResponse{}, nil
	}

	// Mount --bind
	// The source for a bind mount is the staging path.
	// The target is the target path.
	klog.Infof("Bind mounting %s to %s", stagingPath, targetPath)
	if err := mounter.Mount(stagingPath, targetPath, "none", []string{"bind"}); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to bind mount: %v", err)
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *Driver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	targetPath := req.GetTargetPath()
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path is empty")
	}

	mounter := mount.New("")

	// Check if mounted before unmounting to be idempotent
	notMnt, err := mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "Check mount point failed: %v", err)
	}
	if notMnt {
		klog.Infof("Volume %s not mounted, skipping unmount", targetPath)
		// Clean up directory just in case
		os.Remove(targetPath)
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	klog.Infof("Unmounting %s", targetPath)
	if err := mounter.Unmount(targetPath); err != nil {
		return nil, status.Errorf(codes.Internal, "Unmount failed: %v", err)
	}
	// Remove mount point directory
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		klog.Warningf("Failed to remove target path %s: %v", targetPath, err)
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (d *Driver) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "NodeGetVolumeStats not implemented yet")
}

func (d *Driver) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	volID := req.GetVolumeId()
	volumePath := req.GetVolumePath()

	if volID == "" || volumePath == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID and Path must be provided")
	}

	klog.Infof("NodeExpandVolume: Expanding FS at %s", volumePath)

	mounter := mount.New("")
	// We need to find the device path mounted at volumePath for ResizeFS
	devicePath, _, err := mount.GetDeviceNameFromMount(mounter, volumePath)
	if err != nil {
		// Fallback: try to guess or use volumePath if supported?
		// But let's log error and fail, as safe resize requires knowing the device.
		return nil, status.Errorf(codes.Internal, "Failed to determine device path for %s: %v", volumePath, err)
	}

	executor := exec.New()
	resizer := mount.NewResizeFs(executor)

	// Check if we need to resize
	needResize, err := resizer.NeedResize(devicePath, volumePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not check if resize needed: %v", err)
	}

	if needResize {
		klog.Infof("Resizing filesystem on %s at %s", devicePath, volumePath)
		if _, err := resizer.Resize(devicePath, volumePath); err != nil {
			return nil, status.Errorf(codes.Internal, "Could not resize filesystem: %v", err)
		}
	} else {
		klog.Info("Filesystem does not need resizing")
	}

	return &csi.NodeExpandVolumeResponse{
		CapacityBytes: req.GetCapacityRange().GetRequiredBytes(),
	}, nil
}

func (d *Driver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
					},
				},
			},
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
					},
				},
			},
		},
	}, nil
}

func (d *Driver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: d.nodeID,
	}, nil
}

// ----------------------------------------------------------------------------
// iSCSI Session Reaper
// ----------------------------------------------------------------------------

type iscsiSession struct {
	IQN    string
	Portal string
}

// startReaper runs a background lopp that monitors iSCSI sessions.
// If enabled, it terminates stale sessions (those without mapped devices).
// If disabled, it just logs the count of active sessions.
func (d *Driver) startReaper() {
	klog.Info("Starting iSCSI Session Reaper")
	if d.reaperEnabled {
		klog.Info("Reaper is ENABLED - stale sessions will be terminated")
	} else {
		klog.Info("Reaper is DISABLED - only logging session counts")
	}

	ticker := time.NewTicker(d.reaperInterval)
	defer ticker.Stop()

	for range ticker.C {
		sessions := d.scanSessions()

		// Always log the count, this allows monitoring for leaks even if reaper is disabled
		klog.Infof("Reaper: Found %d active iSCSI sessions", len(sessions))

		// If disabled, skip cleanup logic
		if !d.reaperEnabled {
			continue
		}

		for _, s := range sessions {
			iqn := s.IQN
			if iqn == "" {
				continue
			}

			// Check if device exists for this session
			if d.deviceExistsForIQN(iqn) {
				// Seen with device, clear any 'unseen' marker
				d.unseenMu.Lock()
				delete(d.unseenWithoutDevice, iqn)
				d.unseenMu.Unlock()
				continue
			}

			// Not seen: mark first time seen without device
			d.unseenMu.Lock()
			first, ok := d.unseenWithoutDevice[iqn]
			if !ok {
				d.unseenWithoutDevice[iqn] = time.Now()
				d.unseenMu.Unlock()
				continue
			}
			d.unseenMu.Unlock()

			// Check if it has been stale long enough
			// Hardcoded 60s stale time for now, or use env var if we added it (we didn't yet)
			if time.Since(first) >= 60*time.Second {
				klog.Infof("Reaper: Session %s (Portal %s) is stale (>60s without device). Terminating...", iqn, s.Portal)
				if err := d.terminateISCSISession(iqn); err != nil {
					klog.Warningf("Reaper: Failed to terminate session %s: %v", iqn, err)
				} else {
					klog.Infof("Reaper: Successfully terminated session %s", iqn)
					d.unseenMu.Lock()
					delete(d.unseenWithoutDevice, iqn)
					d.unseenMu.Unlock()
				}
			}
		}
	}
}

// scanSessions parses 'iscsiadm -m session' output
func (d *Driver) scanSessions() []iscsiSession {
	cmd := exec.New().Command("iscsiadm", "-m", "session")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Exit code 21 means no sessions
		if exitErr, ok := err.(exec.ExitError); ok && exitErr.ExitStatus() == 21 {
			return nil
		}
		// Don't log error on every tick if just empty or minor issue, but debug helpful
		klog.V(4).Infof("scanSessions: iscsiadm error: %v", err)
		return nil
	}

	lines := strings.Split(string(out), "\n")
	var sessions []iscsiSession
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		// Format: tcp: [1] 10.x.x.x:3260,1 iqn.xxxx
		fields := strings.Fields(l)
		var iqn, portal string
		for _, f := range fields {
			if strings.HasPrefix(f, "iqn.") || strings.HasPrefix(f, "eui.") {
				iqn = f
			}
			// Portal usually contains IP:port
			if strings.Contains(f, ":") && (strings.Contains(f, ",") || strings.Contains(f, ".")) {
				portal = strings.TrimSuffix(f, ",1")
			}
		}
		if iqn != "" {
			sessions = append(sessions, iscsiSession{IQN: iqn, Portal: portal})
		}
	}
	return sessions
}

// deviceExistsForIQN checks if any device in /dev/disk/by-path/ matches the IQN
func (d *Driver) deviceExistsForIQN(iqn string) bool {
	// Pattern: /dev/disk/by-path/*iqn*
	pattern := fmt.Sprintf("/dev/disk/by-path/*%s*", iqn)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		klog.Errorf("deviceExistsForIQN: glob error: %v", err)
		return false
	}
	return len(matches) > 0
}

func (d *Driver) terminateISCSISession(iqn string) error {
	cmd := exec.New().Command("iscsiadm", "-m", "node", "-T", iqn, "--logout")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("logout failed: %v, output: %s", err, string(out))
	}
	return nil
}
