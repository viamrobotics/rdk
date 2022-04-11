// Package server contains a gRPC based robot.Robot server implementation.
//
// It should be used by an rpc.Server.
package server

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/action"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/operation"
	pb "go.viam.com/rdk/proto/api/robot/v1"
	"go.viam.com/rdk/robot"
	rdkutils "go.viam.com/rdk/utils"
)

// Server implements the contract from robot.proto that ultimately satisfies
// an robot.Robot as a gRPC server.
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

// DoAction runs an action on the underlying robot.
func (s *Server) DoAction(
	ctx context.Context,
	req *pb.DoActionRequest,
) (*pb.DoActionResponse, error) {
	act := action.LookupAction(req.Name)
	if act == nil {
		return nil, errors.Errorf("unknown action name [%s]", req.Name)
	}
	s.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer s.activeBackgroundWorkers.Done()
		act(s.cancelCtx, s.r)
	})
	return &pb.DoActionResponse{}, nil
}

type runCommander interface {
	RunCommand(ctx context.Context, name string, args map[string]interface{}) (map[string]interface{}, error)
}

// ResourceRunCommand runs an arbitrary command on a resource if it supports it.
func (s *Server) ResourceRunCommand(
	ctx context.Context,
	req *pb.ResourceRunCommandRequest,
) (*pb.ResourceRunCommandResponse, error) {
	// TODO(RDK-38): support all resources
	// we know only gps has this right now, so just look at sensors!
	resource, err := s.r.ResourceByName(gps.Named(req.ResourceName))
	if err != nil {
		return nil, err
	}
	commander, ok := rdkutils.UnwrapProxy(resource).(runCommander)
	if !ok {
		return nil, errors.New("cannot run commands on this resource")
	}
	result, err := commander.RunCommand(ctx, req.CommandName, req.Args.AsMap())
	if err != nil {
		return nil, err
	}
	resultPb, err := structpb.NewStruct(result)
	if err != nil {
		return nil, err
	}

	return &pb.ResourceRunCommandResponse{Result: resultPb}, nil
}

// ListOperations lists all running operations.
func (s *Server) ListOperations(ctx context.Context, req *pb.ListOperationsRequest) (*pb.ListOperationsResponse, error) {
	me := operation.Get(ctx)

	all := operation.CurrentOps()

	res := &pb.ListOperationsResponse{}
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
	b, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}

	s := &structpb.Struct{}
	err = protojson.Unmarshal(b, s)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// KillOperation kills an operations.
func (s *Server) KillOperation(ctx context.Context, req *pb.KillOperationRequest) (*pb.KillOperationResponse, error) {
	op := operation.FindOpString(req.Id)
	if op != nil {
		op.Cancel()
	}
	return &pb.KillOperationResponse{}, nil
}

// BlockForOperation blocks for an operation to finish.
func (s *Server) BlockForOperation(ctx context.Context, req *pb.BlockForOperationRequest) (*pb.BlockForOperationResponse, error) {
	for {
		op := operation.FindOpString(req.Id)
		if op == nil {
			return &pb.BlockForOperationResponse{}, nil
		}

		if !utils.SelectContextOrWait(ctx, time.Millisecond*5) {
			return nil, ctx.Err()
		}
	}
}

