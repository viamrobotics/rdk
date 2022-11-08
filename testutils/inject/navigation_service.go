package inject

import (
	"context"

	geo "github.com/kellydunn/golang-geo"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"go.viam.com/rdk/services/navigation"
)

// NavigationService represents a fake instance of a navigation service.
type NavigationService struct {
	navigation.Service
	ModeFunc    func(ctx context.Context, extra map[string]interface{}) (navigation.Mode, error)
	SetModeFunc func(ctx context.Context, mode navigation.Mode, extra map[string]interface{}) error

	LocationFunc func(ctx context.Context, extra map[string]interface{}) (*geo.Point, error)

	WaypointsFunc      func(ctx context.Context, extra map[string]interface{}) ([]navigation.Waypoint, error)
	AddWaypointFunc    func(ctx context.Context, point *geo.Point, extra map[string]interface{}) error
	RemoveWaypointFunc func(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error
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
func (ns *NavigationService) Location(ctx context.Context, extra map[string]interface{}) (*geo.Point, error) {
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
