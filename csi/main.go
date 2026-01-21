package main

import (
	"flag"
	"os"

	"github.com/scaleoutsean/santricity-go/csi/driver"
	"k8s.io/klog/v2"
)

var (
	endpoint      = flag.String("endpoint", "unix:///var/lib/kubelet/plugins/santricity.scaleoutsean.github.io/csi.sock", "CSI endpoint")
	nodeID        = flag.String("nodeid", "", "node id")
	apiUrl        = flag.String("api-url", "", "SANtricity API URL")
	userId        = flag.String("user", "admin", "SANtricity API User")
	password      = flag.String("password", "", "SANtricity API Password")
	runController = flag.Bool("controller", false, "Run controller service")
	runNode       = flag.Bool("node", false, "Run node service")
	version       = flag.Bool("version", false, "Print the version and exit")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	if *version {
		// print version logic here
		os.Exit(0)
	}

	if *nodeID == "" {
		klog.Warning("nodeid is empty")
	}

	handle()
}

func handle() {
	drv, err := driver.NewDriver(*nodeID, *endpoint, *apiUrl, *userId, *password)
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
			drv, _ = driver.NewDriver(iqn, *endpoint, *apiUrl, *userId, *password)
		} else {
			klog.Warningf("Could not auto-detect iSCSI IQN: %v. Using provided NodeID: %s", err, *nodeID)
		}
	}

	if err := drv.Run(isController, isNode); err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}
}
