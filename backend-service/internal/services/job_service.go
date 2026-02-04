package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/electric-power/backend-service/internal/models"
	"github.com/electric-power/backend-service/internal/storage"
	"github.com/electric-power/backend-service/internal/ws"
)

type JobService struct {
	store       *storage.MySQLStore
	cache       *storage.RedisCache
	hub         *ws.Hub
	schemeKey   string
	progressNS  string
}

func NewJobService(store *storage.MySQLStore, cache *storage.RedisCache, hub *ws.Hub, schemeKey, progressNS string) *JobService {
	return &JobService{store: store, cache: cache, hub: hub, schemeKey: schemeKey, progressNS: progressNS}
}

func (s *JobService) CacheSchemes(ctx context.Context, schemes []models.Scheme) error {
	return s.cache.SetJSON(ctx, s.schemeKey, schemes, 5*time.Minute)
}

func (s *JobService) GetCachedSchemes(ctx context.Context) ([]models.Scheme, error) {
	var schemes []models.Scheme
	err := s.cache.GetJSON(ctx, s.schemeKey, &schemes)
	return schemes, err
}

func (s *JobService) CreateJob(ctx context.Context, jobID, schemeCode, userID, dataRef, params string) error {
	return s.store.InsertJob(ctx, jobID, schemeCode, userID, dataRef, params)
}

func (s *JobService) UpdateProgress(ctx context.Context, msg models.ProgressMsg) error {
	_ = s.store.UpdateProgress(ctx, msg.TaskID, int(msg.Percentage), msg.Message)
	key := s.progressNS + msg.TaskID
	_ = s.cache.SetJSON(ctx, key, msg, 10*time.Minute)
	payload, _ := json.Marshal(msg)
	s.hub.Broadcast(msg.TaskID, payload)
	return nil
}

func (s *JobService) FinishJob(ctx context.Context, jobID, resultJSON string) error {
	return s.store.FinishJob(ctx, jobID, resultJSON)
}

func (s *JobService) FailJob(ctx context.Context, jobID, errorLog string) error {
	return s.store.FailJob(ctx, jobID, errorLog)
}

func (s *JobService) CancelJob(ctx context.Context, jobID, message string) error {
	return s.store.CancelJob(ctx, jobID, message)
}

func (s *JobService) GetJob(ctx context.Context, jobID string) (map[string]any, error) {
	return s.store.GetJob(ctx, jobID)
}

func (s *JobService) IsFinished(ctx context.Context, jobID string) bool {
	job, err := s.store.GetJob(ctx, jobID)
	if err != nil {
		return false
	}
	status, _ := job["status"].(string)
	return status == "SUCCESS" || status == "FAILED"
}

func (s *JobService) OnJobSuccess(jobID string) {
	// Hook for additional post-processing
}
