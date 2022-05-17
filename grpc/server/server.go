// Package server contains a gRPC based robot.Robot server implementation.
//
// It should be used by an rpc.Server.
package server

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.viam.com/utils"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/operation"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/robot/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot"
)

// Server implements the contract from robot.proto that ultimately satisfies
// a robot.Robot as a gRPC server.
type Server struct {
	pb.UnimplementedRobotServiceServer
	r                       robot.Robot
	activeBackgroundWorkers sync.WaitGroup
	cancelCtx               context.Context
	cancel                  func()
}

// New constructs a gRPC service server for a Robot.
// CR erodkin: kinda a bummer to add new args here. See if we can do without. if the r
// was a LocalRobot instead then we could use a LocalRobot accessor to get hold of the
// framesystem directly, and wouldn't need the subtype.Service?
func New(r robot.Robot) pb.RobotServiceServer {
	cancelCtx, cancel := context.WithCancel(context.Background())
	return &Server{
		r:         r,
		cancelCtx: cancelCtx,
		cancel:    cancel,
	}
}

// Close cleanly shuts down the server.
func (s *Server) Close() {
	s.cancel()
	s.activeBackgroundWorkers.Wait()
}

// GetOperations lists all running operations.
func (s *Server) GetOperations(ctx context.Context, req *pb.GetOperationsRequest) (*pb.GetOperationsResponse, error) {
	me := operation.Get(ctx)

	all := s.r.OperationManager().All()

	res := &pb.GetOperationsResponse{}
	for _, o := range all {
		if o == me {
			continue
		}

		s, err := convertInterfaceToStruct(o.Arguments)
		if err != nil {
			return nil, err
		}

		res.Operations = append(res.Operations, &pb.Operation{
			Id:        o.ID.String(),
			Method:    o.Method,
			Arguments: s,
			Started:   timestamppb.New(o.Started),
		})
	}

	return res, nil
}

func convertInterfaceToStruct(i interface{}) (*structpb.Struct, error) {
	if i == nil {
		return &structpb.Struct{}, nil // TODO(cheuk): should InterfaceToMap handle nil?
	}
	m, err := protoutils.InterfaceToMap(i)
	if err != nil {
		return nil, err
	}

	return structpb.NewStruct(m)
}

// CancelOperation kills an operations.
func (s *Server) CancelOperation(ctx context.Context, req *pb.CancelOperationRequest) (*pb.CancelOperationResponse, error) {
	op := s.r.OperationManager().FindString(req.Id)
	if op != nil {
		op.Cancel()
	}
	return &pb.CancelOperationResponse{}, nil
}

// BlockForOperation blocks for an operation to finish.
func (s *Server) BlockForOperation(ctx context.Context, req *pb.BlockForOperationRequest) (*pb.BlockForOperationResponse, error) {
	for {
		op := s.r.OperationManager().FindString(req.Id)
		if op == nil {
			return &pb.BlockForOperationResponse{}, nil
		}

		if !utils.SelectContextOrWait(ctx, time.Millisecond*5) {
			return nil, ctx.Err()
		}
	}
}

// ResourceNames returns the list of resources.
func (s *Server) ResourceNames(ctx context.Context, _ *pb.ResourceNamesRequest) (*pb.ResourceNamesResponse, error) {
	all := s.r.ResourceNames()
	rNames := make([]*commonpb.ResourceName, 0, len(all))
	for _, m := range all {
		rNames = append(
			rNames,
			protoutils.ResourceNameToProto(m),
		)
	}
	return &pb.ResourceNamesResponse{Resources: rNames}, nil
}

func (s *Server) FrameSystemConfig(
	ctx context.Context,
	req *pb.FrameSystemConfigRequest,
) (*pb.FrameSystemConfigResponse, error) {
	sortedParts, err := s.r.FrameSystemConfig(ctx, req.GetSupplementalTransforms())
	if err != nil {
		return nil, err
	}
	configs := make([]*pb.FrameSystemConfig, len(sortedParts))
	for i, part := range sortedParts {
		c, err := part.ToProtobuf()
		if err != nil {
			if errors.Is(err, referenceframe.ErrNoModelInformation) {
				configs[i] = nil
				continue
			}
			return nil, err
		}
		configs[i] = c
	}
	return &pb.FrameSystemConfigResponse{FrameSystemConfigs: configs}, nil
}

func (s *Server) TransformPose(ctx context.Context, req *pb.TransformPoseRequest) (*pb.TransformPoseResponse, error) {
	dst := req.Destination
	pF := referenceframe.ProtobufToPoseInFrame(req.Source)
	transformedPose, err := s.r.TransformPose(ctx, pF, dst, req.GetSupplementalTransforms())

	return &pb.TransformPoseResponse{Pose: referenceframe.PoseInFrameToProtobuf(transformedPose)}, err
}
