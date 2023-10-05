package navigation

import (
	"context"

	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/navigation/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// serviceServer implements the contract from navigation.proto.
type serviceServer struct {
	pb.UnimplementedNavigationServiceServer
	coll resource.APIResourceCollection[Service]
}

// NewRPCServiceServer constructs a navigation gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceCollection[Service]) interface{} {
	return &serviceServer{coll: coll}
}

func (server *serviceServer) GetMode(ctx context.Context, req *pb.GetModeRequest) (*pb.GetModeResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	mode, err := svc.Mode(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	protoMode := pb.Mode_MODE_UNSPECIFIED
	switch mode {
	case ModeManual:
		protoMode = pb.Mode_MODE_MANUAL
	case ModeWaypoint:
		protoMode = pb.Mode_MODE_WAYPOINT
	case ModeExplore:
		protoMode = pb.Mode_MODE_EXPLORE
	}
	return &pb.GetModeResponse{
		Mode: protoMode,
	}, nil
}

func (server *serviceServer) SetMode(ctx context.Context, req *pb.SetModeRequest) (*pb.SetModeResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	switch req.Mode {
	case pb.Mode_MODE_MANUAL:
		if err := svc.SetMode(ctx, ModeManual, req.Extra.AsMap()); err != nil {
			return nil, err
		}
	case pb.Mode_MODE_WAYPOINT:
		if err := svc.SetMode(ctx, ModeWaypoint, req.Extra.AsMap()); err != nil {
			return nil, err
		}
	case pb.Mode_MODE_EXPLORE:
		if err := svc.SetMode(ctx, ModeExplore, req.Extra.AsMap()); err != nil {
			return nil, err
		}
	case pb.Mode_MODE_UNSPECIFIED:
		fallthrough
	default:
		return nil, errors.Errorf("unknown mode %q", req.Mode.String())
	}
	return &pb.SetModeResponse{}, nil
}

func (server *serviceServer) GetLocation(ctx context.Context, req *pb.GetLocationRequest) (*pb.GetLocationResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	geoPose, err := svc.Location(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}

	return &pb.GetLocationResponse{
		Location:       &commonpb.GeoPoint{Latitude: geoPose.Location().Lat(), Longitude: geoPose.Location().Lng()},
		CompassHeading: geoPose.Heading(),
	}, nil
}

func (server *serviceServer) GetWaypoints(ctx context.Context, req *pb.GetWaypointsRequest) (*pb.GetWaypointsResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	waypoints, err := svc.Waypoints(ctx, req.Extra.AsMap())
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

func (server *serviceServer) AddWaypoint(ctx context.Context, req *pb.AddWaypointRequest) (*pb.AddWaypointResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	point := geo.NewPoint(req.Location.Latitude, req.Location.Longitude)
	if err = svc.AddWaypoint(ctx, point, req.Extra.AsMap()); err != nil {
		return nil, err
	}
	return &pb.AddWaypointResponse{}, nil
}

func (server *serviceServer) RemoveWaypoint(ctx context.Context, req *pb.RemoveWaypointRequest) (*pb.RemoveWaypointResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	id, err := primitive.ObjectIDFromHex(req.Id)
	if err != nil {
		return nil, err
	}
	if err = svc.RemoveWaypoint(ctx, id, req.Extra.AsMap()); err != nil {
		return nil, err
	}
	return &pb.RemoveWaypointResponse{}, nil
}

func (server *serviceServer) GetObstacles(ctx context.Context, req *pb.GetObstaclesRequest) (*pb.GetObstaclesResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	obstacles, err := svc.GetObstacles(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	protoObs := []*commonpb.GeoObstacle{}
	for _, obstacle := range obstacles {
		protoObs = append(protoObs, spatialmath.GeoObstacleToProtobuf(obstacle))
	}
	return &pb.GetObstaclesResponse{Obstacles: protoObs}, nil
}

func (server *serviceServer) GetPaths(ctx context.Context, req *pb.GetObstaclesRequest) (*pb.GetPathsResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	paths, err := svc.GetPaths(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	var pbPaths []*pb.Path
	for _, path := range paths {
		var pbGeoPt []*commonpb.GeoPoint
		for _, pt := range path.GeoPoints {
			pbGeoPt = append(pbGeoPt, &commonpb.GeoPoint{Latitude: pt.Lat(), Longitude: pt.Lng()})
		}
		pbPaths = append(pbPaths, &pb.Path{
			DestinationWaypointId: path.DestinationWaypointID,
			Geopoints:             pbGeoPt,
		})
	}
	return &pb.GetPathsResponse{Paths: pbPaths}, nil
}

// DoCommand receives arbitrary commands.
func (server *serviceServer) DoCommand(
	ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, svc, req)
}
