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
	// SantricityAPIRequestsLatencies tracks the latency of API requests
	SantricityAPIRequestsLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "santricity_api_request_duration_seconds",
			Help:    "Latency of SANtricity API requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status_code"},
	)

	// SantricityAPIRequestsTotal tracks the total number of API requests
	SantricityAPIRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "santricity_api_requests_total",
			Help: "Total number of SANtricity API requests",
		},
		[]string{"method", "path", "status_code"},
	)

	// DriverVolumesTotal tracks the number of volumes (simple gauge)
	DriverVolumesTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "santricity_volumes_total",
			Help: "Estimated number of volumes managed by this driver instance",
		},
	)
)

func RegisterMetrics() {
	prometheus.MustRegister(SantricityAPIRequestsLatencies)
	prometheus.MustRegister(SantricityAPIRequestsTotal)
	prometheus.MustRegister(DriverVolumesTotal)
}

func StartMetricsServer(port string) {
	http.Handle("/metrics", promhttp.Handler())
	addr := ":" + port
	klog.Infof("Starting metrics server on %s", addr)
	go func() {
		// We use a separate HTTP server for metrics so it doesn't conflict with GRPC or other things
		if err := http.ListenAndServe(addr, nil); err != nil {
			klog.Errorf("Failed to start metrics server: %v", err)
		}
	}()
}

// RequestCallback matches the signature of ClientConfig.OnRequest
func RequestCallback(method string, path string, statusCode int, duration time.Duration) {
	statusStr := strconv.Itoa(statusCode)

	SantricityAPIRequestsLatencies.WithLabelValues(method, path, statusStr).Observe(duration.Seconds())
	SantricityAPIRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
}
