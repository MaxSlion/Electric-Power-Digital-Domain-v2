package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/electric-power/backend-service/internal/storage"

	"github.com/gin-gonic/gin"
)

const (
	IdempotencyHeader = "X-Request-ID"
	IdempotencyTTL    = 10 * time.Minute
)

// Idempotency middleware ensures that duplicate requests with the same X-Request-ID
// are not processed multiple times. This is critical for safety-critical operations
// like applying decision plans (per design doc section 4.1).
func Idempotency(cache *storage.RedisCache) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(IdempotencyHeader)
		if requestID == "" {
			c.Next()
			return
		}

		ctx := c.Request.Context()
		key := "idempotency:" + requestID

		// Check if request was already processed
		var existing string
		err := cache.GetJSON(ctx, key, &existing)
		if err == nil && existing != "" {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error":      "Duplicate request",
				"request_id": requestID,
				"message":    "This request has already been processed",
			})
			return
		}

		// Mark request as processing
		_ = cache.SetJSON(ctx, key, "processing", IdempotencyTTL)

		c.Next()

		// Mark request as completed
		_ = cache.SetJSON(ctx, key, "completed", IdempotencyTTL)
	}
}

// RateLimiter implements a simple sliding window rate limiter using Redis.
// Limits requests per IP/user to prevent abuse.
func RateLimiter(cache *storage.RedisCache, maxRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientID := c.ClientIP()
		if userID := c.GetHeader("X-User-ID"); userID != "" {
			clientID = userID
		}

		key := "ratelimit:" + clientID
		ctx := c.Request.Context()

		var count int
		_ = cache.GetJSON(ctx, key, &count)
		if count >= maxRequests {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": int(window.Seconds()),
			})
			return
		}

		_ = cache.Incr(ctx, key, window)
		c.Next()
	}
}

// Timeout middleware applies request timeout to prevent long-running requests
func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		done := make(chan struct{})
		go func() {
			c.Next()
			close(done)
		}()

		select {
		case <-done:
			return
		case <-ctx.Done():
			c.AbortWithStatusJSON(http.StatusGatewayTimeout, gin.H{
				"error": "Request timeout",
			})
		}
	}
}
