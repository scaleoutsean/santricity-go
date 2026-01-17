package main

import (
"context"
"fmt"
"os"
"path/filepath"
"strings"
"time"

"github.com/dell/goiscsi"
)

func main() {
if len(os.Args) < 2 {
fmt.Println("Usage: iscsi-verify <target-portal-ip> [target-portal-ip...]")
fmt.Println("Example: iscsi-verify 10.10.10.1")
os.Exit(1)
}

portals := os.Args[1:]
client := goiscsi.NewLinuxISCSI(nil)

// Print CSV Header
fmt.Println("Portal,TargetIQN,Status,Devices,WWIDs")

for _, portal := range portals {
// 1. Discover Targets
targets, err := client.DiscoverTargets(portal, false)
if err != nil {
// Print error to stderr so it doesn't break CSV parsing if redirected
fmt.Fprintf(os.Stderr, "Error discovering targets on %s: %v\n", portal, err)
continue
}

// 2. Get Active Sessions to correlate
sessions, err := client.GetSessions()
if err != nil {
fmt.Fprintf(os.Stderr, "Error getting sessions: %v\n", err)
continue
}

for _, t := range targets {
status := "Discovered"
deviceNames := []string{}
wwids := []string{}

// Check if we are logged in to this target
for _, s := range sessions {
if s.Target == t.Target && s.Portal == t.Portal {
status = "LoggedIn"

// Attempt to find devices for this target
devs, ids := findDevicesForTarget(t.Target, t.Portal)
if len(devs) > 0 {
deviceNames = devs 
wwids = ids
status = "Mapped"
}
}
}

// Format columns: Portal, Target, Status, Devices (semicolon joined), WWIDs (semicolon joined)
devStr := strings.Join(deviceNames, ";")
idStr := strings.Join(wwids, ";")
fmt.Printf("%s,%s,%s,%s,%s\n", t.Portal, t.Target, status, devStr, idStr)
}
}
}

// findDevicesForTarget attempts to correlate active iSCSI sessions to block devices
// by looking at /dev/disk/by-path which usually contains entries like:
// ip-<portal>-iscsi-<iqn>-lun-<lun> -> ../../sdx
func findDevicesForTarget(iqn, portal string) ([]string, []string) {
// We'll search for files in /dev/disk/by-path containing the IQN
matches, _ := filepath.Glob("/dev/disk/by-path/*" + iqn + "*")

devMap := make(map[string]bool)
wwidMap := make(map[string]bool)

for _, path := range matches {
// Resolve symlink to get sdX
realPath, err := filepath.EvalSymlinks(path)
if err == nil {
devName := filepath.Base(realPath)
devMap[devName] = true

if wwn, ok := getWWIDForDevice(devName); ok {
wwidMap[wwn] = true
}
}
}

devs := []string{}
for k := range devMap {
devs = append(devs, k)
}

ids := []string{}
for k := range wwidMap {
ids = append(ids, k)
}

return devs, ids
}

// getWWIDForDevice attempts to find the WWID for a given block device name (e.g. sdb)
// This scans /dev/disk/by-id/scsi-* to find which one points to our dev
func getWWIDForDevice(devName string) (string, bool) {
// Look for /dev/disk/by-id/scsi-3* which is standard for DM-MP/SCSI
matches, _ := filepath.Glob("/dev/disk/by-id/scsi-3*")
for _, path := range matches {
realPath, err := filepath.EvalSymlinks(path)
if err == nil {
if filepath.Base(realPath) == devName {
// Found it. Extract WWID from filename "scsi-3..."
return strings.TrimPrefix(filepath.Base(path), "scsi-3"), true
}
}
}
return "", false
}

// Helper to mimic Rescan if needed (not active path currently)
func rescanHost(client goiscsi.ISCSIinterface) {
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
_ = ctx
// In a real scenario, you might call client.Scan(...) or similar if exposed,
// or perform a manual sysfs scan. Detailed rescan implementation omitted for this tool
// as per requirements to be lightweight reader.
client.PerformRescan()
}
