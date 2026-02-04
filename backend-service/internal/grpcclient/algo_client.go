package grpcclient

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/electric-power/backend-service/internal/models"
	pb "github.com/electric-power/backend-service/proto"

	"github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// AlgoClientConfig holds configuration for the algorithm gRPC client
type AlgoClientConfig struct {
	Address            string
	MaxRetries         int
	InitialBackoff     time.Duration
	MaxBackoff         time.Duration
	DialTimeout        time.Duration
	RequestTimeout     time.Duration
	KeepAliveInterval  time.Duration
	KeepAliveTimeout   time.Duration
	MaxConcurrentCalls int
}

// DefaultAlgoClientConfig returns sensible defaults for high-concurrency scenarios
func DefaultAlgoClientConfig(addr string) AlgoClientConfig {
	return AlgoClientConfig{
		Address:            addr,
		MaxRetries:         3,
		InitialBackoff:     100 * time.Millisecond,
		MaxBackoff:         5 * time.Second,
		DialTimeout:        10 * time.Second,
		RequestTimeout:     30 * time.Second,
		KeepAliveInterval:  10 * time.Second,
		KeepAliveTimeout:   3 * time.Second,
		MaxConcurrentCalls: 100,
	}
}

// AlgoClient wraps the gRPC connection to the algorithm service with resilience patterns
type AlgoClient struct {
	conn    *grpc.ClientConn
	client  pb.AlgoControlServiceClient
	config  AlgoClientConfig
	logger  *zap.Logger
	mu      sync.RWMutex
	sem     chan struct{} // Semaphore for concurrency control
	healthy bool
}

// NewAlgoClient creates a new resilient gRPC client
func NewAlgoClient(addr string) (*AlgoClient, error) {
	return NewAlgoClientWithConfig(DefaultAlgoClientConfig(addr), nil)
}

// NewAlgoClientWithConfig creates a client with custom configuration
func NewAlgoClientWithConfig(cfg AlgoClientConfig, logger *zap.Logger) (*AlgoClient, error) {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                cfg.KeepAliveInterval,
			Timeout:             cfg.KeepAliveTimeout,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(100*1024*1024), // 100MB for large data
			grpc.MaxCallSendMsgSize(100*1024*1024),
		),
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, cfg.Address, opts...)
	if err != nil {
		return nil, err
	}

	ac := &AlgoClient{
		conn:    conn,
		client:  pb.NewAlgoControlServiceClient(conn),
		config:  cfg,
		logger:  logger,
		sem:     make(chan struct{}, cfg.MaxConcurrentCalls),
		healthy: true,
	}

	// Start connection state watcher
	go ac.watchConnectionState()

	return ac, nil
}

func (c *AlgoClient) watchConnectionState() {
	for {
		state := c.conn.GetState()
		c.mu.Lock()
		c.healthy = (state == connectivity.Ready || state == connectivity.Idle)
		c.mu.Unlock()

		if !c.conn.WaitForStateChange(context.Background(), state) {
			return
		}
	}
}

// IsHealthy returns true if the connection is in a healthy state
func (c *AlgoClient) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.healthy
}

// Close closes the gRPC connection
func (c *AlgoClient) Close() error {
	return c.conn.Close()
}

// acquireSemaphore blocks until a slot is available for concurrent calls
func (c *AlgoClient) acquireSemaphore(ctx context.Context) error {
	select {
	case c.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *AlgoClient) releaseSemaphore() {
	<-c.sem
}

// retry executes the given operation with exponential backoff
func (c *AlgoClient) retry(ctx context.Context, op func() error) error {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = c.config.InitialBackoff
	b.MaxInterval = c.config.MaxBackoff
	b.MaxElapsedTime = c.config.RequestTimeout

	return backoff.Retry(func() error {
		err := op()
		if err != nil {
			c.logger.Warn("gRPC call failed, retrying", zap.Error(err))
		}
		return err
	}, backoff.WithContext(backoff.WithMaxRetries(b, uint64(c.config.MaxRetries)), ctx))
}

// GetSchemes retrieves available algorithm schemes with retry
func (c *AlgoClient) GetSchemes(ctx context.Context) ([]models.Scheme, error) {
	if err := c.acquireSemaphore(ctx); err != nil {
		return nil, err
	}
	defer c.releaseSemaphore()

	var schemes []models.Scheme
	err := c.retry(ctx, func() error {
		ctx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()

		resp, err := c.client.GetAvailableSchemes(ctx, &pb.Empty{})
		if err != nil {
			return err
		}
		schemes = make([]models.Scheme, 0, len(resp.Schemes))
		for _, s := range resp.Schemes {
			schemes = append(schemes, models.Scheme{
				Model:        s.Model,
				Code:         s.Code,
				Name:         s.Name,
				ClassName:    s.ClassName,
				ResourceType: s.ResourceType,
			})
		}
		return nil
	})
	return schemes, err
}

// SubmitJob submits a job with retry logic
func (c *AlgoClient) SubmitJob(ctx context.Context, schemeCode, dataRef string, params map[string]any, taskID string) error {
	if err := c.acquireSemaphore(ctx); err != nil {
		return err
	}
	defer c.releaseSemaphore()

	payload, _ := json.Marshal(params)
	return c.retry(ctx, func() error {
		ctx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()

		_, err := c.client.SubmitTask(ctx, &pb.TaskRequest{
			TaskId:     taskID,
			SchemeCode: schemeCode,
			DataRef:    dataRef,
			ParamsJson: string(payload),
		})
		return err
	})
}

// WatchProgress streams progress updates for a task
// Note: Streaming calls are not retried automatically, caller should handle reconnection
func (c *AlgoClient) WatchProgress(ctx context.Context, taskID string) (pb.AlgoControlService_WatchTaskProgressClient, error) {
	return c.client.WatchTaskProgress(ctx, &pb.TaskIdentity{TaskId: taskID})
}

// Health performs a health check with timeout
func (c *AlgoClient) Health(ctx context.Context) (*pb.HealthStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.client.CheckHealth(ctx, &pb.Empty{})
}

// ListTasks retrieves all tasks
func (c *AlgoClient) ListTasks(ctx context.Context) (*pb.TaskList, error) {
	if err := c.acquireSemaphore(ctx); err != nil {
		return nil, err
	}
	defer c.releaseSemaphore()

	var result *pb.TaskList
	err := c.retry(ctx, func() error {
		ctx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()

		var err error
		result, err = c.client.ListTasks(ctx, &pb.Empty{})
		return err
	})
	return result, err
}

// GetTaskStatus retrieves status for a specific task
func (c *AlgoClient) GetTaskStatus(ctx context.Context, taskID string) (*pb.TaskStatus, error) {
	if err := c.acquireSemaphore(ctx); err != nil {
		return nil, err
	}
	defer c.releaseSemaphore()

	var result *pb.TaskStatus
	err := c.retry(ctx, func() error {
		ctx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()

		var err error
		result, err = c.client.GetTaskStatus(ctx, &pb.TaskIdentity{TaskId: taskID})
		return err
	})
	return result, err
}

// CancelTask requests cancellation of a task
// If force is true, the algorithm service will immediately kill the process
func (c *AlgoClient) CancelTask(ctx context.Context, taskID string, force bool) (*pb.CancelResponse, error) {
	if err := c.acquireSemaphore(ctx); err != nil {
		return nil, err
	}
	defer c.releaseSemaphore()

	var result *pb.CancelResponse
	err := c.retry(ctx, func() error {
		ctx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()

		var err error
		result, err = c.client.CancelTask(ctx, &pb.CancelRequest{TaskId: taskID, Force: force})
		return err
	})
	return result, err
}
