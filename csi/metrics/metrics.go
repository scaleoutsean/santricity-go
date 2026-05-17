package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
)

var (
	SantricityAPIRequestsLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "santricity_api_request_duration_seconds",
			Help:    "Latency of SANtricity API requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status_code"},
	)

	SantricityAPIRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "santricity_api_requests_total",
			Help: "Total number of SANtricity API requests",
		},
		[]string{"method", "path", "status_code"},
	)

	DriverVolumesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "santricity_volumes_total",
			Help: "Estimated number of volumes managed by this driver instance",
		},
		[]string{"system_id", "pool_id"},
	)

	DriverVolumeInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "santricity_volume_info",
			Help: "Physical capacity in bytes allocated on the SANtricity array per PVC.",
		},
		[]string{"pvc_namespace", "pvc_name", "volume_id", "volume_name", "csi_driver"},
	)
)

var registry = prometheus.NewRegistry()

func RegisterMetrics() {
	registry.MustRegister(SantricityAPIRequestsLatencies)
	registry.MustRegister(SantricityAPIRequestsTotal)
	registry.MustRegister(DriverVolumesTotal)
	registry.MustRegister(DriverVolumeInfo)
}

func StartMetricsServer(port string) {
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	http.Handle("/metrics", handler)

	addr := ":" + port
	klog.Infof("Starting metrics server on %s", addr)
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			klog.Errorf("Failed to start metrics server: %v", err)
		}
	}()
}

func RequestCallback(method string, path string, statusCode int, duration time.Duration) {
	statusStr := strconv.Itoa(statusCode)

	SantricityAPIRequestsLatencies.WithLabelValues(method, path, statusStr).Observe(duration.Seconds())
	SantricityAPIRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
}
