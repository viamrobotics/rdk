// Package arm contains a gRPC based arm service server.
package arm

import (
	"context"
	"strings"
	"time"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

const unimplemented = "unimplemented"

// serviceServer implements the ArmService from arm.proto.
type serviceServer struct {
	pb.UnimplementedArmServiceServer
	coll resource.APIResourceGetter[Arm]
}

// NewRPCServiceServer constructs an arm gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[Arm]) interface{} {
	return &serviceServer{coll: coll}
}

// GetEndPosition returns the position of the arm specified.
func (s *serviceServer) GetEndPosition(
	ctx context.Context,
	req *pb.GetEndPositionRequest,
) (*pb.GetEndPositionResponse, error) {
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := arm.EndPosition(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	// Return a default empty value if the position returned is nil,
	// this guards against nil objects being transferred over the wire.
	if pos == nil {
		pose := &commonpb.Pose{}
		return &pb.GetEndPositionResponse{Pose: pose}, nil
	}

	return &pb.GetEndPositionResponse{Pose: spatialmath.PoseToProtobuf(pos)}, nil
}

// GetJointPositions gets the current joint position of an arm of the underlying robot.
func (s *serviceServer) GetJointPositions(ctx context.Context, req *pb.GetJointPositionsRequest) (*pb.GetJointPositionsResponse, error) {
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := arm.JointPositions(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	// safe to ignore error because conversion function below can handle nil values and warning messages are logged from client
	//nolint:errcheck
	m, _ := arm.Kinematics(ctx)
	jp, err := referenceframe.JointPositionsFromInputs(m, pos)
	if err != nil {
		return nil, err
	}
	return &pb.GetJointPositionsResponse{Positions: jp}, nil
}

// MoveToPosition returns the position of the arm specified.
func (s *serviceServer) MoveToPosition(ctx context.Context, req *pb.MoveToPositionRequest) (*pb.MoveToPositionResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.MoveToPositionResponse{}, arm.MoveToPosition(
		ctx,
		spatialmath.NewPoseFromProtobuf(req.GetTo()),
		req.Extra.AsMap(),
	)
}

// MoveToJointPositions moves an arm of the underlying robot to the requested joint positions.
func (s *serviceServer) MoveToJointPositions(
	ctx context.Context,
	req *pb.MoveToJointPositionsRequest,
) (*pb.MoveToJointPositionsResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	// safe to ignore error because conversion function below can handle nil values and warning messages are logged from client
	//nolint:errcheck
	m, _ := arm.Kinematics(ctx)
	inputs, err := referenceframe.InputsFromJointPositions(m, req.Positions)
	if err != nil {
		return nil, err
	}
	return &pb.MoveToJointPositionsResponse{}, arm.MoveToJointPositions(ctx, inputs, req.Extra.AsMap())
}

// MoveThroughJointPositions moves an arm of the underlying robot through the requested joint positions.
func (s *serviceServer) MoveThroughJointPositions(
	ctx context.Context,
	req *pb.MoveThroughJointPositionsRequest,
) (*pb.MoveThroughJointPositionsResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	// safe to ignore error because conversion function below can handle nil values and warning messages are logged from client
	//nolint:errcheck
	m, _ := arm.Kinematics(ctx)
	allInputs := make([][]referenceframe.Input, 0, len(req.Positions))
	for _, position := range req.Positions {
		inputs, err := referenceframe.InputsFromJointPositions(m, position)
		if err != nil {
			return nil, err
		}
		allInputs = append(allInputs, inputs)
	}
	err = arm.MoveThroughJointPositions(ctx, allInputs, moveOptionsFromProtobuf(req.Options), req.Extra.AsMap())
	return &pb.MoveThroughJointPositionsResponse{}, err
}

// Stop stops the arm specified.
func (s *serviceServer) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.StopResponse{}, arm.Stop(ctx, req.Extra.AsMap())
}

// IsMoving queries of a component is in motion.
func (s *serviceServer) IsMoving(ctx context.Context, req *pb.IsMovingRequest) (*pb.IsMovingResponse, error) {
	arm, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	moving, err := arm.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.IsMovingResponse{IsMoving: moving}, nil
}

func (s *serviceServer) GetGeometries(ctx context.Context, req *commonpb.GetGeometriesRequest) (*commonpb.GetGeometriesResponse, error) {
	res, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	geometries, err := res.Geometries(ctx, req.Extra.AsMap())
	if err != nil {
		// if the error tells us the method is unimplemented, then we
		// can use the kinematics and joint positions endpoints to
		// construct the geometries of the arm
		if strings.Contains(err.Error(), unimplemented) {
			kinematicsPbResp, err := s.GetKinematics(ctx, &commonpb.GetKinematicsRequest{Name: req.GetName()})
			if err != nil {
				return nil, err
			}
			model, err := referenceframe.KinematicModelFromProtobuf(req.GetName(), kinematicsPbResp)
			if err != nil {
				return nil, err
			}

			jointPbResp, err := s.GetJointPositions(ctx, &pb.GetJointPositionsRequest{Name: req.GetName()})
			if err != nil {
				return nil, err
			}
			jointPositionsPb := jointPbResp.GetPositions()

			// Joint positions are in degrees but model.Geometries expects radians, so we convert them here.
			jointPositionsRads, err := referenceframe.InputsFromJointPositions(model, jointPositionsPb)
			if err != nil {
				return nil, err
			}
			gifs, err := model.Geometries(jointPositionsRads)
			if err != nil {
				return nil, err
			}
			return &commonpb.GetGeometriesResponse{Geometries: referenceframe.NewGeometriesToProto(
				gifs.Geometries())}, nil
		}
		return nil, err
	}
	return &commonpb.GetGeometriesResponse{Geometries: referenceframe.NewGeometriesToProto(geometries)}, nil
}

// Get3DModels returns the 3D models of the arm.
func (s *serviceServer) Get3DModels(ctx context.Context, req *commonpb.Get3DModelsRequest) (*commonpb.Get3DModelsResponse, error) {
	arm, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	models, err := arm.Get3DModels(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &commonpb.Get3DModelsResponse{Models: models}, nil
}

// GetKinematics returns the kinematics information associated with the arm.
func (s *serviceServer) GetKinematics(ctx context.Context, req *commonpb.GetKinematicsRequest) (*commonpb.GetKinematicsResponse, error) {
	arm, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	model, err := arm.Kinematics(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.KinematicModelToProtobuf(model), nil
}

// StreamJointPositions streams joint positions at the requested rate.
func (s *serviceServer) StreamJointPositions(
	req *pb.StreamJointPositionsRequest,
	stream pb.ArmService_StreamJointPositionsServer,
) error {
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return err
	}

	// Get kinematics model once for efficiency
	model, _ := arm.Kinematics(stream.Context())

	// Default to 30 FPS if not specified
	fps := req.GetFps()
	if fps <= 0 {
		fps = 30
	}

	// Create ticker for consistent timing
	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()

		case <-ticker.C:
			// Get current joint positions
			positions, err := arm.JointPositions(stream.Context(), req.Extra.AsMap())
			if err != nil {
				return err
			}

			// Convert to protobuf format
			jp, err := referenceframe.JointPositionsFromInputs(model, positions)
			if err != nil {
				return err
			}

			// Send response with timestamp
			resp := &pb.StreamJointPositionsResponse{
				Positions: jp,
				Timestamp: timestamppb.Now(),
			}

			if err := stream.Send(resp); err != nil {
				return err
			}
		}
	}
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	arm, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, arm, req)
}
