package robotimpl

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	modulepb "go.viam.com/api/module/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/examples/customresources/models/mybase"
	"go.viam.com/rdk/examples/customresources/models/mygizmo"
	"go.viam.com/rdk/examples/customresources/models/mynavigation"
	"go.viam.com/rdk/examples/customresources/models/mysum"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/navigation"
	rtestutils "go.viam.com/rdk/testutils"
)

func setupLocalRobotWithFakeConfig(t *testing.T) robot.LocalRobot {
	t.Helper()

	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	cfg, err := config.Read(ctx, "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)
	return setupLocalRobot(t, ctx, cfg, logger)
}

var (
	workingAPI   = resource.APINamespace("acme").WithComponentType("working-discovery")
	workingModel = resource.DefaultModelFamily.WithModel("workingModel")
	workingQ     = resource.NewDiscoveryQuery(workingAPI, workingModel, nil)

	failAPI   = resource.APINamespace("acme").WithComponentType("failing-discovery")
	failModel = resource.DefaultModelFamily.WithModel("failModel")
	failQ     = resource.NewDiscoveryQuery(failAPI, failModel, nil)

	noDiscoverModel = resource.DefaultModelFamily.WithModel("nodiscoverModel")
	noDiscoverQ     = resource.DiscoveryQuery{failAPI, noDiscoverModel, nil}

	modManagerAPI   = resource.NewAPI("rdk-internal", "service", "module-manager")
	modManagerModel = resource.NewModel("rdk-internal", "builtin", "module-manager")
	modManagerQ     = resource.NewDiscoveryQuery(modManagerAPI, modManagerModel, nil)

	missingQ = resource.NewDiscoveryQuery(failAPI, resource.DefaultModelFamily.WithModel("missing"), nil)

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
		Discover: func(ctx context.Context, logger logging.Logger, extra map[string]any) (interface{}, error) {
			return workingDiscovery, nil
		},
	})

	resource.Register(failQ.API, failQ.Model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
		) (resource.Resource, error) {
			return nil, errors.New("no")
		},
		Discover: func(ctx context.Context, logger logging.Logger, extra map[string]interface{}) (interface{}, error) {
			return nil, errFailed
		},
	})
}

func TestDiscovery(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		r := setupLocalRobotWithFakeConfig(t)
		discoveries, err := r.DiscoverComponents(context.Background(), []resource.DiscoveryQuery{missingQ})
		test.That(t, discoveries, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("no Discover", func(t *testing.T) {
		r := setupLocalRobotWithFakeConfig(t)
		discoveries, err := r.DiscoverComponents(context.Background(), []resource.DiscoveryQuery{noDiscoverQ})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("failing Discover", func(t *testing.T) {
		r := setupLocalRobotWithFakeConfig(t)
		_, err := r.DiscoverComponents(context.Background(), []resource.DiscoveryQuery{failQ})
		test.That(t, err, test.ShouldBeError, &resource.DiscoverError{Query: failQ, Cause: errFailed})
	})

	t.Run("working Discover", func(t *testing.T) {
		r := setupLocalRobotWithFakeConfig(t)
		discoveries, err := r.DiscoverComponents(context.Background(), []resource.DiscoveryQuery{workingQ})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldResemble, []resource.Discovery{{Query: workingQ, Results: workingDiscovery}})
	})

	t.Run("duplicated working Discover", func(t *testing.T) {
		r := setupLocalRobotWithFakeConfig(t)
		discoveries, err := r.DiscoverComponents(context.Background(), []resource.DiscoveryQuery{workingQ, workingQ, workingQ})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldResemble, []resource.Discovery{{Query: workingQ, Results: workingDiscovery}})
	})

	t.Run("working and missing Discover", func(t *testing.T) {
		r := setupLocalRobotWithFakeConfig(t)
		discoveries, err := r.DiscoverComponents(context.Background(), []resource.DiscoveryQuery{workingQ, missingQ})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldResemble, []resource.Discovery{{Query: workingQ, Results: workingDiscovery}})
	})

	t.Run("internal module manager Discover", func(t *testing.T) {
		r := setupLocalRobotWithFakeConfig(t)
		ctx := context.Background()

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

func TestGetModelsFromModules(t *testing.T) {
	t.Run("no modules configured", func(t *testing.T) {
		r := setupLocalRobotWithFakeConfig(t)
		models, err := r.GetModelsFromModules(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, models, test.ShouldBeEmpty)
	})
	t.Run("local and registry modules are configured", func(t *testing.T) {
		r := setupLocalRobotWithFakeConfig(t)
		ctx := context.Background()

		// add modules
		complexPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/complexmodule")
		simplePath := rtestutils.BuildTempModule(t, "examples/customresources/demos/simplemodule")
		cfg := &config.Config{
			Modules: []config.Module{
				{
					Name:    "simple",
					ExePath: simplePath,
					Type:    config.ModuleTypeRegistry,
				},
				{
					Name:    "complex",
					ExePath: complexPath,
					Type:    config.ModuleTypeLocal,
				},
			},
		}
		r.Reconfigure(ctx, cfg)
		models, err := r.GetModelsFromModules(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, models, test.ShouldHaveLength, 5)

		for _, model := range models {
			switch model.Model {
			case mygizmo.Model:
				test.That(t, model.FromLocalModule, test.ShouldEqual, true)
				test.That(t, model.ModuleName, test.ShouldEqual, "complex")
				test.That(t, model.API, test.ShouldResemble, gizmoapi.API)
			case mysum.Model:
				test.That(t, model.FromLocalModule, test.ShouldEqual, true)
				test.That(t, model.ModuleName, test.ShouldEqual, "complex")
				test.That(t, model.API, test.ShouldResemble, summationapi.API)
			case mybase.Model:
				test.That(t, model.FromLocalModule, test.ShouldEqual, true)
				test.That(t, model.ModuleName, test.ShouldEqual, "complex")
				test.That(t, model.API, test.ShouldResemble, base.API)
			case mynavigation.Model:
				test.That(t, model.FromLocalModule, test.ShouldEqual, true)
				test.That(t, model.ModuleName, test.ShouldEqual, "complex")
				test.That(t, model.API, test.ShouldResemble, navigation.API)
			case resource.NewModel("acme", "demo", "mycounter"):
				test.That(t, model.FromLocalModule, test.ShouldEqual, false)
				test.That(t, model.ModuleName, test.ShouldEqual, "simple")
				test.That(t, model.API, test.ShouldResemble, generic.API)
			default:
				t.Fail()
				t.Logf("test GetModelsFromModules failure: unrecoginzed model %v", model.Model)
			}
		}
	})
}
