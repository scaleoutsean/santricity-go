# iscsi-verify

`iscsi-verify` is a lightweight, read-only CLI tool designed for "Day 2" verification of iSCSI connectivity on Linux hosts. It validates target discovery, session status, and block device mappings without modifying system configuration.

## Features

*   **Target Discovery**: Lists available targets from provided portal IPs.
*   **Session Validation**: Checks if the host is currently logged in to discovered targets.
*   **Device Correlation**: Maps active sessions to local block devices (e.g., `/dev/sdX`) and retrieves their WWIDs.
*   **Safe**: Read-only operation; does not perform logins, logouts, or format disks.
*   **No Secrets**: Intentionally does not support CHAP secrets to ensure safe usage in restricted environments.

## Prerequisites

*   Linux Host
*   Root privileges (required to read iSCSI session details and sysfs device maps)
*   `iscsi-initiator-utils` (or equivalent package providing `iscsiadm`)

## Build

```bash
cd tools/iscsi-verify
go build -o iscsi-verify .
```

## Usage

Run the tool with one or more Target Portal IPs:

```bash
sudo ./iscsi-verify <portal-ip-1> [portal-ip-2] ...
```

### Example

```bash
sudo ./iscsi-verify 10.10.10.1 10.10.10.2
```

## Output Format

The tool outputs standard CSV:

```csv
Portal,TargetIQN,Status,Devices,WWIDs
```

| Column | Description |
|--------|-------------|
| **Portal** | The IP address of the iSCSI portal checked. |
| **TargetIQN** | The IQN of the target discovered on that portal. |
| **Status** | `Discovered` (seen but not logged in), `LoggedIn` (active session), or `Mapped` (session + block devices found). |
| **Devices** | Semicolon-separated list of local block devices (e.g., `sdb;sdc`). |
| **WWIDs** | Semicolon-separated list of WWIDs corresponding to the devices. |
