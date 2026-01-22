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
	santricity "github.com/scaleoutsean/santricity-go"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

const (
	DriverName = "santricity.scaleoutsean.github.io"
	Version    = "0.1.11"
)

type Driver struct {
	csi.UnimplementedIdentityServer
	csi.UnimplementedControllerServer
	csi.UnimplementedNodeServer

	nodeID   string
	endpoint string
	client   *santricity.Client

	// Server
	srv *grpc.Server
	m   sync.Mutex
}

func NewDriver(nodeID, endpoint, apiUrl, user, password string) (*Driver, error) {
	klog.Infof("Driver: %v Version: %v", DriverName, Version)

	// If API URL is provided, initialize client
	var client *santricity.Client

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
			// For Embedded Web Services (on-controller), ArrayID is typically "1".
			ArrayID: "1",
			DebugTraceFlags: map[string]bool{
				"method": true,
				"api":    true,
			},
			VerifyTLS: false, // Explicitly disable verification for lab use
		}

		client = santricity.NewAPIClient(context.Background(), config)

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
		nodeID:   nodeID,
		endpoint: endpoint,
		client:   client,
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
		grpc.UnaryInterceptor(logGRPC),
	}
	d.srv = grpc.NewServer(opts...)

	// Register Identity (Required for all components)
	csi.RegisterIdentityServer(d.srv, d)

	if isController {
		klog.Info("Registering Controller Server")
		csi.RegisterControllerServer(d.srv, d)
	}

	// Always register Node Server, even if running as Controller,
	// because some sidecars (like csi-resizer) create a client connection
	// and inspect Node capabilities indiscriminately.
	klog.Info("Registering Node Server")
	csi.RegisterNodeServer(d.srv, d)

	klog.Info("Serving GRPC")
	return d.srv.Serve(lis)
}

func logGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	klog.Infof("GRPC call: %s", info.FullMethod)
	resp, err := handler(ctx, req)
	if err != nil {
		klog.Errorf("GRPC error: %v", err)
	}
	return resp, err
}
