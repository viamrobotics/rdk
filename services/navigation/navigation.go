// Package navigation is the service that allows you to navigate along waypoints.
// For more information, see the [navigation service docs].
//
// [navigation service docs]: https://docs.viam.com/services/navigation/
package navigation

import (
	"context"

	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	servicepb "go.viam.com/api/service/navigation/v1"

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
}

// Mode describes what mode to operate the service in.
type Mode uint8

// MapType describes what map the navigation service is operating on.
type MapType uint8

// The set of known modes.
const (
	ModeManual = Mode(iota)
	ModeWaypoint
	ModeExplore

	NoMap = MapType(iota)
	GPSMap
)

func (m Mode) String() string {
	switch m {
	case ModeManual:
		return "Manual"
	case ModeWaypoint:
		return "Waypoint"
	case ModeExplore:
		return "Explore"
	default:
		return "UNKNOWN"
	}
}

func (m MapType) String() string {
	switch m {
	case NoMap:
		return "None"
	case GPSMap:
		return "GPS"
	default:
		return "UNKNOWN"
	}
}

// StringToMapType converts an input string into one of the valid map type if possible.
func StringToMapType(mapTypeName string) (MapType, error) {
	switch mapTypeName {
	case "None":
		return NoMap, nil
	case "GPS", "":
		return GPSMap, nil
	}
	return 0, errors.Errorf("invalid map_type '%v' given", mapTypeName)
}

// Properties returns information about the MapType that the configured navigation service is using.
type Properties struct {
	MapType MapType
}

// A Service controls the navigation for a robot.
// For more information, see the [navigation service docs].
//
// Mode example:
//
//	// Get the Mode the service is operating in.
//	mode, err := myNav.Mode(context.Background(), nil)
//
// SetMode example:
//
//	// Set the Mode the service is operating in to ModeWaypoint and begin navigation.
//	err := myNav.SetMode(context.Background(), navigation.ModeWaypoint, nil)
//
// Location example:
//
//	// Get the current location of the robot in the navigation service.
//	location, err := myNav.Location(context.Background(), nil)
//
// Waypoints example:
//
//	waypoints, err := myNav.Waypoints(context.Background(), nil)
//
// AddWaypoint example:
//
//	// Create a new waypoint with latitude and longitude values of 0 degrees.
//	// Assumes you have imported "github.com/kellydunn/golang-geo" as `geo`.
//	location := geo.NewPoint(0, 0)
//
//	// Add your waypoint to the service's data storage.
//	err := myNav.AddWaypoint(context.Background(), location, nil)
//
// RemoveWaypoint example:
//
//	// Assumes you have already called AddWaypoint once and the waypoint has not yet been reached.
//	waypoints, err := myNav.Waypoints(context.Background(), nil)
//	if (err != nil || len(waypoints) == 0) {
//	    return
//	}
//
//	// Remove the first waypoint from the service's data storage.
//	err = myNav.RemoveWaypoint(context.Background(), waypoints[0].ID, nil)
//
// Obstacles example:
//
//	// Get an array containing each obstacle stored by the navigation service.
//	obstacles, err := myNav.Obstacles(context.Background(), nil)
//
// Paths example:
//
//	// Get an array containing each path stored by the navigation service.
//	paths, err := myNav.Paths(context.Background(), nil)
//
// Properties example:
//
//	// Get the properties of the current navigation service.
//	navProperties, err := myNav.Properties(context.Background())
//
// [navigation service docs]: https://docs.viam.com/services/navigation/
type Service interface {
	resource.Resource

	// Mode returns the Mode the service is operating in.
	Mode(ctx context.Context, extra map[string]interface{}) (Mode, error)

	// SetMode sets the mode the service is operating in.
	SetMode(ctx context.Context, mode Mode, extra map[string]interface{}) error

	// Location returns the current location of the machine in the navigation service.
	Location(ctx context.Context, extra map[string]interface{}) (*spatialmath.GeoPose, error)

	// Waypoints returns an array of waypoints currently in the service's data storage which have not yet been reached.
	// These are locations designated within a path for the machine to navigate to.
	Waypoints(ctx context.Context, extra map[string]interface{}) ([]Waypoint, error)

	// AddWaypoint adds a waypoint to the service's data storage.
	AddWaypoint(ctx context.Context, point *geo.Point, extra map[string]interface{}) error

	// RemoveWaypoint removes a waypoint from the service's data storage.
	// If the machine is currently navigating to this waypoint, the motion will be canceled, and the machine will proceed to the next waypoint.
	RemoveWaypoint(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error

	// Obstacles returns a list of obstacles to avoid, both transient and predefined, identified by the vision and navigation services.
	Obstacles(ctx context.Context, extra map[string]interface{}) ([]*spatialmath.GeoGeometry, error)

	// Paths returns each path, which is a series of geo points.
	// These points outline the planned travel route to a destination waypoint in the machine’s motion planning.
	Paths(ctx context.Context, extra map[string]interface{}) ([]*Path, error)

	// Properties returns information about the configured navigation service.
	Properties(ctx context.Context) (Properties, error)
}

// SubtypeName is the name of the type of service.
const SubtypeName = "navigation"

// API is a variable that identifies the navigation service resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// Named is a helper for getting the named navigation service's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromRobot is a helper for getting the named navigation service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

func mapTypeToProtobuf(mapType MapType) servicepb.MapType {
	switch mapType {
	case NoMap:
		return servicepb.MapType_MAP_TYPE_NONE
	case GPSMap:
		return servicepb.MapType_MAP_TYPE_GPS
	default:
		return servicepb.MapType_MAP_TYPE_UNSPECIFIED
	}
}

func protobufToMapType(mapType servicepb.MapType) (MapType, error) {
	switch mapType {
	case servicepb.MapType_MAP_TYPE_NONE:
		return NoMap, nil
	case servicepb.MapType_MAP_TYPE_GPS:
		return GPSMap, nil
	case servicepb.MapType_MAP_TYPE_UNSPECIFIED:
		fallthrough
	default:
		return 0, errors.New("map type unspecified")
	}
}
