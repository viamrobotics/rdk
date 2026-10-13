package navigation

import (
	"time"

	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/navigation/v1"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/spatialmath"
)

func mapTypeToProtobuf(mapType MapType) pb.MapType {
	switch mapType {
	case LocalMap:
		return pb.MapType_MAP_TYPE_LOCAL
	case GPSMap:
		return pb.MapType_MAP_TYPE_GPS
	default:
		return pb.MapType_MAP_TYPE_UNSPECIFIED
	}
}

func protobufToMapType(mapType pb.MapType) (MapType, error) {
	switch mapType {
	case pb.MapType_MAP_TYPE_LOCAL:
		return LocalMap, nil
	case pb.MapType_MAP_TYPE_GPS:
		return GPSMap, nil
	case pb.MapType_MAP_TYPE_UNSPECIFIED:
		fallthrough
	default:
		return 0, errors.New("map type unspecified")
	}
}

func goalStateToProtobuf(state GoalState) pb.GoalState {
	switch state {
	case GoalStateActive:
		return pb.GoalState_GOAL_STATE_ACTIVE
	case GoalStateSuspended:
		return pb.GoalState_GOAL_STATE_SUSPENDED
	case GoalStateSucceeded:
		return pb.GoalState_GOAL_STATE_SUCCEEDED
	case GoalStateFailed:
		return pb.GoalState_GOAL_STATE_FAILED
	case GoalStateCanceled:
		return pb.GoalState_GOAL_STATE_CANCELED
	default:
		return pb.GoalState_GOAL_STATE_UNSPECIFIED
	}
}

func protobufToGoalState(state pb.GoalState) GoalState {
	switch state {
	case pb.GoalState_GOAL_STATE_ACTIVE:
		return GoalStateActive
	case pb.GoalState_GOAL_STATE_SUSPENDED:
		return GoalStateSuspended
	case pb.GoalState_GOAL_STATE_SUCCEEDED:
		return GoalStateSucceeded
	case pb.GoalState_GOAL_STATE_FAILED:
		return GoalStateFailed
	case pb.GoalState_GOAL_STATE_CANCELED:
		return GoalStateCanceled
	default:
		return GoalStateUnspecified
	}
}

func navModeToProtobuf(mode NavMode) pb.NavMode {
	switch mode {
	case NavModePlanned:
		return pb.NavMode_NAV_MODE_PLANNED
	case NavModeDirect:
		return pb.NavMode_NAV_MODE_DIRECT
	case NavModeRoute:
		return pb.NavMode_NAV_MODE_ROUTE
	default:
		return pb.NavMode_NAV_MODE_UNSPECIFIED
	}
}

func protobufToNavMode(mode pb.NavMode) NavMode {
	switch mode {
	case pb.NavMode_NAV_MODE_PLANNED:
		return NavModePlanned
	case pb.NavMode_NAV_MODE_DIRECT:
		return NavModeDirect
	case pb.NavMode_NAV_MODE_ROUTE:
		return NavModeRoute
	default:
		return NavModeUnspecified
	}
}

func navErrorCodeToProtobuf(code NavErrorCode) pb.NavErrorCode {
	switch code {
	case NavErrorCodeNoPath:
		return pb.NavErrorCode_NAV_ERROR_CODE_NO_PATH
	case NavErrorCodeBlocked:
		return pb.NavErrorCode_NAV_ERROR_CODE_BLOCKED
	case NavErrorCodeLocalizationLost:
		return pb.NavErrorCode_NAV_ERROR_CODE_LOCALIZATION_LOST
	case NavErrorCodeBaseError:
		return pb.NavErrorCode_NAV_ERROR_CODE_BASE_ERROR
	case NavErrorCodeTimeout:
		return pb.NavErrorCode_NAV_ERROR_CODE_TIMEOUT
	case NavErrorCodeOther:
		return pb.NavErrorCode_NAV_ERROR_CODE_OTHER
	default:
		return pb.NavErrorCode_NAV_ERROR_CODE_UNSPECIFIED
	}
}

func protobufToNavErrorCode(code pb.NavErrorCode) NavErrorCode {
	switch code {
	case pb.NavErrorCode_NAV_ERROR_CODE_NO_PATH:
		return NavErrorCodeNoPath
	case pb.NavErrorCode_NAV_ERROR_CODE_BLOCKED:
		return NavErrorCodeBlocked
	case pb.NavErrorCode_NAV_ERROR_CODE_LOCALIZATION_LOST:
		return NavErrorCodeLocalizationLost
	case pb.NavErrorCode_NAV_ERROR_CODE_BASE_ERROR:
		return NavErrorCodeBaseError
	case pb.NavErrorCode_NAV_ERROR_CODE_TIMEOUT:
		return NavErrorCodeTimeout
	case pb.NavErrorCode_NAV_ERROR_CODE_OTHER:
		return NavErrorCodeOther
	default:
		return NavErrorCodeUnspecified
	}
}

func geoPoseToProtobuf(gp *GeoPose) (*commonpb.GeoPose, error) {
	if gp == nil || gp.Location == nil {
		return nil, errors.New("geo_pose location is required")
	}
	out := &commonpb.GeoPose{
		Location: &commonpb.GeoPoint{
			Latitude:  gp.Location.Lat(),
			Longitude: gp.Location.Lng(),
		},
	}
	if gp.Altitude != nil {
		alt := *gp.Altitude
		out.Altitude = &alt
	}
	switch {
	case gp.Heading != nil && gp.Orientation != nil:
		return nil, errors.New("geo_pose cannot set both heading and orientation")
	case gp.Heading != nil:
		out.Orientation = &commonpb.GeoPose_Heading{Heading: *gp.Heading}
	case gp.Orientation != nil:
		out.Orientation = &commonpb.GeoPose_OrientationVector{OrientationVector: &commonpb.Orientation{
			OX:    gp.Orientation.OX,
			OY:    gp.Orientation.OY,
			OZ:    gp.Orientation.OZ,
			Theta: gp.Orientation.Theta,
		}}
	default:
		return nil, errors.New("geo_pose must set heading or orientation")
	}
	return out, nil
}

func protobufToGeoPose(gp *commonpb.GeoPose) (*GeoPose, error) {
	if gp == nil || gp.GetLocation() == nil {
		return nil, errors.New("geo_pose location is required")
	}
	out := &GeoPose{
		Location: geo.NewPoint(gp.GetLocation().GetLatitude(), gp.GetLocation().GetLongitude()),
	}
	if gp.Altitude != nil {
		alt := gp.GetAltitude()
		out.Altitude = &alt
	}
	switch gp.GetOrientation().(type) {
	case *commonpb.GeoPose_Heading:
		heading := gp.GetHeading()
		out.Heading = &heading
	case *commonpb.GeoPose_OrientationVector:
		ov := gp.GetOrientationVector()
		if ov == nil {
			return nil, errors.New("geo_pose orientation_vector is required")
		}
		out.Orientation = &spatialmath.OrientationVectorDegrees{
			OX: ov.GetOX(), OY: ov.GetOY(), OZ: ov.GetOZ(), Theta: ov.GetTheta(),
		}
	default:
		return nil, errors.New("geo_pose must set heading or orientation")
	}
	return out, nil
}

func positionToProtobuf(p Position) (*pb.Position, error) {
	switch {
	case p.Pose != nil && p.GeoPose != nil:
		return nil, errors.New("position cannot set both pose and geo_pose")
	case p.Pose != nil:
		return &pb.Position{Value: &pb.Position_Pose{Pose: spatialmath.PoseToProtobuf(p.Pose)}}, nil
	case p.GeoPose != nil:
		gp, err := geoPoseToProtobuf(p.GeoPose)
		if err != nil {
			return nil, err
		}
		return &pb.Position{Value: &pb.Position_GeoPose{GeoPose: gp}}, nil
	default:
		return nil, errors.New("position must set pose or geo_pose")
	}
}

func protobufToPosition(p *pb.Position) (Position, error) {
	if p == nil {
		return Position{}, errors.New("position is required")
	}
	switch v := p.GetValue().(type) {
	case *pb.Position_Pose:
		return Position{Pose: spatialmath.NewPoseFromProtobuf(v.Pose)}, nil
	case *pb.Position_GeoPose:
		gp, err := protobufToGeoPose(v.GeoPose)
		if err != nil {
			return Position{}, err
		}
		return Position{GeoPose: gp}, nil
	default:
		return Position{}, errors.New("position must set pose or geo_pose")
	}
}

func optionalPositionToProtobuf(p *Position) (*pb.Position, error) {
	if p == nil {
		return nil, nil
	}
	return positionToProtobuf(*p)
}

func positionsToProtobuf(ps []Position) ([]*pb.Position, error) {
	out := make([]*pb.Position, 0, len(ps))
	for _, p := range ps {
		pbPos, err := positionToProtobuf(p)
		if err != nil {
			return nil, err
		}
		out = append(out, pbPos)
	}
	return out, nil
}

func protobufToPositions(ps []*pb.Position) ([]Position, error) {
	out := make([]Position, 0, len(ps))
	for _, p := range ps {
		pos, err := protobufToPosition(p)
		if err != nil {
			return nil, err
		}
		out = append(out, pos)
	}
	return out, nil
}

func locationToProtobuf(loc Location) (*pb.Location, error) {
	pos, err := positionToProtobuf(loc.Position)
	if err != nil {
		return nil, err
	}
	return &pb.Location{Name: loc.Name, Position: pos}, nil
}

func protobufToLocation(loc *pb.Location) (Location, error) {
	if loc == nil {
		return Location{}, errors.New("location is required")
	}
	pos, err := protobufToPosition(loc.GetPosition())
	if err != nil {
		return Location{}, err
	}
	return Location{Name: loc.GetName(), Position: pos}, nil
}

func locationsToProtobuf(locs []Location) ([]*pb.Location, error) {
	out := make([]*pb.Location, 0, len(locs))
	for _, loc := range locs {
		pbLoc, err := locationToProtobuf(loc)
		if err != nil {
			return nil, err
		}
		out = append(out, pbLoc)
	}
	return out, nil
}

func protobufToLocations(locs []*pb.Location) ([]Location, error) {
	out := make([]Location, 0, len(locs))
	for _, loc := range locs {
		l, err := protobufToLocation(loc)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

func waypointToProtobuf(wp Waypoint) (*pb.Waypoint, error) {
	switch {
	case wp.LocationName != "" && wp.Position != nil:
		return nil, errors.New("waypoint cannot set both location and position")
	case wp.LocationName != "":
		return &pb.Waypoint{Value: &pb.Waypoint_Location{Location: wp.LocationName}}, nil
	case wp.Position != nil:
		pos, err := positionToProtobuf(*wp.Position)
		if err != nil {
			return nil, err
		}
		return &pb.Waypoint{Value: &pb.Waypoint_Position{Position: pos}}, nil
	default:
		return nil, errors.New("waypoint must set location or position")
	}
}

func protobufToWaypoint(wp *pb.Waypoint) (Waypoint, error) {
	if wp == nil {
		return Waypoint{}, errors.New("waypoint is required")
	}
	switch v := wp.GetValue().(type) {
	case *pb.Waypoint_Location:
		return Waypoint{LocationName: v.Location}, nil
	case *pb.Waypoint_Position:
		pos, err := protobufToPosition(v.Position)
		if err != nil {
			return Waypoint{}, err
		}
		return Waypoint{Position: &pos}, nil
	default:
		return Waypoint{}, errors.New("waypoint must set location or position")
	}
}

func waypointsToProtobuf(wps []Waypoint) ([]*pb.Waypoint, error) {
	out := make([]*pb.Waypoint, 0, len(wps))
	for _, wp := range wps {
		pbWp, err := waypointToProtobuf(wp)
		if err != nil {
			return nil, err
		}
		out = append(out, pbWp)
	}
	return out, nil
}

func protobufToWaypoints(wps []*pb.Waypoint) ([]Waypoint, error) {
	out := make([]Waypoint, 0, len(wps))
	for _, wp := range wps {
		w, err := protobufToWaypoint(wp)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

func routeToProtobuf(route Route) (*pb.Route, error) {
	wps, err := waypointsToProtobuf(route.Waypoints)
	if err != nil {
		return nil, err
	}
	return &pb.Route{Name: route.Name, Waypoints: wps}, nil
}

func protobufToRoute(route *pb.Route) (Route, error) {
	if route == nil {
		return Route{}, errors.New("route is required")
	}
	wps, err := protobufToWaypoints(route.GetWaypoints())
	if err != nil {
		return Route{}, err
	}
	return Route{Name: route.GetName(), Waypoints: wps}, nil
}

func routesToProtobuf(routes []Route) ([]*pb.Route, error) {
	out := make([]*pb.Route, 0, len(routes))
	for _, route := range routes {
		pbRoute, err := routeToProtobuf(route)
		if err != nil {
			return nil, err
		}
		out = append(out, pbRoute)
	}
	return out, nil
}

func protobufToRoutes(routes []*pb.Route) ([]Route, error) {
	out := make([]Route, 0, len(routes))
	for _, route := range routes {
		r, err := protobufToRoute(route)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func zoneToProtobuf(zone Zone) *pb.Zone {
	return &pb.Zone{
		Name:     zone.Name,
		Type:     zone.Type,
		Geometry: zone.Geometry,
		MaxSpeed: zone.MaxSpeed,
	}
}

func protobufToZone(zone *pb.Zone) (Zone, error) {
	if zone == nil {
		return Zone{}, errors.New("zone is required")
	}
	return Zone{
		Name:     zone.GetName(),
		Type:     zone.GetType(),
		Geometry: zone.GetGeometry(),
		MaxSpeed: zone.MaxSpeed,
	}, nil
}

func zonesToProtobuf(zones []Zone) []*pb.Zone {
	out := make([]*pb.Zone, 0, len(zones))
	for _, zone := range zones {
		out = append(out, zoneToProtobuf(zone))
	}
	return out
}

func protobufToZones(zones []*pb.Zone) ([]Zone, error) {
	out := make([]Zone, 0, len(zones))
	for _, zone := range zones {
		z, err := protobufToZone(zone)
		if err != nil {
			return nil, err
		}
		out = append(out, z)
	}
	return out, nil
}

func planToProtobuf(plan Plan) (*pb.Plan, error) {
	path, err := positionsToProtobuf(plan.Path)
	if err != nil {
		return nil, err
	}
	return &pb.Plan{
		Path:     path,
		Location: plan.Location,
		LengthM:  plan.LengthM,
	}, nil
}

func protobufToPlan(plan *pb.Plan) (Plan, error) {
	if plan == nil {
		return Plan{}, errors.New("plan is required")
	}
	path, err := protobufToPositions(plan.GetPath())
	if err != nil {
		return Plan{}, err
	}
	out := Plan{Path: path, LengthM: plan.GetLengthM()}
	if plan.Location != nil {
		loc := plan.GetLocation()
		out.Location = &loc
	}
	return out, nil
}

func progressToProtobuf(p *Progress) (*pb.Progress, error) {
	if p == nil {
		return nil, nil
	}
	path, err := positionsToProtobuf(p.Path)
	if err != nil {
		return nil, err
	}
	return &pb.Progress{
		DistanceRemainingM: p.DistanceRemainingM,
		Path:               path,
		LengthM:            p.LengthM,
	}, nil
}

func protobufToProgress(p *pb.Progress) (*Progress, error) {
	if p == nil {
		return nil, nil
	}
	path, err := protobufToPositions(p.GetPath())
	if err != nil {
		return nil, err
	}
	return &Progress{
		DistanceRemainingM: p.GetDistanceRemainingM(),
		Path:               path,
		LengthM:            p.GetLengthM(),
	}, nil
}

func routeProgressToProtobuf(rp *RouteProgress) (*pb.RouteProgress, error) {
	if rp == nil {
		return nil, nil
	}
	wps, err := waypointsToProtobuf(rp.Waypoints)
	if err != nil {
		return nil, err
	}
	return &pb.RouteProgress{
		Name:      rp.Name,
		Waypoints: wps,
		Index:     rp.Index,
		Loop:      rp.Loop,
		Lap:       rp.Lap,
	}, nil
}

func protobufToRouteProgress(rp *pb.RouteProgress) (*RouteProgress, error) {
	if rp == nil {
		return nil, nil
	}
	wps, err := protobufToWaypoints(rp.GetWaypoints())
	if err != nil {
		return nil, err
	}
	return &RouteProgress{
		Name:      rp.GetName(),
		Waypoints: wps,
		Index:     rp.GetIndex(),
		Loop:      rp.GetLoop(),
		Lap:       rp.GetLap(),
	}, nil
}

func navErrorToProtobuf(err *NavError) *pb.NavError {
	if err == nil {
		return nil
	}
	return &pb.NavError{
		Code:    navErrorCodeToProtobuf(err.Code),
		Message: err.Message,
	}
}

func protobufToNavError(err *pb.NavError) *NavError {
	if err == nil {
		return nil
	}
	return &NavError{
		Code:    protobufToNavErrorCode(err.GetCode()),
		Message: err.GetMessage(),
	}
}

func goalStatusToProtobuf(gs *GoalStatus) (*pb.GoalStatus, error) {
	if gs == nil {
		return nil, nil
	}
	target, err := positionToProtobuf(gs.Target)
	if err != nil {
		return nil, err
	}
	progress, err := progressToProtobuf(gs.Progress)
	if err != nil {
		return nil, err
	}
	route, err := routeProgressToProtobuf(gs.Route)
	if err != nil {
		return nil, err
	}
	out := &pb.GoalStatus{
		State:         goalStateToProtobuf(gs.State),
		Mode:          navModeToProtobuf(gs.Mode),
		Target:        target,
		Location:      gs.Location,
		Progress:      progress,
		Route:         route,
		SuspendReason: gs.SuspendReason,
		Error:         navErrorToProtobuf(gs.Error),
		StartedAt:     timestamppb.New(gs.StartedAt),
	}
	if gs.FinishedAt != nil {
		out.FinishedAt = timestamppb.New(*gs.FinishedAt)
	}
	return out, nil
}

func protobufToGoalStatus(gs *pb.GoalStatus) (*GoalStatus, error) {
	if gs == nil {
		return nil, nil
	}
	target, err := protobufToPosition(gs.GetTarget())
	if err != nil {
		return nil, err
	}
	progress, err := protobufToProgress(gs.GetProgress())
	if err != nil {
		return nil, err
	}
	route, err := protobufToRouteProgress(gs.GetRoute())
	if err != nil {
		return nil, err
	}
	out := &GoalStatus{
		State:     protobufToGoalState(gs.GetState()),
		Mode:      protobufToNavMode(gs.GetMode()),
		Target:    target,
		Progress:  progress,
		Route:     route,
		Error:     protobufToNavError(gs.GetError()),
		StartedAt: gs.GetStartedAt().AsTime(),
	}
	if gs.Location != nil {
		loc := gs.GetLocation()
		out.Location = &loc
	}
	if gs.SuspendReason != nil {
		reason := gs.GetSuspendReason()
		out.SuspendReason = &reason
	}
	if gs.FinishedAt != nil {
		t := gs.GetFinishedAt().AsTime()
		out.FinishedAt = &t
	}
	return out, nil
}

func statusToProtobuf(status Status) (*pb.Status, error) {
	pos, err := positionToProtobuf(status.Position)
	if err != nil {
		return nil, err
	}
	goal, err := goalStatusToProtobuf(status.Goal)
	if err != nil {
		return nil, err
	}
	extra, err := protoutils.StructToStructPb(status.Extra)
	if err != nil {
		return nil, err
	}
	updatedAt := status.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	return &pb.Status{
		Position:  pos,
		Localized: status.Localized,
		Velocity: &pb.Velocity{
			LinearMPerSec:    status.Velocity.LinearMPerSec,
			AngularDegPerSec: status.Velocity.AngularDegPerSec,
		},
		Goal:      goal,
		UpdatedAt: timestamppb.New(updatedAt),
		Extra:     extra,
	}, nil
}

func protobufToStatus(status *pb.Status) (Status, error) {
	if status == nil {
		return Status{}, errors.New("status is required")
	}
	pos, err := protobufToPosition(status.GetPosition())
	if err != nil {
		return Status{}, err
	}
	goal, err := protobufToGoalStatus(status.GetGoal())
	if err != nil {
		return Status{}, err
	}
	out := Status{
		Position:  pos,
		Localized: status.GetLocalized(),
		Goal:      goal,
		UpdatedAt: status.GetUpdatedAt().AsTime(),
		Extra:     status.GetExtra().AsMap(),
	}
	if v := status.GetVelocity(); v != nil {
		out.Velocity = Velocity{
			LinearMPerSec:    v.GetLinearMPerSec(),
			AngularDegPerSec: v.GetAngularDegPerSec(),
		}
	}
	return out, nil
}

func propertiesToProtobuf(props Properties) *pb.Properties {
	return &pb.Properties{
		MapType:           mapTypeToProtobuf(props.MapType),
		FootprintLengthM:  props.FootprintLengthM,
		FootprintWidthM:   props.FootprintWidthM,
		RobotRadiusM:      props.RobotRadiusM,
		PlanningSupported: props.PlanningSupported,
		CostmapSupported:  props.CostmapSupported,
		ZonesSupported:    props.ZonesSupported,
		RoutesSupported:   props.RoutesSupported,
	}
}

func protobufToProperties(props *pb.Properties) (Properties, error) {
	if props == nil {
		return Properties{}, errors.New("properties is required")
	}
	mapType, err := protobufToMapType(props.GetMapType())
	if err != nil {
		return Properties{}, err
	}
	return Properties{
		MapType:           mapType,
		FootprintLengthM:  props.GetFootprintLengthM(),
		FootprintWidthM:   props.GetFootprintWidthM(),
		RobotRadiusM:      props.GetRobotRadiusM(),
		PlanningSupported: props.GetPlanningSupported(),
		CostmapSupported:  props.GetCostmapSupported(),
		ZonesSupported:    props.GetZonesSupported(),
		RoutesSupported:   props.GetRoutesSupported(),
	}, nil
}

func costmapToProtobuf(cm Costmap) *pb.GetCostmapResponse {
	return &pb.GetCostmapResponse{
		Available:  cm.Available,
		Layer:      cm.Layer,
		NavBackend: cm.NavBackend,
		OriginX:    cm.OriginX,
		OriginY:    cm.OriginY,
		Resolution: cm.Resolution,
		Width:      cm.Width,
		Height:     cm.Height,
		Encoding:   cm.Encoding,
		Data:       cm.Data,
	}
}

func protobufToCostmap(resp *pb.GetCostmapResponse) Costmap {
	if resp == nil {
		return Costmap{}
	}
	return Costmap{
		Available:  resp.GetAvailable(),
		Layer:      resp.GetLayer(),
		NavBackend: resp.GetNavBackend(),
		OriginX:    resp.GetOriginX(),
		OriginY:    resp.GetOriginY(),
		Resolution: resp.GetResolution(),
		Width:      resp.GetWidth(),
		Height:     resp.GetHeight(),
		Encoding:   resp.GetEncoding(),
		Data:       resp.GetData(),
	}
}
