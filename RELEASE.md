# RELEASE

# Go Client 

```sh
VER="v1.0.0-beta.1"
git tag ${VER}
git push origin ${VER}
GOPROXY=proxy.golang.org go list -m github.com/scaleoutsean/santricity-go@${VER}
```

# CSI

- Update version in ./csi/driver/driver.go 

```sh
git add csi/driver/driver.go
git commit -m "Bump CSI driver version to 0.1.12"
git tag csi-v0.1.12
git push origin csi-v0.1.12
# GOPROXY=proxy.golang.org go list -m github.com/scaleoutsean/santricity-go@{VER}
```
