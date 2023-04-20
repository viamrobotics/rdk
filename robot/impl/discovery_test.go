package robotimpl_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
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
	workingSubtype = resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("working-discovery"))
	workingModel   = resource.NewDefaultModel("workingModel")
	workingQ       = resource.NewDiscoveryQuery(workingSubtype, workingModel)

	failSubtype = resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("failing-discovery"))
	failModel   = resource.NewDefaultModel("failModel")
	failQ       = resource.NewDiscoveryQuery(failSubtype, failModel)

	noDiscoverModel = resource.NewDefaultModel("nodiscoverModel")
	noDiscoverQ     = resource.DiscoveryQuery{failSubtype, noDiscoverModel}

	missingQ = resource.NewDiscoveryQuery(failSubtype, resource.NewDefaultModel("missing"))

	workingDiscovery = map[string]interface{}{"position": "up"}
	errFailed        = errors.New("can't get discovery")
)

func init() {
	resource.Register(workingQ.API, workingQ.Model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (resource.Resource, error) {
			return nil, errors.New("no")
		},
		Discover: func(ctx context.Context, logger golog.Logger) (interface{}, error) { return workingDiscovery, nil },
	})

	resource.Register(failQ.API, failQ.Model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (resource.Resource, error) {
			return nil, errors.New("no")
		},
		Discover: func(ctx context.Context, logger golog.Logger) (interface{}, error) { return nil, errFailed },
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
