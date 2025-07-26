package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chronixpkgs_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chronixpkgs_http_request_duration_seconds",
			Help:    "HTTP request latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Event processing metrics
	EventsProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chronixpkgs_events_processed_total",
			Help: "Total number of events processed",
		},
		[]string{"type", "repository", "status"},
	)

	EventProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chronixpkgs_event_processing_duration_seconds",
			Help:    "Event processing duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"type"},
	)

	// Storage metrics
	StorageOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chronixpkgs_storage_operations_total",
			Help: "Total number of storage operations",
		},
		[]string{"operation", "status"},
	)

	StorageOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chronixpkgs_storage_operation_duration_seconds",
			Help:    "Storage operation duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	// GitHub API metrics
	GitHubAPICallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chronixpkgs_github_api_calls_total",
			Help: "Total number of GitHub API calls",
		},
		[]string{"endpoint", "status"},
	)

	GitHubRateLimitRemaining = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chronixpkgs_github_rate_limit_remaining",
			Help: "Remaining GitHub API rate limit",
		},
	)

	// Rate limiter metrics
	RateLimitRejectionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chronixpkgs_rate_limit_rejections_total",
			Help: "Total number of rate limit rejections",
		},
		[]string{"ip"},
	)

	// Database metrics
	DatabaseConnectionsOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chronixpkgs_database_connections_open",
			Help: "Number of open database connections",
		},
	)

	DatabaseConnectionsInUse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chronixpkgs_database_connections_in_use",
			Help: "Number of database connections in use",
		},
	)

	// Partition metrics
	PartitionsTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chronixpkgs_partitions_total",
			Help: "Total number of database partitions",
		},
	)

	EventsStoredTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "chronixpkgs_events_stored_total",
			Help: "Total number of events stored",
		},
	)
)

// RecordHTTPRequest records HTTP request metrics
func RecordHTTPRequest(method, path string, status int, duration float64) {
	HTTPRequestsTotal.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
	HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
}

// RecordEventProcessed records event processing metrics
func RecordEventProcessed(eventType, repository string, success bool, duration float64) {
	status := "success"
	if !success {
		status = "failure"
	}
	EventsProcessedTotal.WithLabelValues(eventType, repository, status).Inc()
	EventProcessingDuration.WithLabelValues(eventType).Observe(duration)
}

// RecordStorageOperation records storage operation metrics
func RecordStorageOperation(operation string, success bool, duration float64) {
	status := "success"
	if !success {
		status = "failure"
	}
	StorageOperationsTotal.WithLabelValues(operation, status).Inc()
	StorageOperationDuration.WithLabelValues(operation).Observe(duration)
}

// RecordGitHubAPICall records GitHub API call metrics
func RecordGitHubAPICall(endpoint string, statusCode int) {
	status := "success"
	if statusCode >= 400 {
		status = "error"
	}
	GitHubAPICallsTotal.WithLabelValues(endpoint, status).Inc()
}
