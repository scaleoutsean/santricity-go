package santricity

const (
	StorageAPITimeoutSeconds = 90
	MinTLSVersion            = 0x0303 // TLS 1.2
)

var OrchestratorTelemetry = struct {
	Platform        string
	PlatformVersion string
}{
	Platform:        "kubernetes",
	PlatformVersion: "unknown",
}
