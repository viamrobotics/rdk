// Package navigation contains a navigation service, along with a gRPC server and client
package navigation

import (
	"context"
	"fmt"

	geo "github.com/kellydunn/golang-geo"
	"go.mongodb.org/mongo-driver/bson/primitive"
	servicepb "go.viam.com/api/service/navigation/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype[Service]{
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
)

// A Service controls the navigation for a robot.
type Service interface {
	resource.Resource
	Mode(ctx context.Context, extra map[string]interface{}) (Mode, error)
	SetMode(ctx context.Context, mode Mode, extra map[string]interface{}) error

	Location(ctx context.Context, extra map[string]interface{}) (*geo.Point, error)

	// Waypoint
	Waypoints(ctx context.Context, extra map[string]interface{}) ([]Waypoint, error)
	AddWaypoint(ctx context.Context, point *geo.Point, extra map[string]interface{}) error
	RemoveWaypoint(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("navigation")

// Subtype is a constant that identifies the navigation service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named navigation service's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// FromRobot is a helper for getting the named navigation service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// Config describes how to configure the service.
type Config struct {
	Store              StoreConfig `json:"store"`
	BaseName           string      `json:"base"`
	MovementSensorName string      `json:"movement_sensor"`
	DegPerSecDefault   float64     `json:"degs_per_sec"`
	MMPerSecDefault    float64     `json:"mm_per_sec"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) error {
	if err := conf.Store.Validate(fmt.Sprintf("%s.%s", path, "store")); err != nil {
		return err
	}
	if conf.BaseName == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "base")
	}
	if conf.MovementSensorName == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "movement_sensor")
	}
	return nil
}
