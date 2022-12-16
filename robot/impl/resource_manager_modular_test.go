package robotimpl

import (
	"context"
	"sync"
	"testing"

	"github.com/edaniels/golog"
	"github.com/jhump/protoreflect/desc"
	"go.viam.com/test"

	"go.viam.com/rdk/components/motor"
	_ "go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/module/modmanager"
	"go.viam.com/rdk/module/modmaninterface"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
)

func TestModularResources(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	var (
		compSubtype = resource.NewSubtype(
			resource.Namespace("acme"),
			resource.ResourceTypeComponent,
			resource.SubtypeName("anvil"),
		)
		compName  = resource.NameFromSubtype(compSubtype, "oneton")
		compModel = resource.NewModel("acme", "anvil", "2000")

		svcSubtype = resource.NewSubtype(
			resource.Namespace("acme"),
			resource.ResourceTypeService,
			resource.SubtypeName("sign"),
		)
		svcName  = resource.NameFromSubtype(svcSubtype, "ouch")
		svcModel = resource.NewModel("acme", "signage", "handheld")
	)

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
		Constructor: func(ctx context.Context, deps registry.Dependencies, cfg config.Component, logger golog.Logger) (interface{}, error) {
			return mod.AddResource(ctx, cfg, modmanager.DepsToNames(deps))
		},
	})

	registry.RegisterResourceSubtype(svcSubtype, registry.ResourceSubtype{ReflectRPCServiceDesc: &desc.ServiceDescriptor{}})
	registry.RegisterService(svcSubtype, svcModel, registry.Service{
		Constructor: func(ctx context.Context, deps registry.Dependencies, cfg config.Service, logger golog.Logger) (interface{}, error) {
			return mod.AddResource(ctx, config.ServiceConfigToShared(cfg), modmanager.DepsToNames(deps))
		},
	})

	defer func() {
		// deregister to not interfere with other tests or when test.count > 1
		registry.DeregisterComponent(compSubtype, compModel)
		registry.DeregisterService(svcSubtype, svcModel)
		registry.DeregisterResourceSubtype(compSubtype)
		registry.DeregisterResourceSubtype(svcSubtype)
	}()

	t.Run("process component", func(t *testing.T) {
		builtinCompName := resource.NameFromSubtype(motor.Subtype, "built-in")

		// modular
		cfg := config.Component{Name: "oneton", API: compSubtype, Model: compModel, Attributes: config.AttributeMap{"arg1": "one"}}
		_, err := cfg.Validate("test")
		test.That(t, err, test.ShouldBeNil)

		// changed attribute
		cfg2 := config.Component{Name: "oneton", API: compSubtype, Model: compModel, Attributes: config.AttributeMap{"arg1": "two"}}
		_, err = cfg2.Validate("test")
		test.That(t, err, test.ShouldBeNil)

		// non-modular
		cfg3 := config.Component{Name: "built-in", API: motor.Subtype, Model: resource.NewDefaultModel("fake")}
		_, err = cfg3.Validate("test")
		test.That(t, err, test.ShouldBeNil)

		mod.iface = &dummyRes{}
		// Add a modular component
		c, err := res.processComponent(ctx, compName, cfg, nil, r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, c, test.ShouldNotBeNil)
		res.resources.AddNode(compName, c)
		test.That(t, mod.needsModCount, test.ShouldEqual, 0)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, mod.add[0], test.ShouldResemble, cfg)

		old := c

		// Reconfigure a modular component
		c, err = res.processComponent(ctx, compName, cfg2, old, r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, c, test.ShouldNotBeNil)
		test.That(t, mod.needsModCount, test.ShouldEqual, 1)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, len(mod.reconf), test.ShouldEqual, 1)
		test.That(t, mod.reconf[0], test.ShouldResemble, cfg2)

		// Add a non-modular component
		c, err = res.processComponent(ctx, builtinCompName, cfg3, nil, r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, c, test.ShouldNotBeNil)
		res.resources.AddNode(builtinCompName, c)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, len(mod.reconf), test.ShouldEqual, 1)
		test.That(t, mod.needsModCount, test.ShouldEqual, 1)
	})

	t.Run("process service", func(t *testing.T) {
		// reset counters
		mod = &dummyModMan{}
		r.modules = mod

		builtinSvcName := resource.NameFromSubtype(motion.Subtype, "built-in")

		// modular
		cfg := config.Service{
			Name:       "adder",
			Namespace:  svcSubtype.Namespace,
			Type:       svcSubtype.ResourceSubtype,
			Model:      svcModel,
			Attributes: config.AttributeMap{"arg1": "one"},
		}
		_, err := cfg.Validate("test")
		test.That(t, err, test.ShouldBeNil)

		// changed attribute
		cfg2 := config.Service{
			Name:       "adder",
			Namespace:  svcSubtype.Namespace,
			Type:       svcSubtype.ResourceSubtype,
			Model:      svcModel,
			Attributes: config.AttributeMap{"arg1": "two"},
		}
		_, err = cfg2.Validate("test")
		test.That(t, err, test.ShouldBeNil)

		// non-modular
		cfg3 := config.Service{
			Name:      "built-in",
			Namespace: motion.Subtype.Namespace,
			Type:      motion.Subtype.ResourceSubtype,
			Model:     resource.DefaultServiceModel,
		}
		_, err = cfg3.Validate("test")
		test.That(t, err, test.ShouldBeNil)

		mod.iface = &dummyRes{}
		test.That(t, err, test.ShouldBeNil)

		// Add a modular service
		c, err := res.processService(ctx, cfg, nil, r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, c, test.ShouldNotBeNil)
		res.resources.AddNode(svcName, c)
		test.That(t, mod.needsModCount, test.ShouldEqual, 0)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, mod.add[0], test.ShouldResemble, config.ServiceConfigToShared(cfg))

		old := c

		// Reconfigure a modular service
		c, err = res.processService(ctx, cfg2, old, r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, c, test.ShouldNotBeNil)
		test.That(t, mod.needsModCount, test.ShouldEqual, 1)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, len(mod.reconf), test.ShouldEqual, 1)
		test.That(t, mod.reconf[0], test.ShouldResemble, config.ServiceConfigToShared(cfg2))

		// Add a non-modular service
		c, err = res.processService(ctx, cfg3, nil, r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, c, test.ShouldNotBeNil)
		res.resources.AddNode(builtinSvcName, c)
		test.That(t, len(mod.add), test.ShouldEqual, 1)
		test.That(t, len(mod.reconf), test.ShouldEqual, 1)
		test.That(t, mod.needsModCount, test.ShouldEqual, 1)
	})

	t.Run("close", func(t *testing.T) {
		// reset counters
		mod = &dummyModMan{}
		r.modules = mod

		err := r.manager.Close(ctx, r)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, len(mod.add), test.ShouldEqual, 0)
		test.That(t, len(mod.reconf), test.ShouldEqual, 0)
		test.That(t, len(mod.remove), test.ShouldEqual, 2)
		for _, rem := range mod.remove {
			if rem.Name == compName.Name {
				test.That(t, rem, test.ShouldResemble, compName)
			} else {
				test.That(t, rem, test.ShouldResemble, svcName)
			}
		}
		test.That(t, mod.isModCount, test.ShouldEqual, 4)
		test.That(t, mod.needsModCount, test.ShouldEqual, 0)
	})
}

type dummyRes struct{}

type dummyModMan struct {
	modmaninterface.ModuleManager
	mu            sync.Mutex
	add           []config.Component
	reconf        []config.Component
	remove        []resource.Name
	isModCount    int
	needsModCount int
	iface         interface{}
}

func (m *dummyModMan) AddResource(ctx context.Context, cfg config.Component, deps []string) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.add = append(m.add, cfg)
	return m.iface, nil
}

func (m *dummyModMan) ReconfigureResource(ctx context.Context, cfg config.Component, deps []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconf = append(m.reconf, cfg)
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

func (m *dummyModMan) Provides(cfg config.Component) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.needsModCount++
	return cfg.Name != "built-in"
}
