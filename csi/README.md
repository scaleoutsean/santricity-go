# SANtricity CSI Driver

This is a Container Storage Interface (CSI) driver for NetApp SANtricity storage systems. It enables dynamic provisioning of persistent volumes on Kubernetes clusters using E-Series storage.

## Features

- **Dynamic Provisioning**: Create volumes on demand.
- **iSCSI and NVMe/RoCE Connectivity**: Automatically mounts volumes to pods using iSCSI or NVMe/RoCE (support built in, needs testing)
- **Volume Expansion**: Resizing of PVCs.
- **Volume Metadata Tagging**: PVC metadata are stored in SANtricity volume metadata.
- **DDP Support**: Optimized for Dynamic Disk Pools (DDP) with RAID1/RAID6 volume support.

## Prerequisites

- Kubernetes 1.20+
- NetApp E-Series Array with SANtricity Web Services Proxy (or embedded API)
- iSCSI initiator tools installed on all worker nodes (`open-iscsi`).

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
   
   Edit `csi/deploy/controller.yaml` to set your SANtricity API Endpoint (`SANTRICITY_ENDPOINT`).

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

## Configuration Strategy: Dynamic Disk Pools (DDP)

This driver is designed to work efficiently with DDP. The recommended strategy is to create a single large DDP on your array and use Kubernetes StorageClasses to define different service levels (e.g., RAID levels or specific pools).

### 1. Identify your Pool ID
Retrieve the `VolumeGroupRef` (Pool ID) of your DDP from the SANtricity UI or CLI.

### 2. Configure StorageClasses
Edit `csi/deploy/csi-driver.yaml` to point to your specific Pool ID. You can define multiple classes for the same pool to expose different RAID features supported by DDP.

## Protocol Support & Limitations

This driver currently supports **iSCSI** and **NVMe-oF (RoCE)**. 

**Limitation: One Protocol per Cluster**
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
