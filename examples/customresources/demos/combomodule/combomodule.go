// Package main is a module demonstrating a colliding composite: one model serving two co-equal APIs
// whose method sets collide by name. combodevice serves rdk:component:camera and
// rdk:component:movement_sensor from a single identity; both APIs declare a Properties method with
// DIFFERENT return types, so neither can live on the shared struct. Each colliding method is carried
// by a thin per-API facade embedding the one shared *comboDevice, and resource.Compose assembles them
// into one composite advertised under both APIs.
package main

import (
	"context"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// Model is registered under every API it serves via RegisterMultiAPI.
var Model = resource.NewModel("acme", "demo", "combodevice")

func main() {
	// One registration declares the full co-equal API set; the single constructor is registered under
	// each. camera and movement_sensor collide on Properties, which is the whole point of this demo.
	resource.RegisterMultiAPI(
		[]resource.API{camera.API, movementsensor.API}, Model,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: newComboDevice,
		},
	)

	// ExpandModel reads back every API this Model was registered under, so main() never re-lists them —
	// the module advertises the composite on all of its APIs.
	module.ModularMain(resource.ExpandModel(Model)...)
}

func newComboDevice(
	_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
) (resource.Resource, error) {
	d := &comboDevice{Named: conf.ResourceName().AsNamed()}
	// Compose builds one composite from the two per-API facades; both embed the one shared *comboDevice
	// so the composite is one device with one lifecycle, reachable under camera and movement_sensor.
	return resource.Compose(conf.ResourceName(), camera.AsSub(camFacade{d}), movementsensor.AsSub(imuFacade{d}))
}

// comboDevice holds the shared state and every NON-colliding method of both APIs. resource.Named
// supplies Name/DoCommand/Status; AlwaysRebuild supplies Reconfigure; TriviallyCloseable supplies
// Close. It deliberately has no Properties method — that is the colliding method, carried by the
// facades below.
type comboDevice struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
}

// camera (rdk:component:camera) non-colliding methods.

func (d *comboDevice) Images(
	context.Context, []string, map[string]interface{},
) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	return nil, resource.ResponseMetadata{}, nil
}

func (d *comboDevice) NextPointCloud(context.Context, map[string]interface{}) (pointcloud.PointCloud, error) {
	return nil, nil
}

func (d *comboDevice) Geometries(context.Context, map[string]interface{}) ([]spatialmath.Geometry, error) {
	return nil, nil
}

// movement_sensor (rdk:component:movement_sensor) non-colliding methods.

func (d *comboDevice) Position(context.Context, map[string]interface{}) (*geo.Point, float64, error) {
	return geo.NewPoint(0, 0), 0, nil
}

func (d *comboDevice) LinearVelocity(context.Context, map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, nil
}

func (d *comboDevice) AngularVelocity(context.Context, map[string]interface{}) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{}, nil
}

func (d *comboDevice) LinearAcceleration(context.Context, map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, nil
}

func (d *comboDevice) CompassHeading(context.Context, map[string]interface{}) (float64, error) {
	return 0, nil
}

func (d *comboDevice) Orientation(context.Context, map[string]interface{}) (spatialmath.Orientation, error) {
	return spatialmath.NewZeroOrientation(), nil
}

func (d *comboDevice) Accuracy(context.Context, map[string]interface{}) (*movementsensor.Accuracy, error) {
	return &movementsensor.Accuracy{}, nil
}

func (d *comboDevice) Readings(context.Context, map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"reading": 7}, nil
}

// camFacade carries the camera API's Properties method. It embeds the shared *comboDevice for every
// other camera method, so camFacade satisfies camera.Camera.
type camFacade struct{ *comboDevice }

// Properties returns the camera Properties (distinguishable from the movement-sensor Properties by its
// FrameRate). This is the colliding method routed to the camera facade.
func (f camFacade) Properties(context.Context) (camera.Properties, error) {
	return camera.Properties{SupportsPCD: true, FrameRate: 30}, nil
}

// imuFacade carries the movement_sensor API's Properties method. It embeds the shared *comboDevice for
// every other movement-sensor method, so imuFacade satisfies movementsensor.MovementSensor.
type imuFacade struct{ *comboDevice }

// Properties returns the movement-sensor Properties (a different return type than the camera's, and
// distinguishable by AngularVelocitySupported). This is the colliding method routed to the IMU facade.
func (f imuFacade) Properties(context.Context, map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{PositionSupported: true, AngularVelocitySupported: true}, nil
}
