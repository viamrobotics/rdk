package inject

import (
	"context"

	geo "github.com/kellydunn/golang-geo"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"braces.dev/errtrace"
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

	ObstaclesFunc func(ctx context.Context, extra map[string]interface{}) ([]*spatialmath.GeoGeometry, error)
	PathsFunc     func(ctx context.Context, extra map[string]interface{}) ([]*navigation.Path, error)

	PropertiesFunc func(ctx context.Context) (navigation.Properties, error)

	DoCommandFunc func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	StatusFunc    func(ctx context.Context) (map[string]interface{}, error)
	CloseFunc     func(ctx context.Context) error
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
		return errtrace.Wrap2(ns.Service.Mode(ctx, extra))
	}
	return errtrace.Wrap2(ns.ModeFunc(ctx, extra))
}

// SetMode calls the injected SetModeFunc or the real version.
func (ns *NavigationService) SetMode(ctx context.Context, mode navigation.Mode, extra map[string]interface{}) error {
	if ns.SetModeFunc == nil {
		return errtrace.Wrap(ns.Service.SetMode(ctx, mode, extra))
	}
	return errtrace.Wrap(ns.SetModeFunc(ctx, mode, extra))
}

// Location calls the injected LocationFunc or the real version.
func (ns *NavigationService) Location(ctx context.Context, extra map[string]interface{}) (*spatialmath.GeoPose, error) {
	if ns.LocationFunc == nil {
		return errtrace.Wrap2(ns.Service.Location(ctx, extra))
	}
	return errtrace.Wrap2(ns.LocationFunc(ctx, extra))
}

// Waypoints calls the injected WaypointsFunc or the real version.
func (ns *NavigationService) Waypoints(ctx context.Context, extra map[string]interface{}) ([]navigation.Waypoint, error) {
	if ns.WaypointsFunc == nil {
		return errtrace.Wrap2(ns.Service.Waypoints(ctx, extra))
	}
	return errtrace.Wrap2(ns.WaypointsFunc(ctx, extra))
}

// AddWaypoint calls the injected AddWaypointFunc or the real version.
func (ns *NavigationService) AddWaypoint(ctx context.Context, point *geo.Point, extra map[string]interface{}) error {
	if ns.AddWaypointFunc == nil {
		return errtrace.Wrap(ns.Service.AddWaypoint(ctx, point, extra))
	}
	return errtrace.Wrap(ns.AddWaypointFunc(ctx, point, extra))
}

// RemoveWaypoint calls the injected RemoveWaypointFunc or the real version.
func (ns *NavigationService) RemoveWaypoint(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error {
	if ns.RemoveWaypointFunc == nil {
		return errtrace.Wrap(ns.Service.RemoveWaypoint(ctx, id, extra))
	}
	return errtrace.Wrap(ns.RemoveWaypointFunc(ctx, id, extra))
}

// Obstacles calls the injected Obstacles or the real version.
func (ns *NavigationService) Obstacles(ctx context.Context, extra map[string]interface{}) ([]*spatialmath.GeoGeometry, error) {
	if ns.ObstaclesFunc == nil {
		return errtrace.Wrap2(ns.Service.Obstacles(ctx, extra))
	}
	return errtrace.Wrap2(ns.ObstaclesFunc(ctx, extra))
}

// Paths calls the injected Paths or the real version.
func (ns *NavigationService) Paths(ctx context.Context, extra map[string]interface{}) ([]*navigation.Path, error) {
	if ns.PathsFunc == nil {
		return errtrace.Wrap2(ns.Service.Paths(ctx, extra))
	}
	return errtrace.Wrap2(ns.PathsFunc(ctx, extra))
}

// Properties calls the injected Properties or the real variant.
func (ns *NavigationService) Properties(ctx context.Context) (navigation.Properties, error) {
	if ns.PropertiesFunc == nil {
		return errtrace.Wrap2(ns.Service.Properties(ctx))
	}
	return errtrace.Wrap2(ns.PropertiesFunc(ctx))
}

// DoCommand calls the injected DoCommand or the real variant.
func (ns *NavigationService) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	if ns.DoCommandFunc == nil {
		return errtrace.Wrap2(ns.Service.DoCommand(ctx, cmd))
	}
	return errtrace.Wrap2(ns.DoCommandFunc(ctx, cmd))
}

// Status calls the injected Status or the real version.
func (ns *NavigationService) Status(ctx context.Context) (map[string]interface{}, error) {
	if ns.StatusFunc != nil {
		return errtrace.Wrap2(ns.StatusFunc(ctx))
	}
	if ns.Service != nil {
		return errtrace.Wrap2(ns.Service.Status(ctx))
	}
	return map[string]interface{}{}, nil
}

// Close calls the injected Close or the real version.
func (ns *NavigationService) Close(ctx context.Context) error {
	if ns.CloseFunc == nil {
		if ns.Service == nil {
			return nil
		}
		return errtrace.Wrap(ns.Service.Close(ctx))
	}
	return errtrace.Wrap(ns.CloseFunc(ctx))
}
