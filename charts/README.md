# Helm Chart for SANtricity CSI 

Available values are in `./charts/santricity-csi/values.yaml`. Create `my-values.yaml` in repository root with whatever you want to override:

```yaml
controller:
  endpoint: "https://10.10.10.10:8443" # SANtricity controller(s) management IPs
  dataIPs: "192.168.1.1,192.168.2.1"   # iSCSI or NVMe Portal IPs (dual fabric)
  credentials:
    username: "admin" # SANtricity has a 'storage' role with a smaller scope.
    password: ""      # SANtricity 'storage' (alternatively, 'admin') account password

storageClasses:
  - name: santricity-nvme-raid1
    isDefault: false
    # The poolID (DDP Ref) you want to use for this class
    poolID: "0000000000000000000000000000000000000000" 
    reclaimPolicy: Delete
    volumeBindingMode: Immediate
    allowVolumeExpansion: true
    parameters:
      mediaType: "nvme"
      fsType: "xfs"
      raidLevel: "raid1" # raid0 on DDP or raid1 on a R0 disk group won't work
```

Deploy (from Github repository root):

```sh
helm install santricity-csi ./charts/santricity-csi -f my-values.yaml --namespace santricity-csi --create-namespace
```

Uninstall:

```sh
helm uninstall santricity-csi -n santricity-csi
```

Example of a manual clean-up of cluster-scoped resources after SANtricity CSI was installed in the default namespace:

```sh
kubectl delete clusterrole santricity-csi-controller-role santricity-csi-node-role
kubectl delete clusterrolebinding santricity-csi-controller-binding santricity-csi-node-binding
kubectl delete csidriver santricity.scaleoutsean.github.com # (the driver name you used in chart)
```
