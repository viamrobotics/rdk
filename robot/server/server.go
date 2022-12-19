// Package server contains a gRPC based robot.Robot server implementation.
//
// It should be used by an rpc.Server.
package server

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/utils"
	vprotoutils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/session"
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

// GetSessions lists all active sessions.
func (s *Server) GetSessions(ctx context.Context, req *pb.GetSessionsRequest) (*pb.GetSessionsResponse, error) {
	allSessions := s.r.SessionManager().All()

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

// ResourceRPCSubtypes returns the list of resource RPC subtypes.
func (s *Server) ResourceRPCSubtypes(ctx context.Context, _ *pb.ResourceRPCSubtypesRequest) (*pb.ResourceRPCSubtypesResponse, error) {
	resSubtypes := s.r.ResourceRPCSubtypes()
	protoTypes := make([]*pb.ResourceRPCSubtype, 0, len(resSubtypes))
	for _, rt := range resSubtypes {
		protoTypes = append(protoTypes, &pb.ResourceRPCSubtype{
			Subtype: protoutils.ResourceNameToProto(resource.Name{
				Subtype: rt.Subtype,
				Name:    "",
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
	queries := make([]discovery.Query, 0, len(req.Queries))
	for _, q := range req.Queries {
		m, err := resource.NewModelFromString(q.Model)
		if err != nil {
			return nil, err
		}
		if !strings.ContainsRune(q.Subtype, ':') {
			nonTriplet = true
			q.Subtype = "rdk:component:" + q.Subtype
		}
		s, err := resource.NewSubtypeFromString(q.Subtype)
		if err != nil {
			return nil, err
		}
		queries = append(queries, discovery.Query{API: s, Model: m})
	}

	discoveries, err := s.r.DiscoverComponents(ctx, queries)
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
			pbQuery.Subtype = string(discovery.Query.API.ResourceSubtype)
			pbQuery.Model = string(discovery.Query.Model.Name)
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
func (s *Server) FrameSystemConfig(
	ctx context.Context,
	req *pb.FrameSystemConfigRequest,
) (*pb.FrameSystemConfigResponse, error) {
	transforms, err := referenceframe.LinkInFramesFromTransformsProtobuf(req.GetSupplementalTransforms())
	if err != nil {
		return nil, err
	}
	sortedParts, err := s.r.FrameSystemConfig(ctx, transforms)
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
	transforms, err := referenceframe.LinkInFramesFromTransformsProtobuf(req.GetSupplementalTransforms())
	if err != nil {
		return nil, err
	}
	transformedPose, err := s.r.TransformPose(ctx, referenceframe.ProtobufToPoseInFrame(req.Source), req.Destination, transforms)
	return &pb.TransformPoseResponse{Pose: referenceframe.PoseInFrameToProtobuf(transformedPose)}, err
}

// GetStatus takes a list of resource names and returns their corresponding statuses. If no names are passed in, return all statuses.
func (s *Server) GetStatus(ctx context.Context, req *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	resourceNames := make([]resource.Name, 0, len(req.ResourceNames))
	for _, name := range req.ResourceNames {
		resourceNames = append(resourceNames, protoutils.ResourceNameFromProto(name))
	}

	statuses, err := s.r.Status(ctx, resourceNames)
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

// StopAll will stop all current and outstanding operations for the robot and stops all actuators and movement.
func (s *Server) StopAll(ctx context.Context, req *pb.StopAllRequest) (*pb.StopAllResponse, error) {
	extra := map[resource.Name]map[string]interface{}{}
	for _, e := range req.Extra {
		extra[protoutils.ResourceNameFromProto(e.Name)] = e.Params.AsMap()
	}
	if err := s.r.StopAll(ctx, extra); err != nil {
		return nil, err
	}
	return &pb.StopAllResponse{}, nil
}

// StartSession creates a new session that expects heartbeats at the given interval. If the interval
// lapses, any resources that have safety heart monitored methods, where this session was the last caller
// on the resource, will be stopped.
func (s *Server) StartSession(ctx context.Context, req *pb.StartSessionRequest) (*pb.StartSessionResponse, error) {
	authUID, _ := rpc.ContextAuthSubject(ctx)
	if _, ok := session.FromContext(ctx); ok {
		return nil, errors.New("session already exists")
	}
	if req.Resume != "" {
		resumeWith, err := uuid.Parse(req.Resume)
		if err != nil {
			return nil, err
		}
		if sess, err := s.r.SessionManager().FindByID(resumeWith, authUID); err != nil {
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
	sess, err := s.r.SessionManager().Start(
		authUID,
		peerConnectionInfoToProto(rpc.PeerConnectionInfoFromContext(ctx)),
	)
	if err != nil {
		return nil, err
	}
	return &pb.StartSessionResponse{
		Id:              sess.ID().String(),
		HeartbeatWindow: durationpb.New(sess.HeartbeatWindow()),
	}, nil
}

func peerConnectionInfoToProto(info rpc.PeerConnectionInfo) *pb.PeerConnectionInfo {
	var connType pb.PeerConnectionType
	switch info.ConnectionType {
	case rpc.PeerConnectionTypeGRPC:
		connType = pb.PeerConnectionType_PEER_CONNECTION_TYPE_GRPC
	case rpc.PeerConnectionTypeWebRTC:
		connType = pb.PeerConnectionType_PEER_CONNECTION_TYPE_WEBRTC
	case rpc.PeerConnectionTypeUnknown:
		fallthrough
	default:
		connType = pb.PeerConnectionType_PEER_CONNECTION_TYPE_UNSPECIFIED
	}

	return &pb.PeerConnectionInfo{
		Type:          connType,
		LocalAddress:  &info.LocalAddress,
		RemoteAddress: &info.RemoteAddress,
	}
}

// SendSessionHeartbeat sends a heartbeat to the given session.
func (s *Server) SendSessionHeartbeat(ctx context.Context, req *pb.SendSessionHeartbeatRequest) (*pb.SendSessionHeartbeatResponse, error) {
	authUID, _ := rpc.ContextAuthSubject(ctx)
	sessID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, err
	}
	if _, err := s.r.SessionManager().FindByID(sessID, authUID); err != nil {
		return nil, err
	}
	return &pb.SendSessionHeartbeatResponse{}, nil
}
