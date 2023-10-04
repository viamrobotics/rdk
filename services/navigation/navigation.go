// Package navigation is the service that allows you to navigate along waypoints.
package navigation

import (
	"context"

	geo "github.com/kellydunn/golang-geo"
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

// The set of known modes.
const (
	ModeManual = Mode(iota)
	ModeWaypoint
	ModeExplore
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

// A Service controls the navigation for a robot.
type Service interface {
	resource.Resource
	Mode(ctx context.Context, extra map[string]interface{}) (Mode, error)
	SetMode(ctx context.Context, mode Mode, extra map[string]interface{}) error
	Location(ctx context.Context, extra map[string]interface{}) (*spatialmath.GeoPose, error)

	// Waypoint
	Waypoints(ctx context.Context, extra map[string]interface{}) ([]Waypoint, error)
	AddWaypoint(ctx context.Context, point *geo.Point, extra map[string]interface{}) error
	RemoveWaypoint(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error

	GetObstacles(ctx context.Context, extra map[string]interface{}) ([]*spatialmath.GeoObstacle, error)
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
