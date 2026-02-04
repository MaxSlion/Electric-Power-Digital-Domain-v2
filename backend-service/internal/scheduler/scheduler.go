package scheduler

import (
	"context"
	"time"

	"github.com/electric-power/backend-service/internal/grpcclient"
	"github.com/electric-power/backend-service/internal/storage"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// Scheduler manages background jobs for the backend service
type Scheduler struct {
	cron   *cron.Cron
	store  *storage.MySQLStore
	cache  *storage.RedisCache
	algo   *grpcclient.AlgoClient
	logger *zap.Logger
}

// NewScheduler creates a new scheduler instance
func NewScheduler(store *storage.MySQLStore, cache *storage.RedisCache, algo *grpcclient.AlgoClient, logger *zap.Logger) *Scheduler {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}
	return &Scheduler{
		cron:   cron.New(cron.WithSeconds()),
		store:  store,
		cache:  cache,
		algo:   algo,
		logger: logger,
	}
}

// Start begins the scheduled jobs
func (s *Scheduler) Start() {
	// Zombie task cleanup every 5 minutes
	_, _ = s.cron.AddFunc("0 */5 * * * *", s.cleanupZombieTasks)

	// Algorithm service health check every 30 seconds
	_, _ = s.cron.AddFunc("*/30 * * * * *", s.checkAlgoHealth)

	// Cache refresh every minute
	_, _ = s.cron.AddFunc("0 * * * * *", s.refreshSchemeCache)

	s.cron.Start()
	s.logger.Info("Scheduler started")
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() context.Context {
	return s.cron.Stop()
}

// cleanupZombieTasks marks stuck tasks as failed
func (s *Scheduler) cleanupZombieTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Tasks running for more than 30 minutes are considered zombies
	zombies, err := s.store.FindZombieTasks(ctx, 30*time.Minute)
	if err != nil {
		s.logger.Error("Failed to find zombie tasks", zap.Error(err))
		return
	}

	if len(zombies) == 0 {
		return
	}

	s.logger.Warn("Found zombie tasks", zap.Int("count", len(zombies)), zap.Strings("job_ids", zombies))

	if err := s.store.MarkZombieAsFailed(ctx, zombies); err != nil {
		s.logger.Error("Failed to mark zombies as failed", zap.Error(err))
		return
	}

	s.logger.Info("Cleaned up zombie tasks", zap.Int("count", len(zombies)))
}

// checkAlgoHealth verifies the algorithm service is responsive
func (s *Scheduler) checkAlgoHealth() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := s.algo.Health(ctx)
	if err != nil {
		s.logger.Warn("Algorithm service health check failed", zap.Error(err))
		_ = s.cache.SetJSON(ctx, "sys:algo:health", map[string]any{
			"status":  "DOWN",
			"checked": time.Now().Unix(),
			"error":   err.Error(),
		}, 1*time.Minute)
		return
	}

	_ = s.cache.SetJSON(ctx, "sys:algo:health", map[string]any{
		"status":  status.Status.String(),
		"checked": time.Now().Unix(),
		"metrics": status.Metrics,
	}, 1*time.Minute)
}

// refreshSchemeCache refreshes the algorithm scheme cache
func (s *Scheduler) refreshSchemeCache() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	schemes, err := s.algo.GetSchemes(ctx)
	if err != nil {
		s.logger.Warn("Failed to refresh scheme cache", zap.Error(err))
		return
	}

	if err := s.cache.SetJSON(ctx, "sys:algo:schemes", schemes, 10*time.Minute); err != nil {
		s.logger.Warn("Failed to cache schemes", zap.Error(err))
	}
}
