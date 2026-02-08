package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/electric-power/backend-service/internal/models"
)

// MockStore implements a mock for MySQLStore
type MockStore struct {
	mock.Mock
}

func (m *MockStore) InsertJob(ctx context.Context, jobID, schemeCode, userID, dataRef, params string) error {
	args := m.Called(ctx, jobID, schemeCode, userID, dataRef, params)
	return args.Error(0)
}

func (m *MockStore) UpdateProgress(ctx context.Context, jobID string, progress int, message string) error {
	args := m.Called(ctx, jobID, progress, message)
	return args.Error(0)
}

func (m *MockStore) FinishJob(ctx context.Context, jobID, resultJSON string) error {
	args := m.Called(ctx, jobID, resultJSON)
	return args.Error(0)
}

func (m *MockStore) FailJob(ctx context.Context, jobID, errorLog string) error {
	args := m.Called(ctx, jobID, errorLog)
	return args.Error(0)
}

func (m *MockStore) CancelJob(ctx context.Context, jobID, message string) error {
	args := m.Called(ctx, jobID, message)
	return args.Error(0)
}

func (m *MockStore) GetJob(ctx context.Context, jobID string) (map[string]any, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]any), args.Error(1)
}

// MockCache implements a mock for RedisCache
type MockCache struct {
	mock.Mock
	data map[string]interface{}
}

func NewMockCache() *MockCache {
	return &MockCache{data: make(map[string]interface{})}
}

func (m *MockCache) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *MockCache) GetJSON(ctx context.Context, key string, dest interface{}) error {
	if v, ok := m.data[key]; ok {
		// Simple type assertion for schemes
		if schemes, ok := v.([]models.Scheme); ok {
			if destSchemes, ok := dest.(*[]models.Scheme); ok {
				*destSchemes = schemes
				return nil
			}
		}
	}
	return assert.AnError
}

// MockHub implements a mock for WebSocket Hub
type MockHub struct {
	mock.Mock
	broadcasts []struct {
		Topic   string
		Payload []byte
	}
}

func (m *MockHub) Broadcast(topic string, payload []byte) {
	m.broadcasts = append(m.broadcasts, struct {
		Topic   string
		Payload []byte
	}{topic, payload})
}

// TestJobService tests
func TestJobServiceCacheSchemes(t *testing.T) {
	cache := NewMockCache()

	schemes := []models.Scheme{
		{Code: "KBM-WF01", Name: "Test 1"},
		{Code: "KBM-WF02", Name: "Test 2"},
	}

	// Store schemes
	err := cache.SetJSON(context.Background(), "algo:schemes", schemes, 5*time.Minute)
	assert.NoError(t, err)

	// Retrieve schemes
	var retrieved []models.Scheme
	err = cache.GetJSON(context.Background(), "algo:schemes", &retrieved)
	assert.NoError(t, err)
	assert.Len(t, retrieved, 2)
}

func TestJobServiceCacheMiss(t *testing.T) {
	cache := NewMockCache()

	var schemes []models.Scheme
	err := cache.GetJSON(context.Background(), "nonexistent", &schemes)
	assert.Error(t, err)
}

// TestSchemeModel tests
func TestSchemeModel(t *testing.T) {
	scheme := models.Scheme{
		Model:        "KBM",
		Code:         "KBM-WF01",
		Name:         "Knowledge Base - Workflow 1",
		ClassName:    "KBMWF01",
		ResourceType: "CPU",
		Description:  "Sample workflow",
	}

	assert.Equal(t, "KBM", scheme.Model)
	assert.Equal(t, "KBM-WF01", scheme.Code)
	assert.Equal(t, "CPU", scheme.ResourceType)
}

// TestJobModel tests
func TestJobModel(t *testing.T) {
	job := models.Job{
		JobID:      "test-job-123",
		SchemeCode: "SCM-WF01",
		UserID:     "user-001",
		Status:     "PENDING",
		Progress:   0,
		DataRef:    "data/test.csv",
		Params:     `{"threshold": 0.9}`,
	}

	assert.Equal(t, "test-job-123", job.JobID)
	assert.Equal(t, "SCM-WF01", job.SchemeCode)
	assert.Equal(t, "PENDING", job.Status)
}

// TestProgressMsg tests
func TestProgressMsg(t *testing.T) {
	msg := models.ProgressMsg{
		TaskID:     "task-001",
		Percentage: 50,
		Message:    "Processing...",
		Timestamp:  time.Now().Unix(),
		Stage:      "inference",
	}

	assert.Equal(t, "task-001", msg.TaskID)
	assert.Equal(t, int32(50), msg.Percentage)
	assert.Equal(t, "inference", msg.Stage)
}
