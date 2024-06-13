// Package server contains a gRPC based robot.Robot server implementation.
//
// It should be used by an rpc.Server.
package server

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/utils"
	vprotoutils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
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
const logTSKey = "log_ts"

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

// DiscoverComponents takes a list of discovery queries and returns corresponding
// component configurations.
func (s *Server) DiscoverComponents(ctx context.Context, req *pb.DiscoverComponentsRequest) (*pb.DiscoverComponentsResponse, error) {
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
		queries = append(queries, resource.DiscoveryQuery{API: s, Model: m})
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
		pbQuery := &pb.DiscoveryQuery{Subtype: discovery.Query.API.String(), Model: discovery.Query.Model.String()}
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
	return &pb.TransformPoseResponse{Pose: referenceframe.PoseInFrameToProtobuf(transformedPose)}, err
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
	var buf bytes.Buffer
	buf.Grow(200 + (final.Size() * 4 * 4)) // 4 numbers per point, each 4 bytes
	err = pointcloud.ToPCD(final, &buf, pointcloud.PCDBinary)
	if err != nil {
		return nil, err
	}
	return &pb.TransformPCDResponse{PointCloudPcd: buf.Bytes()}, err
}

// GetStatus takes a list of resource names and returns their corresponding statuses. If no names are passed in, return all statuses.
func (s *Server) GetStatus(ctx context.Context, req *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	resourceNames := make([]resource.Name, 0, len(req.ResourceNames))
	for _, name := range req.ResourceNames {
		resourceNames = append(resourceNames, protoutils.ResourceNameFromProto(name))
	}

	statuses, err := s.robot.Status(ctx, resourceNames)
	if err != nil {
		return nil, err
	}

	statusesP := make([]*pb.Status, 0, len(statuses))
	for _, status := range statuses {
		statusP, err := vprotoutils.StructToStructPb(status.Status)
		if err != nil {
			return nil, err
		}
		statusesP = append(
			statusesP,
			&pb.Status{
				Name:             protoutils.ResourceNameToProto(status.Name),
				LastReconfigured: timestamppb.New(status.LastReconfigured),
				Status:           statusP,
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
		if !utils.SelectContextOrWaitChan(streamServer.Context(), ticker.C) {
			return streamServer.Context().Err()
		}

		status, err := s.GetStatus(streamServer.Context(), &pb.GetStatusRequest{ResourceNames: req.ResourceNames})
		switch {
		case err == nil:
		case grpcstatus.Code(err) == codes.Unimplemented:
			return nil
		default:
			return err
		}

		if err := streamServer.Send(&pb.StreamStatusResponse{Status: status.Status}); err != nil {
			return err
		}
	}
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

	// Use a sublogger of robot logger with correct logger name. Set a level of
	// DEBUG to allow gRPC logs at DEBUG level even when RDK is not on DEBUG
	// level. Disable caller to mimic caller passed in from gRPC request.
	logger := s.robot.Logger().Sublogger(log.LoggerName)
	logger.SetLevel(logging.DEBUG)
	l := logger.WithOptions(zap.WithCaller(false))

	fields := make([]any, 0, len(log.Fields)*2)
	for _, fieldP := range log.Fields {
		key, val, err := logging.FieldKeyAndValueFromProto(fieldP)
		if err != nil {
			return nil, err
		}
		fields = append(fields, key, val)
	}

	// Insert field of `{"log_ts": log.Time}` to encode the timestamp of this
	// log.
	fields = append(fields, logTSKey, log.Time.AsTime())

	level, err := logging.LevelFromString(log.Level)
	switch {
	case err != nil:
		l.Warn("logger named %q sent a log over gRPC with an invalid level %q", log.LoggerName, log.Level)
	case level == logging.DEBUG:
		l.Debugw(log.Message, fields...)
	case level == logging.INFO:
		l.Infow(log.Message, fields...)
	case level == logging.WARN:
		l.Warnw(log.Message, fields...)
	case level == logging.ERROR:
		l.Errorw(log.Message, fields...)
	default:
	}

	return &pb.LogResponse{}, nil
}

// GetCloudMetadata returns app-related information about the robot.
func (s *Server) GetCloudMetadata(ctx context.Context, _ *pb.GetCloudMetadataRequest) (*pb.GetCloudMetadataResponse, error) {
	md, err := s.robot.CloudMetadata(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetCloudMetadataResponse{
		// TODO: RSDK-7181 remove RobotPartId
		RobotPartId:   md.MachinePartID, // Deprecated: Duplicates MachinePartId
		PrimaryOrgId:  md.PrimaryOrgID,
		LocationId:    md.LocationID,
		MachineId:     md.MachineID,
		MachinePartId: md.MachinePartID,
	}, nil
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
