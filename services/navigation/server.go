package navigation

import (
	"context"

	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/navigation/v1"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the contract from navigation.proto.
type subtypeServer struct {
	pb.UnimplementedNavigationServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a framesystem gRPC service server.
func NewServer(s subtype.Service) pb.NavigationServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service(serviceName string) (Service, error) {
	resource := server.subtypeSvc.Resource(serviceName)
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Named(serviceName))
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("navigation.Service", resource)
	}
	return svc, nil
}

func (server *subtypeServer) GetMode(ctx context.Context, req *pb.GetModeRequest) (
	*pb.GetModeResponse, error,
) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	mode, err := svc.GetMode(ctx)
	if err != nil {
		return nil, err
	}
	protoMode := pb.Mode_MODE_UNSPECIFIED
	switch mode {
	case ModeManual:
		protoMode = pb.Mode_MODE_MANUAL
	case ModeWaypoint:
		protoMode = pb.Mode_MODE_WAYPOINT
	}
	return &pb.GetModeResponse{
		Mode: protoMode,
	}, nil
}

func (server *subtypeServer) SetMode(ctx context.Context, req *pb.SetModeRequest) (
	*pb.SetModeResponse, error,
) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	switch req.Mode {
	case pb.Mode_MODE_MANUAL:
		if err := svc.SetMode(ctx, ModeManual); err != nil {
			return nil, err
		}
	case pb.Mode_MODE_WAYPOINT:
		if err := svc.SetMode(ctx, ModeWaypoint); err != nil {
			return nil, err
		}
	case pb.Mode_MODE_UNSPECIFIED:
		fallthrough
	default:
		return nil, errors.Errorf("unknown mode %q", req.Mode.String())
	}
	return &pb.SetModeResponse{}, nil
}

func (server *subtypeServer) GetLocation(ctx context.Context, req *pb.GetLocationRequest) (
	*pb.GetLocationResponse, error,
) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	loc, err := svc.GetLocation(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetLocationResponse{
		Location: &commonpb.GeoPoint{Latitude: loc.Lat(), Longitude: loc.Lng()},
	}, nil
}

func (server *subtypeServer) GetWaypoints(ctx context.Context, req *pb.GetWaypointsRequest) (
	*pb.GetWaypointsResponse, error,
) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	waypoints, err := svc.GetWaypoints(ctx)
	if err != nil {
		return nil, err
	}
	protoWaypoints := make([]*pb.Waypoint, 0, len(waypoints))
	for _, wp := range waypoints {
		protoWaypoints = append(protoWaypoints, &pb.Waypoint{
			Id:       wp.ID.Hex(),
			Location: &commonpb.GeoPoint{Latitude: wp.Lat, Longitude: wp.Long},
		})
	}
	return &pb.GetWaypointsResponse{
		Waypoints: protoWaypoints,
	}, nil
}

func (server *subtypeServer) AddWaypoint(ctx context.Context, req *pb.AddWaypointRequest) (
	*pb.AddWaypointResponse, error,
) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	point := geo.NewPoint(req.Location.Latitude, req.Location.Longitude)
	if err = svc.AddWaypoint(ctx, point); err != nil {
		return nil, err
	}
	return &pb.AddWaypointResponse{}, nil
}

func (server *subtypeServer) RemoveWaypoint(ctx context.Context, req *pb.RemoveWaypointRequest) (
	*pb.RemoveWaypointResponse, error,
) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	id, err := primitive.ObjectIDFromHex(req.Id)
	if err != nil {
		return nil, err
	}
	if err = svc.RemoveWaypoint(ctx, id); err != nil {
		return nil, err
	}
	return &pb.RemoveWaypointResponse{}, nil
}
