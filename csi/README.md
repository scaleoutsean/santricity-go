# SANtricity CSI Driver

This is a Container Storage Interface (CSI) driver for NetApp SANtricity storage systems. It enables dynamic provisioning of persistent volumes on Kubernetes clusters using E-Series storage.

## Features

- **Dynamic Provisioning**: Create volumes on demand.
- **iSCSI and NVMe/RoCE Connectivity**: Automatically mounts volumes to pods using iSCSI or NVMe/RoCE (support built in, needs testing)
- **Volume Expansion**: Resizing of PVCs.
- **Volume Metadata Tagging**: PVC metadata are stored in SANtricity volume metadata.
- **DDP Support**: Optimized for Dynamic Disk Pools (DDP) with RAID1/RAID6 volume support.
- **Indepdendent Multiple Backends for Multi-Rack, Multi-Array Clusters**: One egg per basket. Independent CSI for each rack and in-rack E-Series array(s)

## Prerequisites

- Kubernetes 1.20+
- NetApp E-Series Array with SANtricity (embedded API only; Web Services Proxy is not supported).
- iSCSI initiator tools installed on all worker nodes (`open-iscsi`) or (experimental) NVMe/RoCE.

## Building the Driver

To build the CSI driver image:

```bash
docker build -t santricity-csi:latest -f csi/Dockerfile .
```

*Note: You may need to push this to a registry accessible by your cluster.*

## Deployment

1. **Configure Credentials**:
   Edit `csi/deploy/secret-example.yaml` with your SANtricity username and password, then apply it:
   ```bash
   kubectl apply -f csi/deploy/secret-example.yaml
   ```
   If you convert password to `base64`, remember to strip (or not include) newline character from the string before conversion.
   
   Edit `csi/deploy/controller.yaml` to set your SANtricity API Endpoint (`SANTRICITY_ENDPOINT`). You can specify multiple management IPs separated by commas (e.g., `"https://10.10.1.10:8443,https://10.10.1.11:8443"`) to ensure the driver remains functional if one controller is unreachable (e.g. during upgrades or network issues). 

   Note that SANtricity volume PVC metadata feature is enabled with `--extra-create-metadata=true` on the external sidecar.


2. **Verify Kubelet Path (Node Service)**:
   The default `csi/deploy/node.yaml` uses `/var/lib/kubelet`. If you are using a distribution with a different path, you **must** update the `hostPath` entries in `node.yaml`.
   *   **k0s**: `/var/lib/k0s/kubelet`
   *   **k3s**: `/var/lib/rancher/k3s/agent/kubelet`
   *   **MicroK8s**: `/var/snap/microk8s/common/var/lib/kubelet`

3. **Deploy Manifests**:

   ```bash
   kubectl apply -f csi/deploy/csi-driver.yaml
   kubectl apply -f csi/deploy/controller.yaml
   kubectl apply -f csi/deploy/node.yaml
   ```

Note about backend names (whether it's one or more):

- CSIDriver Object: The metadata.name in csi-driver.yaml must match your custom `--driver-name`.
- StorageClass: The provisioner field must also match that name.

### Deployment with multiple backends

Vast majority of users will have one E-Series and one backend per cluster. If you have multiple storage systems per cluster:
- Name them uniquely
- Deploy each CSI backend to its own namespace

Example:

- Backend A (E-Series 1): Uses Portals 10.10.1.10, 10.10.2.11
- Backend B (E-Series 2): Uses Portals 10.10.3.10, 10.10.4.11

```yaml
# Set driver-name in YAML for the first instance and deploy to own namespace
args:
  - "--driver-name=santricity.backend-a"
  - "--endpoint=unix:///var/lib/kubelet/plugins/santricity.backend-a/csi.sock"
  - "--api-url=https://10.10.1.10"
# Another instance: give it unique name, deploy to another namespace
args:
  - "--driver-name=santricity.backend-b"
  - "--endpoint=unix:///var/lib/kubelet/plugins/santricity.backend-b/csi.sock"
  - "--api-url=https://10.10.3.10"
```

## Configuration Strategy: Dynamic Disk Pools (DDP)

This driver is designed to work efficiently with DDP. The recommended strategy is to create a single large DDP on your array and use Kubernetes StorageClasses to define different service levels (e.g., RAID levels or specific pools).

### 1. Identify your Pool ID
Retrieve the `VolumeGroupRef` (Pool ID) of your DDP from the SANtricity UI or CLI.

### 2. Configure StorageClasses
Edit `csi/deploy/csi-driver.yaml` to point to your specific Pool ID. You can define multiple classes for the same pool to expose different RAID features supported by DDP. Another 

## Protocol Support & Limitations

This driver currently supports **iSCSI** and **NVMe-oF (RoCE)** with ext3, ext4, btrfs filesystems.

**LUKS** is not supported. 

**Limitation: One Protocol per Kubernetes Cluster**

Due to the way hosts are identified (Single IQN per node for iSCSI, Single NQN per node for NVMe), a Kubernetes node should be configured to strictly use **one** data protocol for CSI traffic. Mixing protocols on the same node (e.g., some pods using iSCSI and others using NVMe) is not supported, as it would require duplicate or complex host registrations on the array.
If you can group worker nodes by protocol type, you could potentially create and use two "virtual" backends to the same storage pool.

- **iSCSI**: Uses the node's Initiator IQN (`/etc/iscsi/initiatorname.iscsi`).
- **NVMe-oF**: Uses the node's Host NQN (`/etc/nvme/hostnqn`).

**Example: Fast (RAID 1 equivalent on DDP)**
```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: santricity-iscsi-raid1
provisioner: santricity.scaleoutsean.github.io
parameters:
  poolID: "04000000600A098000E3C1B000002CED62CF874D" # Your DDP ID
  raidLevel: "raid1"
```

**Example: Capacity (RAID 6 equivalent on DDP)**
```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: santricity-iscsi-raid6
provisioner: santricity.scaleoutsean.github.io
parameters:
  poolID: "04000000600A098000E3C1B000002CED62CF874D" 
  raidLevel: "raid6"
```

This allows you to manage capacity at the single DDP level while offering different performance/protection tiers to Kubernetes users.

### Capabilities 

Current capabilities include:

- [x] `CONTROLLER_SERVICE`
- [ ] `VOLUME_ACCESSIBILITY_CONSTRAINTS` (see below on multi-rack deployment)
- [x] `CREATE_DELETE_VOLUME`
- [x] `PUBLISH_UNPUBLISH_VOLUME`
- [x] `EXPAND_VOLUME` 
- [x] `STAGE_UNSTAGE_VOLUME`
- [ ] `CREATE_DELETE_SNAPSHOT`
- [ ] `CLONE_VOLUME`
- [ ] `GROUP_CONTROLLER_SERVICE`

#### Multi-rack deployment

- Deploy CSI Driver "A" with `--driver-name=santricity.rack1`
- Deploy CSI Driver "B" with `--driver-name=santricity.rack2`

```yaml
allowedTopologies:
- matchLabelExpressions:
  - key: topology.kubernetes.io/zone
    values:
    - rack1
```

## Troubleshooting

Check the logs of the controller:
```bash
kubectl logs -f deployment/santricity-csi-controller -n kube-system -c csi-driver
```

Check the logs of the node plugin on a specific node:
```bash
kubectl logs -f daemonset/santricity-csi-node -n kube-system
```

## Versioning and releases

```sh
vim ./csi/driver/driver.go
git tag csi-v<version>
git push origin master --tags
```

## Monitoring

The driver exposes Prometheus metrics on port 8080 (default) at `/metrics`.

### Available Metrics

- `santricity_api_requests_total`: Counter of API requests to the array, labeled by method, path, and status code.
- `santricity_api_request_duration_seconds`: Histogram of API request latencies.
- `santricity_volumes_total`: Gauge of total volumes on the array (updated every 5 minutes).

### scraping Configuration

Add the following annotations to your driver Pods (DaemonSet/Deployment) to enable automatic scraping:

```yaml
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "8080"
    prometheus.io/path: "/metrics"
```

If you need to collect these without authentication and from outside of Kubernetes, you need to open access to every node via `NodePort` service type. 

```sh
kubectl apply -f csi/deploy/metrics-service-example.yaml
```

To open that port only on the node where `santricity-csi-controller` runs:

```yaml
spec:
  type: NodePort
  externalTrafficPolicy: Local  # <--- The Magic Switch
  ports:
    - port: 8080
      nodePort: 32080
```

The best approach security-wise is to leverage Kubernetes for authentication as it is done with other services that require authentication.
