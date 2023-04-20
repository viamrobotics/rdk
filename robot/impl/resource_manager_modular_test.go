package robotimpl

import (
	"context"
	"sync"
	"testing"

	"github.com/edaniels/golog"
	"github.com/jhump/protoreflect/desc"
	"go.viam.com/test"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/module/modmanager"
	"go.viam.com/rdk/module/modmaninterface"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/utils"
)

func TestModularResources(t *testing.T) {
	ctx := context.Background()

	var (
		compSubtype = resource.NewSubtype(
			resource.Namespace("acme"),
			resource.ResourceTypeComponent,
			resource.SubtypeName("anvil"),
		)
		compModel  = resource.NewModel("acme", "anvil", "2000")
		compModel2 = resource.NewModel("acme", "anvil", "3000")

		svcSubtype = resource.NewSubtype(
			resource.Namespace("acme"),
			resource.ResourceTypeService,
			resource.SubtypeName("sign"),
		)
		svcModel = resource.NewModel("acme", "signage", "handheld")
	)

	setupTest := func(t *testing.T) (*localRobot, *dummyModMan, func()) {
		logger := golog.NewTestLogger(t)
		compSubtypeSvc, err := resource.NewSubtypeCollection[resource.Resource](compSubtype, nil)
		test.That(t, err, test.ShouldBeNil)
		svcSubtypeSvc, err := resource.NewSubtypeCollection[resource.Resource](svcSubtype, nil)
		test.That(t, err, test.ShouldBeNil)
		mod := &dummyModMan{
			compSubtypeSvc: compSubtypeSvc,
			svcSubtypeSvc:  svcSubtypeSvc,
		}

		r, err := New(context.Background(), &config.Config{}, logger)
		test.That(t, err, test.ShouldBeNil)
		actualR := r.(*localRobot)
		actualR.modules = mod

		resource.RegisterSubtype(compSubtype,
			resource.SubtypeRegistration[resource.Resource]{ReflectRPCServiceDesc: &desc.ServiceDescriptor{}})
		resource.RegisterComponent(compSubtype, compModel, resource.Registration[resource.Resource, any]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (resource.Resource, error) {
				return mod.AddResource(ctx, conf, modmanager.DepsToNames(deps))
			},
		})
		resource.RegisterComponent(compSubtype, compModel2, resource.Registration[resource.Resource, any]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (resource.Resource, error) {
				return mod.AddResource(ctx, conf, modmanager.DepsToNames(deps))
			},
		})

		resource.RegisterSubtype(svcSubtype,
			resource.SubtypeRegistration[resource.Resource]{ReflectRPCServiceDesc: &desc.ServiceDescriptor{}})
		resource.Register(svcSubtype, svcModel, resource.Registration[resource.Resource, any]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (resource.Resource, error) {
				return mod.AddResource(ctx, conf, modmanager.DepsToNames(deps))
			},
		})

		return actualR, mod, func() {
			// deregister to not interfere with other tests or when test.count > 1
			resource.Deregister(compSubtype, compModel)
			resource.Deregister(compSubtype, compModel2)
			resource.Deregister(svcSubtype, svcModel)
			resource.DeregisterSubtype(compSubtype)
			resource.DeregisterSubtype(svcSubtype)
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}
	}

	t.Run("process component", func(t *testing.T) {
		r, mod, teardown := setupTest(t)
		defer teardown()

		// modular
		cfg := resource.Config{Name: "oneton", API: compSubtype, Model: compModel, Attributes: utils.AttributeMap{"arg1": "one"}}
		_, err := cfg.Validate("test", resource.ResourceTypeComponent)
		test.That(t, err, test.ShouldBeNil)

		// changed attribute
		cfg2 := resource.Config{Name: "oneton", API: compSubtype, Model: compModel, Attributes: utils.AttributeMap{"arg1": "two"}}
		_, err = cfg2.Validate("test", resource.ResourceTypeComponent)
		test.That(t, err, test.ShouldBeNil)

		// non-modular
		cfg3 := resource.Config{
			Name:                "builtin",
			API:                 motor.Subtype,
			Model:               resource.NewDefaultModel("fake"),
			ConvertedAttributes: &fake.Config{},
		}
		_, err = cfg3.Validate("test", resource.ResourceTypeComponent)
		test.That(t, err, test.ShouldBeNil)

		// changed name
		cfg4 := resource.Config{Name: "oneton2", API: compSubtype, Model: compModel, Attributes: utils.AttributeMap{"arg1": "two"}}
		_, err = cfg4.Validate("test", resource.ResourceTypeComponent)
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
	})

	t.Run("process service", func(t *testing.T) {
		r, mod, teardown := setupTest(t)
		defer teardown()

		// modular
		cfg := resource.Config{
			Name:       "adder",
			API:        svcSubtype,
			Model:      svcModel,
			Attributes: utils.AttributeMap{"arg1": "one"},
		}
		_, err := cfg.Validate("test", resource.ResourceTypeService)
		test.That(t, err, test.ShouldBeNil)

		// changed attribute
		cfg2 := resource.Config{
			Name:       "adder",
			API:        svcSubtype,
			Model:      svcModel,
			Attributes: utils.AttributeMap{"arg1": "two"},
		}
		_, err = cfg2.Validate("test", resource.ResourceTypeService)
		test.That(t, err, test.ShouldBeNil)

		// non-modular
		cfg3 := resource.Config{
			Name:                "builtin",
			API:                 motion.Subtype,
			Model:               resource.DefaultServiceModel,
			ConvertedAttributes: &fake.Config{},
		}
		_, err = cfg3.Validate("test", resource.ResourceTypeService)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, err, test.ShouldBeNil)

		// Add a modular service
		r.Reconfigure(context.Background(), &config.Config{
			Components: []resource.Config{cfg},
		})
		_, err = r.ResourceByName(cfg.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, mod.add[0], test.ShouldResemble, cfg)

		// Reconfigure a modular service
		r.Reconfigure(context.Background(), &config.Config{
			Components: []resource.Config{cfg2},
		})
		_, err = r.ResourceByName(cfg2.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, len(mod.reconf), test.ShouldEqual, 1)
		test.That(t, mod.reconf[0], test.ShouldResemble, cfg2)

		// Add a non-modular service
		r.Reconfigure(context.Background(), &config.Config{
			Components: []resource.Config{cfg2, cfg3},
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

		compCfg := resource.Config{Name: "oneton", API: compSubtype, Model: compModel, Attributes: utils.AttributeMap{"arg1": "one"}}
		_, err := compCfg.Validate("test", resource.ResourceTypeComponent)
		test.That(t, err, test.ShouldBeNil)

		svcCfg := resource.Config{
			Name:       "adder",
			API:        svcSubtype,
			Model:      svcModel,
			Attributes: utils.AttributeMap{"arg1": "one"},
		}
		_, err = svcCfg.Validate("test", resource.ResourceTypeComponent)
		test.That(t, err, test.ShouldBeNil)

		r.Reconfigure(context.Background(), &config.Config{
			Components: []resource.Config{compCfg, svcCfg},
		})
		_, err = r.ResourceByName(compCfg.ResourceName())
		test.That(t, err, test.ShouldBeNil)
		_, err = r.ResourceByName(svcCfg.ResourceName())
		test.That(t, err, test.ShouldBeNil)

		test.That(t, len(mod.add), test.ShouldEqual, 2)

		test.That(t, r.manager.Close(ctx, r), test.ShouldBeNil)

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
		cfg := resource.Config{Name: "oneton2", API: compSubtype, Model: compModel, Attributes: utils.AttributeMap{"arg1": "one"}}
		_, err := cfg.Validate("test", resource.ResourceTypeComponent)
		test.That(t, err, test.ShouldBeNil)

		// non-modular
		cfg2 := resource.Config{
			Name:                "builtin",
			API:                 motor.Subtype,
			Model:               resource.NewDefaultModel("fake"),
			ConvertedAttributes: &fake.Config{},
			ImplicitDependsOn:   []string{"oneton"},
		}
		_, err = cfg2.Validate("test", resource.ResourceTypeComponent)
		test.That(t, err, test.ShouldBeNil)

		// modular we want
		cfg3 := resource.Config{Name: "oneton", API: compSubtype, Model: compModel, Attributes: utils.AttributeMap{"arg1": "one"}}
		_, err = cfg3.Validate("test", resource.ResourceTypeComponent)
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

		cfg := resource.Config{Name: "oneton", API: compSubtype, Model: compModel, Attributes: utils.AttributeMap{"arg1": "one"}}
		_, err := cfg.Validate("test", resource.ResourceTypeComponent)
		test.That(t, err, test.ShouldBeNil)

		r.Reconfigure(context.Background(), &config.Config{
			Components: []resource.Config{cfg},
		})
		res1, err := r.ResourceByName(cfg.ResourceName())
		test.That(t, err, test.ShouldBeNil)

		cfg2 := resource.Config{Name: "oneton", API: compSubtype, Model: compModel2, Attributes: utils.AttributeMap{"arg1": "one"}}
		_, err = cfg2.Validate("test", resource.ResourceTypeComponent)
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
	mu             sync.Mutex
	add            []resource.Config
	reconf         []resource.Config
	remove         []resource.Name
	compSubtypeSvc resource.SubtypeCollection[resource.Resource]
	svcSubtypeSvc  resource.SubtypeCollection[resource.Resource]
}

func (m *dummyModMan) AddResource(ctx context.Context, conf resource.Config, deps []string) (resource.Resource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.add = append(m.add, conf)
	res := &dummyRes{
		Named: conf.ResourceName().AsNamed(),
	}
	if conf.API.ResourceType == resource.ResourceTypeComponent {
		if err := m.compSubtypeSvc.Add(conf.ResourceName(), res); err != nil {
			return nil, err
		}
	} else {
		if err := m.svcSubtypeSvc.Add(conf.ResourceName(), res); err != nil {
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
	if name.ResourceType == resource.ResourceTypeComponent {
		if err := m.compSubtypeSvc.Remove(name); err != nil {
			return err
		}
	} else {
		if err := m.svcSubtypeSvc.Remove(name); err != nil {
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

func (m *dummyModMan) Close(ctx context.Context) error {
	return nil
}
