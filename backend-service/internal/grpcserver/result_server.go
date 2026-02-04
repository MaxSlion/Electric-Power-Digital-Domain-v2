package grpcserver

import (
	"context"

	"github.com/electric-power/backend-service/internal/services"
	pb "github.com/electric-power/backend-service/proto"
)

type ResultServer struct {
	pb.UnimplementedResultReceiverServiceServer
	jobs *services.JobService
}

func NewResultServer(jobs *services.JobService) *ResultServer {
	return &ResultServer{jobs: jobs}
}

func (s *ResultServer) ReportResult(ctx context.Context, req *pb.TaskResult) (*pb.Ack, error) {
	if s.jobs.IsFinished(ctx, req.TaskId) {
		return &pb.Ack{Success: true}, nil
	}

	if req.Status == pb.TaskResult_SUCCESS {
		_ = s.jobs.FinishJob(ctx, req.TaskId, req.ResultJson)
		go s.jobs.OnJobSuccess(req.TaskId)
	} else {
		_ = s.jobs.FailJob(ctx, req.TaskId, req.ErrorMessage)
	}

	return &pb.Ack{Success: true}, nil
}
