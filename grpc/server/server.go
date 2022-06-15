// Package server contains a gRPC based robot.Robot server implementation.
//
// It should be used by an rpc.Server.
package server

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/operation"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/robot/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
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
		return &structpb.Struct{}, nil
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

// DiscoverComponents takes a list of discovery queries and returns corresponding
// component configurations.
func (s *Server) DiscoverComponents(ctx context.Context, req *pb.DiscoverComponentsRequest) (*pb.DiscoverComponentsResponse, error) {
	queries := make([]discovery.Query, 0, len(req.Queries))
	for _, q := range req.Queries {
		queries = append(queries, discovery.Query{resource.SubtypeName(q.Subtype), q.Model})
	}

	discoveries, err := s.r.DiscoverComponents(ctx, queries)
	if err != nil {
		return nil, err
	}

	pbDiscoveries := make([]*pb.Discovery, 0, len(discoveries))
	for _, discovery := range discoveries {
		pbResults, err := protoutils.StructToStructPb(discovery.Results)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to construct a structpb.Struct from discovery for %q", discovery.Query)
		}
		pbQuery := &pb.DiscoveryQuery{Subtype: string(discovery.Query.SubtypeName), Model: discovery.Query.Model}
		pbDiscoveries = append(
			pbDiscoveries,
			&pb.Discovery{
				Query:   pbQuery,
				Results: pbResults,
			},
		)
	}

	return &pb.DiscoverComponentsResponse{Discovery: pbDiscoveries}, nil
}

// FrameSystemConfig returns the info of each individual part that makes up the frame system.
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

// TransformPose will transform the pose of the requested poseInFrame to the desired frame in the robot's frame system.
func (s *Server) TransformPose(ctx context.Context, req *pb.TransformPoseRequest) (*pb.TransformPoseResponse, error) {
	dst := req.Destination
	pF := referenceframe.ProtobufToPoseInFrame(req.Source)
	transformedPose, err := s.r.TransformPose(ctx, pF, dst, req.GetSupplementalTransforms())

	return &pb.TransformPoseResponse{Pose: referenceframe.PoseInFrameToProtobuf(transformedPose)}, err
}

// GetStatus takes a list of resource names and returns their corresponding statuses. If no names are passed in, return all statuses.
func (s *Server) GetStatus(ctx context.Context, req *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	resourceNames := make([]resource.Name, 0, len(req.ResourceNames))
	for _, name := range req.ResourceNames {
		resourceNames = append(resourceNames, protoutils.ResourceNameFromProto(name))
	}

	statuses, err := s.r.GetStatus(ctx, resourceNames)
	if err != nil {
		return nil, err
	}

	statusesP := make([]*pb.Status, 0, len(statuses))
	for _, status := range statuses {
		// InterfaceToMap necessary because structpb.NewStruct only accepts []interface{} for slices and mapstructure does not do the
		// conversion necessary.
		encoded, err := protoutils.InterfaceToMap(status.Status)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to convert status for %q to a form acceptable to structpb.NewStruct", status.Name)
		}
		statusP, err := structpb.NewStruct(encoded)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to construct a structpb.Struct from status for %q", status.Name)
		}
		statusesP = append(
			statusesP,
			&pb.Status{
				Name:   protoutils.ResourceNameToProto(status.Name),
				Status: statusP,
			},
		)
	}

	return &pb.GetStatusResponse{Status: statusesP}, nil
}

const defaultStreamInterval = 1 * time.Second

// StreamStatus periodically sends the status of all statuses requested. An empty request signifies all resources.
func (s *Server) StreamStatus(req *pb.StreamStatusRequest, streamServer pb.RobotService_StreamStatusServer) error {
	every := defaultStreamInterval
	if reqEvery := req.Every.AsDuration(); reqEvery != time.Duration(0) {
		every = reqEvery
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-streamServer.Context().Done():
			return streamServer.Context().Err()
		default:
		}
		select {
		case <-streamServer.Context().Done():
			return streamServer.Context().Err()
		case <-ticker.C:
		}
		status, err := s.GetStatus(streamServer.Context(), &pb.GetStatusRequest{ResourceNames: req.ResourceNames})
		if err != nil {
			return err
		}
		if err := streamServer.Send(&pb.StreamStatusResponse{Status: status.Status}); err != nil {
			return err
		}
	}
}
