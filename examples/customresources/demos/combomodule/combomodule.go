// Package main is a module demonstrating a composite that spans a colliding pair AND a custom API:
// one model serving three co-equal APIs from a single identity. combodevice serves
// rdk:component:camera, rdk:component:movement_sensor, and the custom acme:component:gizmo API.
// camera and movement_sensor collide on Properties — both declare a Properties method with DIFFERENT
// return types, so neither can live on the shared struct — while gizmo is a module-defined (custom)
// API alongside them, showing composites span builtin and custom APIs alike. Each API's methods are
// carried by a thin per-API facade embedding the one shared *comboDevice, and resource.Compose
// assembles them into one composite advertised under all three APIs.
package main

import (
	"context"
	"fmt"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/movementsensor"
	gizmoapi "go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// Model is registered under every API it serves via RegisterMultiAPI.
var Model = resource.NewModel("acme", "demo", "combodevice")

// wantArg is the gizmo argument the demo treats as a match; a named constant keeps the trivial demo
// logic in one place.
const wantArg = "combo"

func main() {
	// One registration declares the full co-equal API set; the single constructor is registered under
	// each. camera and movement_sensor collide on Properties (the point of this demo), and the custom
	// gizmo API sits alongside them to show a composite spans builtin and module-defined APIs alike.
	resource.RegisterMultiAPI(
		[]resource.API{camera.API, movementsensor.API, gizmoapi.API}, Model,
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
	// Compose builds one composite from the three per-API facades; all embed the one shared
	// *comboDevice so the composite is one device with one lifecycle, reachable under camera,
	// movement_sensor, and the custom gizmo API.
	return resource.Compose(
		conf.ResourceName(),
		camera.AsSub(camFacade{d}),
		movementsensor.AsSub(imuFacade{d}),
		gizmoapi.AsSub(gizmoFacade{d}),
	)
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

// gizmoFacade carries the custom acme:component:gizmo API's methods. Gizmo's method names do not
// collide with camera's or movement_sensor's, but it rides on a facade like the others so the module
// fans a correctly-typed per-API sub-resource out for every API. It embeds the shared *comboDevice
// for Name/DoCommand/Reconfigure/Close, so gizmoFacade satisfies gizmoapi.Gizmo.
type gizmoFacade struct{ *comboDevice }

func (f gizmoFacade) DoOne(_ context.Context, arg1 string) (bool, error) {
	return arg1 == wantArg, nil
}

func (f gizmoFacade) DoOneClientStream(_ context.Context, arg1 []string) (bool, error) {
	for _, arg := range arg1 {
		if arg != wantArg {
			return false, nil
		}
	}
	return len(arg1) > 0, nil
}

func (f gizmoFacade) DoOneServerStream(_ context.Context, arg1 string) ([]bool, error) {
	return []bool{arg1 == wantArg, false}, nil
}

func (f gizmoFacade) DoOneBiDiStream(_ context.Context, arg1 []string) ([]bool, error) {
	rets := make([]bool, 0, len(arg1))
	for _, arg := range arg1 {
		rets = append(rets, arg == wantArg)
	}
	return rets, nil
}

func (f gizmoFacade) DoTwo(_ context.Context, arg1 bool) (string, error) {
	return fmt.Sprintf("arg1=%t", arg1), nil
}
