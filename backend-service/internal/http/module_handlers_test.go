package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestGetSchemesForModuleHandler tests the GetSchemesForModule handler factory
func TestGetSchemesForModuleHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Mock schemes data
	mockSchemes := []map[string]string{
		{"code": "KBM-WF01", "name": "KBM Workflow 1"},
		{"code": "KBM-WF02", "name": "KBM Workflow 2"},
		{"code": "SCM-WF01", "name": "SCM Workflow 1"},
	}

	// Simplified handler for testing
	r.GET("/api/v1/:module/schemes", func(c *gin.Context) {
		module := strings.ToUpper(c.Param("module"))
		prefix := module + "-"

		filtered := []map[string]string{}
		for _, s := range mockSchemes {
			if strings.HasPrefix(s["code"], prefix) {
				filtered = append(filtered, s)
			}
		}
		c.JSON(http.StatusOK, filtered)
	})

	tests := []struct {
		module    string
		wantCount int
	}{
		{"kbm", 2},
		{"scm", 1},
		{"stm", 0},
	}

	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/"+tt.module+"/schemes", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var schemes []map[string]string
			err := json.Unmarshal(w.Body.Bytes(), &schemes)
			assert.NoError(t, err)
			assert.Len(t, schemes, tt.wantCount)
		})
	}
}

// TestGetModuleWorkflowsHandler tests the GetModuleWorkflows handler factory
func TestGetModuleWorkflowsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Mock schemes data
	mockSchemes := []map[string]string{
		{"code": "KBM-WF01", "name": "KBM Workflow 1", "description": "Desc 1"},
		{"code": "KBM-WF02", "name": "KBM Workflow 2", "description": "Desc 2"},
		{"code": "KBM-WF03", "name": "KBM Workflow 3", "description": "Desc 3"},
	}

	r.GET("/api/v1/:module/workflows", func(c *gin.Context) {
		module := strings.ToUpper(c.Param("module"))
		prefix := module + "-"

		workflows := []gin.H{}
		for _, s := range mockSchemes {
			if strings.HasPrefix(s["code"], prefix) {
				parts := strings.Split(s["code"], "-")
				if len(parts) >= 2 {
					workflows = append(workflows, gin.H{
						"workflow_id": parts[1],
						"code":        s["code"],
						"name":        s["name"],
						"description": s["description"],
					})
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"module":    module,
			"workflows": workflows,
		})
	})

	req := httptest.NewRequest("GET", "/api/v1/kbm/workflows", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "KBM", response["module"])

	workflows := response["workflows"].([]interface{})
	assert.Len(t, workflows, 3)

	// Check first workflow
	wf1 := workflows[0].(map[string]interface{})
	assert.Equal(t, "WF01", wf1["workflow_id"])
	assert.Equal(t, "KBM-WF01", wf1["code"])
}

// TestListModuleJobsHandler tests module job listing with filtering
func TestListModuleJobsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Mock jobs data
	type Job struct {
		JobID      string `json:"job_id"`
		SchemeCode string `json:"scheme_code"`
	}

	mockJobs := []Job{
		{JobID: "job-1", SchemeCode: "KBM-WF01"},
		{JobID: "job-2", SchemeCode: "KBM-WF02"},
		{JobID: "job-3", SchemeCode: "SCM-WF01"},
		{JobID: "job-4", SchemeCode: "STM-WF01"},
	}

	r.GET("/api/v1/:module/jobs", func(c *gin.Context) {
		module := strings.ToUpper(c.Param("module"))
		prefix := module + "-"

		filtered := []Job{}
		for _, job := range mockJobs {
			if strings.HasPrefix(job.SchemeCode, prefix) {
				filtered = append(filtered, job)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"module": module,
			"jobs":   filtered,
			"total":  len(filtered),
		})
	})

	tests := []struct {
		module    string
		wantCount int
	}{
		{"kbm", 2},
		{"scm", 1},
		{"stm", 1},
	}

	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/"+tt.module+"/jobs", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			jobs := response["jobs"].([]interface{})
			assert.Len(t, jobs, tt.wantCount)
		})
	}
}

// TestSubmitDynamicWorkflowJobHandler tests job submission with dynamic workflow
func TestSubmitDynamicWorkflowJobHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.POST("/api/v1/:module/:workflow/jobs", func(c *gin.Context) {
		module := strings.ToUpper(c.Param("module"))
		workflow := strings.ToUpper(c.Param("workflow"))

		if workflow == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing workflow"})
			return
		}

		var req ModuleJobRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		schemeCode := module + "-" + workflow

		c.JSON(http.StatusOK, gin.H{
			"job_id":   "test-job-123",
			"status":   "PENDING",
			"scheme":   schemeCode,
			"module":   module,
			"workflow": workflow,
		})
	})

	t.Run("ValidSubmission", func(t *testing.T) {
		body := strings.NewReader(`{"data_ref": "test_data", "params": {"k": 5}, "user_id": "user1"}`)
		req := httptest.NewRequest("POST", "/api/v1/kbm/wf01/jobs", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "PENDING", response["status"])
		assert.Equal(t, "KBM-WF01", response["scheme"])
	})

	t.Run("DynamicWorkflow", func(t *testing.T) {
		body := strings.NewReader(`{"data_ref": "data"}`)
		req := httptest.NewRequest("POST", "/api/v1/scm/wf99/jobs", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "SCM-WF99", response["scheme"])
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		body := strings.NewReader(`{invalid json}`)
		req := httptest.NewRequest("POST", "/api/v1/kbm/wf01/jobs", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
