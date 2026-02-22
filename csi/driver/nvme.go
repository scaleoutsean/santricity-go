package driver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// NVMe constants
const (
	NVMeSysPath     = "/sys/class/nvme-subsystem"
	NVMeSubsysNQN   = "/subsysnqn"
	NVMeDefaultPort = "4420" // Default for NVMe/TCP and NVMe/RoCE
	NVMeTransport   = "rdma" // Default transport for SANtricity (RoCE)
)

var (
	nvmeControllerRegex = regexp.MustCompile(`^nvme[0-9]+$`)
	nvmeNamespaceRegex  = regexp.MustCompile(`^nvme[0-9]+n[0-9]+$`)
)

// ConnectNVMeSubsystem establishes an NVMe connection (RoCE/RDMA) to the target
func ConnectNVMeSubsystem(ctx context.Context, targetNQN string, targetIP string, targetPort string) error {
	if targetPort == "" {
		targetPort = NVMeDefaultPort
	}

	// Check if already connected?
	// The nvme connect command is generally idempotent or handles it, but let's be safe later.
	// For now, simple wrapper.

	// nvme connect -t rdma -n <nqn> -a <ip> -s <port> -l -1
	// -l -1 means infinite retry/ctrl-loss-tmo? Trident CSI uses it. SANtricity docs recommend 3600s, but lets do -1.
	// We might want to use default system settings or user configurable ones.

	args := []string{"connect", "-t", NVMeTransport, "-n", targetNQN, "-a", targetIP, "-s", targetPort, "-l", "-1"}
	klog.V(4).Infof("Running: nvme %v", args)

	cmd := exec.CommandContext(ctx, "nvme", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If "already connected", nvme connect might return success or a specific code.
		// If it's already connected, we might see "operation already in progress" or strict success.
		// We'll trust the output for now and improve robustness later.
		if strings.Contains(string(out), "already connected") {
			klog.V(4).Infof("NVMe subsystem %s already connected to %s", targetNQN, targetIP)
			return nil
		}
		return fmt.Errorf("failed to connect NVMe subsystem %s at %s: %v, output: %s", targetNQN, targetIP, err, string(out))
	}

	return nil
}

// DisconnectNVMeSubsystem disconnects from the target subsystem
func DisconnectNVMeSubsystem(ctx context.Context, targetNQN string) error {
	args := []string{"disconnect", "-n", targetNQN}
	klog.V(4).Infof("Running: nvme %v", args)

	cmd := exec.CommandContext(ctx, "nvme", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If not connected, it might fail.
		if strings.Contains(string(out), "no such device") || strings.Contains(string(out), "not found") {
			return nil
		}
		return fmt.Errorf("failed to disconnect NVMe subsystem %s: %v, output: %s", targetNQN, err, string(out))
	}

	return nil
}

// FindNVMeDeviceForNQN locates the /dev/nvmeXnY device for a given NQN and LUN (NSID)
// This scans /sys/class/nvme-subsystem/nvme-subsys*/nvmeXnY/nsid
func FindNVMeDeviceForNQN(ctx context.Context, targetNQN string, lun int32) (string, error) {
	// 1. Iterate over /sys/class/nvme-subsystem/nvme-subsys*
	entries, err := os.ReadDir(NVMeSysPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("NVMe subsystem class not found at %s. IS NVMe loaded?", NVMeSysPath)
	}
	if err != nil {
		return "", fmt.Errorf("failed to list NVMe subsystems: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "nvme-subsys") {
			continue
		}

		// Read the NQN of this subsystem
		subsysDir := filepath.Join(NVMeSysPath, name)
		nqnPath := filepath.Join(subsysDir, "subsysnqn")
		content, err := os.ReadFile(nqnPath)
		if err != nil {
			klog.Warningf("Failed to read %s: %v", nqnPath, err)
			continue
		}

		foundNQN := strings.TrimSpace(string(content))
		if foundNQN == targetNQN {
			// Found the correct subsystem!
			// Check items in subsystem directory for namespaces (e.g. nvme0n1)
			subItems, err := os.ReadDir(subsysDir)
			if err != nil {
				return "", fmt.Errorf("failed to read subsystem dir %s: %v", subsysDir, err)
			}

			targetNSID := fmt.Sprintf("%d", lun)

			// Strategy 1: Look for namespaces directly under subsystem (e.g. /sys/class/nvme-subsystem/nvme-subsys0/nvme0n1)
			for _, item := range subItems {
				if nvmeNamespaceRegex.MatchString(item.Name()) {
					nsPath := filepath.Join(subsysDir, item.Name())
					nsidPath := filepath.Join(nsPath, "nsid")

					nsidBytes, err := os.ReadFile(nsidPath)
					if err == nil {
						nsidStr := strings.TrimSpace(string(nsidBytes))
						if nsidStr == targetNSID {
							devPath := fmt.Sprintf("/dev/%s", item.Name())
							klog.Infof("Found matching NVMe namespace %s for NQN=%s LUN=%d (NSID=%s)", devPath, targetNQN, lun, nsidStr)
							return devPath, nil
						}
					}
				}
			}

			// Strategy 2: Look for namespaces under controllers (e.g. /sys/class/nvme-subsystem/nvme-subsys0/nvme0/nvme0n1)
			for _, item := range subItems {
				if nvmeControllerRegex.MatchString(item.Name()) {
					ctrlPath := filepath.Join(subsysDir, item.Name())
					ctrlItems, err := os.ReadDir(ctrlPath)
					if err == nil {
						for _, ctrlItem := range ctrlItems {
							if nvmeNamespaceRegex.MatchString(ctrlItem.Name()) {
								nsPath := filepath.Join(ctrlPath, ctrlItem.Name())
								nsidPath := filepath.Join(nsPath, "nsid")

								nsidBytes, err := os.ReadFile(nsidPath)
								if err == nil {
									nsidStr := strings.TrimSpace(string(nsidBytes))
									if nsidStr == targetNSID {
										// Warning: Usually the device name is unique system-wide (e.g. nvme0n1).
										// Constructing /dev/nvme0n1 is correct regardless of where we found it in sysfs.
										devPath := fmt.Sprintf("/dev/%s", ctrlItem.Name())
										klog.Infof("Found matching NVMe namespace %s via controller %s for NQN=%s LUN=%d (NSID=%s)", devPath, item.Name(), targetNQN, lun, nsidStr)
										return devPath, nil
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("device not found for NQN %s LUN %d", targetNQN, lun)
}

// WaitForNVMeDevice waits for the device to appear
func WaitForNVMeDevice(ctx context.Context, targetNQN string, lun int32, timeout time.Duration) (string, error) {
	start := time.Now()
	for {
		dev, err := FindNVMeDeviceForNQN(ctx, targetNQN, lun)
		if err == nil && dev != "" {
			return dev, nil
		}

		if time.Since(start) > timeout {
			return "", fmt.Errorf("timeout waiting for device for NQN %s LUN %d", targetNQN, lun)
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(500 * time.Millisecond):
			continue
		}
	}
}
