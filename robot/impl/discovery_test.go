package robotimpl_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/config"

	"go.viam.com/rdk/discovery"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
)

func setupNewLocalRobot(t *testing.T) robot.LocalRobot {
	t.Helper()

	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	return r
}

var (
	workingSubtype = resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("working"))
	workingModel   = "workingModel"
	workingKey     = discovery.Key{workingSubtype.ResourceSubtype, workingModel}

	failSubtype = resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("fail"))
	failModel   = "failModel"
	failKey     = discovery.Key{failSubtype.ResourceSubtype, failModel}

	noDiscoverSubtype = resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("nodiscover"))
	noDiscoverModel   = "nodiscover"
	noDiscoverKey     = discovery.Key{failSubtype.ResourceSubtype, noDiscoverModel}

	missingKey = discovery.Key{failSubtype.ResourceSubtype, "missing"}

	workingDiscovery = map[string]interface{}{"position": "up"}
	errFailed        = errors.New("can't get discovery")

	mockReconfigurable = func(resource interface{}) (resource.Reconfigurable, error) {
		return nil, nil
	}
)

func init() {
	// Subtype with a working discovery function for a subtype model
	registry.RegisterResourceSubtype(
		workingSubtype,
		registry.ResourceSubtype{Reconfigurable: mockReconfigurable},
	)

	registry.RegisterDiscoveryFunction(
		workingSubtype.ResourceSubtype,
		workingModel,
		func(ctx context.Context, subtypeName resource.SubtypeName, model string) (interface{}, error) {
			return workingDiscovery, nil
		},
	)

	// Subtype without discovery function
	registry.RegisterResourceSubtype(
		noDiscoverSubtype,
		registry.ResourceSubtype{Reconfigurable: mockReconfigurable},
	)

	// Subtype with a failing discovery function for a subtype model
	registry.RegisterResourceSubtype(
		failSubtype,
		registry.ResourceSubtype{Reconfigurable: mockReconfigurable},
	)

	registry.RegisterDiscoveryFunction(
		failSubtype.ResourceSubtype,
		failModel,
		func(ctx context.Context, subtypeName resource.SubtypeName, model string) (interface{}, error) {
			return nil, errFailed
		},
	)
}

func TestDiscovery(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		r := setupNewLocalRobot(t)
		discoveries, err := r.Discover(context.Background(), []discovery.Key{missingKey})
		test.That(t, discoveries, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("no Discover", func(t *testing.T) {
		r := setupNewLocalRobot(t)

		discoveries, err := r.Discover(context.Background(), []discovery.Key{noDiscoverKey})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("failing Discover", func(t *testing.T) {
		r := setupNewLocalRobot(t)

		_, err := r.Discover(context.Background(), []discovery.Key{failKey})
		test.That(t, err, test.ShouldBeError, &discovery.DiscoverError{failKey})
	})

	t.Run("working Discover", func(t *testing.T) {
		r := setupNewLocalRobot(t)

		discoveries, err := r.Discover(context.Background(), []discovery.Key{workingKey})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldResemble, []discovery.Discovery{{Key: workingKey, Discovered: workingDiscovery}})
	})

	t.Run("duplicated working Discover", func(t *testing.T) {
		r := setupNewLocalRobot(t)

		discoveries, err := r.Discover(context.Background(), []discovery.Key{workingKey, workingKey, workingKey})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldResemble, []discovery.Discovery{{Key: workingKey, Discovered: workingDiscovery}})
	})

	t.Run("working and missing Discover", func(t *testing.T) {
		r := setupNewLocalRobot(t)

		discoveries, err := r.Discover(context.Background(), []discovery.Key{workingKey, missingKey})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldResemble, []discovery.Discovery{{Key: workingKey, Discovered: workingDiscovery}})
	})
}
