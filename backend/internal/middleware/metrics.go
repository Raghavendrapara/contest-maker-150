package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/contest-maker-150/backend/internal/infrastructure"
)

// MetricsMiddleware creates a middleware that records HTTP metrics
func MetricsMiddleware(metrics *infrastructure.TelemetryMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		status := c.Writer.Status()
		method := c.Request.Method
		path := c.FullPath() // Use route pattern, not actual path

		// If no route matched, use a generic path
		if path == "" {
			path = "unknown"
		}

		attrs := []attribute.KeyValue{
			attribute.String("http.method", method),
			attribute.String("http.route", path),
			attribute.Int("http.status_code", status),
		}

		// Record request duration
		metrics.HTTPRequestDuration.Record(c.Request.Context(), duration,
			metric.WithAttributes(attrs...),
		)

		// Record request count
		metrics.HTTPRequestCount.Add(c.Request.Context(), 1,
			metric.WithAttributes(attrs...),
		)
	}
}
