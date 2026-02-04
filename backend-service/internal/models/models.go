package models

import (
	"database/sql"
	"time"
)

// Scheme represents an algorithm scheme definition
type Scheme struct {
	Model          string   `json:"model" db:"model"`
	Code           string   `json:"code" db:"code"`
	Name           string   `json:"name" db:"name"`
	ClassName      string   `json:"class_name" db:"class_name"`
	ResourceType   string   `json:"resource_type" db:"resource_type"`
	Description    string   `json:"description,omitempty" db:"description"`
	RequiredParams []string `json:"required_params,omitempty" db:"-"`
}

// Job represents an algorithm job record
type Job struct {
	JobID      string       `db:"job_id" json:"job_id"`
	SchemeCode string       `db:"scheme_code" json:"scheme_code"`
	UserID     string       `db:"user_id" json:"user_id"`
	Status     string       `db:"status" json:"status"`
	Progress   int          `db:"progress" json:"progress"`
	DataRef    string       `db:"data_ref" json:"data_ref"`
	Params     string       `db:"params" json:"params"`
	ResultJSON string       `db:"result_summary" json:"result_summary"`
	ErrorLog   string       `db:"error_log" json:"error_log,omitempty"`
	CreatedAt  time.Time    `db:"created_at" json:"created_at"`
	UpdatedAt  sql.NullTime `db:"updated_at" json:"updated_at,omitempty"`
	FinishedAt sql.NullTime `db:"finished_at" json:"finished_at,omitempty"`
}

// ProgressMsg represents a progress update message
type ProgressMsg struct {
	TaskID     string            `json:"task_id"`
	Percentage int32             `json:"percentage"`
	Message    string            `json:"message"`
	Timestamp  int64             `json:"timestamp"`
	Stage      string            `json:"stage,omitempty"`
	Metrics    map[string]string `json:"metrics,omitempty"`
}

// JobSubmitRequest represents a job submission request
type JobSubmitRequest struct {
	SchemeCode     string         `json:"scheme" binding:"required"`
	DataRef        string         `json:"data_id" binding:"required"`
	Params         map[string]any `json:"params"`
	UserID         string         `json:"user_id"`
	Priority       int            `json:"priority,omitempty"`
	TimeoutSeconds int            `json:"timeout_seconds,omitempty"`
	CallbackURL    string         `json:"callback_url,omitempty"`
}

// JobResponse represents a job query response
type JobResponse struct {
	JobID        string         `json:"job_id"`
	SchemeCode   string         `json:"scheme_code"`
	UserID       string         `json:"user_id,omitempty"`
	Status       string         `json:"status"`
	Progress     int            `json:"progress"`
	DataRef      string         `json:"data_ref,omitempty"`
	Params       map[string]any `json:"params,omitempty"`
	Result       any            `json:"result,omitempty"`
	ErrorMessage string         `json:"error_message,omitempty"`
	CreatedAt    string         `json:"created_at"`
	FinishedAt   string         `json:"finished_at,omitempty"`
	DurationMs   int64          `json:"duration_ms,omitempty"`
}

// PaginatedResult represents a paginated query result
type PaginatedResult[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Pages    int `json:"pages"`
}

// HealthCheck represents the health status of a service component
type HealthCheck struct {
	Status  string            `json:"status"` // healthy, degraded, unhealthy
	Checks  map[string]string `json:"checks,omitempty"`
	Message string            `json:"message,omitempty"`
}

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type      string `json:"type"` // progress, result, error, ping, pong
	TaskID    string `json:"task_id,omitempty"`
	Payload   any    `json:"payload,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// DataUploadMeta contains metadata for uploaded data files
type DataUploadMeta struct {
	DataRef     string    `json:"data_ref"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	Checksum    string    `json:"checksum"`
	UploadedAt  time.Time `json:"uploaded_at"`
	UploadedBy  string    `json:"uploaded_by,omitempty"`
}
