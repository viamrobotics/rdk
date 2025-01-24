// Package server contains a gRPC based robot.Robot server implementation.
//
// It should be used by an rpc.Server.
package server

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/utils"
	vprotoutils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/session"
)

// logTSKey is the key used in conjunction with the timestamp of logs received
// by the RDK.
const (
	logTSKey = "log_ts"
)

// Server implements the contract from robot.proto that ultimately satisfies
// a robot.Robot as a gRPC server.
type Server struct {
	pb.UnimplementedRobotServiceServer
	robot robot.Robot
}

// New constructs a gRPC service server for a Robot.
func New(robot robot.Robot) pb.RobotServiceServer {
	return &Server{
		robot: robot,
	}
}

// Close cleanly shuts down the server.
func (s *Server) Close() {
}

// GetOperations lists all running operations.
func (s *Server) GetOperations(ctx context.Context, req *pb.GetOperationsRequest) (*pb.GetOperationsResponse, error) {
	me := operation.Get(ctx)

	all := s.robot.OperationManager().All()

	res := &pb.GetOperationsResponse{}
	for _, o := range all {
		if o == me {
			continue
		}

		s, err := convertInterfaceToStruct(o.Arguments)
		if err != nil {
			return nil, err
		}

		pbOp := &pb.Operation{
			Id:        o.ID.String(),
			Method:    o.Method,
			Arguments: s,
			Started:   timestamppb.New(o.Started),
		}
		if o.SessionID != uuid.Nil {
			sid := o.SessionID.String()
			pbOp.SessionId = &sid
		}
		res.Operations = append(res.Operations, pbOp)
	}

	return res, nil
}

func convertInterfaceToStruct(i interface{}) (*structpb.Struct, error) {
	if i == nil {
		return &structpb.Struct{}, nil
	}
	return vprotoutils.StructToStructPb(i)
}

// CancelOperation kills an operations.
func (s *Server) CancelOperation(ctx context.Context, req *pb.CancelOperationRequest) (*pb.CancelOperationResponse, error) {
	op := s.robot.OperationManager().FindString(req.Id)
	if op != nil {
		op.Cancel()
	}
	return &pb.CancelOperationResponse{}, nil
}

// BlockForOperation blocks for an operation to finish.
func (s *Server) BlockForOperation(ctx context.Context, req *pb.BlockForOperationRequest) (*pb.BlockForOperationResponse, error) {
	for {
		op := s.robot.OperationManager().FindString(req.Id)
		if op == nil {
			return &pb.BlockForOperationResponse{}, nil
		}

		if !utils.SelectContextOrWait(ctx, time.Millisecond*5) {
			return nil, ctx.Err()
		}
	}
}

// GetSessions lists all active sessions.
func (s *Server) GetSessions(ctx context.Context, req *pb.GetSessionsRequest) (*pb.GetSessionsResponse, error) {
	allSessions := s.robot.SessionManager().All()

	resp := &pb.GetSessionsResponse{}
	for _, sess := range allSessions {
		resp.Sessions = append(resp.Sessions, &pb.Session{
			Id:                 sess.ID().String(),
			PeerConnectionInfo: sess.PeerConnectionInfo(),
		})
	}

	return resp, nil
}

// ResourceNames returns the list of resources.
func (s *Server) ResourceNames(ctx context.Context, _ *pb.ResourceNamesRequest) (*pb.ResourceNamesResponse, error) {
	all := s.robot.ResourceNames()
	rNames := make([]*commonpb.ResourceName, 0, len(all))
	for _, m := range all {
		rNames = append(
			rNames,
			protoutils.ResourceNameToProto(m),
		)
	}
	return &pb.ResourceNamesResponse{Resources: rNames}, nil
}

// ResourceRPCSubtypes returns the list of resource RPC APIs.
// Subtypes is an older name but preserved in proto.
func (s *Server) ResourceRPCSubtypes(ctx context.Context, _ *pb.ResourceRPCSubtypesRequest) (*pb.ResourceRPCSubtypesResponse, error) {
	resAPIs := s.robot.ResourceRPCAPIs()
	protoTypes := make([]*pb.ResourceRPCSubtype, 0, len(resAPIs))
	for _, rt := range resAPIs {
		protoTypes = append(protoTypes, &pb.ResourceRPCSubtype{
			Subtype: protoutils.ResourceNameToProto(resource.Name{
				API:  rt.API,
				Name: "",
			}),
			ProtoService: rt.Desc.GetFullyQualifiedName(),
		})
	}
	return &pb.ResourceRPCSubtypesResponse{ResourceRpcSubtypes: protoTypes}, nil
}

// DiscoverComponents is DEPRECATED!!! Please use the Discovery Service instead.
// DiscoverComponents takes a list of discovery queries and returns corresponding
// component configurations.
//
//nolint:deprecated,staticcheck
func (s *Server) DiscoverComponents(ctx context.Context, req *pb.DiscoverComponentsRequest) (*pb.DiscoverComponentsResponse, error) {
	s.robot.Logger().CWarn(ctx,
		"DiscoverComponents is deprecated and will be removed on March 10th 2025. Please use the Discovery Service instead.")
	// nonTriplet indicates older syntax for type and model E.g. "camera" instead of "rdk:component:camera"
	// TODO(PRODUCT-344): remove triplet checking here after complete
	var nonTriplet bool
	queries := make([]resource.DiscoveryQuery, 0, len(req.Queries))
	for _, q := range req.Queries {
		m, err := resource.NewModelFromString(q.Model)
		if err != nil {
			return nil, err
		}
		if !strings.ContainsRune(q.Subtype, ':') {
			nonTriplet = true
			q.Subtype = "rdk:component:" + q.Subtype
		}
		s, err := resource.NewAPIFromString(q.Subtype)
		if err != nil {
			return nil, err
		}
		queries = append(queries, resource.DiscoveryQuery{API: s, Model: m, Extra: q.Extra.AsMap()})
	}

	discoveries, err := s.robot.DiscoverComponents(ctx, queries)
	if err != nil {
		return nil, err
	}

	pbDiscoveries := make([]*pb.Discovery, 0, len(discoveries))
	for _, discovery := range discoveries {
		pbResults, err := vprotoutils.StructToStructPb(discovery.Results)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to construct a structpb.Struct from discovery for %q", discovery.Query)
		}
		extra, err := structpb.NewStruct(discovery.Query.Extra)
		if err != nil {
			return nil, err
		}
		pbQuery := &pb.DiscoveryQuery{
			Subtype: discovery.Query.API.String(),
			Model:   discovery.Query.Model.String(),
			Extra:   extra,
		}
		if nonTriplet {
			pbQuery.Subtype = discovery.Query.API.SubtypeName
			pbQuery.Model = discovery.Query.Model.Name
		}
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
func (s *Server) FrameSystemConfig(ctx context.Context, req *pb.FrameSystemConfigRequest) (*pb.FrameSystemConfigResponse, error) {
	fsCfg, err := s.robot.FrameSystemConfig(ctx)
	if err != nil {
		return nil, err
	}
	configs := make([]*pb.FrameSystemConfig, len(fsCfg.Parts))
	for i, part := range fsCfg.Parts {
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
	transforms, err := referenceframe.LinkInFramesFromTransformsProtobuf(req.GetSupplementalTransforms())
	if err != nil {
		return nil, err
	}
	transformedPose, err := s.robot.TransformPose(ctx, referenceframe.ProtobufToPoseInFrame(req.Source), req.Destination, transforms)
	if err != nil {
		return nil, err
	}
	return &pb.TransformPoseResponse{Pose: referenceframe.PoseInFrameToProtobuf(transformedPose)}, nil
}

// TransformPCD will transform the pointcloud to the desired frame in the robot's frame system.
// Do not move the robot between the generation of the initial pointcloud and the receipt
// of the transformed pointcloud because that will make the transformations inaccurate.
// TODO(RSDK-1123): PCD files have a field called VIEWPOINT which encodes an offset as a translation+quaternion.
// if we used VIEWPOINT, you only need to query the frame system to get the transform between the source and destination frame.
// Then, you put that transform as a translation+quaternion in the VIEWPOINT field. You would only change one line in the PCD file,
// rather than having to decode and then encode every point in the PCD. Would be a considerable speed up.
func (s *Server) TransformPCD(ctx context.Context, req *pb.TransformPCDRequest) (*pb.TransformPCDResponse, error) {
	// transform PCD bytes to pointcloud
	pc, err := pointcloud.ReadPCD(bytes.NewReader(req.PointCloudPcd))
	if err != nil {
		return nil, err
	}
	// transform
	final, err := s.robot.TransformPointCloud(ctx, pc, req.Source, req.Destination)
	if err != nil {
		return nil, err
	}
	// transform pointcloud back to PCD bytes
	bytes, err := pointcloud.ToBytes(final)
	if err != nil {
		return nil, err
	}
	return &pb.TransformPCDResponse{PointCloudPcd: bytes}, err
}

// StopAll will stop all current and outstanding operations for the robot and stops all actuators and movement.
func (s *Server) StopAll(ctx context.Context, req *pb.StopAllRequest) (*pb.StopAllResponse, error) {
	extra := map[resource.Name]map[string]interface{}{}
	for _, e := range req.Extra {
		extra[protoutils.ResourceNameFromProto(e.Name)] = e.Params.AsMap()
	}
	if err := s.robot.StopAll(ctx, extra); err != nil {
		return nil, err
	}
	return &pb.StopAllResponse{}, nil
}

// StartSession creates a new session that expects heartbeats at the given interval. If the interval
// lapses, any resources that have safety heart monitored methods, where this session was the last caller
// on the resource, will be stopped.
func (s *Server) StartSession(ctx context.Context, req *pb.StartSessionRequest) (*pb.StartSessionResponse, error) {
	var authUID string
	if authEntity, ok := rpc.ContextAuthEntity(ctx); ok {
		authUID = authEntity.Entity
	}
	if _, ok := session.FromContext(ctx); ok {
		return nil, errors.New("session already exists")
	}
	if req.Resume != "" {
		resumeWith, err := uuid.Parse(req.Resume)
		if err != nil {
			return nil, err
		}
		if sess, err := s.robot.SessionManager().FindByID(ctx, resumeWith, authUID); err != nil {
			if !errors.Is(err, session.ErrNoSession) {
				return nil, err
			}
		} else {
			return &pb.StartSessionResponse{
				Id:              req.Resume,
				HeartbeatWindow: durationpb.New(sess.HeartbeatWindow()),
			}, nil
		}
	}
	sess, err := s.robot.SessionManager().Start(
		ctx,
		authUID,
	)
	if err != nil {
		return nil, err
	}
	return &pb.StartSessionResponse{
		Id:              sess.ID().String(),
		HeartbeatWindow: durationpb.New(sess.HeartbeatWindow()),
	}, nil
}

// SendSessionHeartbeat sends a heartbeat to the given session.
func (s *Server) SendSessionHeartbeat(ctx context.Context, req *pb.SendSessionHeartbeatRequest) (*pb.SendSessionHeartbeatResponse, error) {
	var authUID string
	if authEntity, ok := rpc.ContextAuthEntity(ctx); ok {
		authUID = authEntity.Entity
	}
	sessID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, err
	}
	if _, err := s.robot.SessionManager().FindByID(ctx, sessID, authUID); err != nil {
		return nil, err
	}
	return &pb.SendSessionHeartbeatResponse{}, nil
}

// Log receives logs to be logged by this robot.
func (s *Server) Log(ctx context.Context, req *pb.LogRequest) (*pb.LogResponse, error) {
	if req.Logs == nil {
		return nil, errors.New("LogRequest received with no associated logs")
	}
	if len(req.Logs) > 1 {
		return nil, errors.New("LogRequest received with multiple logs; batching not yet supported")
	}
	log := req.Logs[0]

	level, err := logging.LevelFromString(log.Level)
	if err != nil {
		return nil, fmt.Errorf("LogRequest received with invalid level %q", log.Level)
	}

	// Create a custom log entry and write entry unconditionally to logger. We
	// trust module libraries to handle their own levels and only send us logs
	// over gRPC that we should be outputting.
	zEntry := zapcore.Entry{
		Level:      level.AsZap(),
		Time:       time.Now(),
		LoggerName: log.LoggerName,
		Message:    log.Message,
		// `Caller` is already encoded in `Message` above
		// `Stack` is not included
	}
	fields := make([]zapcore.Field, 0, len(log.Fields)*2)
	for _, fieldP := range log.Fields {
		field, err := logging.FieldFromProto(fieldP)
		if err != nil {
			return nil, errors.Wrap(err, "error converting LogRequest log field from proto")
		}
		fields = append(fields, field)
	}
	// Insert field of `{"log_ts": log.Time}` to encode the timestamp of this
	// log.
	tsField := zapcore.Field{
		Key:    logTSKey,
		Type:   zapcore.StringType,
		String: log.Time.AsTime().Format(logging.DefaultTimeFormatStr),
	}
	fields = append(fields, tsField)
	entry := logging.LogEntry{
		Entry:  zEntry,
		Fields: fields,
	}

	s.robot.Logger().Write(&entry)
	return &pb.LogResponse{}, nil
}

// GetCloudMetadata returns app-related information about the robot.
func (s *Server) GetCloudMetadata(ctx context.Context, _ *pb.GetCloudMetadataRequest) (*pb.GetCloudMetadataResponse, error) {
	md, err := s.robot.CloudMetadata(ctx)
	if err != nil {
		return nil, err
	}
	return protoutils.MetadataToProto(md), nil
}

// RestartModule restarts a module by name or ID.
func (s *Server) RestartModule(ctx context.Context, req *pb.RestartModuleRequest) (*pb.RestartModuleResponse, error) {
	goReq := robot.RestartModuleRequest{
		ModuleID:   req.GetModuleId(),
		ModuleName: req.GetModuleName(),
	}
	err := s.robot.RestartModule(ctx, goReq)
	if err != nil {
		return nil, err
	}
	return &pb.RestartModuleResponse{}, nil
}

// Shutdown shuts down the robot.
func (s *Server) Shutdown(ctx context.Context, _ *pb.ShutdownRequest) (*pb.ShutdownResponse, error) {
	err := s.robot.Shutdown(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ShutdownResponse{}, nil
}

// GetMachineStatus returns the current status of the robot.
func (s *Server) GetMachineStatus(ctx context.Context, _ *pb.GetMachineStatusRequest) (*pb.GetMachineStatusResponse, error) {
	var result pb.GetMachineStatusResponse

	mStatus, err := s.robot.MachineStatus(ctx)
	if err != nil {
		return nil, err
	}
	result.Config = &pb.ConfigStatus{
		Revision:    mStatus.Config.Revision,
		LastUpdated: timestamppb.New(mStatus.Config.LastUpdated),
	}
	result.Resources = make([]*pb.ResourceStatus, 0, len(mStatus.Resources))
	for _, resStatus := range mStatus.Resources {
		pbResStatus := &pb.ResourceStatus{
			Name:          protoutils.ResourceNameToProto(resStatus.Name),
			LastUpdated:   timestamppb.New(resStatus.LastUpdated),
			Revision:      resStatus.Revision,
			CloudMetadata: protoutils.MetadataToProto(resStatus.CloudMetadata),
		}

		switch resStatus.State {
		case resource.NodeStateUnknown:
			s.robot.Logger().CErrorw(ctx, "resource in an unknown state", "resource", resStatus.Name.String())
			pbResStatus.State = pb.ResourceStatus_STATE_UNSPECIFIED
		case resource.NodeStateUnconfigured:
			pbResStatus.State = pb.ResourceStatus_STATE_UNCONFIGURED
		case resource.NodeStateConfiguring:
			pbResStatus.State = pb.ResourceStatus_STATE_CONFIGURING
		case resource.NodeStateReady:
			pbResStatus.State = pb.ResourceStatus_STATE_READY
		case resource.NodeStateRemoving:
			pbResStatus.State = pb.ResourceStatus_STATE_REMOVING
		case resource.NodeStateUnhealthy:
			pbResStatus.State = pb.ResourceStatus_STATE_UNHEALTHY
			if resStatus.Error != nil {
				pbResStatus.Error = resStatus.Error.Error()
			}
		}

		result.Resources = append(result.Resources, pbResStatus)
	}

	switch mStatus.State {
	case robot.StateUnknown:
		s.robot.Logger().CError(ctx, "machine in an unknown state")
		result.State = pb.GetMachineStatusResponse_STATE_UNSPECIFIED
	case robot.StateInitializing:
		result.State = pb.GetMachineStatusResponse_STATE_INITIALIZING
	case robot.StateRunning:
		result.State = pb.GetMachineStatusResponse_STATE_RUNNING
	}

	return &result, nil
}

// GetVersion returns version information about the robot.
func (s *Server) GetVersion(ctx context.Context, _ *pb.GetVersionRequest) (*pb.GetVersionResponse, error) {
	result, err := robot.Version()
	if err != nil {
		return nil, err
	}

	return &pb.GetVersionResponse{
		Platform:   result.Platform,
		Version:    result.Version,
		ApiVersion: result.APIVersion,
	}, nil
}
