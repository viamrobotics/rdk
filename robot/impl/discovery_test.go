package robotimpl

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	modulepb "go.viam.com/api/module/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	rtestutils "go.viam.com/rdk/testutils"
)

func setupNewLocalRobot(t *testing.T) robot.LocalRobot {
	t.Helper()

	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := New(context.Background(), cfg, logger)
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

	modManagerAPI   = resource.NewAPI("rdk-internal", "service", "module-manager")
	modManagerModel = resource.NewModel("rdk-internal", "builtin", "module-manager")
	modManagerQ     = resource.NewDiscoveryQuery(modManagerAPI, modManagerModel)

	missingQ = resource.NewDiscoveryQuery(failAPI, resource.DefaultModelFamily.WithModel("missing"))

	workingDiscovery = map[string]interface{}{"position": "up"}
	errFailed        = errors.New("can't get discovery")
)

func init() {
	resource.Register(workingQ.API, workingQ.Model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
		) (resource.Resource, error) {
			return nil, errors.New("no")
		},
		Discover: func(ctx context.Context, logger logging.Logger) (interface{}, error) {
			return workingDiscovery, nil
		},
	})

	resource.Register(failQ.API, failQ.Model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
		) (resource.Resource, error) {
			return nil, errors.New("no")
		},
		Discover: func(ctx context.Context, logger logging.Logger) (interface{}, error) {
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

	t.Run("internal module manager Discover", func(t *testing.T) {
		r := setupNewLocalRobot(t)
		ctx := context.Background()
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()

		// test with empty modmanager
		discoveries, err := r.DiscoverComponents(context.Background(), []resource.DiscoveryQuery{modManagerQ})
		test.That(t, err, test.ShouldBeNil)
		expectedHandlerMap := map[string]modulepb.HandlerMap{}
		expectedDiscovery := moduleManagerDiscoveryResult{
			ResourceHandles: expectedHandlerMap,
		}
		test.That(t, discoveries, test.ShouldResemble, []resource.Discovery{{Query: modManagerQ, Results: expectedDiscovery}})

		// add modules
		complexPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/complexmodule")
		simplePath := rtestutils.BuildTempModule(t, "examples/customresources/demos/simplemodule")
		cfg := &config.Config{
			Modules: []config.Module{
				{
					Name:    "simple",
					ExePath: simplePath,
				},
				{
					Name:    "complex",
					ExePath: complexPath,
				},
			},
		}
		r.Reconfigure(ctx, cfg)

		// rerun discovery expecting a full tree of resources
		discoveries, err = r.DiscoverComponents(context.Background(), []resource.DiscoveryQuery{modManagerQ})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldHaveLength, 1)
		modManagerDiscovery, ok := discoveries[0].Results.(moduleManagerDiscoveryResult)
		test.That(t, ok, test.ShouldBeTrue)
		resourceHandles := modManagerDiscovery.ResourceHandles
		test.That(t, resourceHandles, test.ShouldHaveLength, 2)
		//nolint:govet // we copy an internal lock -- it is okay
		simpleHandles, ok := resourceHandles["simple"]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, simpleHandles.Handlers, test.ShouldHaveLength, 1)
		test.That(t, simpleHandles.Handlers[0].Models, test.ShouldResemble, []string{"acme:demo:mycounter"})
		test.That(t, simpleHandles.Handlers[0].Subtype.Subtype.Namespace, test.ShouldResemble, "rdk")
		test.That(t, simpleHandles.Handlers[0].Subtype.Subtype.Type, test.ShouldResemble, "component")
		test.That(t, simpleHandles.Handlers[0].Subtype.Subtype.Subtype, test.ShouldResemble, "generic")

		//nolint:govet // we copy an internal lock -- it is okay
		complexHandles, ok := resourceHandles["complex"]
		test.That(t, ok, test.ShouldBeTrue)
		// confirm that complex handlers are also present in the map
		test.That(t, len(complexHandles.Handlers), test.ShouldBeGreaterThan, 1)
	})
}
