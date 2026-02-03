package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// RequestIDKey is the context key for the request ID
	RequestIDKey = "requestID"
)

// LoggingMiddleware creates a logging middleware that logs all requests
func LoggingMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Generate request ID
		requestID := uuid.New().String()
		c.Set(RequestIDKey, requestID)
		c.Header("X-Request-ID", requestID)

		// Create request-scoped logger
		reqLogger := logger.With(
			zap.String("request_id", requestID),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("client_ip", c.ClientIP()),
		)

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Log based on status code
		status := c.Writer.Status()
		logFields := []zap.Field{
			zap.Int("status", status),
			zap.Duration("duration", duration),
			zap.Int("response_size", c.Writer.Size()),
		}

		// Add query params if present
		if c.Request.URL.RawQuery != "" {
			logFields = append(logFields, zap.String("query", c.Request.URL.RawQuery))
		}

		// Add user ID if authenticated
		if userID, exists := c.Get(UserIDKey); exists {
			logFields = append(logFields, zap.String("user_id", userID.(uuid.UUID).String()))
		}

		// Add error if present
		if len(c.Errors) > 0 {
			logFields = append(logFields, zap.Strings("errors", c.Errors.Errors()))
		}

		switch {
		case status >= 500:
			reqLogger.Error("Server error", logFields...)
		case status >= 400:
			reqLogger.Warn("Client error", logFields...)
		default:
			reqLogger.Info("Request completed", logFields...)
		}
	}
}

// GetRequestID extracts the request ID from the gin context
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get(RequestIDKey); exists {
		return requestID.(string)
	}
	return ""
}

// RecoveryMiddleware creates a recovery middleware that recovers from panics
func RecoveryMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				requestID := GetRequestID(c)

				logger.Error("Panic recovered",
					zap.String("request_id", requestID),
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
					zap.Any("error", err),
					zap.Stack("stack"),
				)

				c.JSON(500, gin.H{
					"error":      "Internal server error",
					"request_id": requestID,
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
