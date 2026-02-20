package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/scaleoutsean/santricity-go/csi/driver"
	"github.com/scaleoutsean/santricity-go/csi/metrics"
	log "github.com/sirupsen/logrus"
	"k8s.io/klog/v2"
)

var (
	endpoint      = flag.String("endpoint", "unix:///var/lib/kubelet/plugins/santricity.scaleoutsean.github.io/csi.sock", "CSI endpoint")
	driverName    = flag.String("driver-name", "", "Name of the CSI driver (default: santricity.scaleoutsean.github.io)")
	nodeID        = flag.String("nodeid", "", "node id")
	apiUrl        = flag.String("api-url", "", "SANtricity API URL")
	userId        = flag.String("user", "admin", "SANtricity API User")
	password      = flag.String("password", "", "SANtricity API Password")
	runController = flag.Bool("controller", false, "Run controller service")
	runNode       = flag.Bool("node", false, "Run node service")
	version       = flag.Bool("version", false, "Print the version and exit")
	metricsPort   = flag.String("metrics-port", "8080", "Port to serve Prometheus metrics on")
	logLevel      = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	protocol      = flag.String("protocol", "", "Force specific protocol for Node ID (iscsi, nvme)")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// Configure Logrus
	level, err := log.ParseLevel(*logLevel)
	if err != nil {
		klog.Errorf("Invalid log level %s: %v", *logLevel, err)
		level = log.InfoLevel
	}
	log.SetLevel(level)
	log.SetFormatter(&log.JSONFormatter{}) // Optional: Use JSON for better machine parsing
	klog.Infof("Log level set to %s", level)

	if *version {
		// print version logic here
		os.Exit(0)
	}

	if *nodeID == "" {
		klog.Warning("nodeid is empty")
	}

	metrics.RegisterMetrics()
	metrics.StartMetricsServer(*metricsPort)

	handle()
}

func handle() {
	drv, err := driver.NewDriver(*driverName, *nodeID, *endpoint, *apiUrl, *userId, *password)
	if err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}

	// Determine mode
	// If neither is set, run both (monolith mode for simple testing)
	// Or standard logic:
	// - Controller plugin runs mainly Identity + Controller
	// - Node plugin runs Identity + Node

	isController := *runController
	isNode := *runNode

	if !isController && !isNode {
		klog.Info("Running in Monolith mode (Controller + Node)")
		isController = true
		isNode = true
	}

	// If running as Node, try to auto-detect IQN and override nodeID
	if isNode {
		var detectedID string
		var protocolErr error

		// Check for explicit preference or default auto-detection
		preferNVMe := *protocol == "nvme" || *protocol == "" // Default to checking NVMe first if empty
		preferISCSI := *protocol == "iscsi"

		if preferNVMe {
			if nqn, err := driver.GetNVMeInitiatorName(); err == nil && nqn != "" {
				detectedID = nqn
				klog.Infof("Auto-detected NQN: %s", nqn)
			} else if *protocol == "nvme" {
				protocolErr = fmt.Errorf("NVMe protocol forced but no NQN found: %v", err)
			}
		}

		// If NVMe failed or wasn't preferred/forced, try iSCSI
		if detectedID == "" && (preferISCSI || *protocol == "") {
			if iqn, err := driver.GetISCSIInitiatorName(); err == nil && iqn != "" {
				detectedID = iqn
				klog.Infof("Auto-detected IQN: %s", iqn)
			} else if *protocol == "iscsi" {
				protocolErr = fmt.Errorf("iSCSI protocol forced but no IQN found: %v", err)
			}
		}

		if detectedID != "" {
			klog.Infof("Using %s as Node ID.", detectedID)
			drv, _ = driver.NewDriver(*driverName, detectedID, *endpoint, *apiUrl, *userId, *password)
		} else {
			if protocolErr != nil {
				klog.Error(protocolErr)
			}
			klog.Warningf("Could not auto-detect NQN or iSCSI IQN. Using provided NodeID: %s", *nodeID)
		}
	}

	if err := drv.Run(isController, isNode); err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}
}
