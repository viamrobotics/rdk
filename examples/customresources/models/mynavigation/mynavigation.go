// Package mynavigation contains an example navigation service that stores locations and reports a fixed position.
package mynavigation

import (
	"context"
	"errors"
	"sync"

	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/navigation"
)

var errUnimplemented = errors.New("unimplemented")

// Model is the full model definition.
var Model = resource.NewModel("acme", "demo", "mynavigation")

func init() {
	resource.RegisterService(navigation.API, Model, resource.Registration[navigation.Service, *Config]{
		Constructor: newNav,
	})
}

// Config is the navigation model's config.
type Config struct {
	Lat  *float64 `json:"lat,omitempty"`
	Long *float64 `json:"long,omitempty"`
	resource.TriviallyValidateConfig
}

func newNav(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (navigation.Service, error) {
	navConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}
	lat := -48.876667
	if navConfig.Lat != nil {
		lat = *navConfig.Lat
	}
	lng := -48.876667
	if navConfig.Long != nil {
		lng = *navConfig.Long
	}
	heading := 0.0
	return &navSvc{
		Named:     conf.ResourceName().AsNamed(),
		logger:    logger,
		locations: map[string]navigation.Location{},
		pos: navigation.Position{GeoPose: &navigation.GeoPose{
			Location: geo.NewPoint(lat, lng),
			Heading:  &heading,
		}},
	}, nil
}

type navSvc struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	logger    logging.Logger
	mu        sync.Mutex
	locations map[string]navigation.Location
	pos       navigation.Position
}

func (svc *navSvc) AddLocation(ctx context.Context, req navigation.AddLocationReq) (navigation.Location, error) {
	pos := req.Position
	if pos == nil {
		pos = &svc.pos
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if _, ok := svc.locations[req.Name]; ok {
		return navigation.Location{}, errors.New("location already exists")
	}
	loc := navigation.Location{Name: req.Name, Position: *pos}
	svc.locations[req.Name] = loc
	return loc, nil
}

func (svc *navSvc) GetLocation(ctx context.Context, name string, extra map[string]interface{}) (navigation.Location, error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	loc, ok := svc.locations[name]
	if !ok {
		return navigation.Location{}, errors.New("location not found")
	}
	return loc, nil
}

func (svc *navSvc) ListLocations(ctx context.Context, extra map[string]interface{}) ([]navigation.Location, error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	out := make([]navigation.Location, 0, len(svc.locations))
	for _, loc := range svc.locations {
		out = append(out, loc)
	}
	return out, nil
}

func (svc *navSvc) UpdateLocation(ctx context.Context, req navigation.UpdateLocationReq) (navigation.Location, error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	loc, ok := svc.locations[req.Name]
	if !ok {
		return navigation.Location{}, errors.New("location not found")
	}
	if req.Position != nil {
		loc.Position = *req.Position
	}
	name := req.Name
	if req.NewName != nil && *req.NewName != "" {
		delete(svc.locations, req.Name)
		name = *req.NewName
		loc.Name = name
	}
	svc.locations[name] = loc
	return loc, nil
}

func (svc *navSvc) DeleteLocation(ctx context.Context, name string, extra map[string]interface{}) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if _, ok := svc.locations[name]; !ok {
		return errors.New("location not found")
	}
	delete(svc.locations, name)
	return nil
}

func (svc *navSvc) AddRoute(context.Context, navigation.AddRouteReq) (navigation.Route, error) {
	return navigation.Route{}, errUnimplemented
}

func (svc *navSvc) GetRoute(context.Context, string, map[string]interface{}) (navigation.Route, error) {
	return navigation.Route{}, errUnimplemented
}

func (svc *navSvc) ListRoutes(context.Context, map[string]interface{}) ([]navigation.Route, error) {
	return nil, errUnimplemented
}

func (svc *navSvc) UpdateRoute(context.Context, navigation.UpdateRouteReq) (navigation.Route, error) {
	return navigation.Route{}, errUnimplemented
}

func (svc *navSvc) DeleteRoute(context.Context, string, map[string]interface{}) error {
	return errUnimplemented
}

func (svc *navSvc) AddZone(context.Context, navigation.AddZoneReq) (navigation.Zone, error) {
	return navigation.Zone{}, errUnimplemented
}

func (svc *navSvc) GetZone(context.Context, string, map[string]interface{}) (navigation.Zone, error) {
	return navigation.Zone{}, errUnimplemented
}

func (svc *navSvc) ListZones(context.Context, string, map[string]interface{}) ([]navigation.Zone, error) {
	return nil, errUnimplemented
}

func (svc *navSvc) UpdateZone(context.Context, navigation.UpdateZoneReq) (navigation.Zone, error) {
	return navigation.Zone{}, errUnimplemented
}

func (svc *navSvc) DeleteZone(context.Context, string, map[string]interface{}) error {
	return errUnimplemented
}

func (svc *navSvc) NavigateToLocation(context.Context, string, map[string]interface{}) (navigation.Status, error) {
	return navigation.Status{}, errUnimplemented
}

func (svc *navSvc) NavigateToPoint(context.Context, navigation.Position, map[string]interface{}) (navigation.Status, error) {
	return navigation.Status{}, errUnimplemented
}

func (svc *navSvc) NavigateRoute(context.Context, navigation.NavigateRouteReq) (navigation.Status, error) {
	return navigation.Status{}, errUnimplemented
}

func (svc *navSvc) PlanToPoint(context.Context, navigation.PlanToPointReq) (navigation.Plan, error) {
	return navigation.Plan{}, errUnimplemented
}

func (svc *navSvc) PlanToLocation(context.Context, navigation.PlanToLocationReq) (navigation.Plan, error) {
	return navigation.Plan{}, errUnimplemented
}

func (svc *navSvc) ExecutePlan(context.Context, navigation.Plan, map[string]interface{}) (navigation.Status, error) {
	return navigation.Status{}, errUnimplemented
}

func (svc *navSvc) GoToLocation(context.Context, navigation.GoToLocationReq) (navigation.Status, error) {
	return navigation.Status{}, errUnimplemented
}

func (svc *navSvc) GoToPoint(context.Context, navigation.GoToPointReq) (navigation.Status, error) {
	return navigation.Status{}, errUnimplemented
}

func (svc *navSvc) Suspend(context.Context, string, map[string]interface{}) (navigation.Status, error) {
	return navigation.Status{}, errUnimplemented
}

func (svc *navSvc) Resume(context.Context, map[string]interface{}) (navigation.Status, error) {
	return navigation.Status{}, errUnimplemented
}

func (svc *navSvc) Cancel(context.Context, map[string]interface{}) (navigation.Status, error) {
	return navigation.Status{}, errUnimplemented
}

func (svc *navSvc) GetPosition(context.Context, map[string]interface{}) (navigation.Position, error) {
	return svc.pos, nil
}

func (svc *navSvc) GetNavigationStatus(context.Context, map[string]interface{}) (navigation.Status, error) {
	return navigation.Status{Position: svc.pos, Localized: true}, nil
}

func (svc *navSvc) GetProperties(context.Context) (navigation.Properties, error) {
	return navigation.Properties{MapType: navigation.GPSMap}, nil
}

func (svc *navSvc) GetCostmap(context.Context, navigation.CostmapReq) (navigation.Costmap, error) {
	return navigation.Costmap{}, errUnimplemented
}
