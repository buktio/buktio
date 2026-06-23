package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "buktio_http_requests_total",
		Help: "Total buktio API HTTP requests by method and status.",
	}, []string{"method", "status"})

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "buktio_http_request_duration_seconds",
		Help:    "buktio API HTTP request duration by method.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method"})
)

// RecordHTTP records one HTTP request's metrics (called from the request logger
// middleware, which already wraps the response writer).
func RecordHTTP(method string, status int, dur time.Duration) {
	httpRequests.WithLabelValues(method, strconv.Itoa(status)).Inc()
	httpDuration.WithLabelValues(method).Observe(dur.Seconds())
}

// MetricsHandler exposes the Prometheus registry (Go runtime + process + buktio
// HTTP metrics).
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
