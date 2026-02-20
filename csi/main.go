package main

import (
	"flag"
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
		if iqn, err := driver.GetISCSIInitiatorName(); err == nil && iqn != "" {
			klog.Infof("Auto-detected IQN: %s. Using as Node ID.", iqn)
			// Reset the driver instance or just update the field?
			// Since driver struct is private fields, we need a setter or just recreate it?
			// Driver struct has nodeID private.
			// Let's just recreate it or better yet, make NewDriver smart?
			// But NewDriver doesn't know 'isNode' yet.
			// Recreating is safest.
			drv, _ = driver.NewDriver(*driverName, iqn, *endpoint, *apiUrl, *userId, *password)
		} else {
			klog.Warningf("Could not auto-detect iSCSI IQN: %v. Using provided NodeID: %s", err, *nodeID)
		}
	}

	if err := drv.Run(isController, isNode); err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}
}
