package navigation

import (
	"context"

	"github.com/pkg/errors"
	pb "go.viam.com/api/service/navigation/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.NavigationServiceClient
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Service, error) {
	grpcClient := pb.NewNavigationServiceClient(conn)
	c := &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.Name,
		client: grpcClient,
		logger: logger,
	}
	return c, nil
}

func (c *client) AddLocation(ctx context.Context, req AddLocationReq) (Location, error) {
	ext, err := protoutils.StructToStructPb(req.Extra)
	if err != nil {
		return Location{}, err
	}
	pos, err := optionalPositionToProtobuf(req.Position)
	if err != nil {
		return Location{}, err
	}
	resp, err := c.client.AddLocation(ctx, &pb.AddLocationRequest{
		Name:         c.name,
		LocationName: req.Name,
		Position:     pos,
		Extra:        ext,
	})
	if err != nil {
		return Location{}, err
	}
	return protobufToLocation(resp.GetLocation())
}

func (c *client) GetLocation(ctx context.Context, name string, extra map[string]interface{}) (Location, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return Location{}, err
	}
	resp, err := c.client.GetLocation(ctx, &pb.GetLocationRequest{
		Name:         c.name,
		LocationName: name,
		Extra:        ext,
	})
	if err != nil {
		return Location{}, err
	}
	return protobufToLocation(resp.GetLocation())
}

func (c *client) ListLocations(ctx context.Context, extra map[string]interface{}) ([]Location, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.ListLocations(ctx, &pb.ListLocationsRequest{Name: c.name, Extra: ext})
	if err != nil {
		return nil, err
	}
	return protobufToLocations(resp.GetLocations())
}

func (c *client) UpdateLocation(ctx context.Context, req UpdateLocationReq) (Location, error) {
	ext, err := protoutils.StructToStructPb(req.Extra)
	if err != nil {
		return Location{}, err
	}
	pos, err := optionalPositionToProtobuf(req.Position)
	if err != nil {
		return Location{}, err
	}
	resp, err := c.client.UpdateLocation(ctx, &pb.UpdateLocationRequest{
		Name:         c.name,
		LocationName: req.Name,
		Position:     pos,
		NewName:      req.NewName,
		Extra:        ext,
	})
	if err != nil {
		return Location{}, err
	}
	return protobufToLocation(resp.GetLocation())
}

func (c *client) DeleteLocation(ctx context.Context, name string, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.DeleteLocation(ctx, &pb.DeleteLocationRequest{
		Name:         c.name,
		LocationName: name,
		Extra:        ext,
	})
	return err
}

func (c *client) AddRoute(ctx context.Context, req AddRouteReq) (Route, error) {
	ext, err := protoutils.StructToStructPb(req.Extra)
	if err != nil {
		return Route{}, err
	}
	wps, err := waypointsToProtobuf(req.Waypoints)
	if err != nil {
		return Route{}, err
	}
	resp, err := c.client.AddRoute(ctx, &pb.AddRouteRequest{
		Name:      c.name,
		RouteName: req.Name,
		Waypoints: wps,
		Extra:     ext,
	})
	if err != nil {
		return Route{}, err
	}
	return protobufToRoute(resp.GetRoute())
}

func (c *client) GetRoute(ctx context.Context, name string, extra map[string]interface{}) (Route, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return Route{}, err
	}
	resp, err := c.client.GetRoute(ctx, &pb.GetRouteRequest{
		Name:      c.name,
		RouteName: name,
		Extra:     ext,
	})
	if err != nil {
		return Route{}, err
	}
	return protobufToRoute(resp.GetRoute())
}

func (c *client) ListRoutes(ctx context.Context, extra map[string]interface{}) ([]Route, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.ListRoutes(ctx, &pb.ListRoutesRequest{Name: c.name, Extra: ext})
	if err != nil {
		return nil, err
	}
	return protobufToRoutes(resp.GetRoutes())
}

func (c *client) UpdateRoute(ctx context.Context, req UpdateRouteReq) (Route, error) {
	ext, err := protoutils.StructToStructPb(req.Extra)
	if err != nil {
		return Route{}, err
	}
	var wps *pb.WaypointList
	if req.Waypoints != nil {
		converted, convErr := waypointsToProtobuf(req.Waypoints)
		if convErr != nil {
			return Route{}, convErr
		}
		wps = &pb.WaypointList{Waypoints: converted}
	}
	resp, err := c.client.UpdateRoute(ctx, &pb.UpdateRouteRequest{
		Name:      c.name,
		RouteName: req.Name,
		NewName:   req.NewName,
		Waypoints: wps,
		Extra:     ext,
	})
	if err != nil {
		return Route{}, err
	}
	return protobufToRoute(resp.GetRoute())
}

func (c *client) DeleteRoute(ctx context.Context, name string, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.DeleteRoute(ctx, &pb.DeleteRouteRequest{
		Name:      c.name,
		RouteName: name,
		Extra:     ext,
	})
	return err
}

func (c *client) AddZone(ctx context.Context, req AddZoneReq) (Zone, error) {
	ext, err := protoutils.StructToStructPb(req.Extra)
	if err != nil {
		return Zone{}, err
	}
	resp, err := c.client.AddZone(ctx, &pb.AddZoneRequest{
		Name:     c.name,
		ZoneName: req.Name,
		Type:     req.Type,
		Geometry: req.Geometry,
		MaxSpeed: req.MaxSpeed,
		Extra:    ext,
	})
	if err != nil {
		return Zone{}, err
	}
	return protobufToZone(resp.GetZone())
}

func (c *client) GetZone(ctx context.Context, name string, extra map[string]interface{}) (Zone, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return Zone{}, err
	}
	resp, err := c.client.GetZone(ctx, &pb.GetZoneRequest{
		Name:     c.name,
		ZoneName: name,
		Extra:    ext,
	})
	if err != nil {
		return Zone{}, err
	}
	return protobufToZone(resp.GetZone())
}

func (c *client) ListZones(ctx context.Context, zoneType string, extra map[string]interface{}) ([]Zone, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.ListZones(ctx, &pb.ListZonesRequest{
		Name:  c.name,
		Type:  zoneType,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return protobufToZones(resp.GetZones())
}

func (c *client) UpdateZone(ctx context.Context, req UpdateZoneReq) (Zone, error) {
	ext, err := protoutils.StructToStructPb(req.Extra)
	if err != nil {
		return Zone{}, err
	}
	resp, err := c.client.UpdateZone(ctx, &pb.UpdateZoneRequest{
		Name:     c.name,
		ZoneName: req.Name,
		Type:     req.Type,
		Geometry: req.Geometry,
		MaxSpeed: req.MaxSpeed,
		NewName:  req.NewName,
		Extra:    ext,
	})
	if err != nil {
		return Zone{}, err
	}
	return protobufToZone(resp.GetZone())
}

func (c *client) DeleteZone(ctx context.Context, name string, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.DeleteZone(ctx, &pb.DeleteZoneRequest{
		Name:     c.name,
		ZoneName: name,
		Extra:    ext,
	})
	return err
}

func (c *client) NavigateToLocation(ctx context.Context, name string, extra map[string]interface{}) (Status, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return Status{}, err
	}
	resp, err := c.client.NavigateToLocation(ctx, &pb.NavigateToLocationRequest{
		Name:         c.name,
		LocationName: name,
		Extra:        ext,
	})
	if err != nil {
		return Status{}, err
	}
	return protobufToStatus(resp.GetStatus())
}

func (c *client) NavigateToPoint(ctx context.Context, destination Position, extra map[string]interface{}) (Status, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return Status{}, err
	}
	dest, err := positionToProtobuf(destination)
	if err != nil {
		return Status{}, err
	}
	resp, err := c.client.NavigateToPoint(ctx, &pb.NavigateToPointRequest{
		Name:        c.name,
		Destination: dest,
		Extra:       ext,
	})
	if err != nil {
		return Status{}, err
	}
	return protobufToStatus(resp.GetStatus())
}

func (c *client) NavigateRoute(ctx context.Context, req NavigateRouteReq) (Status, error) {
	ext, err := protoutils.StructToStructPb(req.Extra)
	if err != nil {
		return Status{}, err
	}
	wps, err := waypointsToProtobuf(req.Waypoints)
	if err != nil {
		return Status{}, err
	}
	resp, err := c.client.NavigateRoute(ctx, &pb.NavigateRouteRequest{
		Name:         c.name,
		RouteName:    req.Name,
		Waypoints:    wps,
		StartIndex:   req.StartIndex,
		StartNearest: req.StartNearest,
		Loop:         req.Loop,
		Wait:         req.Wait,
		Extra:        ext,
	})
	if err != nil {
		return Status{}, err
	}
	return protobufToStatus(resp.GetStatus())
}

func (c *client) PlanToPoint(ctx context.Context, req PlanToPointReq) (Plan, error) {
	ext, err := protoutils.StructToStructPb(req.Extra)
	if err != nil {
		return Plan{}, err
	}
	dest, err := positionToProtobuf(req.Destination)
	if err != nil {
		return Plan{}, err
	}
	start, err := optionalPositionToProtobuf(req.Start)
	if err != nil {
		return Plan{}, err
	}
	resp, err := c.client.PlanToPoint(ctx, &pb.PlanToPointRequest{
		Name:        c.name,
		Destination: dest,
		Start:       start,
		Extra:       ext,
	})
	if err != nil {
		return Plan{}, err
	}
	return protobufToPlan(resp.GetPlan())
}

func (c *client) PlanToLocation(ctx context.Context, req PlanToLocationReq) (Plan, error) {
	ext, err := protoutils.StructToStructPb(req.Extra)
	if err != nil {
		return Plan{}, err
	}
	start, err := optionalPositionToProtobuf(req.Start)
	if err != nil {
		return Plan{}, err
	}
	resp, err := c.client.PlanToLocation(ctx, &pb.PlanToLocationRequest{
		Name:         c.name,
		LocationName: req.Name,
		Start:        start,
		Extra:        ext,
	})
	if err != nil {
		return Plan{}, err
	}
	return protobufToPlan(resp.GetPlan())
}

func (c *client) ExecutePlan(ctx context.Context, plan Plan, extra map[string]interface{}) (Status, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return Status{}, err
	}
	pbPlan, err := planToProtobuf(plan)
	if err != nil {
		return Status{}, err
	}
	resp, err := c.client.ExecutePlan(ctx, &pb.ExecutePlanRequest{
		Name:  c.name,
		Plan:  pbPlan,
		Extra: ext,
	})
	if err != nil {
		return Status{}, err
	}
	return protobufToStatus(resp.GetStatus())
}

func (c *client) GoToLocation(ctx context.Context, req GoToReq) (Status, error) {
	ext, err := protoutils.StructToStructPb(req.Extra)
	if err != nil {
		return Status{}, err
	}
	resp, err := c.client.GoToLocation(ctx, &pb.GoToLocationRequest{
		Name:          c.name,
		LocationName:  req.Name,
		Wait:          req.Wait,
		LinearMPerSec: req.LinearMPerSec,
		Extra:         ext,
	})
	if err != nil {
		return Status{}, err
	}
	return protobufToStatus(resp.GetStatus())
}

func (c *client) GoToPoint(ctx context.Context, req GoToReq) (Status, error) {
	ext, err := protoutils.StructToStructPb(req.Extra)
	if err != nil {
		return Status{}, err
	}
	if req.Destination == nil {
		return Status{}, errors.New("destination is required")
	}
	dest, err := positionToProtobuf(*req.Destination)
	if err != nil {
		return Status{}, err
	}
	resp, err := c.client.GoToPoint(ctx, &pb.GoToPointRequest{
		Name:          c.name,
		Destination:   dest,
		Wait:          req.Wait,
		LinearMPerSec: req.LinearMPerSec,
		Extra:         ext,
	})
	if err != nil {
		return Status{}, err
	}
	return protobufToStatus(resp.GetStatus())
}

func (c *client) Suspend(ctx context.Context, reason string, extra map[string]interface{}) (Status, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return Status{}, err
	}
	resp, err := c.client.Suspend(ctx, &pb.SuspendRequest{
		Name:   c.name,
		Reason: reason,
		Extra:  ext,
	})
	if err != nil {
		return Status{}, err
	}
	return protobufToStatus(resp.GetStatus())
}

func (c *client) Resume(ctx context.Context, extra map[string]interface{}) (Status, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return Status{}, err
	}
	resp, err := c.client.Resume(ctx, &pb.ResumeRequest{Name: c.name, Extra: ext})
	if err != nil {
		return Status{}, err
	}
	return protobufToStatus(resp.GetStatus())
}

func (c *client) Cancel(ctx context.Context, extra map[string]interface{}) (Status, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return Status{}, err
	}
	resp, err := c.client.Cancel(ctx, &pb.CancelRequest{Name: c.name, Extra: ext})
	if err != nil {
		return Status{}, err
	}
	return protobufToStatus(resp.GetStatus())
}

func (c *client) GetPosition(ctx context.Context, extra map[string]interface{}) (Position, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return Position{}, err
	}
	resp, err := c.client.GetPosition(ctx, &pb.GetPositionRequest{Name: c.name, Extra: ext})
	if err != nil {
		return Position{}, err
	}
	return protobufToPosition(resp.GetPosition())
}

func (c *client) GetNavigationStatus(ctx context.Context, extra map[string]interface{}) (Status, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return Status{}, err
	}
	resp, err := c.client.GetNavigationStatus(ctx, &pb.GetNavigationStatusRequest{Name: c.name, Extra: ext})
	if err != nil {
		return Status{}, err
	}
	return protobufToStatus(resp.GetStatus())
}

func (c *client) GetProperties(ctx context.Context) (Properties, error) {
	resp, err := c.client.GetProperties(ctx, &pb.GetPropertiesRequest{Name: c.name})
	if err != nil {
		return Properties{}, errors.Wrapf(err, "failure to get properties")
	}
	return protobufToProperties(resp.GetProperties())
}

func (c *client) GetCostmap(ctx context.Context, req CostmapReq) (Costmap, error) {
	ext, err := protoutils.StructToStructPb(req.Extra)
	if err != nil {
		return Costmap{}, err
	}
	resp, err := c.client.GetCostmap(ctx, &pb.GetCostmapRequest{
		Name:   c.name,
		Layer:  req.Layer,
		Stride: req.Stride,
		Extra:  ext,
	})
	if err != nil {
		return Costmap{}, err
	}
	return protobufToCostmap(resp), nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

func (c *client) Status(ctx context.Context) (map[string]interface{}, error) {
	return rprotoutils.GetStatusFromResourceClient(ctx, c.client, c.name)
}
