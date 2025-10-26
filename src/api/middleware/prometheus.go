package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status", "service"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "service"},
	)

	dialogMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dialog_messages_total",
			Help: "Total number of dialog messages processed",
		},
		[]string{"operation", "status", "service"},
	)

	dialogMessageDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dialog_message_duration_seconds",
			Help:    "Duration of dialog message operations in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"operation", "service"},
	)

	dialogErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dialog_errors_total",
			Help: "Total number of dialog operation errors",
		},
		[]string{"operation", "error_type", "service"},
	)
)

func PrometheusMiddleware(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		httpRequestsTotal.WithLabelValues(
			c.Request.Method,
			path,
			status,
			serviceName,
		).Inc()

		httpRequestDuration.WithLabelValues(
			c.Request.Method,
			path,
			serviceName,
		).Observe(duration)
	}
}

func RecordDialogOperation(operation, status, serviceName string, duration time.Duration, err error) {
	dialogMessagesTotal.WithLabelValues(operation, status, serviceName).Inc()
	dialogMessageDuration.WithLabelValues(operation, serviceName).Observe(duration.Seconds())

	if err != nil {
		errorType := "unknown"
		if err.Error() != "" {
			errorType = err.Error()
		}
		dialogErrors.WithLabelValues(operation, errorType, serviceName).Inc()
	}
}
