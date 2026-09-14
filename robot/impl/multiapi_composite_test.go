package robotimpl

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var comboModel = resource.NewModel("acme", "test", "composite-sensor")

// comboSensor serves both sensor.Sensor and generic.Resource from one identity.
type comboSensor struct {
	resource.Named
	resource.AlwaysRebuild
}

func (c *comboSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"reading": 7}, nil
}

func (c *comboSensor) Close(ctx context.Context) error { return nil }

func TestCompositeResourceEndToEnd(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	resource.RegisterMultiAPI(
		[]resource.API{sensor.API, generic.API}, comboModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (resource.Resource, error) {
				return &comboSensor{Named: conf.ResourceName().AsNamed()}, nil
			},
		},
	)
	defer resource.Deregister(sensor.API, comboModel)
	defer resource.Deregister(generic.API, comboModel)

	cfg := &config.Config{
		Components: []resource.Config{
			{
				Name:      "combo",
				API:       sensor.API,
				Model:     comboModel,
				Composite: true,
			},
		},
	}

	r := setupLocalRobot(t, ctx, cfg, logger)

	// 1) A bare (API-less) name resolves to the one handle; AsType extracts the sensor interface.
	res, err := r.ResourceByName(resource.SimpleName("combo"))
	test.That(t, err, test.ShouldBeNil)
	s, err := resource.AsType[sensor.Sensor](res)
	test.That(t, err, test.ShouldBeNil)
	readings, err := s.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["reading"], test.ShouldEqual, 7)

	// 2) The composite is reachable under its OTHER (non-canonical) API via the graph's read-time
	// resolution — same single instance.
	byGeneric, err := r.ResourceByName(resource.NewName(generic.API, "combo"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, byGeneric, test.ShouldEqual, res)

	// 3) And under its canonical API.
	bySensor, err := r.ResourceByName(sensor.Named("combo"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bySensor, test.ShouldEqual, res)

	// 4) It is advertised as N same-named ResourceNames (one per co-equal API).
	var apis []resource.API
	for _, n := range r.ResourceNames() {
		if n.Name == "combo" {
			apis = append(apis, n.API)
		}
	}
	test.That(t, apis, test.ShouldContain, sensor.API)
	test.That(t, apis, test.ShouldContain, generic.API)
}
