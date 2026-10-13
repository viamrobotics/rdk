package inject

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/navigation"
)

// NavigationService represents a fake instance of a navigation service.
type NavigationService struct {
	navigation.Service
	name resource.Name

	AddLocationFunc    func(ctx context.Context, req navigation.AddLocationReq) (navigation.Location, error)
	GetLocationFunc    func(ctx context.Context, name string, extra map[string]interface{}) (navigation.Location, error)
	ListLocationsFunc  func(ctx context.Context, extra map[string]interface{}) ([]navigation.Location, error)
	UpdateLocationFunc func(ctx context.Context, req navigation.UpdateLocationReq) (navigation.Location, error)
	DeleteLocationFunc func(ctx context.Context, name string, extra map[string]interface{}) error

	AddRouteFunc    func(ctx context.Context, req navigation.AddRouteReq) (navigation.Route, error)
	GetRouteFunc    func(ctx context.Context, name string, extra map[string]interface{}) (navigation.Route, error)
	ListRoutesFunc  func(ctx context.Context, extra map[string]interface{}) ([]navigation.Route, error)
	UpdateRouteFunc func(ctx context.Context, req navigation.UpdateRouteReq) (navigation.Route, error)
	DeleteRouteFunc func(ctx context.Context, name string, extra map[string]interface{}) error

	AddZoneFunc    func(ctx context.Context, req navigation.AddZoneReq) (navigation.Zone, error)
	GetZoneFunc    func(ctx context.Context, name string, extra map[string]interface{}) (navigation.Zone, error)
	ListZonesFunc  func(ctx context.Context, zoneType string, extra map[string]interface{}) ([]navigation.Zone, error)
	UpdateZoneFunc func(ctx context.Context, req navigation.UpdateZoneReq) (navigation.Zone, error)
	DeleteZoneFunc func(ctx context.Context, name string, extra map[string]interface{}) error

	NavigateToLocationFunc  func(ctx context.Context, name string, extra map[string]interface{}) (navigation.Status, error)
	NavigateToPointFunc     func(ctx context.Context, destination navigation.Position, extra map[string]interface{}) (navigation.Status, error)
	NavigateRouteFunc       func(ctx context.Context, req navigation.NavigateRouteReq) (navigation.Status, error)
	PlanToPointFunc         func(ctx context.Context, req navigation.PlanToPointReq) (navigation.Plan, error)
	PlanToLocationFunc      func(ctx context.Context, req navigation.PlanToLocationReq) (navigation.Plan, error)
	ExecutePlanFunc         func(ctx context.Context, plan navigation.Plan, extra map[string]interface{}) (navigation.Status, error)
	GoToLocationFunc        func(ctx context.Context, req navigation.GoToReq) (navigation.Status, error)
	GoToPointFunc           func(ctx context.Context, req navigation.GoToReq) (navigation.Status, error)
	SuspendFunc             func(ctx context.Context, reason string, extra map[string]interface{}) (navigation.Status, error)
	ResumeFunc              func(ctx context.Context, extra map[string]interface{}) (navigation.Status, error)
	CancelFunc              func(ctx context.Context, extra map[string]interface{}) (navigation.Status, error)
	GetPositionFunc         func(ctx context.Context, extra map[string]interface{}) (navigation.Position, error)
	GetNavigationStatusFunc func(ctx context.Context, extra map[string]interface{}) (navigation.Status, error)
	GetPropertiesFunc       func(ctx context.Context) (navigation.Properties, error)
	GetCostmapFunc          func(ctx context.Context, req navigation.CostmapReq) (navigation.Costmap, error)

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

// AddLocation calls the injected AddLocationFunc or the real version.
func (ns *NavigationService) AddLocation(ctx context.Context, req navigation.AddLocationReq) (navigation.Location, error) {
	if ns.AddLocationFunc == nil {
		return ns.Service.AddLocation(ctx, req)
	}
	return ns.AddLocationFunc(ctx, req)
}

// GetLocation calls the injected GetLocationFunc or the real version.
func (ns *NavigationService) GetLocation(ctx context.Context, name string, extra map[string]interface{}) (navigation.Location, error) {
	if ns.GetLocationFunc == nil {
		return ns.Service.GetLocation(ctx, name, extra)
	}
	return ns.GetLocationFunc(ctx, name, extra)
}

// ListLocations calls the injected ListLocationsFunc or the real version.
func (ns *NavigationService) ListLocations(ctx context.Context, extra map[string]interface{}) ([]navigation.Location, error) {
	if ns.ListLocationsFunc == nil {
		return ns.Service.ListLocations(ctx, extra)
	}
	return ns.ListLocationsFunc(ctx, extra)
}

// UpdateLocation calls the injected UpdateLocationFunc or the real version.
func (ns *NavigationService) UpdateLocation(ctx context.Context, req navigation.UpdateLocationReq) (navigation.Location, error) {
	if ns.UpdateLocationFunc == nil {
		return ns.Service.UpdateLocation(ctx, req)
	}
	return ns.UpdateLocationFunc(ctx, req)
}

// DeleteLocation calls the injected DeleteLocationFunc or the real version.
func (ns *NavigationService) DeleteLocation(ctx context.Context, name string, extra map[string]interface{}) error {
	if ns.DeleteLocationFunc == nil {
		return ns.Service.DeleteLocation(ctx, name, extra)
	}
	return ns.DeleteLocationFunc(ctx, name, extra)
}

// AddRoute calls the injected AddRouteFunc or the real version.
func (ns *NavigationService) AddRoute(ctx context.Context, req navigation.AddRouteReq) (navigation.Route, error) {
	if ns.AddRouteFunc == nil {
		return ns.Service.AddRoute(ctx, req)
	}
	return ns.AddRouteFunc(ctx, req)
}

// GetRoute calls the injected GetRouteFunc or the real version.
func (ns *NavigationService) GetRoute(ctx context.Context, name string, extra map[string]interface{}) (navigation.Route, error) {
	if ns.GetRouteFunc == nil {
		return ns.Service.GetRoute(ctx, name, extra)
	}
	return ns.GetRouteFunc(ctx, name, extra)
}

// ListRoutes calls the injected ListRoutesFunc or the real version.
func (ns *NavigationService) ListRoutes(ctx context.Context, extra map[string]interface{}) ([]navigation.Route, error) {
	if ns.ListRoutesFunc == nil {
		return ns.Service.ListRoutes(ctx, extra)
	}
	return ns.ListRoutesFunc(ctx, extra)
}

// UpdateRoute calls the injected UpdateRouteFunc or the real version.
func (ns *NavigationService) UpdateRoute(ctx context.Context, req navigation.UpdateRouteReq) (navigation.Route, error) {
	if ns.UpdateRouteFunc == nil {
		return ns.Service.UpdateRoute(ctx, req)
	}
	return ns.UpdateRouteFunc(ctx, req)
}

// DeleteRoute calls the injected DeleteRouteFunc or the real version.
func (ns *NavigationService) DeleteRoute(ctx context.Context, name string, extra map[string]interface{}) error {
	if ns.DeleteRouteFunc == nil {
		return ns.Service.DeleteRoute(ctx, name, extra)
	}
	return ns.DeleteRouteFunc(ctx, name, extra)
}

// AddZone calls the injected AddZoneFunc or the real version.
func (ns *NavigationService) AddZone(ctx context.Context, req navigation.AddZoneReq) (navigation.Zone, error) {
	if ns.AddZoneFunc == nil {
		return ns.Service.AddZone(ctx, req)
	}
	return ns.AddZoneFunc(ctx, req)
}

// GetZone calls the injected GetZoneFunc or the real version.
func (ns *NavigationService) GetZone(ctx context.Context, name string, extra map[string]interface{}) (navigation.Zone, error) {
	if ns.GetZoneFunc == nil {
		return ns.Service.GetZone(ctx, name, extra)
	}
	return ns.GetZoneFunc(ctx, name, extra)
}

// ListZones calls the injected ListZonesFunc or the real version.
func (ns *NavigationService) ListZones(ctx context.Context, zoneType string, extra map[string]interface{}) ([]navigation.Zone, error) {
	if ns.ListZonesFunc == nil {
		return ns.Service.ListZones(ctx, zoneType, extra)
	}
	return ns.ListZonesFunc(ctx, zoneType, extra)
}

// UpdateZone calls the injected UpdateZoneFunc or the real version.
func (ns *NavigationService) UpdateZone(ctx context.Context, req navigation.UpdateZoneReq) (navigation.Zone, error) {
	if ns.UpdateZoneFunc == nil {
		return ns.Service.UpdateZone(ctx, req)
	}
	return ns.UpdateZoneFunc(ctx, req)
}

// DeleteZone calls the injected DeleteZoneFunc or the real version.
func (ns *NavigationService) DeleteZone(ctx context.Context, name string, extra map[string]interface{}) error {
	if ns.DeleteZoneFunc == nil {
		return ns.Service.DeleteZone(ctx, name, extra)
	}
	return ns.DeleteZoneFunc(ctx, name, extra)
}

// NavigateToLocation calls the injected NavigateToLocationFunc or the real version.
func (ns *NavigationService) NavigateToLocation(ctx context.Context, name string, extra map[string]interface{}) (navigation.Status, error) {
	if ns.NavigateToLocationFunc == nil {
		return ns.Service.NavigateToLocation(ctx, name, extra)
	}
	return ns.NavigateToLocationFunc(ctx, name, extra)
}

// NavigateToPoint calls the injected NavigateToPointFunc or the real version.
func (ns *NavigationService) NavigateToPoint(ctx context.Context, destination navigation.Position, extra map[string]interface{}) (navigation.Status, error) {
	if ns.NavigateToPointFunc == nil {
		return ns.Service.NavigateToPoint(ctx, destination, extra)
	}
	return ns.NavigateToPointFunc(ctx, destination, extra)
}

// NavigateRoute calls the injected NavigateRouteFunc or the real version.
func (ns *NavigationService) NavigateRoute(ctx context.Context, req navigation.NavigateRouteReq) (navigation.Status, error) {
	if ns.NavigateRouteFunc == nil {
		return ns.Service.NavigateRoute(ctx, req)
	}
	return ns.NavigateRouteFunc(ctx, req)
}

// PlanToPoint calls the injected PlanToPointFunc or the real version.
func (ns *NavigationService) PlanToPoint(ctx context.Context, req navigation.PlanToPointReq) (navigation.Plan, error) {
	if ns.PlanToPointFunc == nil {
		return ns.Service.PlanToPoint(ctx, req)
	}
	return ns.PlanToPointFunc(ctx, req)
}

// PlanToLocation calls the injected PlanToLocationFunc or the real version.
func (ns *NavigationService) PlanToLocation(ctx context.Context, req navigation.PlanToLocationReq) (navigation.Plan, error) {
	if ns.PlanToLocationFunc == nil {
		return ns.Service.PlanToLocation(ctx, req)
	}
	return ns.PlanToLocationFunc(ctx, req)
}

// ExecutePlan calls the injected ExecutePlanFunc or the real version.
func (ns *NavigationService) ExecutePlan(ctx context.Context, plan navigation.Plan, extra map[string]interface{}) (navigation.Status, error) {
	if ns.ExecutePlanFunc == nil {
		return ns.Service.ExecutePlan(ctx, plan, extra)
	}
	return ns.ExecutePlanFunc(ctx, plan, extra)
}

// GoToLocation calls the injected GoToLocationFunc or the real version.
func (ns *NavigationService) GoToLocation(ctx context.Context, req navigation.GoToReq) (navigation.Status, error) {
	if ns.GoToLocationFunc == nil {
		return ns.Service.GoToLocation(ctx, req)
	}
	return ns.GoToLocationFunc(ctx, req)
}

// GoToPoint calls the injected GoToPointFunc or the real version.
func (ns *NavigationService) GoToPoint(ctx context.Context, req navigation.GoToReq) (navigation.Status, error) {
	if ns.GoToPointFunc == nil {
		return ns.Service.GoToPoint(ctx, req)
	}
	return ns.GoToPointFunc(ctx, req)
}

// Suspend calls the injected SuspendFunc or the real version.
func (ns *NavigationService) Suspend(ctx context.Context, reason string, extra map[string]interface{}) (navigation.Status, error) {
	if ns.SuspendFunc == nil {
		return ns.Service.Suspend(ctx, reason, extra)
	}
	return ns.SuspendFunc(ctx, reason, extra)
}

// Resume calls the injected ResumeFunc or the real version.
func (ns *NavigationService) Resume(ctx context.Context, extra map[string]interface{}) (navigation.Status, error) {
	if ns.ResumeFunc == nil {
		return ns.Service.Resume(ctx, extra)
	}
	return ns.ResumeFunc(ctx, extra)
}

// Cancel calls the injected CancelFunc or the real version.
func (ns *NavigationService) Cancel(ctx context.Context, extra map[string]interface{}) (navigation.Status, error) {
	if ns.CancelFunc == nil {
		return ns.Service.Cancel(ctx, extra)
	}
	return ns.CancelFunc(ctx, extra)
}

// GetPosition calls the injected GetPositionFunc or the real version.
func (ns *NavigationService) GetPosition(ctx context.Context, extra map[string]interface{}) (navigation.Position, error) {
	if ns.GetPositionFunc == nil {
		return ns.Service.GetPosition(ctx, extra)
	}
	return ns.GetPositionFunc(ctx, extra)
}

// GetNavigationStatus calls the injected GetNavigationStatusFunc or the real version.
func (ns *NavigationService) GetNavigationStatus(ctx context.Context, extra map[string]interface{}) (navigation.Status, error) {
	if ns.GetNavigationStatusFunc == nil {
		return ns.Service.GetNavigationStatus(ctx, extra)
	}
	return ns.GetNavigationStatusFunc(ctx, extra)
}

// GetProperties calls the injected GetPropertiesFunc or the real version.
func (ns *NavigationService) GetProperties(ctx context.Context) (navigation.Properties, error) {
	if ns.GetPropertiesFunc == nil {
		return ns.Service.GetProperties(ctx)
	}
	return ns.GetPropertiesFunc(ctx)
}

// GetCostmap calls the injected GetCostmapFunc or the real version.
func (ns *NavigationService) GetCostmap(ctx context.Context, req navigation.CostmapReq) (navigation.Costmap, error) {
	if ns.GetCostmapFunc == nil {
		return ns.Service.GetCostmap(ctx, req)
	}
	return ns.GetCostmapFunc(ctx, req)
}

// DoCommand calls the injected DoCommand or the real variant.
func (ns *NavigationService) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if ns.DoCommandFunc == nil {
		return ns.Service.DoCommand(ctx, cmd)
	}
	return ns.DoCommandFunc(ctx, cmd)
}

// Status calls the injected Status or the real version.
func (ns *NavigationService) Status(ctx context.Context) (map[string]interface{}, error) {
	if ns.StatusFunc != nil {
		return ns.StatusFunc(ctx)
	}
	if ns.Service != nil {
		return ns.Service.Status(ctx)
	}
	return map[string]interface{}{}, nil
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
