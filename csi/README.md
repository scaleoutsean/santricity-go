# SANtricity CSI Driver

This is a Container Storage Interface (CSI) driver for NetApp SANtricity storage systems. It enables dynamic provisioning of persistent volumes on Kubernetes clusters using E-Series storage.

## Features

- **Dynamic Provisioning**: Create, publish and remove volumes on demand.
- **iSCSI and NVMe/RoCE Connectivity**: Automatically mounts volumes to pods using iSCSI or NVMe/RoCE
- **Volume Expansion**: Resizing of PVCs.
- **Volume Metadata Tagging**: PVC metadata are stored in SANtricity volume metadata.
- **DDP Support**: Optimized for Dynamic Disk Pools (DDP) with RAID1/RAID6 volume support.
- **Independent Multiple Backends for Multi-Rack, Multi-Array Clusters**: One egg per basket. Independent CSI for each rack and in-rack E-Series array(s)
- **Prometheus Metrics**: volume and storage pool metrics sufficient to cross-reference SANtricity CSI resources with information gathered by collectors such as [EPA](https://github.com/scaleoutsean/eseries-perf-analyzer)
- **Snapshots and Linked Clones**: TODO

## Prerequisites

- Kubernetes 1.30+
- NetApp E-Series array with SANtricity v11.90+ (no SANtricity Web Services Proxy and similar)
- Appropriate client package (`open-iscsi` and `multipath-tools` for iSCSI, or `nvme-cli` for NVMe/RoCE) installed on all worker nodes

## Building the Driver

To build the CSI driver image for both Controller and Node:

```bash
docker build -t santricity-csi:latest -f csi/Dockerfile .
```

*Note: You may need to push this to a registry accessible by your cluster.*

Alternatively, use a GHCR image from [this page](https://github.com/scaleoutsean/santricity-go/pkgs/container/santricity-go). Pick a recent `:csi-{VERSION}` tag and set the Helm chart's CSI driver's image tag to that (see below).

## Deployment

The recommended way to deploy the driver is using the included Helm chart.

1. **Configure Parameters**:
  Copy `charts/santricity-csi/values.yaml` to `my-values.yaml` and edit to match your environment.
  
  *   **Credentials**: Set `controller.credentials.username` and `controller.credentials.password`.
  *   **Controller Endpoint**: Set `controller.endpoint` to your SANtricity management IP(s) (e.g., `"https://10.10.1.10:8443"`).
  *   **Data IPs**: Set `controller.dataIPs` to the iSCSI/NVMe-oF data interfaces.
  *   **Kubelet Directory**: If using a distribution like k0s, k3s, or MicroK8s, set `node.kubeletDir`.
      *   **Standard**: `/var/lib/kubelet` (default)
      *   **k0s**: `/var/lib/k0s/kubelet`
      *   **k3s**: `/var/lib/rancher/k3s/agent/kubelet`
      *   **MicroK8s**: `/var/snap/microk8s/common/var/lib/kubelet`
      *   **Talos**: (default)

2. **Install with Helm**:
  Run the install command from the root of the repository, referencing the correct location of `my-values.yaml`:

  ```bash
  helm upgrade --install santricity-csi ./charts/santricity-csi -f my-values.yaml --namespace santricity-csi --create-namespace
  ```

  Or override the values directly (example for k0s):

  ```bash
  helm install santricity-csi ./charts/santricity-csi \
     --namespace kube-system \
     --set controller.endpoint="https://10.10.1.10:8443,https://10.10.1.11:8443" \
     --set controller.credentials.username="admin" \
     --set controller.credentials.password="password" \
     --set node.kubeletDir="/var/lib/k0s/kubelet"
  ```

  Check SANtricity CSI's status:
  ```sh
  kubectl get pods -n santricity-csi # -l app.kubernetes.io/name=santricity-csi
  ```

  If no issues, delete `my-values.yaml` or edit out your SANtricity credentials from it. Next, create Storage Classes (examples are available below).

### Manual Deployment (Alternative)

If you cannot use Helm in your cluster, a single-file manifest is available at `csi/deploy/bundle.yaml`.

1. **Prepare the Manifest**:
   Open `csi/deploy/bundle.yaml` and edit the following sections:
   *   **Secret**: Find `kind: Secret` (name `santricity-csi-credentials`) and set `stringData.username` and `stringData.password`.
   *   **ConfigMap / Controller Deployment**: Find the `Deployment` named `santricity-csi-controller` and update the `SANTRICITY_ENDPOINT` env var or args if hardcoded. (Note: The bundle is generated from default values, so you might need to find where the endpoint is defined).
   *   **Node DaemonSet**: If using a custom kubelet directory (e.g. k0s), find the `DaemonSet` named `santricity-csi-node` and update all `/var/lib/kubelet` paths to your custom path.

2. **Deploy**:

   ```bash
   kubectl apply -f csi/deploy/bundle.yaml
   ```

Note about backend names (whether it's one or more):

- CSIDriver Object: The metadata.name in csi-driver.yaml must match your custom `--driver-name`.
- StorageClass: The provisioner field must also match that name.

### Deployment with multiple backends

Vast majority of users will have one E-Series and one backend per cluster. If you have multiple storage systems per cluster:
- Name them uniquely
- Deploy each CSI backend to its own namespace

Example:

- Backend A (E-Series 1): Uses API addresses 10.10.1.10, 10.10.2.11
- Backend B (E-Series 2): Uses API addresses 10.10.3.10, 10.10.4.11

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

Note that in this case, if you enable and don't customize metrics ports, there may be port conflicts if you don't set unique values on a per-CSI instance basis.

## Configuration Strategy: Dynamic Disk Pools (DDP)

This driver is designed to work efficiently with DDP. The recommended strategy is to create a single large DDP on your array and use Kubernetes StorageClasses to define different service levels (e.g., RAID levels or specific pools).

### 1. Identify your Pool ID

Retrieve the `VolumeGroupRef` (Pool ID) of your DDP from the SANtricity UI or CLI.

You may use `santricity-cli` for that. build in repository's root and use it to get storage pool information. Example:

```sh
go build ./cmd/santricity-cli/
./santricity-cli --endpoint 10.0.0.1 --username monitor --password "monitor123" get pools --insecure
```

See [README](../README.md) for SANtricity CLI or use E-Series Swagger if you get stuck.

### 2. Configure StorageClasses

Edit Storage Class samples below to point to your specific Pool ID. You can define multiple classes for the same pool to expose different RAID features supported by DDP.

Technically, SANtricity CSI does not distinguish pools by type, but DDP is the recommended type. If you have many small Kubernetes volumes or run very specific workloads, feel free to try a classic storage pool - or multiple, or mixed DDP and classic - as well.

Volume sector size can be defined with the `block_size` parameter.

## Protocol Support & Limitations

This driver is currently expected to work with **iSCSI** and **NVMe-oF (RoCE)** with ext3, ext4, btrfs filesystems.

**LUKS** is not supported. 

**Limitation: One Protocol per Kubernetes Cluster**

Due to the way hosts are identified (Single IQN per node for iSCSI, Single NQN per node for NVMe), a Kubernetes node should be configured to strictly use **one** data protocol for CSI data traffic. Mixing protocols on the same node (e.g., some pods using iSCSI and others using NVMe) is not supported, as it would require duplicate or complex host registrations on the array.
If you can group worker nodes by protocol type, you could potentially create and use two "virtual" backends to the same storage pool.

- **iSCSI**: Uses the node's Initiator IQN (`/etc/iscsi/initiatorname.iscsi`).
- **NVMe-oF**: Uses the node's Host NQN (`/etc/nvme/hostnqn`).

**Example: Fast (RAID 1 equivalent on DDP)**
```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: santricity-iscsi-raid1
  annotations:
    storageclass.kubernetes.io/is-default-class: "false"
provisioner: santricity.scaleoutsean.github.io
volumeBindingMode: WaitForFirstConsumer
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
  annotations:
    storageclass.kubernetes.io/is-default-class: "false"
provisioner: santricity.scaleoutsean.github.io
volumeBindingMode: WaitForFirstConsumer
parameters:
  poolID: "04000000600A098000E3C1B000002CED62CF874D" 
  raidLevel: "raid6"
```

Notes:

- Storage Class annotation on a SC may be set to `true` if you want to make that SC default
- Change `provisioner` values if your CSI driver is named differently

This allows you to manage capacity at the single DDP level while offering different performance/protection tiers to Kubernetes users.

Create a PVC using one of the SCs above:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: santricity-pg-wal
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: santricity-iscsi-raid1
```

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

Access modes:

- [x] `SINGLE_NODE_WRITER`
- [x] `SINGLE_NODE_SINGLE_WRITER`
- [x] `SINGLE_NODE_MULTI_WRITER` (`MountVolume`, `BlockVolume`, `ReadWriteOnce` access)
- [ ] `SINGLE_NODE_READER_ONLY` (requires testing, possibly improvements)
- [ ] `MULTI_NODE_READER_ONLY`
- [ ] `MULTI_NODE_SINGLE_WRITER`
- [ ] `MULTI_NODE_MULTI_WRITER` (likely works, but hasn't been tested)

Multi-node access requires host coordination (e.g. Ceph, BeeGFS), but SCSI-3 reservations and `hostGroup` are both supported, so that should work out of box.

API access:

- [x] `MountVolume` ("filesystem")
- [x] `BlockVolume` ("raw device" mode for KubeVirt and more)

#### Multi-rack deployment

- Deploy CSI Driver "A" with `--driver-name=santricity.rack1`
- Deploy CSI Driver "B" with `--driver-name=santricity.rack2`

For topology-aware provisioning, add `allowedTopologies` to your `StorageClass`:

```yaml
allowedTopologies:
- matchLabelExpressions:
  - key: topology.kubernetes.io/zone
    values:
    - rack1
```

#### Multi-protocol deployment

EF-Series can't serve two supported protocols at once, but even so - one may have E4000 (iSCSI) and EF600 (NVMe/RoCE) in the same cluster. To handle that situation, install two instances of SANtricity CSI and configure them like so:

- **Instance 1** - the "NVMe-oF" Driver:
  - Driver Name: --driver-name=santricity.nvme
  - Node Selector: node-protocol: nvme
  - Args: --protocol=nvme (optional for safety; CSI Node prefers `nvmeof`)
  - StorageClass: provisioner: santricity.nvme
- **Instance 2** - the "iSCSI" Driver:
  - Driver Name: --driver-name=santricity.iscsi
  - Node Selector: node-protocol: iscsi
  - Args: --protocol=iscsi
  - StorageClass: provisioner: santricity.iscsi

## Troubleshooting

### Logs and debug mode

Check the logs of the controller:

```sh
kubectl get pods -n santricity-csi
kubectl logs <pod> -n santricity-csi 
```

**NOTE:** you may run SANtricity CSI in debug mode, but note that secrets may leak into debug logs.

### CSI Controller API connectivity issues (CNI/Routing)

If the controller pod logs show `context deadline exceeded` when connecting to the SANtricity API, but you can reach the API from the Kubernetes nodes directly, your cluster's CNI may be failing to route or SNAT pod traffic to the external management network.

As a quick workaround, you can force the controller deployment to use the host's network:

```bash
kubectl patch deployment santricity-csi-controller -n santricity-csi -p '{"spec":{"template":{"spec":{"hostNetwork":true}}}}'
```

### Uninstall and re-install keeping custom arguments from Helm 

If you upgrade SANtricity CSI and use custom kubelet directory, keep the customization:

```sh
helm upgrade --install santricity-csi ./charts/santricity-csi \
  --set node.kubeletDir=/var/lib/k0s/kubelet
```

Uninstall wrong upgrade:

```sh
helm uninstall santricity-csi -n santricity-csi 
```

### Finding the leader CSI Controller

Use `get lease` to find the holder.

```sh
$ kubectl get lease -n santricity-csi 
NAME                                                         HOLDER                                                 AGE
external-attacher-leader-santricity-scaleoutsean-github-io   santricity-csi-controller-6bcc5679b7-bbns5             26m
external-resizer-santricity-scaleoutsean-github-io           santricity-csi-controller-6bcc5679b7-xqwlb             26m
santricity-scaleoutsean-github-io                            1778993623808-8215-santricity-scaleoutsean-github-io   26m
```

## Monitoring

Controller and Node are both enabled; you may disable or modify these `my-values.yaml` when deploying SANtricity CSI.

```yaml
metrics:
  enabled: true
  port: 8080
  enableNodeMetrics: true
  nodePort: 8081
```

The driver exposes Prometheus metrics on port 8080 (default) for the controller, and 8081 for the nodes, at `/metrics`. 

To quickly check the generated metrics manually, you can use `kubectl exec`:

```sh
# Check controller metrics
kubectl exec -it -n santricity-csi deployment/santricity-csi-controller -c csi-driver -- wget -qO- 127.0.0.1:8080/metrics

# Check node metrics (pick a specific pod from the daemonset)
NODE_POD=$(kubectl get pod -n santricity-csi -o name | grep node | head -n 1)
kubectl exec -it -n santricity-csi $NODE_POD -c csi-driver -- wget -qO- 127.0.0.1:8081/metrics
```

### Available metrics

- `santricity_api_requests_total`: Counter of API requests to the array, labeled by method, path, and status code.
- `santricity_api_request_duration_seconds`: Histogram of API request latencies.
- `santricity_volume_info_bytes`: Physical capacity in bytes allocated on the SANtricity array per PVC.
- `santricity_volumes_total`: Estimated number of volumes on pools used by this instance (updated every 5 minutes).

**Note on Node PVC Metrics:** The SANtricity CSI Node pods currently do not export custom metrics by design. For deep node-level PVC filesystem statistics (which were recently deprecated/removed from native Kubernetes kubelet metrics), we recommend using community tools such as [kubelet-volume-stats-exporter](https://github.com/dkaliberda/kubelet-volume-stats-exporter) alongside your standard array monitoring tools.
