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
	workingQ       = discovery.NewQuery(workingSubtype.ResourceSubtype, workingModel)

	failSubtype = resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("fail"))
	failModel   = "failModel"
	failQ       = discovery.NewQuery(failSubtype.ResourceSubtype, failModel)

	noDiscoverSubtype = resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("nodiscover"))
	noDiscoverModel   = "nodiscover"
	noDiscoverQ       = discovery.Query{failSubtype.ResourceSubtype, noDiscoverModel}

	missingQ = discovery.NewQuery(failSubtype.ResourceSubtype, "missing")

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
		workingQ,
		func(ctx context.Context) (interface{}, error) { return workingDiscovery, nil },
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
		failQ,
		func(ctx context.Context) (interface{}, error) { return nil, errFailed },
	)
}

func TestDiscovery(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		r := setupNewLocalRobot(t)
		discoveries, err := r.Discover(context.Background(), []discovery.Query{missingQ})
		test.That(t, discoveries, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("no Discover", func(t *testing.T) {
		r := setupNewLocalRobot(t)

		discoveries, err := r.Discover(context.Background(), []discovery.Query{noDiscoverQ})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("failing Discover", func(t *testing.T) {
		r := setupNewLocalRobot(t)

		_, err := r.Discover(context.Background(), []discovery.Query{failQ})
		test.That(t, err, test.ShouldBeError, &discovery.DiscoverError{failQ})
	})

	t.Run("working Discover", func(t *testing.T) {
		r := setupNewLocalRobot(t)

		discoveries, err := r.Discover(context.Background(), []discovery.Query{workingQ})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldResemble, []discovery.Discovery{{Query: workingQ, Discovered: workingDiscovery}})
	})

	t.Run("duplicated working Discover", func(t *testing.T) {
		r := setupNewLocalRobot(t)

		discoveries, err := r.Discover(context.Background(), []discovery.Query{workingQ, workingQ, workingQ})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldResemble, []discovery.Discovery{{Query: workingQ, Discovered: workingDiscovery}})
	})

	t.Run("working and missing Discover", func(t *testing.T) {
		r := setupNewLocalRobot(t)

		discoveries, err := r.Discover(context.Background(), []discovery.Query{workingQ, missingQ})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldResemble, []discovery.Discovery{{Query: workingQ, Discovered: workingDiscovery}})
	})
}
