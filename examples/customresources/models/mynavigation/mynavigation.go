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
	"go.viam.com/rdk/spatialmath"
)

// Model is the full model definition.
var Model = resource.NewModel("acme", "demo", "mynavigation")

func init() {
	resource.RegisterService(navigation.API, Model, resource.Registration[navigation.Service, resource.NoNativeConfig]{
		Constructor: newNav,
	})
}

// Config is the navigation model's config
type Config struct {
	Lat  *float64 `json:"lat,omitempty"`
	Long *float64 `json:"long,omitempty"`
	resource.TriviallyValidateConfig
}

func newNav(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (navigation.Service, error) {
	navConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	lat := -48.876667
	if navConfig.Lat != nil {
		lat = *navConfig.Lat
	}

	lng := -48.876667
	if navConfig.Lat != nil {
		lng = *navConfig.Long
	}

	navSvc := &navSvc{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
		loc:    geo.NewPoint(lat, lng),
	}
	return navSvc, nil
}

type navSvc struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable

	loc    *geo.Point
	logger golog.Logger

	waypointsMu sync.RWMutex
	waypoints   []navigation.Waypoint
}

func (svc *navSvc) Mode(ctx context.Context, extra map[string]interface{}) (navigation.Mode, error) {
	return 0, nil
}

func (svc *navSvc) SetMode(ctx context.Context, mode navigation.Mode, extra map[string]interface{}) error {
	return nil
}

func (svc *navSvc) Location(ctx context.Context, extra map[string]interface{}) (*spatialmath.GeoPose, error) {
	svc.waypointsMu.RLock()
	defer svc.waypointsMu.RUnlock()
	geoPose := spatialmath.NewGeoPose(svc.loc, 0)
	return geoPose, nil
}

func (svc *navSvc) Waypoints(ctx context.Context, extra map[string]interface{}) ([]navigation.Waypoint, error) {
	svc.waypointsMu.RLock()
	defer svc.waypointsMu.RUnlock()
	wpsCopy := make([]navigation.Waypoint, len(svc.waypoints))
	copy(wpsCopy, svc.waypoints)
	return wpsCopy, nil
}

func (svc *navSvc) AddWaypoint(ctx context.Context, point *geo.Point, extra map[string]interface{}) error {
	svc.waypointsMu.Lock()
	defer svc.waypointsMu.Unlock()
	svc.waypoints = append(svc.waypoints, navigation.Waypoint{Lat: point.Lat(), Long: point.Lng()})
	return nil
}

func (svc *navSvc) RemoveWaypoint(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error {
	svc.waypointsMu.Lock()
	defer svc.waypointsMu.Unlock()
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

func (svc *navSvc) GetObstacles(ctx context.Context, extra map[string]interface{}) ([]*spatialmath.GeoObstacle, error) {
	return []*spatialmath.GeoObstacle{}, nil
}
