package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/electric-power/backend-service/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ModuleJobRequest represents a job submission for a specific module/workflow
// @Description Module-specific job submission request
type ModuleJobRequest struct {
	DataRef string         `json:"data_ref" binding:"required" example:"sample_001"`
	Params  map[string]any `json:"params" example:"{\"threshold\": 0.9}"`
	UserID  string         `json:"user_id" example:"user_001"`
}

// SubmitModuleJob returns a handler that binds module and workflow to job submission
// @Summary      Submit a job for a specific module workflow
// @Description  Creates a new job for the specified module (KBM/SCM/STM) and workflow (WF01/WF02/WF03)
// @Tags         modules
// @Accept       json
// @Produce      json
// @Param        request  body      ModuleJobRequest  true  "Job submission request"
// @Success      200      {object}  map[string]string "Returns job_id and status"
// @Failure      400      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
func (h *Handler) SubmitModuleJob(module, workflow string) gin.HandlerFunc {
	return func(c *gin.Context) {
		h.submitModuleJobInternal(c, module, workflow)
	}
}

// SubmitDynamicWorkflowJob returns a handler that accepts workflow from path parameter
// This enables dynamic workflow discovery - any workflow registered in algorithm-service can be invoked
// @Summary      Submit a job for a dynamically discovered workflow
// @Description  Creates a new job for the specified module with workflow ID from path parameter
// @Tags         modules
// @Accept       json
// @Produce      json
// @Param        workflow path      string            true  "Workflow ID (e.g., WF01, WF02)"
// @Param        request  body      ModuleJobRequest  true  "Job submission request"
// @Success      200      {object}  map[string]string "Returns job_id and status"
// @Failure      400      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
func (h *Handler) SubmitDynamicWorkflowJob(module string) gin.HandlerFunc {
	return func(c *gin.Context) {
		workflow := strings.ToUpper(c.Param("workflow"))
		if workflow == "" {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Missing workflow",
				Message: "workflow path parameter is required",
				Code:    400,
			})
			return
		}
		h.submitModuleJobInternal(c, module, workflow)
	}
}

// submitModuleJobInternal contains the shared logic for job submission
func (h *Handler) submitModuleJobInternal(c *gin.Context, module, workflow string) {
	var req ModuleJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
			Code:    400,
		})
		return
	}

	// Construct scheme code from module and workflow
	schemeCode := fmt.Sprintf("%s-%s", strings.ToUpper(module), strings.ToUpper(workflow))

	jobID := uuid.NewString()
	paramsJSON, _ := json.Marshal(req.Params)

	if err := h.jobs.CreateJob(c.Request.Context(), jobID, schemeCode, req.UserID, req.DataRef, string(paramsJSON)); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to create job",
			Message: err.Error(),
		})
		return
	}

	if err := h.algo.SubmitJob(c.Request.Context(), schemeCode, req.DataRef, req.Params, jobID); err != nil {
		_ = h.jobs.FailJob(c.Request.Context(), jobID, "Failed to submit to algorithm service: "+err.Error())
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to submit job",
			Message: err.Error(),
		})
		return
	}

	go h.watchProgress(jobID)
	c.JSON(http.StatusOK, gin.H{
		"job_id":   jobID,
		"status":   "PENDING",
		"scheme":   schemeCode,
		"module":   module,
		"workflow": workflow,
	})
}

// GetSchemesForModule returns a handler that filters schemes by module prefix
// @Summary      Get available schemes for a module
// @Description  Returns algorithm schemes filtered by module (KBM/SCM/STM)
// @Tags         modules
// @Accept       json
// @Produce      json
// @Success      200  {array}   models.Scheme
// @Failure      500  {object}  ErrorResponse
func (h *Handler) GetSchemesForModule(module string) gin.HandlerFunc {
	return func(c *gin.Context) {
		allSchemes, err := h.jobs.GetCachedSchemes(c.Request.Context())
		if err != nil {
			allSchemes, err = h.algo.GetSchemes(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "Failed to get schemes",
					Message: err.Error(),
				})
				return
			}
			_ = h.jobs.CacheSchemes(c.Request.Context(), allSchemes)
		}

		prefix := strings.ToUpper(module) + "-"
		filtered := make([]models.Scheme, 0)
		for _, s := range allSchemes {
			if strings.HasPrefix(strings.ToUpper(s.Code), prefix) {
				filtered = append(filtered, s)
			}
		}

		c.JSON(http.StatusOK, filtered)
	}
}

// ListModuleJobs returns a handler that lists jobs filtered by module
// @Summary      List jobs for a module
// @Description  Returns paginated jobs filtered by module (scheme code prefix)
// @Tags         modules
// @Accept       json
// @Produce      json
// @Param        page      query  int     false  "Page number"     default(1)
// @Param        page_size query  int     false  "Items per page"  default(20)
// @Param        user_id   query  string  false  "Filter by user"
// @Param        status    query  string  false  "Filter by status"
// @Success      200  {object}  map[string]any
// @Failure      500  {object}  ErrorResponse
func (h *Handler) ListModuleJobs(module string) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		// Get all jobs and filter by module prefix
		// Note: For production, add module filtering to the SQL query
		jobs, _, err := h.store.ListJobsWithPagination(c.Request.Context(), userID, status, page, pageSize)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Failed to list jobs",
				Message: err.Error(),
			})
			return
		}

		// Filter by module prefix
		prefix := strings.ToUpper(module) + "-"
		filtered := make([]models.Job, 0)
		for _, job := range jobs {
			if strings.HasPrefix(strings.ToUpper(job.SchemeCode), prefix) {
				filtered = append(filtered, job)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"jobs":      filtered,
			"total":     len(filtered),
			"page":      page,
			"page_size": pageSize,
			"module":    module,
		})
	}
}

// GetModuleWorkflows returns a handler that lists available workflows for a module
// @Summary      Get available workflows for a module
// @Description  Returns list of workflow IDs (WF01, WF02, WF03) available for the module
// @Tags         modules
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]any
func (h *Handler) GetModuleWorkflows(module string) gin.HandlerFunc {
	return func(c *gin.Context) {
		allSchemes, err := h.jobs.GetCachedSchemes(c.Request.Context())
		if err != nil {
			allSchemes, err = h.algo.GetSchemes(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "Failed to get schemes",
					Message: err.Error(),
				})
				return
			}
		}

		prefix := strings.ToUpper(module) + "-"
		workflows := make([]gin.H, 0)
		seen := make(map[string]bool)

		for _, s := range allSchemes {
			if strings.HasPrefix(strings.ToUpper(s.Code), prefix) {
				// Extract workflow from code (e.g., "KBM-WF01" -> "WF01")
				parts := strings.Split(s.Code, "-")
				if len(parts) >= 2 {
					wf := parts[1]
					if !seen[wf] {
						seen[wf] = true
						workflows = append(workflows, gin.H{
							"workflow_id": wf,
							"code":        s.Code,
							"name":        s.Name,
							"description": s.Description,
						})
					}
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"module":    module,
			"workflows": workflows,
		})
	}
}
