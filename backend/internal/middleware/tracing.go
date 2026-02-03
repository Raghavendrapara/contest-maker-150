package middleware

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware creates a middleware that enables distributed tracing
func TracingMiddleware(tracer trace.Tracer) gin.HandlerFunc {
	propagator := otel.GetTextMapPropagator()

	return func(c *gin.Context) {
		// Extract trace context from incoming request
		ctx := propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		// Start a new span
		spanName := c.Request.Method + " " + c.FullPath()
		if c.FullPath() == "" {
			spanName = c.Request.Method + " " + c.Request.URL.Path
		}

		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPMethod(c.Request.Method),
				semconv.HTTPURL(c.Request.URL.String()),
				semconv.HTTPScheme(c.Request.URL.Scheme),
				semconv.NetHostName(c.Request.Host),
				semconv.UserAgentOriginal(c.Request.UserAgent()),
				attribute.String("http.client_ip", c.ClientIP()),
			),
		)
		defer span.End()

		// Add request ID to span
		if requestID := GetRequestID(c); requestID != "" {
			span.SetAttributes(attribute.String("request.id", requestID))
		}

		// Store the new context in the request
		c.Request = c.Request.WithContext(ctx)

		// Process request
		c.Next()

		// Record response attributes
		status := c.Writer.Status()
		span.SetAttributes(semconv.HTTPStatusCode(status))

		// Mark span as error if status >= 400
		if status >= 400 {
			span.SetAttributes(attribute.Bool("error", true))
		}

		// Record any errors
		if len(c.Errors) > 0 {
			span.SetAttributes(attribute.String("error.message", c.Errors.String()))
		}

		// Add user ID if authenticated
		if userID, ok := GetUserID(c); ok {
			span.SetAttributes(attribute.String("user.id", userID.String()))
		}
	}
}
