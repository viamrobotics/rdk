package robotimpl

import (
	"context"
	"sync"
	"testing"

	"github.com/jhump/protoreflect/desc"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module/modmanager"
	"go.viam.com/rdk/module/modmaninterface"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	motionBuiltin "go.viam.com/rdk/services/motion/builtin"
	rtestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/utils"
)

func TestModularResources(t *testing.T) {
	ctx := context.Background()

	var (
		compAPI    = resource.APINamespace("acme").WithComponentType("anvil")
		compModel  = resource.ModelNamespace("acme").WithFamily("anvil").WithModel("2000")
		compModel2 = resource.ModelNamespace("acme").WithFamily("anvil").WithModel("3000")

		svcAPI   = resource.APINamespace("acme").WithServiceType("sign")
		svcModel = resource.ModelNamespace("acme").WithFamily("signage").WithModel("handheld")
	)

	setupTest := func(t *testing.T) (*localRobot, *dummyModMan, func()) {
		logger := logging.NewTestLogger(t)
		compAPISvc, err := resource.NewAPIResourceCollection[resource.Resource](compAPI, nil)
		test.That(t, err, test.ShouldBeNil)
		svcAPISvc, err := resource.NewAPIResourceCollection[resource.Resource](svcAPI, nil)
		test.That(t, err, test.ShouldBeNil)
		mod := &dummyModMan{
			compAPISvc: compAPISvc,
			svcAPISvc:  svcAPISvc,
			state:      make(map[resource.Name]bool),
		}

		r, err := New(context.Background(), &config.Config{}, logger)
		test.That(t, err, test.ShouldBeNil)
		actualR := r.(*localRobot)
		actualR.manager.moduleManager = mod

		resource.RegisterAPI(compAPI,
			resource.APIRegistration[resource.Resource]{ReflectRPCServiceDesc: &desc.ServiceDescriptor{}})
		resource.RegisterComponent(compAPI, compModel, resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (resource.Resource, error) {
				return mod.AddResource(ctx, conf, modmanager.DepsToNames(deps))
			},
		})
		resource.RegisterComponent(compAPI, compModel2, resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (resource.Resource, error) {
				return mod.AddResource(ctx, conf, modmanager.DepsToNames(deps))
			},
		})

		resource.RegisterAPI(svcAPI,
			resource.APIRegistration[resource.Resource]{ReflectRPCServiceDesc: &desc.ServiceDescriptor{}})
		resource.Register(svcAPI, svcModel, resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (resource.Resource, error) {
				return mod.AddResource(ctx, conf, modmanager.DepsToNames(deps))
			},
		})

		return actualR, mod, func() {
			// deregister to not interfere with other tests or when test.count > 1
			resource.Deregister(compAPI, compModel)
			resource.Deregister(compAPI, compModel2)
			resource.Deregister(svcAPI, svcModel)
			resource.DeregisterAPI(compAPI)
			resource.DeregisterAPI(svcAPI)
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}
	}

	t.Run("process component", func(t *testing.T) {
		r, mod, teardown := setupTest(t)
		defer teardown()

		// modular
		cfg := resource.Config{Name: "oneton", API: compAPI, Model: compModel, Attributes: utils.AttributeMap{"arg1": "one"}}
		_, err := cfg.Validate("test", resource.APITypeComponentName)
		test.That(t, err, test.ShouldBeNil)

		// changed attribute
		cfg2 := resource.Config{Name: "oneton", API: compAPI, Model: compModel, Attributes: utils.AttributeMap{"arg1": "two"}}
		_, err = cfg2.Validate("test", resource.APITypeComponentName)
		test.That(t, err, test.ShouldBeNil)

		// non-modular
		cfg3 := resource.Config{
			Name:                "builtin",
			API:                 motor.API,
			Model:               resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{},
		}
		_, err = cfg3.Validate("test", resource.APITypeComponentName)
		test.That(t, err, test.ShouldBeNil)

		// changed name
		cfg4 := resource.Config{Name: "oneton2", API: compAPI, Model: compModel, Attributes: utils.AttributeMap{"arg1": "two"}}
		_, err = cfg4.Validate("test", resource.APITypeComponentName)
		test.That(t, err, test.ShouldBeNil)

		// Add a modular component
		r.Reconfigure(context.Background(), &config.Config{
			Components: []resource.Config{cfg},
		})
		_, err = r.ResourceByName(cfg.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, mod.add[0], test.ShouldResemble, cfg)

		// Reconfigure a modular component
		r.Reconfigure(context.Background(), &config.Config{
			Components: []resource.Config{cfg2},
		})
		_, err = r.ResourceByName(cfg2.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, len(mod.reconf), test.ShouldEqual, 1)
		test.That(t, mod.reconf[0], test.ShouldResemble, cfg2)

		// Add a non-modular component
		r.Reconfigure(context.Background(), &config.Config{
			Components: []resource.Config{cfg2, cfg3},
		})
		_, err = r.ResourceByName(cfg2.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		_, err = r.ResourceByName(cfg3.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, len(mod.reconf), test.ShouldEqual, 1)

		// Change the name of a modular component
		r.Reconfigure(context.Background(), &config.Config{
			Components: []resource.Config{cfg4, cfg3},
		})
		_, err = r.ResourceByName(cfg2.ResourceName())
		test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(cfg2.ResourceName()))
		_, err = r.ResourceByName(cfg4.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		_, err = r.ResourceByName(cfg3.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mod.add, test.ShouldResemble, []resource.Config{cfg, cfg4})
		test.That(t, mod.remove, test.ShouldResemble, []resource.Name{cfg2.ResourceName()})
		test.That(t, mod.reconf, test.ShouldResemble, []resource.Config{cfg2})
		test.That(t, len(mod.state), test.ShouldEqual, 1)
	})

	t.Run("process service", func(t *testing.T) {
		r, mod, teardown := setupTest(t)
		defer teardown()

		// modular
		cfg := resource.Config{
			Name:       "adder",
			API:        svcAPI,
			Model:      svcModel,
			Attributes: utils.AttributeMap{"arg1": "one"},
		}
		_, err := cfg.Validate("test", resource.APITypeServiceName)
		test.That(t, err, test.ShouldBeNil)

		// changed attribute
		cfg2 := resource.Config{
			Name:       "adder",
			API:        svcAPI,
			Model:      svcModel,
			Attributes: utils.AttributeMap{"arg1": "two"},
		}
		_, err = cfg2.Validate("test", resource.APITypeServiceName)
		test.That(t, err, test.ShouldBeNil)

		// non-modular
		cfg3 := resource.Config{
			Name:                "builtin",
			API:                 motion.API,
			Model:               resource.DefaultServiceModel,
			ConvertedAttributes: &motionBuiltin.Config{},
			DependsOn:           []string{framesystem.InternalServiceName.String()},
		}
		_, err = cfg3.Validate("test", resource.APITypeServiceName)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, err, test.ShouldBeNil)

		// Add a modular service
		r.Reconfigure(context.Background(), &config.Config{
			Services: []resource.Config{cfg},
		})
		_, err = r.ResourceByName(cfg.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, mod.add[0], test.ShouldResemble, cfg)

		// Reconfigure a modular service
		r.Reconfigure(context.Background(), &config.Config{
			Services: []resource.Config{cfg2},
		})
		_, err = r.ResourceByName(cfg2.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, len(mod.reconf), test.ShouldEqual, 1)
		test.That(t, mod.reconf[0], test.ShouldResemble, cfg2)

		// Add a non-modular service
		r.Reconfigure(context.Background(), &config.Config{
			Services: []resource.Config{cfg2, cfg3},
		})
		_, err = r.ResourceByName(cfg2.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		_, err = r.ResourceByName(cfg3.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, len(mod.reconf), test.ShouldEqual, 1)
	})

	t.Run("close", func(t *testing.T) {
		r, mod, teardown := setupTest(t)
		defer teardown()

		compCfg := resource.Config{Name: "oneton", API: compAPI, Model: compModel, Attributes: utils.AttributeMap{"arg1": "one"}}
		_, err := compCfg.Validate("test", resource.APITypeComponentName)
		test.That(t, err, test.ShouldBeNil)

		svcCfg := resource.Config{
			Name:       "adder",
			API:        svcAPI,
			Model:      svcModel,
			Attributes: utils.AttributeMap{"arg1": "one"},
		}
		_, err = svcCfg.Validate("test", resource.APITypeComponentName)
		test.That(t, err, test.ShouldBeNil)

		r.Reconfigure(context.Background(), &config.Config{
			Components: []resource.Config{compCfg, svcCfg},
		})
		_, err = r.ResourceByName(compCfg.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		_, err = r.ResourceByName(svcCfg.ResourceName())
		test.That(t, err, test.ShouldBeNil)

		test.That(t, len(mod.add), test.ShouldEqual, 2)

		test.That(t, r.manager.Close(ctx), test.ShouldBeNil)

		test.That(t, len(mod.add), test.ShouldEqual, 2)
		test.That(t, len(mod.reconf), test.ShouldEqual, 0)
		test.That(t, len(mod.remove), test.ShouldEqual, 2)
		expected := map[resource.Name]struct{}{
			compCfg.ResourceName(): {},
			svcCfg.ResourceName():  {},
		}
		for _, rem := range mod.remove {
			test.That(t, expected, test.ShouldContainKey, rem)
			delete(expected, rem)
		}
		test.That(t, expected, test.ShouldBeEmpty)
	})

	t.Run("builtin depends on previously removed but now added modular", func(t *testing.T) {
		r, _, teardown := setupTest(t)
		defer teardown()

		// modular we do not want
		cfg := resource.Config{Name: "oneton2", API: compAPI, Model: compModel, Attributes: utils.AttributeMap{"arg1": "one"}}
		_, err := cfg.Validate("test", resource.APITypeComponentName)
		test.That(t, err, test.ShouldBeNil)

		// non-modular
		cfg2 := resource.Config{
			Name:                "builtin",
			API:                 motor.API,
			Model:               resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{},
			ImplicitDependsOn:   []string{"oneton"},
		}
		_, err = cfg2.Validate("test", resource.APITypeComponentName)
		test.That(t, err, test.ShouldBeNil)

		// modular we want
		cfg3 := resource.Config{Name: "oneton", API: compAPI, Model: compModel, Attributes: utils.AttributeMap{"arg1": "one"}}
		_, err = cfg3.Validate("test", resource.APITypeComponentName)
		test.That(t, err, test.ShouldBeNil)

		// what we want is originally available
		r.Reconfigure(context.Background(), &config.Config{
			Components: []resource.Config{cfg3},
		})
		_, err = r.ResourceByName(cfg3.ResourceName())
		test.That(t, err, test.ShouldBeNil)

		// and then its not but called something else and what wants it cannot get it
		r.Reconfigure(context.Background(), &config.Config{
			Components: []resource.Config{cfg, cfg2},
		})
		_, err = r.ResourceByName(cfg.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		_, err = r.ResourceByName(cfg2.ResourceName())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "pending")
		_, err = r.ResourceByName(cfg3.ResourceName())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(cfg3.ResourceName()))

		// we remove what we do not want and add what we do back in, fixing things
		r.Reconfigure(context.Background(), &config.Config{
			Components: []resource.Config{cfg3, cfg2},
		})
		_, err = r.ResourceByName(cfg3.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		_, err = r.ResourceByName(cfg2.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		_, err = r.ResourceByName(cfg.ResourceName())
		test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(cfg.ResourceName()))
	})

	t.Run("change model", func(t *testing.T) {
		r, _, teardown := setupTest(t)
		defer teardown()

		cfg := resource.Config{Name: "oneton", API: compAPI, Model: compModel, Attributes: utils.AttributeMap{"arg1": "one"}}
		_, err := cfg.Validate("test", resource.APITypeComponentName)
		test.That(t, err, test.ShouldBeNil)

		r.Reconfigure(context.Background(), &config.Config{
			Components: []resource.Config{cfg},
		})
		res1, err := r.ResourceByName(cfg.ResourceName())
		test.That(t, err, test.ShouldBeNil)

		cfg2 := resource.Config{Name: "oneton", API: compAPI, Model: compModel2, Attributes: utils.AttributeMap{"arg1": "one"}}
		_, err = cfg2.Validate("test", resource.APITypeComponentName)
		test.That(t, err, test.ShouldBeNil)

		r.Reconfigure(context.Background(), &config.Config{
			Components: []resource.Config{cfg2},
		})
		res2, err := r.ResourceByName(cfg2.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, res2, test.ShouldNotEqual, res1)
	})
}

type dummyRes struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
}

type dummyModMan struct {
	modmaninterface.ModuleManager
	mu         sync.Mutex
	add        []resource.Config
	reconf     []resource.Config
	remove     []resource.Name
	compAPISvc resource.APIResourceCollection[resource.Resource]
	svcAPISvc  resource.APIResourceCollection[resource.Resource]
	state      map[resource.Name]bool
}

func (m *dummyModMan) AddResource(ctx context.Context, conf resource.Config, deps []string) (resource.Resource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.add = append(m.add, conf)
	m.state[conf.ResourceName()] = true
	res := &dummyRes{
		Named: conf.ResourceName().AsNamed(),
	}
	if conf.API.IsComponent() {
		if err := m.compAPISvc.Add(conf.ResourceName(), res); err != nil {
			return nil, err
		}
	} else {
		if err := m.svcAPISvc.Add(conf.ResourceName(), res); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (m *dummyModMan) ReconfigureResource(ctx context.Context, conf resource.Config, deps []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconf = append(m.reconf, conf)
	return nil
}

func (m *dummyModMan) RemoveResource(ctx context.Context, name resource.Name) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.remove = append(m.remove, name)
	delete(m.state, name)
	if name.API.IsComponent() {
		if err := m.compAPISvc.Remove(name); err != nil {
			return err
		}
	} else {
		if err := m.svcAPISvc.Remove(name); err != nil {
			return err
		}
	}
	return nil
}

func (m *dummyModMan) IsModularResource(name resource.Name) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return name.Name != "builtin"
}

func (m *dummyModMan) Configs() []config.Module {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nil
}

func (m *dummyModMan) Provides(cfg resource.Config) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cfg.Name != "builtin"
}

func (m *dummyModMan) ValidateConfig(ctx context.Context, cfg resource.Config) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nil, nil
}

func (m *dummyModMan) ResolveImplicitDependenciesInConfig(ctx context.Context, conf *config.Diff) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nil
}

func (m *dummyModMan) CleanModuleDataDirectory() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nil
}

func (m *dummyModMan) Close(ctx context.Context) error {
	if len(m.state) != 0 {
		return errors.New("attempt to close with active resources in place")
	}
	return nil
}

func TestDynamicModuleLogging(t *testing.T) {
	modPath, err := rtestutils.BuildTempModule(t, "module/testmodule")
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	logger, observer := logging.NewObservedTestLogger(t)

	helperConf := resource.Config{
		Name:  "helper",
		API:   generic.API,
		Model: resource.NewModel("rdk", "test", "helper"),
		LogConfiguration: resource.LogConfig{
			Level: logging.INFO,
		},
	}
	cfg := &config.Config{
		Components: []resource.Config{helperConf},
		Modules: []config.Module{{
			Name:     "helperModule",
			ExePath:  modPath,
			LogLevel: "info",
			Type:     "local",
		}},
	}

	myRobot, err := RobotFromConfig(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer myRobot.Close(ctx)

	client, err := generic.FromRobot(myRobot, "helper")
	test.That(t, err, test.ShouldBeNil)
	defer client.Close(ctx)

	//nolint:lll
	// Have the module log a line at info. It should appear as:
	// 2023-12-06T15:55:32.590-0500	INFO	process.helperModule_/tmp/TestDynamicModuleLogging3790223620/001/testmodule.StdOut	pexec/managed_process.go:244
	// \_ 2023-12-06T15:55:32.590-0500	INFO	TestModule.rdk:component:generic/helper	testmodule/main.go:147	special rare log line
	logLine := "special rare log line"
	testCmd := map[string]interface{}{"command": "log", "msg": logLine, "level": "info"}
	_, err = client.DoCommand(ctx, testCmd)
	test.That(t, err, test.ShouldBeNil)

	// Our log observer should find one occurrence of the log line.
	test.That(t, observer.FilterMessageSnippet(logLine).Len(), test.ShouldEqual, 1)

	// The module is currently configured to log at info. If the module tries to log at debug,
	// nothing new should be observed.
	testCmd = map[string]interface{}{"command": "log", "msg": logLine, "level": "debug"}
	_, err = client.DoCommand(ctx, testCmd)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, observer.FilterMessageSnippet(logLine).Len(), test.ShouldEqual, 1)
	test.That(t, observer.FilterMessageSnippet(logLine).FilterMessageSnippet("DEBUG").Len(), test.ShouldEqual, 0)

	// Change the modular component to log at DEBUG instead of INFO.
	cfg.Components[0].LogConfiguration.Level = logging.DEBUG
	myRobot.Reconfigure(ctx, cfg)

	// Trying to log again at DEBUG should see our log line pattern show up a second time. Now with
	// DEBUG in the output string.
	testCmd = map[string]interface{}{"command": "log", "msg": logLine, "level": "debug"}
	_, err = client.DoCommand(ctx, testCmd)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, observer.FilterMessageSnippet(logLine).Len(), test.ShouldEqual, 2)
	test.That(t, observer.FilterMessageSnippet(logLine).FilterMessageSnippet("DEBUG").Len(), test.ShouldEqual, 1)
}
