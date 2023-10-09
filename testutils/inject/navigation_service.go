package inject

import (
	"context"

	geo "github.com/kellydunn/golang-geo"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/spatialmath"
)

// NavigationService represents a fake instance of a navigation service.
type NavigationService struct {
	navigation.Service
	name        resource.Name
	ModeFunc    func(ctx context.Context, extra map[string]interface{}) (navigation.Mode, error)
	SetModeFunc func(ctx context.Context, mode navigation.Mode, extra map[string]interface{}) error

	LocationFunc func(ctx context.Context, extra map[string]interface{}) (*spatialmath.GeoPose, error)

	WaypointsFunc      func(ctx context.Context, extra map[string]interface{}) ([]navigation.Waypoint, error)
	AddWaypointFunc    func(ctx context.Context, point *geo.Point, extra map[string]interface{}) error
	RemoveWaypointFunc func(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error

	// GetObstaclesFunc func(ctx context.Context, extra map[string]interface{}) ([]*spatialmath.GeoObstacle, error)
	GetPathsFunc func(ctx context.Context, extra map[string]interface{}) ([]*navigation.Path, error)

	DoCommandFunc func(ctx context.Context,
		cmd map[string]interface{}) (map[string]interface{}, error)
	CloseFunc func(ctx context.Context) error
}

// NewNavigationService returns a new injected navigation service.
func NewNavigationService(name string) *NavigationService {
	return &NavigationService{name: navigation.Named(name)}
}

// Name returns the name of the resource.
func (ns *NavigationService) Name() resource.Name {
	return ns.name
}

// Mode calls the injected ModeFunc or the real version.
func (ns *NavigationService) Mode(ctx context.Context, extra map[string]interface{}) (navigation.Mode, error) {
	if ns.ModeFunc == nil {
		return ns.Service.Mode(ctx, extra)
	}
	return ns.ModeFunc(ctx, extra)
}

// SetMode calls the injected SetModeFunc or the real version.
func (ns *NavigationService) SetMode(ctx context.Context, mode navigation.Mode, extra map[string]interface{}) error {
	if ns.SetModeFunc == nil {
		return ns.Service.SetMode(ctx, mode, extra)
	}
	return ns.SetModeFunc(ctx, mode, extra)
}

// Location calls the injected LocationFunc or the real version.
func (ns *NavigationService) Location(ctx context.Context, extra map[string]interface{}) (*spatialmath.GeoPose, error) {
	if ns.LocationFunc == nil {
		return ns.Service.Location(ctx, extra)
	}
	return ns.LocationFunc(ctx, extra)
}

// Waypoints calls the injected WaypointsFunc or the real version.
func (ns *NavigationService) Waypoints(ctx context.Context, extra map[string]interface{}) ([]navigation.Waypoint, error) {
	if ns.WaypointsFunc == nil {
		return ns.Service.Waypoints(ctx, extra)
	}
	return ns.WaypointsFunc(ctx, extra)
}

// AddWaypoint calls the injected AddWaypointFunc or the real version.
func (ns *NavigationService) AddWaypoint(ctx context.Context, point *geo.Point, extra map[string]interface{}) error {
	if ns.AddWaypointFunc == nil {
		return ns.Service.AddWaypoint(ctx, point, extra)
	}
	return ns.AddWaypointFunc(ctx, point, extra)
}

// RemoveWaypoint calls the injected RemoveWaypointFunc or the real version.
func (ns *NavigationService) RemoveWaypoint(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error {
	if ns.RemoveWaypointFunc == nil {
		return ns.Service.RemoveWaypoint(ctx, id, extra)
	}
	return ns.RemoveWaypointFunc(ctx, id, extra)
}

// GetObstacles calls the injected GetObstacles or the real version.
// func (ns *NavigationService) GetObstacles(ctx context.Context, extra map[string]interface{}) ([]*spatialmath.GeoObstacle, error) {
// 	if ns.GetObstaclesFunc == nil {
// 		return ns.Service.GetObstacles(ctx, extra)
// 	}
// 	return ns.GetObstaclesFunc(ctx, extra)
// }

// GetPaths calls the injected GetPaths or the real version.
func (ns *NavigationService) GetPaths(ctx context.Context, extra map[string]interface{}) ([]*navigation.Path, error) {
	if ns.GetPathsFunc == nil {
		return ns.Service.GetPaths(ctx, extra)
	}
	return ns.GetPathsFunc(ctx, extra)
}

// DoCommand calls the injected DoCommand or the real variant.
func (ns *NavigationService) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	if ns.DoCommandFunc == nil {
		return ns.Service.DoCommand(ctx, cmd)
	}
	return ns.DoCommandFunc(ctx, cmd)
}

// Close calls the injected Close or the real version.
func (ns *NavigationService) Close(ctx context.Context) error {
	if ns.CloseFunc == nil {
		if ns.Service == nil {
			return nil
		}
		return ns.Service.Close(ctx)
	}
	return ns.CloseFunc(ctx)
}
