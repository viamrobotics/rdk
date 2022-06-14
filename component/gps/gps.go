// Package gps defines the interfaces of a GPS device which provides lat/long
// measurements.
package gps

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/sensor"
	pb "go.viam.com/rdk/proto/api/component/gps/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.GPSService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterGPSServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

// SubtypeName is a constant that identifies the component resource subtype string "gps".
const SubtypeName = resource.SubtypeName("gps")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named GPS's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// A GPS represents a GPS that can report lat/long measurements.
type GPS interface {
	ReadLocation(ctx context.Context) (*geo.Point, error) // The current latitude and longitude
	ReadAltitude(ctx context.Context) (float64, error)    // The current altitude in meters
	ReadSpeed(ctx context.Context) (float64, error)       // Current ground speed in mm per sec
	generic.Generic
}

// A LocalGPS represents a GPS that can report accuracy, satellites and valid measurements.
type LocalGPS interface {
	GPS
	ReadAccuracy(ctx context.Context) (float64, float64, error) // Horizontal and vertical position error in meters
	ReadSatellites(ctx context.Context) (int, int, error)       // Number of satellites used for fix, and total in view
	ReadValid(ctx context.Context) (bool, error)                // Whether or not the GPS chip had a valid fix for the most recent dataset
}

var (
	_ = LocalGPS(&reconfigurableGPS{})
	_ = sensor.Sensor(&reconfigurableGPS{})
	_ = resource.Reconfigurable(&reconfigurableGPS{})
)

// FromDependencies is a helper for getting the named gps from a collection of
// dependencies.
func FromDependencies(deps registry.Dependencies, name string) (GPS, error) {
	res, ok := deps[Named(name)]
	if !ok {
		return nil, errors.Errorf("gps %q missing from dependencies", name)
	}
	b, ok := res.(GPS)
	if !ok {
		return nil, errors.Errorf("%q is not a gps", name)
	}
	return b, nil
}

// FromRobot is a helper for getting the named GPS from the given Robot.
func FromRobot(r robot.Robot, name string) (GPS, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(GPS)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("GPS", res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all GPS names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// GetReadings is a helper for getting all readings from a GPS.
func GetReadings(ctx context.Context, g GPS) ([]interface{}, error) {
	loc, err := g.ReadLocation(ctx)
	if err != nil {
		return nil, err
	}
	alt, err := g.ReadAltitude(ctx)
	if err != nil {
		return nil, err
	}
	speed, err := g.ReadSpeed(ctx)
	if err != nil {
		return nil, err
	}

	readings := []interface{}{loc.Lat(), loc.Lng(), alt, speed}

	localG, ok := g.(LocalGPS)
	if !ok {
		return readings, nil
	}

	active, total, err := localG.ReadSatellites(ctx)
	if err != nil {
		return nil, err
	}
	hAcc, vAcc, err := localG.ReadAccuracy(ctx)
	if err != nil {
		return nil, err
	}
	valid, err := localG.ReadValid(ctx)
	if err != nil {
		return nil, err
	}

	return append(readings, active, total, hAcc, vAcc, valid), nil
}

type reconfigurableGPS struct {
	mu     sync.RWMutex
	actual LocalGPS
}

func (r *reconfigurableGPS) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableGPS) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableGPS) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Do(ctx, cmd)
}

func (r *reconfigurableGPS) ReadLocation(ctx context.Context) (*geo.Point, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadLocation(ctx)
}

func (r *reconfigurableGPS) ReadAltitude(ctx context.Context) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadAltitude(ctx)
}

func (r *reconfigurableGPS) ReadSpeed(ctx context.Context) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadSpeed(ctx)
}

func (r *reconfigurableGPS) ReadSatellites(ctx context.Context) (int, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadSatellites(ctx)
}

func (r *reconfigurableGPS) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadAccuracy(ctx)
}

func (r *reconfigurableGPS) ReadValid(ctx context.Context) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadValid(ctx)
}

// GetReadings will use the default GPS GetReadings if not provided.
func (r *reconfigurableGPS) GetReadings(ctx context.Context) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if sensor, ok := r.actual.(sensor.Sensor); ok {
		return sensor.GetReadings(ctx)
	}
	return GetReadings(ctx, r.actual)
}

func (r *reconfigurableGPS) Reconfigure(ctx context.Context, newGPS resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newGPS.(*reconfigurableGPS)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newGPS)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular LocalGPS implementation to a reconfigurableGPS.
// If GPS is already a reconfigurableGPS, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	gps, ok := r.(LocalGPS)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("LocalGPS", r)
	}
	if reconfigurable, ok := gps.(*reconfigurableGPS); ok {
		return reconfigurable, nil
	}
	return &reconfigurableGPS{actual: gps}, nil
}
