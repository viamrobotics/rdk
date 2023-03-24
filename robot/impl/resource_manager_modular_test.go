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
	"go.viam.com/rdk/registry"
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
		compModel = resource.NewModel("acme", "anvil", "2000")

		svcSubtype = resource.NewSubtype(
			resource.Namespace("acme"),
			resource.ResourceTypeService,
			resource.SubtypeName("sign"),
		)
		svcModel = resource.NewModel("acme", "signage", "handheld")
	)

	setupTest := func(t *testing.T) (*localRobot, *resourceManager, *dummyModMan, func()) {
		logger := golog.NewTestLogger(t)
		res := newResourceManager(resourceManagerOptions{}, logger)
		mod := &dummyModMan{}
		r := &localRobot{
			manager: res,
			logger:  logger,
			config:  &config.Config{},
			modules: mod,
		}

		registry.RegisterResourceSubtype(compSubtype, registry.ResourceSubtype{ReflectRPCServiceDesc: &desc.ServiceDescriptor{}})
		registry.RegisterComponent(compSubtype, compModel, registry.Component{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (resource.Resource, error) {
				return mod.AddResource(ctx, conf, modmanager.DepsToNames(deps))
			},
		})

		registry.RegisterResourceSubtype(svcSubtype, registry.ResourceSubtype{ReflectRPCServiceDesc: &desc.ServiceDescriptor{}})
		registry.RegisterResource(svcSubtype, svcModel, registry.Resource{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (resource.Resource, error) {
				return mod.AddResource(ctx, conf, modmanager.DepsToNames(deps))
			},
		})

		return r, res, mod, func() {
			// deregister to not interfere with other tests or when test.count > 1
			registry.DeregisterResource(compSubtype, compModel)
			registry.DeregisterResource(svcSubtype, svcModel)
			registry.DeregisterResourceSubtype(compSubtype)
			registry.DeregisterResourceSubtype(svcSubtype)
		}
	}

	t.Run("process component", func(t *testing.T) {
		r, res, mod, teardown := setupTest(t)
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
			Name:                "built-in",
			API:                 motor.Subtype,
			Model:               resource.NewDefaultModel("fake"),
			ConvertedAttributes: &fake.Config{},
		}
		_, err = cfg3.Validate("test", resource.ResourceTypeComponent)
		test.That(t, err, test.ShouldBeNil)

		// Add a modular component
		c, newlyBuilt, err := res.processResource(ctx, cfg, resource.NewUninitializedNode(), r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newlyBuilt, test.ShouldBeTrue)
		test.That(t, c, test.ShouldNotBeNil)
		cNode := resource.NewConfiguredGraphNode(cfg, c, cfg.Model)
		res.resources.AddNode(c.Name(), cNode)
		test.That(t, mod.needsModCount, test.ShouldEqual, 0)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, mod.add[0], test.ShouldResemble, cfg)

		old := cNode

		// Reconfigure a modular component
		c, newlyBuilt, err = res.processResource(ctx, cfg2, old, r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newlyBuilt, test.ShouldBeFalse)
		test.That(t, c, test.ShouldNotBeNil)
		test.That(t, mod.needsModCount, test.ShouldEqual, 1)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, len(mod.reconf), test.ShouldEqual, 1)
		test.That(t, mod.reconf[0], test.ShouldResemble, cfg2)

		// Add a non-modular component
		c, newlyBuilt, err = res.processResource(ctx, cfg3, resource.NewUninitializedNode(), r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newlyBuilt, test.ShouldBeTrue)
		test.That(t, c, test.ShouldNotBeNil)
		cNode = resource.NewConfiguredGraphNode(cfg3, c, cfg3.Model)
		res.resources.AddNode(c.Name(), cNode)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, len(mod.reconf), test.ShouldEqual, 1)
		test.That(t, mod.needsModCount, test.ShouldEqual, 1)
	})

	t.Run("process service", func(t *testing.T) {
		r, res, mod, teardown := setupTest(t)
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
			Name:                "built-in",
			API:                 motion.Subtype,
			Model:               resource.DefaultServiceModel,
			ConvertedAttributes: &fake.Config{},
		}
		_, err = cfg3.Validate("test", resource.ResourceTypeService)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, err, test.ShouldBeNil)

		// Add a modular service
		c, newlyBuilt, err := res.processResource(ctx, cfg, resource.NewUninitializedNode(), r)
		test.That(t, newlyBuilt, test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, c, test.ShouldNotBeNil)
		test.That(t, c.Name(), test.ShouldResemble, cfg.ResourceName())
		cNode := resource.NewConfiguredGraphNode(cfg, c, cfg.Model)
		res.resources.AddNode(c.Name(), cNode)
		test.That(t, mod.needsModCount, test.ShouldEqual, 0)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, mod.add[0], test.ShouldResemble, cfg)

		old := cNode

		// Reconfigure a modular service
		c, newlyBuilt, err = res.processResource(ctx, cfg2, old, r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newlyBuilt, test.ShouldBeFalse)
		test.That(t, c, test.ShouldNotBeNil)
		test.That(t, mod.needsModCount, test.ShouldEqual, 1)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, len(mod.reconf), test.ShouldEqual, 1)
		test.That(t, mod.reconf[0], test.ShouldResemble, cfg2)

		// Add a non-modular service
		c, newlyBuilt, err = res.processResource(ctx, cfg3, resource.NewUninitializedNode(), r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newlyBuilt, test.ShouldBeTrue)
		test.That(t, c, test.ShouldNotBeNil)
		cNode = resource.NewConfiguredGraphNode(cfg3, c, cfg3.Model)
		res.resources.AddNode(c.Name(), cNode)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, len(mod.reconf), test.ShouldEqual, 1)
		test.That(t, mod.needsModCount, test.ShouldEqual, 1)
	})

	t.Run("close", func(t *testing.T) {
		r, res, mod, teardown := setupTest(t)
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

		c, newlyBuilt, err := res.processResource(ctx, compCfg, resource.NewUninitializedNode(), r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newlyBuilt, test.ShouldBeTrue)
		test.That(t, c, test.ShouldNotBeNil)
		cNode := resource.NewConfiguredGraphNode(compCfg, c, compCfg.Model)
		res.resources.AddNode(c.Name(), cNode)
		svc, newlyBuilt, err := res.processResource(ctx, svcCfg, resource.NewUninitializedNode(), r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newlyBuilt, test.ShouldBeTrue)
		test.That(t, svc, test.ShouldNotBeNil)
		cNode = resource.NewConfiguredGraphNode(svcCfg, svc, svcCfg.Model)
		res.resources.AddNode(svc.Name(), cNode)
		test.That(t, len(mod.add), test.ShouldEqual, 2)

		test.That(t, r.manager.Close(ctx, r), test.ShouldBeNil)

		test.That(t, len(mod.add), test.ShouldEqual, 2)
		test.That(t, len(mod.reconf), test.ShouldEqual, 0)
		test.That(t, len(mod.remove), test.ShouldEqual, 2)
		expected := map[resource.Name]struct{}{
			c.Name():   {},
			svc.Name(): {},
		}
		for _, rem := range mod.remove {
			test.That(t, expected, test.ShouldContainKey, rem)
			delete(expected, rem)
		}
		test.That(t, expected, test.ShouldBeEmpty)
		test.That(t, mod.isModCount, test.ShouldEqual, 2)
		test.That(t, mod.needsModCount, test.ShouldEqual, 0)
	})
}

type dummyRes struct {
	resource.Named
	resource.AlwaysRebuild
}

type dummyModMan struct {
	modmaninterface.ModuleManager
	mu            sync.Mutex
	add           []resource.Config
	reconf        []resource.Config
	remove        []resource.Name
	isModCount    int
	needsModCount int
}

func (m *dummyModMan) AddResource(ctx context.Context, conf resource.Config, deps []string) (resource.Resource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.add = append(m.add, conf)
	return &dummyRes{
		Named: conf.ResourceName().AsNamed(),
	}, nil
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
	return nil
}

func (m *dummyModMan) IsModularResource(name resource.Name) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isModCount++
	return name.Name != "built-in"
}

func (m *dummyModMan) Provides(cfg resource.Config) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.needsModCount++
	return cfg.Name != "built-in"
}
