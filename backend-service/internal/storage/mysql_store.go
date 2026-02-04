package storage

import (
	"context"
	"database/sql"
	"time"

	"github.com/electric-power/backend-service/internal/models"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type MySQLStore struct {
	db *sqlx.DB
}

func NewMySQLStore(dsn string) (*MySQLStore, error) {
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, err
	}
	// Connection pool settings for high concurrency
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(20)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(3 * time.Minute)

	return &MySQLStore{db: db}, nil
}

func (s *MySQLStore) Close() error {
	return s.db.Close()
}

func (s *MySQLStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *MySQLStore) InitSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS t_algo_jobs (
  job_id CHAR(36) PRIMARY KEY,
  scheme_code VARCHAR(50) NOT NULL,
  user_id VARCHAR(50),
  status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
  progress INT DEFAULT 0,
  data_ref VARCHAR(255),
  params JSON,
  result_summary LONGTEXT,
  error_log TEXT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME,
  finished_at DATETIME,
  INDEX idx_user_status (user_id, status),
  INDEX idx_status_created (status, created_at),
  INDEX idx_scheme (scheme_code)
);
`)
	return err
}

func (s *MySQLStore) InsertJob(ctx context.Context, jobID, schemeCode, userID, dataRef, params string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO t_algo_jobs (job_id, scheme_code, user_id, status, progress, data_ref, params, created_at, updated_at)
VALUES (?, ?, ?, 'PENDING', 0, ?, ?, ?, ?)
`, jobID, schemeCode, userID, dataRef, params, now, now)
	return err
}

func (s *MySQLStore) UpdateProgress(ctx context.Context, jobID string, progress int, message string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE t_algo_jobs SET progress = ?, status = 'RUNNING', updated_at = ? WHERE job_id = ?
`, progress, time.Now(), jobID)
	return err
}

func (s *MySQLStore) FinishJob(ctx context.Context, jobID, resultJSON string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
UPDATE t_algo_jobs SET status = 'SUCCESS', result_summary = ?, finished_at = ?, updated_at = ? WHERE job_id = ?
`, resultJSON, now, now, jobID)
	return err
}

func (s *MySQLStore) FailJob(ctx context.Context, jobID, errorLog string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
UPDATE t_algo_jobs SET status = 'FAILED', error_log = ?, finished_at = ?, updated_at = ? WHERE job_id = ?
`, errorLog, now, now, jobID)
	return err
}

func (s *MySQLStore) CancelJob(ctx context.Context, jobID, message string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
UPDATE t_algo_jobs SET status = 'CANCELLED', error_log = ?, finished_at = ?, updated_at = ? WHERE job_id = ?
`, message, now, now, jobID)
	return err
}

func (s *MySQLStore) GetJob(ctx context.Context, jobID string) (map[string]any, error) {
	row := s.db.QueryRowxContext(ctx, `
SELECT job_id, scheme_code, user_id, status, progress, data_ref, params, result_summary, error_log, created_at, updated_at, finished_at 
FROM t_algo_jobs WHERE job_id = ?`, jobID)
	result := map[string]any{}
	if err := row.MapScan(result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetJobTyped returns a strongly typed Job struct
func (s *MySQLStore) GetJobTyped(ctx context.Context, jobID string) (*models.Job, error) {
	var job models.Job
	err := s.db.GetContext(ctx, &job, `
SELECT job_id, scheme_code, user_id, status, progress, data_ref, params, 
       COALESCE(result_summary, '') as result_summary, 
       COALESCE(error_log, '') as error_log, 
       created_at, 
       COALESCE(finished_at, created_at) as finished_at
FROM t_algo_jobs WHERE job_id = ?`, jobID)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// ListJobsWithPagination returns paginated jobs with filters
func (s *MySQLStore) ListJobsWithPagination(ctx context.Context, userID, status string, page, pageSize int) ([]models.Job, int, error) {
	offset := (page - 1) * pageSize
	args := []any{}
	where := "WHERE 1=1"

	if userID != "" {
		where += " AND user_id = ?"
		args = append(args, userID)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}

	// Count total
	var total int
	countSQL := "SELECT COUNT(*) FROM t_algo_jobs " + where
	if err := s.db.GetContext(ctx, &total, countSQL, args...); err != nil {
		return nil, 0, err
	}

	// Fetch page
	querySQL := `
SELECT job_id, scheme_code, user_id, status, progress, data_ref, params, 
       COALESCE(result_summary, '') as result_summary, 
       COALESCE(error_log, '') as error_log, 
       created_at, 
       COALESCE(finished_at, created_at) as finished_at
FROM t_algo_jobs ` + where + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`

	queryArgs := append(args, pageSize, offset)
	var jobs []models.Job
	if err := s.db.SelectContext(ctx, &jobs, querySQL, queryArgs...); err != nil {
		return nil, 0, err
	}

	return jobs, total, nil
}

// FindZombieTasks finds tasks stuck in RUNNING state for longer than timeout
func (s *MySQLStore) FindZombieTasks(ctx context.Context, timeout time.Duration) ([]string, error) {
	cutoff := time.Now().Add(-timeout)
	var jobIDs []string
	err := s.db.SelectContext(ctx, &jobIDs, `
SELECT job_id FROM t_algo_jobs WHERE status = 'RUNNING' AND updated_at < ?`, cutoff)
	return jobIDs, err
}

// MarkZombieAsFailed marks zombie tasks as failed
func (s *MySQLStore) MarkZombieAsFailed(ctx context.Context, jobIDs []string) error {
	if len(jobIDs) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`
UPDATE t_algo_jobs SET status = 'FAILED', error_log = 'Task timeout - marked as zombie', finished_at = ?, updated_at = ? 
WHERE job_id IN (?)`, time.Now(), time.Now(), jobIDs)
	if err != nil {
		return err
	}
	query = s.db.Rebind(query)
	_, err = s.db.ExecContext(ctx, query, args...)
	return err
}

// GetStats returns aggregate statistics
func (s *MySQLStore) GetStats(ctx context.Context) (map[string]any, error) {
	stats := make(map[string]any)

	// Count by status
	rows, err := s.db.QueryxContext(ctx, `
SELECT status, COUNT(*) as count FROM t_algo_jobs GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statusCounts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		statusCounts[status] = count
	}
	stats["status_counts"] = statusCounts

	// Average duration for completed jobs
	var avgDuration sql.NullFloat64
	_ = s.db.GetContext(ctx, &avgDuration, `
SELECT AVG(TIMESTAMPDIFF(SECOND, created_at, finished_at)) FROM t_algo_jobs WHERE status = 'SUCCESS' AND finished_at IS NOT NULL`)
	if avgDuration.Valid {
		stats["avg_duration_seconds"] = avgDuration.Float64
	}

	return stats, nil
}
