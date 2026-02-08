package http

import (
	"net/http"
	"time"

	"github.com/electric-power/backend-service/internal/middleware"
	"github.com/electric-power/backend-service/internal/storage"
	"github.com/electric-power/backend-service/internal/ws"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

// RouterConfig holds configuration for the router
type RouterConfig struct {
	EnableSwagger bool
	RateLimitRPS  int
	RequestTimeout time.Duration
}

// DefaultRouterConfig returns default router configuration
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		EnableSwagger:  true,
		RateLimitRPS:   100,
		RequestTimeout: 30 * time.Second,
	}
}

// NewRouter creates a new Gin router with all routes configured
func NewRouter(handler *Handler, hub *ws.Hub) *gin.Engine {
	return NewRouterWithConfig(handler, hub, handler.cache, nil, DefaultRouterConfig())
}

// NewRouterWithConfig creates a router with custom configuration
func NewRouterWithConfig(handler *Handler, hub *ws.Hub, cache *storage.RedisCache, logger *zap.Logger, cfg RouterConfig) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Recovery middleware
	r.Use(gin.Recovery())

	// Custom middleware
	r.Use(middleware.CORS())
	r.Use(middleware.RequestID())

	if logger != nil {
		r.Use(middleware.StructuredLogger(logger))
	} else {
		r.Use(gin.Logger())
	}

	// Rate limiting for all API routes
	if cache != nil && cfg.RateLimitRPS > 0 {
		r.Use(middleware.RateLimiter(cache, cfg.RateLimitRPS, time.Minute))
	}

	// Swagger documentation
	if cfg.EnableSwagger {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// Health check endpoint (no auth required)
	r.GET("/health", handler.HealthCheck)

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Apply request timeout
		if cfg.RequestTimeout > 0 {
			v1.Use(middleware.Timeout(cfg.RequestTimeout))
		}

		// Algorithm schemes
		algorithms := v1.Group("/algorithms")
		{
			algorithms.GET("/schemes", handler.GetSchemes)
		}

		// Job management
		jobs := v1.Group("/jobs")
		{
			// Idempotency for job creation
			if cache != nil {
				jobs.POST("", middleware.Idempotency(cache), handler.SubmitJob)
			} else {
				jobs.POST("", handler.SubmitJob)
			}
			jobs.GET("", handler.ListJobs)
			jobs.GET("/:id", handler.GetJob)
			jobs.GET("/:id/result", handler.GetJobResult)
			jobs.POST("/:id/cancel", handler.CancelJob)
		}

		// System endpoints
		system := v1.Group("/system")
		{
			system.GET("/health", handler.HealthCheck)
			system.GET("/stats", handler.GetStats)
		}

		// ============================================================
		// Module-specific routes: KBM, SCM, STM
		// Each module has: schemes, workflows, and dynamic workflow job endpoints
		// Workflows are dynamically discovered from algorithm-service
		// ============================================================

		// KBM (Knowledge Base Management) Module
		kbm := v1.Group("/kbm")
		{
			kbm.GET("/schemes", handler.GetSchemesForModule("KBM"))
			kbm.GET("/workflows", handler.GetModuleWorkflows("KBM"))
			kbm.GET("/jobs", handler.ListModuleJobs("KBM"))
			kbm.GET("/jobs/:id", handler.GetJob)
			kbm.GET("/jobs/:id/result", handler.GetJobResult)
			kbm.POST("/jobs/:id/cancel", handler.CancelJob)

			// Dynamic workflow job submission: /api/v1/kbm/:workflow/jobs
			// Supports any workflow discovered from algorithm-service (WF01, WF02, WF03, etc.)
			if cache != nil {
				kbm.POST("/:workflow/jobs", middleware.Idempotency(cache), handler.SubmitDynamicWorkflowJob("KBM"))
			} else {
				kbm.POST("/:workflow/jobs", handler.SubmitDynamicWorkflowJob("KBM"))
			}
		}

		// SCM (Safety Check Module) Module
		scm := v1.Group("/scm")
		{
			scm.GET("/schemes", handler.GetSchemesForModule("SCM"))
			scm.GET("/workflows", handler.GetModuleWorkflows("SCM"))
			scm.GET("/jobs", handler.ListModuleJobs("SCM"))
			scm.GET("/jobs/:id", handler.GetJob)
			scm.GET("/jobs/:id/result", handler.GetJobResult)
			scm.POST("/jobs/:id/cancel", handler.CancelJob)

			if cache != nil {
				scm.POST("/:workflow/jobs", middleware.Idempotency(cache), handler.SubmitDynamicWorkflowJob("SCM"))
			} else {
				scm.POST("/:workflow/jobs", handler.SubmitDynamicWorkflowJob("SCM"))
			}
		}

		// STM (Simulation Twin Module) Module
		stm := v1.Group("/stm")
		{
			stm.GET("/schemes", handler.GetSchemesForModule("STM"))
			stm.GET("/workflows", handler.GetModuleWorkflows("STM"))
			stm.GET("/jobs", handler.ListModuleJobs("STM"))
			stm.GET("/jobs/:id", handler.GetJob)
			stm.GET("/jobs/:id/result", handler.GetJobResult)
			stm.POST("/jobs/:id/cancel", handler.CancelJob)

			if cache != nil {
				stm.POST("/:workflow/jobs", middleware.Idempotency(cache), handler.SubmitDynamicWorkflowJob("STM"))
			} else {
				stm.POST("/:workflow/jobs", handler.SubmitDynamicWorkflowJob("STM"))
			}
		}
	}

	// WebSocket endpoint for real-time progress updates
	r.GET("/ws", func(c *gin.Context) {
		jobID := c.Query("job_id")
		if jobID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "job_id query parameter is required"})
			return
		}

		upgrader := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		userID := c.Query("user_id")
		hub.SubscribeWithUser(jobID, userID, conn)
	})

	// WebSocket health endpoint
	r.GET("/ws/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "clients": hub.GetTotalClients()})
	})

	return r
}
