package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/electric-power/backend-service/internal/grpcclient"
	"github.com/electric-power/backend-service/internal/models"
	"github.com/electric-power/backend-service/internal/services"
	"github.com/electric-power/backend-service/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	jobs  *services.JobService
	algo  *grpcclient.AlgoClient
	store *storage.MySQLStore
	cache *storage.RedisCache
}

// SubmitJobRequest represents the request body for job submission
// @Description Job submission request payload
type SubmitJobRequest struct {
	Scheme string         `json:"scheme" binding:"required" example:"KBM-WF01"`
	DataID string         `json:"data_id" binding:"required" example:"sample_001"`
	Params map[string]any `json:"params" example:"{\"threshold\": 0.9}"`
	UserID string         `json:"user_id" example:"user_001"`
}

// JobResponse represents the response for job queries
// @Description Job information response
type JobResponse struct {
	JobID      string `json:"job_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	SchemeCode string `json:"scheme_code" example:"KBM-WF01"`
	UserID     string `json:"user_id" example:"user_001"`
	Status     string `json:"status" example:"SUCCESS"`
	Progress   int    `json:"progress" example:"100"`
	CreatedAt  string `json:"created_at" example:"2026-02-04T10:00:00Z"`
}

// ErrorResponse represents an error response
// @Description Error response payload
type ErrorResponse struct {
	Error   string `json:"error" example:"Invalid request"`
	Message string `json:"message,omitempty" example:"Detailed error message"`
	Code    int    `json:"code,omitempty" example:"400"`
}

// SuccessResponse represents a generic success response
// @Description Generic success response
type SuccessResponse struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message,omitempty" example:"Operation completed"`
}

// NewHandler creates a new HTTP handler
func NewHandler(jobs *services.JobService, algo *grpcclient.AlgoClient, store *storage.MySQLStore, cache *storage.RedisCache) *Handler {
	return &Handler{jobs: jobs, algo: algo, store: store, cache: cache}
}

// GetSchemes godoc
// @Summary      Get available algorithm schemes
// @Description  Returns a list of all registered algorithm schemes from the algorithm service
// @Tags         algorithms
// @Accept       json
// @Produce      json
// @Success      200  {array}   models.Scheme
// @Failure      500  {object}  ErrorResponse
// @Router       /api/v1/algorithms/schemes [get]
func (h *Handler) GetSchemes(c *gin.Context) {
	schemes, err := h.jobs.GetCachedSchemes(c.Request.Context())
	if err != nil {
		// Try to fetch from algo service directly
		schemes, err = h.algo.GetSchemes(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get schemes", Message: err.Error()})
			return
		}
		// Cache for next time
		_ = h.jobs.CacheSchemes(c.Request.Context(), schemes)
	}
	c.JSON(http.StatusOK, schemes)
}

// SubmitJob godoc
// @Summary      Submit a new algorithm job
// @Description  Creates a new job and dispatches it to the algorithm service for processing
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        X-Request-ID  header    string          false  "Idempotency key for duplicate prevention"
// @Param        request       body      SubmitJobRequest  true   "Job submission request"
// @Success      200  {object}  map[string]string  "Returns job_id"
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/v1/jobs [post]
func (h *Handler) SubmitJob(c *gin.Context) {
	var req SubmitJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request", Message: err.Error(), Code: 400})
		return
	}

	jobID := uuid.NewString()
	paramsJSON, _ := json.Marshal(req.Params)

	if err := h.jobs.CreateJob(c.Request.Context(), jobID, req.Scheme, req.UserID, req.DataID, string(paramsJSON)); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create job", Message: err.Error()})
		return
	}

	if err := h.algo.SubmitJob(c.Request.Context(), req.Scheme, req.DataID, req.Params, jobID); err != nil {
		// Mark job as failed since submission failed
		_ = h.jobs.FailJob(c.Request.Context(), jobID, "Failed to submit to algorithm service: "+err.Error())
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to submit job", Message: err.Error()})
		return
	}

	go h.watchProgress(jobID)
	c.JSON(http.StatusOK, gin.H{"job_id": jobID, "status": "PENDING"})
}

// GetJob godoc
// @Summary      Get job by ID
// @Description  Returns detailed information about a specific job
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Job ID"
// @Success      200  {object}  map[string]any
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/v1/jobs/{id} [get]
func (h *Handler) GetJob(c *gin.Context) {
	jobID := c.Param("id")
	job, err := h.jobs.GetJob(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Job not found", Message: err.Error(), Code: 404})
		return
	}
	c.JSON(http.StatusOK, gin.H{"job": job})
}

// ListJobs godoc
// @Summary      List jobs with pagination
// @Description  Returns a paginated list of jobs with optional filters
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        page      query     int     false  "Page number"  default(1)
// @Param        page_size query     int     false  "Items per page"  default(20)
// @Param        user_id   query     string  false  "Filter by user ID"
// @Param        status    query     string  false  "Filter by status (PENDING, RUNNING, SUCCESS, FAILED)"
// @Success      200  {object}  map[string]any  "Returns jobs array, total count, and pagination info"
// @Failure      500  {object}  ErrorResponse
// @Router       /api/v1/jobs [get]
func (h *Handler) ListJobs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	userID := c.Query("user_id")
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	jobs, total, err := h.store.ListJobsWithPagination(c.Request.Context(), userID, status, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to list jobs", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"jobs":      jobs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"pages":     (total + pageSize - 1) / pageSize,
	})
}

// GetJobResult godoc
// @Summary      Get job result
// @Description  Returns the result data for a completed job
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Job ID"
// @Success      200  {object}  map[string]any
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/v1/jobs/{id}/result [get]
func (h *Handler) GetJobResult(c *gin.Context) {
	jobID := c.Param("id")
	job, err := h.store.GetJobTyped(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Job not found", Message: err.Error(), Code: 404})
		return
	}

	if job.Status != "SUCCESS" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Job not completed", Message: "Job status is " + job.Status, Code: 400})
		return
	}

	var result any
	if job.ResultJSON != "" {
		_ = json.Unmarshal([]byte(job.ResultJSON), &result)
	}

	c.JSON(http.StatusOK, gin.H{
		"job_id": jobID,
		"status": job.Status,
		"result": result,
	})
}

// CancelJob godoc
// @Summary      Cancel a running job
// @Description  Attempts to cancel a pending or running job. Use force=true for immediate termination.
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        id     path      string  true   "Job ID"
// @Param        force  query     bool    false  "Force kill (default: false)"
// @Success      200  {object}  SuccessResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Router       /api/v1/jobs/{id}/cancel [post]
func (h *Handler) CancelJob(c *gin.Context) {
	jobID := c.Param("id")
	forceStr := c.DefaultQuery("force", "false")
	force := forceStr == "true" || forceStr == "1"

	job, err := h.store.GetJobTyped(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Job not found", Message: err.Error(), Code: 404})
		return
	}

	if job.Status == "SUCCESS" || job.Status == "FAILED" || job.Status == "CANCELLED" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Cannot cancel completed job", Code: 400})
		return
	}

	// Request algorithm service to cancel
	resp, err := h.algo.CancelTask(c.Request.Context(), jobID, force)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to cancel job", Message: err.Error()})
		return
	}

	// If cancel accepted and already cancelled, mark in DB
	if resp.GetStatus() == "CANCELLED" || resp.GetStatus() == "KILLED" {
		_ = h.jobs.CancelJob(c.Request.Context(), jobID, "Cancelled by user")
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  resp.GetAccepted(),
		"message":  resp.GetMessage(),
		"status":   resp.GetStatus(),
		"job_id":   jobID,
		"force":    force,
	})
}

// HealthCheck godoc
// @Summary      Health check
// @Description  Returns the health status of the backend service and its dependencies
// @Tags         system
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /api/v1/health [get]
func (h *Handler) HealthCheck(c *gin.Context) {
	ctx := c.Request.Context()

	health := gin.H{
		"status": "healthy",
		"checks": gin.H{},
	}

	// Check MySQL
	if err := h.store.Ping(ctx); err != nil {
		health["checks"].(gin.H)["mysql"] = gin.H{"status": "unhealthy", "error": err.Error()}
		health["status"] = "degraded"
	} else {
		health["checks"].(gin.H)["mysql"] = gin.H{"status": "healthy"}
	}

	// Check Redis
	if err := h.cache.Ping(ctx); err != nil {
		health["checks"].(gin.H)["redis"] = gin.H{"status": "unhealthy", "error": err.Error()}
		health["status"] = "degraded"
	} else {
		health["checks"].(gin.H)["redis"] = gin.H{"status": "healthy"}
	}

	// Check Algorithm Service
	if h.algo.IsHealthy() {
		health["checks"].(gin.H)["algorithm_service"] = gin.H{"status": "healthy"}
	} else {
		health["checks"].(gin.H)["algorithm_service"] = gin.H{"status": "unhealthy"}
		health["status"] = "degraded"
	}

	c.JSON(http.StatusOK, health)
}

// GetStats godoc
// @Summary      Get system statistics
// @Description  Returns aggregate statistics about jobs and system usage
// @Tags         system
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]any
// @Failure      500  {object}  ErrorResponse
// @Router       /api/v1/stats [get]
func (h *Handler) GetStats(c *gin.Context) {
	stats, err := h.store.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get stats", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *Handler) watchProgress(jobID string) {
	ctx := context.Background()

	// Retry connection with backoff
	for retries := 0; retries < 3; retries++ {
		stream, err := h.algo.WatchProgress(ctx, jobID)
		if err != nil {
			continue
		}

		for {
			msg, err := stream.Recv()
			if err != nil {
				break
			}
			_ = h.jobs.UpdateProgress(ctx, models.ProgressMsg{
				TaskID:     msg.TaskId,
				Percentage: msg.Percentage,
				Message:    msg.Message,
				Timestamp:  msg.Timestamp,
			})

			// Check if job is finished
			if msg.Percentage >= 100 {
				return
			}
		}
	}
}
