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

// GoToReq is the request to GoToLocation / GoToPoint.
type GoToReq struct {
	Name          string
	Destination   *Position
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

	AddLocation(ctx context.Context, req AddLocationReq) (Location, error)
	GetLocation(ctx context.Context, name string, extra map[string]interface{}) (Location, error)
	ListLocations(ctx context.Context, extra map[string]interface{}) ([]Location, error)
	UpdateLocation(ctx context.Context, req UpdateLocationReq) (Location, error)
	DeleteLocation(ctx context.Context, name string, extra map[string]interface{}) error

	AddRoute(ctx context.Context, req AddRouteReq) (Route, error)
	GetRoute(ctx context.Context, name string, extra map[string]interface{}) (Route, error)
	ListRoutes(ctx context.Context, extra map[string]interface{}) ([]Route, error)
	UpdateRoute(ctx context.Context, req UpdateRouteReq) (Route, error)
	DeleteRoute(ctx context.Context, name string, extra map[string]interface{}) error

	AddZone(ctx context.Context, req AddZoneReq) (Zone, error)
	GetZone(ctx context.Context, name string, extra map[string]interface{}) (Zone, error)
	ListZones(ctx context.Context, zoneType string, extra map[string]interface{}) ([]Zone, error)
	UpdateZone(ctx context.Context, req UpdateZoneReq) (Zone, error)
	DeleteZone(ctx context.Context, name string, extra map[string]interface{}) error

	NavigateToLocation(ctx context.Context, name string, extra map[string]interface{}) (Status, error)
	NavigateToPoint(ctx context.Context, destination Position, extra map[string]interface{}) (Status, error)
	NavigateRoute(ctx context.Context, req NavigateRouteReq) (Status, error)

	PlanToPoint(ctx context.Context, req PlanToPointReq) (Plan, error)
	PlanToLocation(ctx context.Context, req PlanToLocationReq) (Plan, error)
	ExecutePlan(ctx context.Context, plan Plan, extra map[string]interface{}) (Status, error)

	GoToLocation(ctx context.Context, req GoToReq) (Status, error)
	GoToPoint(ctx context.Context, req GoToReq) (Status, error)

	Suspend(ctx context.Context, reason string, extra map[string]interface{}) (Status, error)
	Resume(ctx context.Context, extra map[string]interface{}) (Status, error)
	Cancel(ctx context.Context, extra map[string]interface{}) (Status, error)

	GetPosition(ctx context.Context, extra map[string]interface{}) (Position, error)
	GetNavigationStatus(ctx context.Context, extra map[string]interface{}) (Status, error)
	GetProperties(ctx context.Context) (Properties, error)
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
