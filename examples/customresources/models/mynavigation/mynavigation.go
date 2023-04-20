// Package mynavigation contains an example navigation service that only stores waypoints, and returns a fixed, configurable location.
package mynavigation

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/navigation"
)

var Model = resource.NewModel(
	resource.Namespace("acme"),
	resource.ModelFamilyName("demo"),
	resource.ModelName("mynavigation"),
)

func init() {
	resource.RegisterService(navigation.Subtype, Model, resource.Registration[navigation.Service, resource.NoNativeConfig]{
		Constructor: newNav,
	})
}

func newNav(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (navigation.Service, error) {
	navSvc := &navSvc{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
		loc: geo.NewPoint(
			conf.Attributes.Float64("lat", -48.876667),
			conf.Attributes.Float64("long", -123.393333),
		),
	}
	return navSvc, nil
}

type navSvc struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	mu        sync.RWMutex
	loc       *geo.Point
	logger    golog.Logger
	waypoints []navigation.Waypoint
}

func (svc *navSvc) Mode(ctx context.Context, extra map[string]interface{}) (navigation.Mode, error) {
	return 0, nil
}

func (svc *navSvc) SetMode(ctx context.Context, mode navigation.Mode, extra map[string]interface{}) error {
	return nil
}

func (svc *navSvc) Location(ctx context.Context, extra map[string]interface{}) (*geo.Point, error) {
	svc.mu.RLock()
	svc.mu.RUnlock()
	return svc.loc, nil
}

func (svc *navSvc) Waypoints(ctx context.Context, extra map[string]interface{}) ([]navigation.Waypoint, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	wpsCopy := make([]navigation.Waypoint, len(svc.waypoints))
	copy(wpsCopy, svc.waypoints)
	return wpsCopy, nil
}

func (svc *navSvc) AddWaypoint(ctx context.Context, point *geo.Point, extra map[string]interface{}) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.waypoints = append(svc.waypoints, navigation.Waypoint{Lat: point.Lat(), Long: point.Lng()})
	return nil
}

func (svc *navSvc) RemoveWaypoint(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	newWps := make([]navigation.Waypoint, 0, len(svc.waypoints)-1)
	for _, wp := range svc.waypoints {
		if wp.ID == id {
			continue
		}
		newWps = append(newWps, wp)
	}
	svc.waypoints = newWps
	return nil
}
