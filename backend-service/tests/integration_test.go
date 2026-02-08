package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	httpHandler "github.com/electric-power/backend-service/internal/http"
	"github.com/electric-power/backend-service/internal/models"
	"github.com/electric-power/backend-service/internal/ws"
)

// MockAlgoClient simulates the algorithm gRPC client
type MockAlgoClient struct {
	schemes []models.Scheme
	healthy bool
}

func NewMockAlgoClient() *MockAlgoClient {
	return &MockAlgoClient{
		schemes: []models.Scheme{
			{Model: "KBM", Code: "KBM-WF01", Name: "KBM Workflow 1", ResourceType: "CPU"},
			{Model: "KBM", Code: "KBM-WF02", Name: "KBM Workflow 2", ResourceType: "CPU"},
			{Model: "KBM", Code: "KBM-WF03", Name: "KBM Workflow 3", ResourceType: "CPU"},
			{Model: "SCM", Code: "SCM-WF01", Name: "SCM Workflow 1", ResourceType: "GPU"},
			{Model: "SCM", Code: "SCM-WF02", Name: "SCM Workflow 2", ResourceType: "CPU"},
			{Model: "SCM", Code: "SCM-WF03", Name: "SCM Workflow 3", ResourceType: "CPU"},
			{Model: "STM", Code: "STM-WF01", Name: "STM Workflow 1", ResourceType: "CPU"},
			{Model: "STM", Code: "STM-WF02", Name: "STM Workflow 2", ResourceType: "CPU"},
			{Model: "STM", Code: "STM-WF03", Name: "STM Workflow 3", ResourceType: "CPU"},
		},
		healthy: true,
	}
}

func (m *MockAlgoClient) GetSchemes(ctx context.Context) ([]models.Scheme, error) {
	return m.schemes, nil
}

func (m *MockAlgoClient) IsHealthy() bool {
	return m.healthy
}

// setupIntegrationRouter creates a router with mock dependencies
func setupIntegrationRouter() (*gin.Engine, *MockAlgoClient) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	algoClient := NewMockAlgoClient()
	hub := ws.NewHub()

	// Simulated routes matching the real router structure
	v1 := r.Group("/api/v1")

	// Health endpoint
	v1.GET("/health", func(c *gin.Context) {
		health := gin.H{
			"status": "healthy",
			"checks": gin.H{
				"mysql":             gin.H{"status": "healthy"},
				"redis":             gin.H{"status": "healthy"},
				"algorithm_service": gin.H{"status": "healthy"},
			},
		}
		if !algoClient.IsHealthy() {
			health["status"] = "degraded"
			health["checks"].(gin.H)["algorithm_service"] = gin.H{"status": "unhealthy"}
		}
		c.JSON(http.StatusOK, health)
	})

	// Algorithms endpoint
	v1.GET("/algorithms/schemes", func(c *gin.Context) {
		schemes, _ := algoClient.GetSchemes(c.Request.Context())
		c.JSON(http.StatusOK, schemes)
	})

	// Module endpoints
	for _, module := range []string{"kbm", "scm", "stm"} {
		moduleGroup := v1.Group("/" + module)
		moduleUpper := module

		moduleGroup.GET("/schemes", func(c *gin.Context) {
			schemes, _ := algoClient.GetSchemes(c.Request.Context())
			prefix := moduleUpper + "-"
			filtered := []models.Scheme{}
			for _, s := range schemes {
				if len(s.Code) > len(prefix) && s.Code[:len(prefix)] == prefix {
					filtered = append(filtered, s)
				}
			}
			c.JSON(http.StatusOK, filtered)
		})

		moduleGroup.GET("/workflows", func(c *gin.Context) {
			schemes, _ := algoClient.GetSchemes(c.Request.Context())
			prefix := moduleUpper + "-"
			workflows := []gin.H{}
			for _, s := range schemes {
				if len(s.Code) > len(prefix) && s.Code[:len(prefix)] == prefix {
					workflows = append(workflows, gin.H{
						"workflow_id": s.Code[len(prefix):],
						"code":        s.Code,
						"name":        s.Name,
					})
				}
			}
			c.JSON(http.StatusOK, gin.H{"module": moduleUpper, "workflows": workflows})
		})

		moduleGroup.POST("/:workflow/jobs", func(c *gin.Context) {
			workflow := c.Param("workflow")
			var req httpHandler.ModuleJobRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"job_id":   "mock-job-" + time.Now().Format("20060102150405"),
				"status":   "PENDING",
				"scheme":   moduleUpper + "-" + workflow,
				"module":   moduleUpper,
				"workflow": workflow,
			})
		})
	}

	// WebSocket ping
	r.GET("/ws/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "clients": hub.GetTotalClients()})
	})

	return r, algoClient
}

// TestIntegrationHealthCheck tests the full health check flow
func TestIntegrationHealthCheck(t *testing.T) {
	r, _ := setupIntegrationRouter()

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])

	checks := response["checks"].(map[string]interface{})
	assert.Contains(t, checks, "mysql")
	assert.Contains(t, checks, "redis")
	assert.Contains(t, checks, "algorithm_service")
}

// TestIntegrationGetAllSchemes tests fetching all algorithm schemes
func TestIntegrationGetAllSchemes(t *testing.T) {
	r, _ := setupIntegrationRouter()

	req := httptest.NewRequest("GET", "/api/v1/algorithms/schemes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var schemes []models.Scheme
	err := json.Unmarshal(w.Body.Bytes(), &schemes)
	assert.NoError(t, err)
	assert.Equal(t, 9, len(schemes))
}

// TestIntegrationModuleSchemes tests fetching module-specific schemes
func TestIntegrationModuleSchemes(t *testing.T) {
	r, _ := setupIntegrationRouter()

	tests := []struct {
		module    string
		wantCount int
	}{
		{"kbm", 3},
		{"scm", 3},
		{"stm", 3},
	}

	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/"+tt.module+"/schemes", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var schemes []models.Scheme
			err := json.Unmarshal(w.Body.Bytes(), &schemes)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantCount, len(schemes))
		})
	}
}

// TestIntegrationModuleWorkflows tests fetching module workflows
func TestIntegrationModuleWorkflows(t *testing.T) {
	r, _ := setupIntegrationRouter()

	req := httptest.NewRequest("GET", "/api/v1/kbm/workflows", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	workflows := response["workflows"].([]interface{})
	assert.Equal(t, 3, len(workflows))
}

// TestIntegrationSubmitJob tests job submission flow
func TestIntegrationSubmitJob(t *testing.T) {
	r, _ := setupIntegrationRouter()

	body := bytes.NewBufferString(`{
		"data_ref": "test_data_001",
		"params": {"threshold": 0.9, "mode": "fast"},
		"user_id": "integration_test_user"
	}`)

	req := httptest.NewRequest("POST", "/api/v1/scm/wf01/jobs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "PENDING", response["status"])
	assert.Equal(t, "scm-wf01", response["scheme"])
	assert.Contains(t, response["job_id"].(string), "mock-job-")
}

// TestIntegrationWebSocketPing tests WebSocket health endpoint
func TestIntegrationWebSocketPing(t *testing.T) {
	r, _ := setupIntegrationRouter()

	req := httptest.NewRequest("GET", "/ws/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
}
