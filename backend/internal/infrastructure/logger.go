package infrastructure

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a new structured logger using zap
func NewLogger(environment string) (*zap.Logger, error) {
	var config zap.Config

	if environment == "production" {
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// Common settings
	config.EncoderConfig.MessageKey = "message"
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.StacktraceKey = "stacktrace"

	logger, err := config.Build(
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return nil, err
	}

	return logger, nil
}

// LoggerWithContext returns a logger with additional context fields
func LoggerWithContext(logger *zap.Logger, fields ...zap.Field) *zap.Logger {
	return logger.With(fields...)
}

// RequestLogger creates a logger for HTTP request logging
func RequestLogger(logger *zap.Logger, requestID, method, path string) *zap.Logger {
	return logger.With(
		zap.String("request_id", requestID),
		zap.String("method", method),
		zap.String("path", path),
	)
}

// LogHTTPRequest logs an HTTP request with timing information
func LogHTTPRequest(logger *zap.Logger, method, path string, status int, duration time.Duration, clientIP string) {
	logger.Info("HTTP request",
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status", status),
		zap.Duration("duration", duration),
		zap.String("client_ip", clientIP),
	)
}

// LogDatabaseQuery logs a database query with timing
func LogDatabaseQuery(logger *zap.Logger, query string, duration time.Duration, rows int64) {
	logger.Debug("Database query",
		zap.String("query", truncateString(query, 200)),
		zap.Duration("duration", duration),
		zap.Int64("rows", rows),
	)
}

// LogError logs an error with stack trace
func LogError(logger *zap.Logger, message string, err error, fields ...zap.Field) {
	allFields := append(fields, zap.Error(err))
	logger.Error(message, allFields...)
}

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// SyncLogger flushes any buffered log entries
func SyncLogger(logger *zap.Logger) {
	if err := logger.Sync(); err != nil {
		// Ignore sync errors for stdout/stderr
		if _, ok := err.(*os.PathError); !ok {
			logger.Error("Failed to sync logger", zap.Error(err))
		}
	}
}
