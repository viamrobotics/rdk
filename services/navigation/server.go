package navigation

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/navigation/v1"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

type serviceServer struct {
	pb.UnimplementedNavigationServiceServer
	coll resource.APIResourceGetter[Service]
}

// NewRPCServiceServer constructs a navigation gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[Service], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

func (server *serviceServer) AddLocation(ctx context.Context, req *pb.AddLocationRequest) (*pb.AddLocationResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	var position *Position
	if req.Position != nil {
		pos, err := protobufToPosition(req.Position)
		if err != nil {
			return nil, err
		}
		position = &pos
	}
	loc, err := svc.AddLocation(ctx, AddLocationReq{
		Name:     req.LocationName,
		Position: position,
		Extra:    req.Extra.AsMap(),
	})
	if err != nil {
		return nil, err
	}
	pbLoc, err := locationToProtobuf(loc)
	if err != nil {
		return nil, err
	}
	return &pb.AddLocationResponse{Location: pbLoc}, nil
}

func (server *serviceServer) GetLocation(ctx context.Context, req *pb.GetLocationRequest) (*pb.GetLocationResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	loc, err := svc.GetLocation(ctx, req.LocationName, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	pbLoc, err := locationToProtobuf(loc)
	if err != nil {
		return nil, err
	}
	return &pb.GetLocationResponse{Location: pbLoc}, nil
}

func (server *serviceServer) ListLocations(ctx context.Context, req *pb.ListLocationsRequest) (*pb.ListLocationsResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	locs, err := svc.ListLocations(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	pbLocs, err := locationsToProtobuf(locs)
	if err != nil {
		return nil, err
	}
	return &pb.ListLocationsResponse{Locations: pbLocs}, nil
}

func (server *serviceServer) UpdateLocation(ctx context.Context, req *pb.UpdateLocationRequest) (*pb.UpdateLocationResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	var position *Position
	if req.Position != nil {
		pos, err := protobufToPosition(req.Position)
		if err != nil {
			return nil, err
		}
		position = &pos
	}
	loc, err := svc.UpdateLocation(ctx, UpdateLocationReq{
		Name:     req.LocationName,
		Position: position,
		NewName:  req.NewName,
		Extra:    req.Extra.AsMap(),
	})
	if err != nil {
		return nil, err
	}
	pbLoc, err := locationToProtobuf(loc)
	if err != nil {
		return nil, err
	}
	return &pb.UpdateLocationResponse{Location: pbLoc}, nil
}

func (server *serviceServer) DeleteLocation(ctx context.Context, req *pb.DeleteLocationRequest) (*pb.DeleteLocationResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	if err := svc.DeleteLocation(ctx, req.LocationName, req.Extra.AsMap()); err != nil {
		return nil, err
	}
	return &pb.DeleteLocationResponse{}, nil
}

func (server *serviceServer) AddRoute(ctx context.Context, req *pb.AddRouteRequest) (*pb.AddRouteResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	wps, err := protobufToWaypoints(req.Waypoints)
	if err != nil {
		return nil, err
	}
	route, err := svc.AddRoute(ctx, AddRouteReq{
		Name:      req.RouteName,
		Waypoints: wps,
		Extra:     req.Extra.AsMap(),
	})
	if err != nil {
		return nil, err
	}
	pbRoute, err := routeToProtobuf(route)
	if err != nil {
		return nil, err
	}
	return &pb.AddRouteResponse{Route: pbRoute}, nil
}

func (server *serviceServer) GetRoute(ctx context.Context, req *pb.GetRouteRequest) (*pb.GetRouteResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	route, err := svc.GetRoute(ctx, req.RouteName, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	pbRoute, err := routeToProtobuf(route)
	if err != nil {
		return nil, err
	}
	return &pb.GetRouteResponse{Route: pbRoute}, nil
}

func (server *serviceServer) ListRoutes(ctx context.Context, req *pb.ListRoutesRequest) (*pb.ListRoutesResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	routes, err := svc.ListRoutes(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	pbRoutes, err := routesToProtobuf(routes)
	if err != nil {
		return nil, err
	}
	return &pb.ListRoutesResponse{Routes: pbRoutes}, nil
}

func (server *serviceServer) UpdateRoute(ctx context.Context, req *pb.UpdateRouteRequest) (*pb.UpdateRouteResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	var wps []Waypoint
	if req.Waypoints != nil {
		wps, err = protobufToWaypoints(req.GetWaypoints().GetWaypoints())
		if err != nil {
			return nil, err
		}
	}
	route, err := svc.UpdateRoute(ctx, UpdateRouteReq{
		Name:      req.RouteName,
		NewName:   req.NewName,
		Waypoints: wps,
		Extra:     req.Extra.AsMap(),
	})
	if err != nil {
		return nil, err
	}
	pbRoute, err := routeToProtobuf(route)
	if err != nil {
		return nil, err
	}
	return &pb.UpdateRouteResponse{Route: pbRoute}, nil
}

func (server *serviceServer) DeleteRoute(ctx context.Context, req *pb.DeleteRouteRequest) (*pb.DeleteRouteResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	if err := svc.DeleteRoute(ctx, req.RouteName, req.Extra.AsMap()); err != nil {
		return nil, err
	}
	return &pb.DeleteRouteResponse{}, nil
}

func (server *serviceServer) AddZone(ctx context.Context, req *pb.AddZoneRequest) (*pb.AddZoneResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	zone, err := svc.AddZone(ctx, AddZoneReq{
		Name:     req.ZoneName,
		Type:     req.Type,
		Geometry: req.Geometry,
		MaxSpeed: req.MaxSpeed,
		Extra:    req.Extra.AsMap(),
	})
	if err != nil {
		return nil, err
	}
	return &pb.AddZoneResponse{Zone: zoneToProtobuf(zone)}, nil
}

func (server *serviceServer) GetZone(ctx context.Context, req *pb.GetZoneRequest) (*pb.GetZoneResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	zone, err := svc.GetZone(ctx, req.ZoneName, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetZoneResponse{Zone: zoneToProtobuf(zone)}, nil
}

func (server *serviceServer) ListZones(ctx context.Context, req *pb.ListZonesRequest) (*pb.ListZonesResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	zones, err := svc.ListZones(ctx, req.Type, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.ListZonesResponse{Zones: zonesToProtobuf(zones)}, nil
}

func (server *serviceServer) UpdateZone(ctx context.Context, req *pb.UpdateZoneRequest) (*pb.UpdateZoneResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	zone, err := svc.UpdateZone(ctx, UpdateZoneReq{
		Name:     req.ZoneName,
		Type:     req.Type,
		Geometry: req.Geometry,
		MaxSpeed: req.MaxSpeed,
		NewName:  req.NewName,
		Extra:    req.Extra.AsMap(),
	})
	if err != nil {
		return nil, err
	}
	return &pb.UpdateZoneResponse{Zone: zoneToProtobuf(zone)}, nil
}

func (server *serviceServer) DeleteZone(ctx context.Context, req *pb.DeleteZoneRequest) (*pb.DeleteZoneResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	if err := svc.DeleteZone(ctx, req.ZoneName, req.Extra.AsMap()); err != nil {
		return nil, err
	}
	return &pb.DeleteZoneResponse{}, nil
}

func (server *serviceServer) NavigateToLocation(
	ctx context.Context,
	req *pb.NavigateToLocationRequest,
) (*pb.NavigateToLocationResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	status, err := svc.NavigateToLocation(ctx, req.LocationName, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	pbStatus, err := statusToProtobuf(status)
	if err != nil {
		return nil, err
	}
	return &pb.NavigateToLocationResponse{Status: pbStatus}, nil
}

func (server *serviceServer) NavigateToPoint(
	ctx context.Context,
	req *pb.NavigateToPointRequest,
) (*pb.NavigateToPointResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	dest, err := protobufToPosition(req.Destination)
	if err != nil {
		return nil, err
	}
	status, err := svc.NavigateToPoint(ctx, dest, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	pbStatus, err := statusToProtobuf(status)
	if err != nil {
		return nil, err
	}
	return &pb.NavigateToPointResponse{Status: pbStatus}, nil
}

func (server *serviceServer) NavigateRoute(ctx context.Context, req *pb.NavigateRouteRequest) (*pb.NavigateRouteResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	wps, err := protobufToWaypoints(req.Waypoints)
	if err != nil {
		return nil, err
	}
	status, err := svc.NavigateRoute(ctx, NavigateRouteReq{
		Name:         req.RouteName,
		Waypoints:    wps,
		StartIndex:   req.StartIndex,
		StartNearest: req.StartNearest,
		Loop:         req.Loop,
		Wait:         req.Wait,
		Extra:        req.Extra.AsMap(),
	})
	if err != nil {
		return nil, err
	}
	pbStatus, err := statusToProtobuf(status)
	if err != nil {
		return nil, err
	}
	return &pb.NavigateRouteResponse{Status: pbStatus}, nil
}

func (server *serviceServer) PlanToPoint(ctx context.Context, req *pb.PlanToPointRequest) (*pb.PlanToPointResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	dest, err := protobufToPosition(req.Destination)
	if err != nil {
		return nil, err
	}
	var start *Position
	if req.Start != nil {
		s, err := protobufToPosition(req.Start)
		if err != nil {
			return nil, err
		}
		start = &s
	}
	plan, err := svc.PlanToPoint(ctx, PlanToPointReq{
		Destination: dest,
		Start:       start,
		Extra:       req.Extra.AsMap(),
	})
	if err != nil {
		return nil, err
	}
	pbPlan, err := planToProtobuf(plan)
	if err != nil {
		return nil, err
	}
	return &pb.PlanToPointResponse{Plan: pbPlan}, nil
}

func (server *serviceServer) PlanToLocation(
	ctx context.Context,
	req *pb.PlanToLocationRequest,
) (*pb.PlanToLocationResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	var start *Position
	if req.Start != nil {
		s, err := protobufToPosition(req.Start)
		if err != nil {
			return nil, err
		}
		start = &s
	}
	plan, err := svc.PlanToLocation(ctx, PlanToLocationReq{
		Name:  req.LocationName,
		Start: start,
		Extra: req.Extra.AsMap(),
	})
	if err != nil {
		return nil, err
	}
	pbPlan, err := planToProtobuf(plan)
	if err != nil {
		return nil, err
	}
	return &pb.PlanToLocationResponse{Plan: pbPlan}, nil
}

func (server *serviceServer) ExecutePlan(ctx context.Context, req *pb.ExecutePlanRequest) (*pb.ExecutePlanResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	plan, err := protobufToPlan(req.Plan)
	if err != nil {
		return nil, err
	}
	status, err := svc.ExecutePlan(ctx, plan, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	pbStatus, err := statusToProtobuf(status)
	if err != nil {
		return nil, err
	}
	return &pb.ExecutePlanResponse{Status: pbStatus}, nil
}

func (server *serviceServer) GoToLocation(ctx context.Context, req *pb.GoToLocationRequest) (*pb.GoToLocationResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	status, err := svc.GoToLocation(ctx, GoToReq{
		Name:          req.LocationName,
		Wait:          req.Wait,
		LinearMPerSec: req.LinearMPerSec,
		Extra:         req.Extra.AsMap(),
	})
	if err != nil {
		return nil, err
	}
	pbStatus, err := statusToProtobuf(status)
	if err != nil {
		return nil, err
	}
	return &pb.GoToLocationResponse{Status: pbStatus}, nil
}

func (server *serviceServer) GoToPoint(ctx context.Context, req *pb.GoToPointRequest) (*pb.GoToPointResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	dest, err := protobufToPosition(req.Destination)
	if err != nil {
		return nil, err
	}
	status, err := svc.GoToPoint(ctx, GoToReq{
		Destination:   &dest,
		Wait:          req.Wait,
		LinearMPerSec: req.LinearMPerSec,
		Extra:         req.Extra.AsMap(),
	})
	if err != nil {
		return nil, err
	}
	pbStatus, err := statusToProtobuf(status)
	if err != nil {
		return nil, err
	}
	return &pb.GoToPointResponse{Status: pbStatus}, nil
}

func (server *serviceServer) Suspend(ctx context.Context, req *pb.SuspendRequest) (*pb.SuspendResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	status, err := svc.Suspend(ctx, req.Reason, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	pbStatus, err := statusToProtobuf(status)
	if err != nil {
		return nil, err
	}
	return &pb.SuspendResponse{Status: pbStatus}, nil
}

func (server *serviceServer) Resume(ctx context.Context, req *pb.ResumeRequest) (*pb.ResumeResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	status, err := svc.Resume(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	pbStatus, err := statusToProtobuf(status)
	if err != nil {
		return nil, err
	}
	return &pb.ResumeResponse{Status: pbStatus}, nil
}

func (server *serviceServer) Cancel(ctx context.Context, req *pb.CancelRequest) (*pb.CancelResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	status, err := svc.Cancel(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	pbStatus, err := statusToProtobuf(status)
	if err != nil {
		return nil, err
	}
	return &pb.CancelResponse{Status: pbStatus}, nil
}

func (server *serviceServer) GetPosition(ctx context.Context, req *pb.GetPositionRequest) (*pb.GetPositionResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := svc.GetPosition(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	pbPos, err := positionToProtobuf(pos)
	if err != nil {
		return nil, err
	}
	return &pb.GetPositionResponse{Position: pbPos}, nil
}

func (server *serviceServer) GetNavigationStatus(
	ctx context.Context,
	req *pb.GetNavigationStatusRequest,
) (*pb.GetNavigationStatusResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	status, err := svc.GetNavigationStatus(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	pbStatus, err := statusToProtobuf(status)
	if err != nil {
		return nil, err
	}
	return &pb.GetNavigationStatusResponse{Status: pbStatus}, nil
}

func (server *serviceServer) GetStatus(ctx context.Context, req *commonpb.GetStatusRequest) (*commonpb.GetStatusResponse, error) {
	res, err := server.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.GetStatusFromResourceServer(ctx, res, req)
}

func (server *serviceServer) GetProperties(ctx context.Context, req *pb.GetPropertiesRequest) (*pb.GetPropertiesResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	props, err := svc.GetProperties(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetPropertiesResponse{Properties: propertiesToProtobuf(props)}, nil
}

func (server *serviceServer) GetCostmap(ctx context.Context, req *pb.GetCostmapRequest) (*pb.GetCostmapResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	cm, err := svc.GetCostmap(ctx, CostmapReq{
		Layer:  req.Layer,
		Stride: req.Stride,
		Extra:  req.Extra.AsMap(),
	})
	if err != nil {
		return nil, err
	}
	return costmapToProtobuf(cm), nil
}

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
