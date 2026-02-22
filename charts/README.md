# Helm Chart for SANtricity CSI 

Create my-values.yaml in repository root:

```yaml
controller:
  endpoint: "https://10.10.10.10:8443" # SANtricity controlle(s) management IPs
  dataIPs: "192.168.1.1,192.168.2.1"   # iSCSI or NVMe Portal IPs (dual fabric)
  credentials:
    username: "admin"
    password: ""      # SANtricity admin password

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
      raidLevel: "raid1"
```

Deploy (from Github repository root):

```sh
helm install santricity-csi ./charts/santricity-csi -f my-values.yaml --namespace santricity-csi --create-namespace
```
