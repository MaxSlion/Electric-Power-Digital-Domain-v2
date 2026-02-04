package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// StructuredLogger returns a middleware that logs HTTP requests using zap
func StructuredLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		fields := []zap.Field{
			zap.Int("status", status),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.Duration("latency", latency),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.String("request_id", c.GetHeader(IdempotencyHeader)),
		}

		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()))
		}

		if status >= 500 {
			logger.Error("Server error", fields...)
		} else if status >= 400 {
			logger.Warn("Client error", fields...)
		} else {
			logger.Info("Request", fields...)
		}
	}
}

// RequestID adds a unique request ID to each request if not provided
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(IdempotencyHeader)
		if requestID == "" {
			requestID = generateRequestID()
			c.Header(IdempotencyHeader, requestID)
		}
		c.Set("request_id", requestID)
		c.Next()
	}
}

func generateRequestID() string {
	// Simple timestamp-based ID, could use UUID in production
	return time.Now().Format("20060102150405.000000")
}
