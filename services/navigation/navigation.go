// Package navigation is the service that allows you to save places and navigate to them.
// For more information, see the [navigation service docs].
//
// [navigation service docs]: https://docs.viam.com/reference/services/navigation/
package navigation

import (
	"context"
	"time"

	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	servicepb "go.viam.com/api/service/navigation/v1"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Service]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           servicepb.RegisterNavigationServiceHandlerFromEndpoint,
		RPCServiceDesc:              &servicepb.NavigationService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: doCommand.String(),
	}, newDoCommandCollector)
}

// MapType describes what map the navigation service is operating on.
type MapType uint8

// The set of known map types.
const (
	UnspecifiedMap = MapType(iota)
	LocalMap
	GPSMap
)

const (
	unknownMap = "Unknown"

	// ZoneTypeKeepout is a region the planner must avoid.
	ZoneTypeKeepout = "keepout"
	// ZoneTypeSpeedLimit is a region where speed is capped.
	ZoneTypeSpeedLimit = "speed_limit"
)

// String returns the map type name used in configuration: "Local", "GPS", or "Unknown".
func (m MapType) String() string {
	switch m {
	case LocalMap:
		return "Local"
	case GPSMap:
		return "GPS"
	case UnspecifiedMap:
		fallthrough
	default:
		return unknownMap
	}
}

// StringToMapType converts an input string into a valid map type if possible.
func StringToMapType(mapTypeName string) (MapType, error) {
	switch mapTypeName {
	case "Local":
		return LocalMap, nil
	case "GPS", "":
		return GPSMap, nil
	}
	return 0, errors.Errorf("invalid map_type '%v' given", mapTypeName)
}

// GoalState is the lifecycle state of a navigation goal.
type GoalState uint8

// The set of known goal states.
const (
	GoalStateUnspecified = GoalState(iota)
	GoalStateActive
	GoalStateSuspended
	GoalStateSucceeded
	GoalStateFailed
	GoalStateCanceled
)

// NavMode is how a goal is being pursued.
type NavMode uint8

// The set of known navigation modes.
const (
	NavModeUnspecified = NavMode(iota)
	NavModePlanned
	NavModeDirect
	NavModeRoute
)

// NavErrorCode classifies a navigation failure.
type NavErrorCode uint8

// The set of known navigation error codes.
const (
	NavErrorCodeUnspecified = NavErrorCode(iota)
	NavErrorCodeNoPath
	NavErrorCodeBlocked
	NavErrorCodeLocalizationLost
	NavErrorCodeBaseError
	NavErrorCodeTimeout
	NavErrorCodeOther
)

// GeoPose is a GPS position and orientation, optionally with altitude.
// Exactly one of Heading or Orientation is set. Heading is a compass heading in
// degrees clockwise from north for a level pose. Orientation is the full
// orientation vector in the east-north-up tangent frame at Location.
type GeoPose struct {
	Location    *geo.Point
	Altitude    *float64
	Heading     *float64
	Orientation *spatialmath.OrientationVectorDegrees
}

// Position is a map-typed position. Exactly one of Pose or GeoPose is set.
type Position struct {
	Pose    spatialmath.Pose
	GeoPose *GeoPose
}

// Location is a named Position on the active map.
type Location struct {
	Name     string
	Position Position
}

// Waypoint is a route stop: either a saved location name or an inline Position.
type Waypoint struct {
	LocationName string
	Position     *Position
}

// Route is an ordered list of waypoints.
type Route struct {
	Name      string
	Waypoints []Waypoint
}

// Zone is a named region on the active map.
type Zone struct {
	Name     string
	Type     string
	Geometry *structpb.Struct
	MaxSpeed *float64
}

// Plan is a path preview returned by PlanToPoint and PlanToLocation.
type Plan struct {
	Path     []Position
	Location *string
	LengthM  float64
}

// Velocity is body-frame velocity.
type Velocity struct {
	LinearMPerSec    float64
	AngularDegPerSec float64
}

// NavError describes a failed goal.
type NavError struct {
	Code    NavErrorCode
	Message string
}

// Progress is reported while a planned or route goal is active.
type Progress struct {
	DistanceRemainingM float64
	Path               []Position
	LengthM            float64
}

// RouteProgress is reported while a route goal is active.
type RouteProgress struct {
	Name      string
	Waypoints []Waypoint
	Index     uint32
	Loop      bool
	Lap       uint32
}

// GoalStatus is one goal and its lifecycle.
type GoalStatus struct {
	State         GoalState
	Mode          NavMode
	Target        Position
	Location      *string
	Progress      *Progress
	Route         *RouteProgress
	SuspendReason *string
	Error         *NavError
	StartedAt     time.Time
	FinishedAt    *time.Time
}

// Status is the robot navigation state.
type Status struct {
	// Position is required. With no pose, implementations set a placeholder
	// (zero Pose, or a GeoPose at 0,0 with Heading 0) and Localized false.
	// Callers should ignore Position while Localized is false.
	Position  Position
	Localized bool
	Velocity  Velocity
	Goal      *GoalStatus
	UpdatedAt time.Time
	Extra     map[string]interface{}
}

// Properties are returned by GetProperties and do not change while the service is running.
type Properties struct {
	MapType           MapType
	FootprintLengthM  float64
	FootprintWidthM   float64
	RobotRadiusM      float64
	PlanningSupported bool
	CostmapSupported  bool
	ZonesSupported    bool
	RoutesSupported   bool
}

// Costmap is a downsampled occupancy grid for UI display.
type Costmap struct {
	Available  bool
	Layer      string
	NavBackend string
	OriginX    float64
	OriginY    float64
	Resolution float64
	Width      uint32
	Height     uint32
	Encoding   string
	Data       []byte
}

// AddLocationReq is the request to AddLocation.
type AddLocationReq struct {
	Name     string
	Position *Position
	Extra    map[string]interface{}
}

// UpdateLocationReq is the request to UpdateLocation.
type UpdateLocationReq struct {
	Name     string
	Position *Position
	NewName  *string
	Extra    map[string]interface{}
}

// AddRouteReq is the request to AddRoute.
type AddRouteReq struct {
	Name      string
	Waypoints []Waypoint
	Extra     map[string]interface{}
}

// UpdateRouteReq is the request to UpdateRoute.
// A nil Waypoints leaves the route's waypoints unchanged. A non-nil slice, including an empty one, replaces them.
type UpdateRouteReq struct {
	Name      string
	NewName   *string
	Waypoints []Waypoint
	Extra     map[string]interface{}
}

// AddZoneReq is the request to AddZone.
type AddZoneReq struct {
	Name     string
	Type     string
	Geometry *structpb.Struct
	MaxSpeed *float64
	Extra    map[string]interface{}
}

// UpdateZoneReq is the request to UpdateZone.
type UpdateZoneReq struct {
	Name     string
	Type     *string
	Geometry *structpb.Struct
	MaxSpeed *float64
	NewName  *string
	Extra    map[string]interface{}
}

// NavigateRouteReq is the request to NavigateRoute.
type NavigateRouteReq struct {
	Name         string
	Waypoints    []Waypoint
	StartIndex   uint32
	StartNearest bool
	Loop         bool
	Wait         bool
	Extra        map[string]interface{}
}

// PlanToPointReq is the request to PlanToPoint.
type PlanToPointReq struct {
	Destination Position
	Start       *Position
	Extra       map[string]interface{}
}

// PlanToLocationReq is the request to PlanToLocation.
type PlanToLocationReq struct {
	Name  string
	Start *Position
	Extra map[string]interface{}
}

// GoToLocationReq is the request to GoToLocation.
type GoToLocationReq struct {
	Name          string
	Wait          *bool
	LinearMPerSec *float64
	Extra         map[string]interface{}
}

// GoToPointReq is the request to GoToPoint.
type GoToPointReq struct {
	Destination   Position
	Wait          *bool
	LinearMPerSec *float64
	Extra         map[string]interface{}
}

// CostmapReq is the request to GetCostmap.
type CostmapReq struct {
	Layer  string
	Stride uint32
	Extra  map[string]interface{}
}

// A Service controls navigation for a robot.
type Service interface {
	resource.Resource

	// AddLocation saves a named position on the active map. If Position is omitted, the robot's current position is saved.
	AddLocation(ctx context.Context, req AddLocationReq) (Location, error)
	// GetLocation returns a saved location by name. It does not return the robot's current position.
	GetLocation(ctx context.Context, name string, extra map[string]interface{}) (Location, error)
	// ListLocations lists all saved locations on the active map.
	ListLocations(ctx context.Context, extra map[string]interface{}) ([]Location, error)
	// UpdateLocation changes a saved location's position or name. Renaming updates every route waypoint that references it.
	UpdateLocation(ctx context.Context, req UpdateLocationReq) (Location, error)
	// DeleteLocation deletes a saved location and removes it from every route that references it.
	DeleteLocation(ctx context.Context, name string, extra map[string]interface{}) error

	// AddRoute creates a named route from an ordered list of waypoints. Every referenced location must already exist.
	AddRoute(ctx context.Context, req AddRouteReq) (Route, error)
	// GetRoute returns a route by name.
	GetRoute(ctx context.Context, name string, extra map[string]interface{}) (Route, error)
	// ListRoutes lists all routes on the active map.
	ListRoutes(ctx context.Context, extra map[string]interface{}) ([]Route, error)
	// UpdateRoute renames a route and/or replaces its waypoints.
	// A nil Waypoints leaves them unchanged. A non-nil slice, including an empty one, replaces them.
	UpdateRoute(ctx context.Context, req UpdateRouteReq) (Route, error)
	// DeleteRoute deletes a route. The locations it references are not affected.
	DeleteRoute(ctx context.Context, name string, extra map[string]interface{}) error

	// AddZone creates a keep-out or speed-limit zone. The planner avoids keep-out zones and caps speed inside speed-limit zones.
	AddZone(ctx context.Context, req AddZoneReq) (Zone, error)
	// GetZone returns a zone by name.
	GetZone(ctx context.Context, name string, extra map[string]interface{}) (Zone, error)
	// ListZones lists zones on the active map. An empty zoneType returns every zone.
	ListZones(ctx context.Context, zoneType string, extra map[string]interface{}) ([]Zone, error)
	// UpdateZone changes a zone's type, geometry, speed limit, or name. A nil field leaves that value unchanged.
	UpdateZone(ctx context.Context, req UpdateZoneReq) (Zone, error)
	// DeleteZone deletes a zone.
	DeleteZone(ctx context.Context, name string, extra map[string]interface{}) error

	// NavigateToLocation plans an obstacle-aware path to a saved location and starts driving it.
	// It returns once the goal is accepted. Track progress with GetNavigationStatus.
	NavigateToLocation(ctx context.Context, name string, extra map[string]interface{}) (Status, error)
	// NavigateToPoint plans an obstacle-aware path to a Position and starts driving it.
	// It returns once the goal is accepted. On a Pose, theta defaults to 0 if omitted.
	NavigateToPoint(ctx context.Context, destination Position, extra map[string]interface{}) (Status, error)
	// NavigateRoute drives a route one waypoint at a time using the obstacle-aware planner.
	// Provide either a saved route name or an inline waypoint list.
	NavigateRoute(ctx context.Context, req NavigateRouteReq) (Status, error)

	// PlanToPoint computes a path from the robot, or from Start when set, to a Position without moving.
	// It returns an error if no path exists.
	PlanToPoint(ctx context.Context, req PlanToPointReq) (Plan, error)
	// PlanToLocation computes a path to a saved location without moving.
	// It returns an error if no path exists.
	PlanToLocation(ctx context.Context, req PlanToLocationReq) (Plan, error)
	// ExecutePlan drives a Plan, typically from PlanToPoint or PlanToLocation.
	// It returns an error if the path is empty.
	ExecutePlan(ctx context.Context, plan Plan, extra map[string]interface{}) (Status, error)

	// GoToLocation drives directly to a saved location with a closed-loop controller, without full path planning.
	// With Wait false, it returns immediately while navigating.
	GoToLocation(ctx context.Context, req GoToLocationReq) (Status, error)
	// GoToPoint drives directly to a Position with a closed-loop controller, without full path planning.
	// With Wait false, it returns immediately while navigating.
	GoToPoint(ctx context.Context, req GoToPointReq) (Status, error)

	// Suspend stops motion but remembers the current goal, including the current waypoint and loop setting for routes.
	// A suspended goal must be canceled before a new one starts.
	Suspend(ctx context.Context, reason string, extra map[string]interface{}) (Status, error)
	// Resume resumes the suspended goal the same way it was started.
	Resume(ctx context.Context, extra map[string]interface{}) (Status, error)
	// Cancel stops all motion and discards both the active goal and any suspended goal.
	Cancel(ctx context.Context, extra map[string]interface{}) (Status, error)

	// GetPosition returns the robot's current position on the active map.
	GetPosition(ctx context.Context, extra map[string]interface{}) (Position, error)
	// GetNavigationStatus returns the full navigation state, including the current position and the latest goal.
	GetNavigationStatus(ctx context.Context, extra map[string]interface{}) (Status, error)
	// GetProperties returns the map type and robot properties. These values do not change while the service is running.
	GetProperties(ctx context.Context) (Properties, error)
	// GetCostmap returns the local or global costmap as a downsampled grid, for display in UIs.
	GetCostmap(ctx context.Context, req CostmapReq) (Costmap, error)
}

// SubtypeName is the name of the type of service.
const SubtypeName = "navigation"

// API is a variable that identifies the navigation service resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// Named is a helper for getting the named navigation service's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// AsSub tags s as the navigation sub-resource of a composite, for resource.Compose.
func AsSub(s Service) resource.Sub { return resource.AsSub(API, s) }

// Deprecated: FromRobot is a helper for getting the named navigation service from the given Robot.
// Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// FromProvider is a helper for getting the named Navigation service
// from a resource Provider (collection of Dependencies or a Robot).
func FromProvider(provider resource.Provider, name string) (Service, error) {
	return resource.FromProvider[Service](provider, Named(name))
}
