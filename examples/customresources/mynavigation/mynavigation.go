// Package mynavigation contains an example navigation service that only stores waypoints.
package mynavigation

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/navigation"
)

var Model = resource.NewModel(
	resource.Namespace("acme"),
	resource.ModelFamilyName("demo"),
	resource.ModelName("mynavigation"),
)

func init() {
	registry.RegisterService(navigation.Subtype, Model, registry.Service{Constructor: newNav})
}

func newNav(ctx context.Context, r robot.Robot, cfg config.Service, logger golog.Logger) (interface{}, error) {
	navSvc := &navSvc{
		logger: logger,
		loc: geo.NewPoint(
			cfg.Attributes.Float64("lat", -48.876667),
			cfg.Attributes.Float64("long", -123.393333),
		),
	}
	return navSvc, nil
}

type navSvc struct {
	mu sync.Mutex
	loc *geo.Point
	logger golog.Logger
	waypoints []navigation.Waypoint
}

func (svc *navSvc) Mode(ctx context.Context) (navigation.Mode, error) {
	return 0, nil
}

func (svc *navSvc) SetMode(ctx context.Context, mode navigation.Mode) error {
	return nil
}

func (svc *navSvc) Location(ctx context.Context) (*geo.Point, error) {
	return svc.loc, nil
}

func (svc *navSvc) Waypoints(ctx context.Context) ([]navigation.Waypoint, error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	wpsCopy := make([]navigation.Waypoint, len(svc.waypoints))
	copy(wpsCopy, svc.waypoints)
	return wpsCopy, nil
}

func (svc *navSvc) AddWaypoint(ctx context.Context, point *geo.Point) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.waypoints = append(svc.waypoints, navigation.Waypoint{Lat: point.Lat(), Long: point.Lng()})
	return nil
}

func (svc *navSvc) RemoveWaypoint(ctx context.Context, id primitive.ObjectID) error {
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
