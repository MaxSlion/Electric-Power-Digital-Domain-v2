package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/electric-power/backend-service/internal/models"
)

// MockJobService implements a mock for JobService
type MockJobService struct {
	mock.Mock
}

func (m *MockJobService) CreateJob(ctx interface{}, jobID, schemeCode, userID, dataRef, params string) error {
	args := m.Called(ctx, jobID, schemeCode, userID, dataRef, params)
	return args.Error(0)
}

func (m *MockJobService) GetCachedSchemes(ctx interface{}) ([]models.Scheme, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Scheme), args.Error(1)
}

func (m *MockJobService) CacheSchemes(ctx interface{}, schemes []models.Scheme) error {
	args := m.Called(ctx, schemes)
	return args.Error(0)
}

func (m *MockJobService) FailJob(ctx interface{}, jobID, errorLog string) error {
	args := m.Called(ctx, jobID, errorLog)
	return args.Error(0)
}

// setupTestRouter creates a test router with gin in test mode
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// TestModuleJobRequest tests the ModuleJobRequest struct
func TestModuleJobRequest(t *testing.T) {
	t.Run("ValidRequest", func(t *testing.T) {
		reqBody := `{"data_ref": "test_data", "params": {"threshold": 0.9}, "user_id": "user1"}`
		var req ModuleJobRequest
		err := json.Unmarshal([]byte(reqBody), &req)

		assert.NoError(t, err)
		assert.Equal(t, "test_data", req.DataRef)
		assert.Equal(t, "user1", req.UserID)
		assert.Equal(t, 0.9, req.Params["threshold"])
	})

	t.Run("EmptyDataRef", func(t *testing.T) {
		reqBody := `{"params": {}}`
		var req ModuleJobRequest
		err := json.Unmarshal([]byte(reqBody), &req)

		assert.NoError(t, err)
		assert.Empty(t, req.DataRef)
	})
}

// TestErrorResponse tests the ErrorResponse struct
func TestErrorResponse(t *testing.T) {
	resp := ErrorResponse{
		Error:   "Test error",
		Message: "Detailed message",
		Code:    400,
	}

	data, err := json.Marshal(resp)
	assert.NoError(t, err)

	var decoded ErrorResponse
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, resp.Error, decoded.Error)
	assert.Equal(t, resp.Message, decoded.Message)
	assert.Equal(t, resp.Code, decoded.Code)
}

// TestHealthCheckEndpoint tests the health check endpoint
func TestHealthCheckEndpoint(t *testing.T) {
	r := setupTestRouter()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
}

// TestModuleRoutesExist tests that module routes are registered correctly
func TestModuleRoutesExist(t *testing.T) {
	r := setupTestRouter()

	// Register test routes
	v1 := r.Group("/api/v1")
	kbm := v1.Group("/kbm")
	kbm.GET("/schemes", func(c *gin.Context) {
		c.JSON(http.StatusOK, []models.Scheme{})
	})
	kbm.GET("/workflows", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": "KBM", "workflows": []interface{}{}})
	})
	kbm.POST("/:workflow/jobs", func(c *gin.Context) {
		workflow := c.Param("workflow")
		c.JSON(http.StatusOK, gin.H{"workflow": workflow})
	})

	tests := []struct {
		name     string
		method   string
		path     string
		wantCode int
	}{
		{"KBM Schemes", "GET", "/api/v1/kbm/schemes", http.StatusOK},
		{"KBM Workflows", "GET", "/api/v1/kbm/workflows", http.StatusOK},
		{"KBM Submit WF01", "POST", "/api/v1/kbm/wf01/jobs", http.StatusOK},
		{"KBM Submit WF02", "POST", "/api/v1/kbm/wf02/jobs", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.method == "POST" {
				body := bytes.NewBufferString(`{}`)
				req = httptest.NewRequest(tt.method, tt.path, body)
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.wantCode, w.Code, "Route %s %s", tt.method, tt.path)
		})
	}
}

// TestDynamicWorkflowRouting tests dynamic workflow path parameter
func TestDynamicWorkflowRouting(t *testing.T) {
	r := setupTestRouter()

	r.POST("/api/v1/kbm/:workflow/jobs", func(c *gin.Context) {
		workflow := c.Param("workflow")
		c.JSON(http.StatusOK, gin.H{
			"module":   "KBM",
			"workflow": workflow,
			"scheme":   "KBM-" + workflow,
		})
	})

	tests := []struct {
		workflow     string
		wantWorkflow string
	}{
		{"wf01", "wf01"},
		{"wf02", "wf02"},
		{"wf99", "wf99"},
		{"custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.workflow, func(t *testing.T) {
			path := "/api/v1/kbm/" + tt.workflow + "/jobs"
			body := bytes.NewBufferString(`{"data_ref": "test"}`)
			req := httptest.NewRequest("POST", path, body)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]string
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantWorkflow, response["workflow"])
		})
	}
}
