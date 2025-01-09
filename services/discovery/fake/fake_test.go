package fake

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/discovery"
)

func TestDiscoverResources(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	dis := newDiscovery(discovery.Named("foo"), logger)

	expectedCfgs := []resource.Config{
		createFakeConfig("fake1", movementsensor.API, nil),
		createFakeConfig("fake2", camera.API, nil),
	}
	cfgs, err := dis.DiscoverResources(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(cfgs), test.ShouldEqual, len(expectedCfgs))
	for index, cfg := range cfgs {
		test.That(t, cfg.Name, test.ShouldEqual, expectedCfgs[index].Name)
		test.That(t, cfg.API, test.ShouldResemble, expectedCfgs[index].API)
		test.That(t, cfg.Model, test.ShouldResemble, expectedCfgs[index].Model)
	}
}
