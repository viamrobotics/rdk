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
	GetModeFunc func(ctx context.Context) (navigation.Mode, error)
	SetModeFunc func(ctx context.Context, mode navigation.Mode) error

	GetLocationFunc func(ctx context.Context) (*geo.Point, error)

	GetWaypointsFunc   func(ctx context.Context) ([]navigation.Waypoint, error)
	AddWaypointFunc    func(ctx context.Context, point *geo.Point) error
	RemoveWaypointFunc func(ctx context.Context, id primitive.ObjectID) error
}

// Mode calls the injected ModeFunc or the real version.
func (ns *NavigationService) Mode(ctx context.Context) (navigation.Mode, error) {
	if ns.GetModeFunc == nil {
		return ns.Service.Mode(ctx)
	}
	return ns.GetModeFunc(ctx)
}

// SetMode calls the injected SetModeFunc or the real version.
func (ns *NavigationService) SetMode(ctx context.Context, mode navigation.Mode) error {
	if ns.SetModeFunc == nil {
		return ns.Service.SetMode(ctx, mode)
	}
	return ns.SetModeFunc(ctx, mode)
}

// Location calls the injected LocationFunc or the real version.
func (ns *NavigationService) Location(ctx context.Context) (*geo.Point, error) {
	if ns.GetLocationFunc == nil {
		return ns.Service.Location(ctx)
	}
	return ns.GetLocationFunc(ctx)
}

// Waypoints calls the injected WaypointsFunc or the real version.
func (ns *NavigationService) Waypoints(ctx context.Context) ([]navigation.Waypoint, error) {
	if ns.GetWaypointsFunc == nil {
		return ns.Service.Waypoints(ctx)
	}
	return ns.GetWaypointsFunc(ctx)
}

// AddWaypoint calls the injected AddWaypointFunc or the real version.
func (ns *NavigationService) AddWaypoint(ctx context.Context, point *geo.Point) error {
	if ns.AddWaypointFunc == nil {
		return ns.Service.AddWaypoint(ctx, point)
	}
	return ns.AddWaypointFunc(ctx, point)
}

// RemoveWaypoint calls the injected RemoveWaypointFunc or the real version.
func (ns *NavigationService) RemoveWaypoint(ctx context.Context, id primitive.ObjectID) error {
	if ns.RemoveWaypointFunc == nil {
		return ns.Service.RemoveWaypoint(ctx, id)
	}
	return ns.RemoveWaypointFunc(ctx, id)
}
