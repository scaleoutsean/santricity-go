package driver

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	santricity "github.com/scaleoutsean/santricity-go"
	"github.com/scaleoutsean/santricity-go/csi/metrics"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

var (
	DriverName = "santricity.scaleoutsean.github.io"
	GitCommit  = "unknown"
)

const (
	Version = "1.0.0-alpha.4"
)

type Driver struct {
	csi.UnimplementedIdentityServer
	csi.UnimplementedControllerServer
	csi.UnimplementedNodeServer

	name     string
	nodeID   string
	endpoint string
	client   *santricity.Client

	// Config
	dataIPs []string // Override for data path IPs (comma-separated SANTRICITY_DATA_IPS)

	// Server
	srv *grpc.Server
	m   sync.Mutex

	// Reaper state
	reaperEnabled       bool
	reaperInterval      time.Duration
	unseenWithoutDevice map[string]time.Time // iqn -> first time seen without device
	unseenMu            sync.Mutex
}

func NewDriver(driverName, nodeID, endpoint, apiUrl, user, password string) (*Driver, error) {
	if driverName == "" {
		driverName = DriverName
	}
	klog.Infof("Driver: %v Version: %v Commit: %v", driverName, Version, GitCommit)

	// If API URL is provided, initialize client
	var client *santricity.Client
	var dataIPs []string

	// Fallback to Env vars if flags are empty, similar to CLI
	if apiUrl == "" {
		apiUrl = os.Getenv("SANTRICITY_ENDPOINT")
	}
	if user == "admin" && os.Getenv("SANTRICITY_USERNAME") != "" {
		// "admin" is the flag default, but if env var is set, prefer that
		user = os.Getenv("SANTRICITY_USERNAME")
	}
	if password == "" {
		password = os.Getenv("SANTRICITY_PASSWORD")
	}

	if apiUrl != "" {
		// Clean the API URL. The library expects just the Hostname/IP in ApiControllers.
		// It constructs the URL itself using strict "https" and "/devmgr/v2".
		var apiHosts []string
		var apiPort int = 8443

		// Split by comma to support multiple controllers
		rawEndpoints := strings.Split(apiUrl, ",")
		for _, rawEndpoint := range rawEndpoints {
			endpoint := strings.TrimSpace(rawEndpoint)
			if endpoint == "" {
				continue
			}

			var host string

			// Handle "https://" prefix if present
			if strings.Contains(endpoint, "://") {
				if u, err := url.Parse(endpoint); err == nil {
					host = u.Hostname()
					// Update port if present (first one wins or overwrites ideally they are same)
					if p := u.Port(); p != "" {
						if portNum, err := strconv.Atoi(p); err == nil {
							apiPort = portNum
						}
					}
				} else {
					klog.Warningf("Failed to parse API URL %s, using as raw string", endpoint)
					host = endpoint
				}
			} else {
				// Handle "host:port" or just "host"
				if h, port, err := net.SplitHostPort(endpoint); err == nil {
					host = h
					if portNum, err := strconv.Atoi(port); err == nil {
						apiPort = portNum
					}
				} else {
					host = endpoint
				}
			}

			if host == "" {
				klog.Warningf("Parsed API host from '%s' is empty, reverting to full provided string", endpoint)
				host = endpoint
			}
			apiHosts = append(apiHosts, host)
		}

		klog.Infof("Initializing SANtricity Client with Hosts: %v, Port: %d", apiHosts, apiPort)

		// Basic configuration for the client
		config := santricity.ClientConfig{
			ApiControllers: apiHosts,
			Username:       user,
			Password:       password,
			ApiPort:        apiPort,
			// For Embedded Web Services (on-controller), acceptable ArrayID (SystemID) "1".
			ArrayID: "1",
			DebugTraceFlags: map[string]bool{
				// Disable tracing sensitive info by default
				"method": true,
				"api":    false, // Potentially logs full requests including credentials
			},
			VerifyTLS: strings.EqualFold(os.Getenv("SANTRICITY_VERIFY_TLS"), "true"), // Explicitly disable verification for lab use by default
			OnRequest: metrics.RequestCallback,
		}

		client = santricity.NewAPIClient(context.Background(), config)

		// Check for Data IPs override (for environments where management and data are split)
		dataIPsEnv := os.Getenv("SANTRICITY_DATA_IPS")
		if dataIPsEnv != "" {
			parts := strings.Split(dataIPsEnv, ",")
			for _, p := range parts {
				ip := strings.TrimSpace(p)
				if ip != "" {
					dataIPs = append(dataIPs, ip)
				}
			}
			klog.Infof("Using explicit Data IPs for target portals: %v", dataIPs)
		} else {
			klog.Infof("No SANTRICITY_DATA_IPS configured; defaulting to Management IPs for data transport.")
		}

		// Verify connectivity immediately
		klog.Info("Verifying SANtricity API connectivity...")
		checkCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		sys, err := client.GetStorageSystem(checkCtx)
		if err != nil {
			klog.Errorf("Connectivity Check Failed: %v", err)
			// Optional: Dump more info or panic?
			// Panic might be good to restart the pod quickly if config is wrong
			// But for now purely logging is safer to debug
		} else {
			klog.Infof("Connectivity Check Passed: Connected to array %s (ID: %s)", sys.Name, sys.ID)
		}
	} else {
		klog.Warning("No valid SANtricity API URL provided. Controller operations will fail.")
	}

	return &Driver{
		name:     driverName,
		nodeID:   nodeID,
		endpoint: endpoint,
		client:   client,
		dataIPs:  dataIPs,

		reaperEnabled:       strings.EqualFold(os.Getenv("SANTRICITY_ENABLE_REAPER"), "true"),
		reaperInterval:      60 * time.Second, // Default 60s
		unseenWithoutDevice: make(map[string]time.Time),
	}, nil
}

func (d *Driver) Run(isController, isNode bool) error {
	u, err := url.Parse(d.endpoint)
	if err != nil {
		return fmt.Errorf("unable to parse address: %q", err)
	}

	addr := u.Path
	if u.Scheme != "unix" {
		addr = u.Host
	}

	klog.Infof("Starting listener on %s", addr)

	if u.Scheme == "unix" {
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %v", addr, err)
		}
	}

	lis, err := net.Listen(u.Scheme, addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(sanitizeGRPC),
	}
	d.srv = grpc.NewServer(opts...)

	// Register Identity (Required for all components)
	csi.RegisterIdentityServer(d.srv, d)

	if isController {
		klog.Info("Registering Controller Server")
		csi.RegisterControllerServer(d.srv, d)
		// Start metrics collection for controller
		go d.runMetricsLoop()
	}

	// Always register Node Server, even if running as Controller,
	// because some sidecars (like csi-resizer) create a client connection
	// and inspect Node capabilities indiscriminately.
	klog.Info("Registering Node Server")
	csi.RegisterNodeServer(d.srv, d)

	// Check node identity for iSCSI specific tasks
	if isNode && strings.HasPrefix(d.nodeID, "iqn.") {
		// Start iSCSI Session Reaper
		go d.startReaper()
	}

	klog.Info("Serving GRPC")
	return d.srv.Serve(lis)
}

func sanitizeGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// Sanitize request for logging
	sanitizedReq := protosanitizer.StripSecrets(req)
	klog.V(2).Infof("GRPC Request: %s: %s", info.FullMethod, sanitizedReq)

	resp, err := handler(ctx, req)
	if err != nil {
		klog.Errorf("GRPC Error: %s: %v", info.FullMethod, err)
	} else {
		// Optionally log response if needed, but requests are more critical for debugging
		// klog.V(5).Infof("GRPC Response: %s: %v", info.FullMethod, resp)
	}
	return resp, err
}

func (d *Driver) runMetricsLoop() {
	// Update metrics immediately
	d.updateVolumeMetrics()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		d.updateVolumeMetrics()
	}
}

func (d *Driver) updateVolumeMetrics() {
	if d.client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	volumes, err := d.client.GetVolumes(ctx)
	if err != nil {
		klog.Warningf("Failed to update volume metrics: %v", err)
		return
	}
	metrics.DriverVolumesTotal.Set(float64(len(volumes)))
}
