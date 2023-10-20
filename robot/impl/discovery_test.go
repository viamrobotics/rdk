package robotimpl_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
)

func setupNewLocalRobot(t *testing.T) robot.LocalRobot {
	t.Helper()

	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	return r
}

var (
	workingAPI   = resource.APINamespace("acme").WithComponentType("working-discovery")
	workingModel = resource.DefaultModelFamily.WithModel("workingModel")
	workingQ     = resource.NewDiscoveryQuery(workingAPI, workingModel)

	failAPI   = resource.APINamespace("acme").WithComponentType("failing-discovery")
	failModel = resource.DefaultModelFamily.WithModel("failModel")
	failQ     = resource.NewDiscoveryQuery(failAPI, failModel)

	noDiscoverModel = resource.DefaultModelFamily.WithModel("nodiscoverModel")
	noDiscoverQ     = resource.DiscoveryQuery{failAPI, noDiscoverModel}

	missingQ = resource.NewDiscoveryQuery(failAPI, resource.DefaultModelFamily.WithModel("missing"))

	workingDiscovery = map[string]interface{}{"position": "up"}
	errFailed        = errors.New("can't get discovery")
)

func init() {
	resource.Register(workingQ.API, workingQ.Model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.ZapCompatibleLogger,
		) (resource.Resource, error) {
			return nil, errors.New("no")
		},
		Discover: func(ctx context.Context, logger logging.ZapCompatibleLogger) (interface{}, error) {
			return workingDiscovery, nil
		},
	})

	resource.Register(failQ.API, failQ.Model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.ZapCompatibleLogger,
		) (resource.Resource, error) {
			return nil, errors.New("no")
		},
		Discover: func(ctx context.Context, logger logging.ZapCompatibleLogger) (interface{}, error) {
			return nil, errFailed
		},
	})
}

func TestDiscovery(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		r := setupNewLocalRobot(t)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()
		discoveries, err := r.DiscoverComponents(context.Background(), []resource.DiscoveryQuery{missingQ})
		test.That(t, discoveries, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("no Discover", func(t *testing.T) {
		r := setupNewLocalRobot(t)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()

		discoveries, err := r.DiscoverComponents(context.Background(), []resource.DiscoveryQuery{noDiscoverQ})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("failing Discover", func(t *testing.T) {
		r := setupNewLocalRobot(t)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()

		_, err := r.DiscoverComponents(context.Background(), []resource.DiscoveryQuery{failQ})
		test.That(t, err, test.ShouldBeError, &resource.DiscoverError{failQ})
	})

	t.Run("working Discover", func(t *testing.T) {
		r := setupNewLocalRobot(t)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()

		discoveries, err := r.DiscoverComponents(context.Background(), []resource.DiscoveryQuery{workingQ})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldResemble, []resource.Discovery{{Query: workingQ, Results: workingDiscovery}})
	})

	t.Run("duplicated working Discover", func(t *testing.T) {
		r := setupNewLocalRobot(t)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()

		discoveries, err := r.DiscoverComponents(context.Background(), []resource.DiscoveryQuery{workingQ, workingQ, workingQ})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldResemble, []resource.Discovery{{Query: workingQ, Results: workingDiscovery}})
	})

	t.Run("working and missing Discover", func(t *testing.T) {
		r := setupNewLocalRobot(t)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()

		discoveries, err := r.DiscoverComponents(context.Background(), []resource.DiscoveryQuery{workingQ, missingQ})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldResemble, []resource.Discovery{{Query: workingQ, Results: workingDiscovery}})
	})
}
