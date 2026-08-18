package robotimpl

import (
	"context"
	"sync/atomic"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// multiAPIModel is a builtin model that serves two co-equal APIs from one instance: the sensor API
// (Readings) and the generic API (DoCommand). It is registered via resource.RegisterMultiAPI.
var multiAPIModel = resource.DefaultModelFamily.WithModel("multiapi_integration")

// multiAPICloseCount counts Close calls on instances of multiAPIThing, to verify a composite
// instance is closed exactly once even though it is reachable under multiple APIs.
var multiAPICloseCount atomic.Int64

type multiAPIThing struct {
	resource.Named
	resource.TriviallyReconfigurable
}

func (m *multiAPIThing) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"reading": 42}, nil
}

func (m *multiAPIThing) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"did": "it"}, nil
}

func (m *multiAPIThing) Close(ctx context.Context) error {
	multiAPICloseCount.Add(1)
	return nil
}

// TestMultiAPIResourceEndToEnd verifies, through a real local robot, that one model serving a set of
// co-equal APIs is a single instance reachable and advertised under each of its APIs — configured
// with NO `api` field (resolved from the model) — and that it advertises both APIs, and is closed
// exactly once when removed.
func TestMultiAPIResourceEndToEnd(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	multiAPICloseCount.Store(0)

	resource.RegisterMultiAPI(
		[]resource.API{sensor.API, generic.API},
		multiAPIModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (resource.Resource, error) {
				return &multiAPIThing{Named: conf.ResourceName().AsNamed()}, nil
			},
		},
	)
	defer func() {
		resource.Deregister(sensor.API, multiAPIModel)
		resource.Deregister(generic.API, multiAPIModel)
	}()

	// A composite config entry: name + model, NO api. Ensure runs the same validation/partial-name
	// resolution the production config path (config.Read) does, which fills the API from the model.
	cfg := &config.Config{
		Components: []resource.Config{
			{Name: "combo", Model: multiAPIModel},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	r := setupLocalRobot(t, ctx, cfg, logger)

	// Advertised under BOTH APIs in ResourceNames.
	names := r.ResourceNames()
	test.That(t, names, test.ShouldContain, sensor.Named("combo"))
	test.That(t, names, test.ShouldContain, generic.Named("combo"))

	// (item 2) Advertised under both APIs in ResourceRPCAPIs (what a connecting client discovers).
	var advertised []resource.API
	for _, rpcAPI := range r.ResourceRPCAPIs() {
		advertised = append(advertised, rpcAPI.API)
	}
	test.That(t, advertised, test.ShouldContain, sensor.API)
	test.That(t, advertised, test.ShouldContain, generic.API)

	// Reachable and usable under the sensor API.
	s, err := sensor.FromProvider(r, "combo")
	test.That(t, err, test.ShouldBeNil)
	readings, err := s.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["reading"], test.ShouldEqual, 42)

	// Reachable under the generic API — the SAME instance.
	g, err := generic.FromProvider(r, "combo")
	test.That(t, err, test.ShouldBeNil)
	resp, err := g.DoCommand(ctx, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["did"], test.ShouldEqual, "it")

	// Both API lookups resolve to one underlying instance.
	viaSensor, err := r.ResourceByName(sensor.Named("combo"))
	test.That(t, err, test.ShouldBeNil)
	viaGeneric, err := r.ResourceByName(generic.Named("combo"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, viaSensor, test.ShouldEqual, viaGeneric)

	// (item 5) Removing the resource (reconfigure to empty) tears it down under both APIs and closes
	// the single instance exactly once.
	empty := &config.Config{}
	test.That(t, empty.Ensure(false, logger), test.ShouldBeNil)
	r.Reconfigure(ctx, empty)

	namesAfter := r.ResourceNames()
	test.That(t, namesAfter, test.ShouldNotContain, sensor.Named("combo"))
	test.That(t, namesAfter, test.ShouldNotContain, generic.Named("combo"))
	test.That(t, multiAPICloseCount.Load(), test.ShouldEqual, 1)
}
