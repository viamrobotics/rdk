package module

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	v1 "go.viam.com/api/app/v1"
	pb "go.viam.com/api/module/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/shell"
)

// setupLocalModule sets up a module without a parent connection.
func setupLocalModule(t *testing.T, ctx context.Context, logger logging.Logger) *Module {
	t.Helper()

	// Use 'foo.sock' for arbitrary module to test AddModelFromRegistry.
	m, err := NewModule(ctx, filepath.Join(t.TempDir(), "foo.sock"), logger)
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() {
		m.Close(ctx)
	})

	// Hit Ready as a way to close m.pcFailed, so that AddResource can proceed. Set
	// NoModuleParentEnvVar so that parent connection will not be attempted.
	test.That(t, os.Setenv(NoModuleParentEnvVar, "true"), test.ShouldBeNil)
	t.Cleanup(func() {
		test.That(t, os.Unsetenv(NoModuleParentEnvVar), test.ShouldBeNil)
	})

	_, err = m.Ready(ctx, &pb.ReadyRequest{})
	test.That(t, err, test.ShouldBeNil)
	return m
}

type doCommandDependerConfig struct {
	DependsOn []string
}

func (dcdc *doCommandDependerConfig) Validate(path string) ([]string, []string, error) {
	if len(dcdc.DependsOn) > 0 {
		return []string{}, dcdc.DependsOn, nil
	}

	return []string{}, []string{}, nil
}

type doCommandDepender struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable

	dependsOn []resource.Resource
	closed    bool

	logger logging.Logger
}

func (dcd *doCommandDepender) Close(ctx context.Context) error {
	dcd.logger.Infof("Closing %v %p, depends: %p", dcd.Name(), dcd, dcd.dependsOn)
	dcd.closed = true
	return nil
}

func (dcd *doCommandDepender) DoCommand(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	if dcd.closed {
		return nil, errors.New("was closed")
	}

	// Dan: I do some fancy stuff here to chain the API calls through to dependencies. But the
	// values are not asserted on. They were just deemed to be some way to summarize the path a
	// request took.
	switch {
	case len(dcd.dependsOn) == 0:
		return map[string]any{
			"outgoing": 0,
			"incoming": 1,
		}, nil

	case len(dcd.dependsOn) == 1:
		if val, exists := cmd["outgoing"]; exists {
			cmd["outgoing"] = val.(int) + 1
		} else {
			cmd["outgoing"] = 1
		}

		resp, err := dcd.dependsOn[0].DoCommand(ctx, cmd)
		if err != nil {
			return nil, err
		}

		resp["incoming"] = resp["incoming"].(int) + 1
		return resp, nil
	default:
		// > 1
		for _, dep := range dcd.dependsOn {
			_, err := dep.DoCommand(ctx, cmd)
			if err != nil {
				return nil, err
			}
		}

		return map[string]any{
			"outgoing": len(dcd.dependsOn),
			"incoming": 1,
		}, nil
	}
}

// testLocal gets a fresh local resource for each input string and asserts its `DoCommand` succeeds.
func testLocal(ctx context.Context, t *testing.T, mod *Module, resStrings ...string) {
	t.Helper()
	for _, resStr := range resStrings {
		res, err := mod.getLocalResource(ctx, generic.Named(resStr))
		test.That(t, err, test.ShouldBeNil)

		_, err = res.DoCommand(ctx, map[string]any{})
		test.That(t, err, test.ShouldBeNil)
	}
}

func TestOptimizedModuleCommunication(t *testing.T) {
	ctx := t.Context()
	logger := logging.NewTestLogger(t)

	// We expose one model that satisfies the (component) generic API and can have a variable number
	// of dependents. The test will create multiple of these components that may or may not depend
	// on others.
	modelName := utils.RandomAlphaString(20)
	model := resource.DefaultModelFamily.WithModel(modelName)
	logger.Info("Randomized model name:", modelName)

	// We will use this to count how often a particular resource gets constructed.
	superConstructCount := 0
	resource.RegisterComponent(generic.API, model, resource.Registration[resource.Resource, *doCommandDependerConfig]{
		Constructor: func(
			ctx context.Context, deps resource.Dependencies, rcfg resource.Config, logger logging.Logger,
		) (resource.Resource, error) {
			if rcfg.Name == "super" {
				superConstructCount++
			}

			cfg, err := resource.NativeConfig[*doCommandDependerConfig](rcfg)
			if err != nil {
				return nil, err
			}

			ret := &doCommandDepender{
				Named:  rcfg.ResourceName().AsNamed(),
				logger: logger,
			}

			for _, depStr := range cfg.DependsOn {
				dep, err := generic.FromProvider(deps, depStr)
				if err != nil {
					return nil, err
				}

				ret.dependsOn = append(ret.dependsOn, dep)
			}
			logger.Infof("I %v (%p) depend on %p Config.DependsOn: %v",
				rcfg.Name, ret, ret.dependsOn, cfg.DependsOn)

			return ret, nil
		},
	})
	t.Cleanup(func() {
		resource.Deregister(shell.API, model)
	})

	module := setupLocalModule(t, ctx, logging.NewTestLogger(t))
	test.That(t, module.AddModelFromRegistry(ctx, generic.API, model), test.ShouldBeNil)

	// This test will ultimately create three resources:
	// 1) A leaf that depends on nothing
	// 2) A branch that depends on the leaf
	// 3) A trunk that depends on the branch
	//
	// We create these resources in proper dependency order.
	//
	// Add a leaf resource that depends on nothing.
	_, err := module.AddResource(ctx, &pb.AddResourceRequest{Config: &v1.ComponentConfig{
		Name: "leaf", Api: generic.API.String(), Model: model.String(),
	}})
	test.That(t, err, test.ShouldBeNil)

	// Assert leaf can be used.
	testLocal(ctx, t, module, "leaf")

	// Build up and add a branch resource that depends on the leaf resource.
	attrsBuf, err := protoutils.StructToStructPb(&doCommandDependerConfig{
		DependsOn: []string{"leaf"},
	})
	test.That(t, err, test.ShouldBeNil)

	_, err = module.AddResource(ctx, &pb.AddResourceRequest{
		Dependencies: []string{generic.Named("leaf").String()},
		Config: &v1.ComponentConfig{
			Name: "branch", Api: generic.API.String(), Model: model.String(),
			DependsOn:  []string{"leaf"}, // unnecessary but mimics reality?
			Attributes: attrsBuf,
		},
	})
	test.That(t, err, test.ShouldBeNil)

	// Assert branch can use its dependency.
	testLocal(ctx, t, module, "branch")

	// Get a handle on the branch resource that we are going to invalidate.
	staleBranchRes, err := module.getLocalResource(ctx, generic.Named("branch"))
	test.That(t, err, test.ShouldBeNil)

	// Reconfigure the leaf. This results in a `Close` -> `Constructor`. Invaliding the above
	// `staleBranchRes`.
	logger.Info("Reconfiguring leaf first time")
	_, err = module.ReconfigureResource(ctx, &pb.ReconfigureResourceRequest{
		Config: &v1.ComponentConfig{ // Same config.
			Name: "leaf", Api: generic.API.String(), Model: model.String(),
		},
	})
	test.That(t, err, test.ShouldBeNil)

	// Assert that the original `branchRes` has its dependency invalidated.
	_, err = staleBranchRes.DoCommand(ctx, map[string]any{})
	test.That(t, err, test.ShouldNotBeNil)

	// Assert getting a fresh value for the `branchRes` succeeds in using its dependency.
	testLocal(ctx, t, module, "branch")

	// Build up and add a branch resource that depends on the leaf resource.
	trunkImplicitDependsOn, err := protoutils.StructToStructPb(&doCommandDependerConfig{
		DependsOn: []string{"branch"},
	})
	test.That(t, err, test.ShouldBeNil)
	// Add a trunk that depends on the branch resource.
	_, err = module.AddResource(ctx, &pb.AddResourceRequest{
		Dependencies: []string{generic.Named("branch").String()},
		Config: &v1.ComponentConfig{
			Name: "trunk", Api: generic.API.String(), Model: model.String(),
			Attributes: trunkImplicitDependsOn,
		},
	})
	test.That(t, err, test.ShouldBeNil)

	testLocal(ctx, t, module, "trunk")

	logger.Info("Reconfiguring leaf second time")

	// Reconfigure the leaf again. This results in a `Close` -> `Constructor` on both the leaf
	// _and_ the branch _and_ the trunk. Refetch the branch and trunk and assert they
	// both trunk can continue respond to `DoCommand`s that require dependencies to be valid.
	_, err = module.ReconfigureResource(ctx, &pb.ReconfigureResourceRequest{
		Config: &v1.ComponentConfig{ // Same config.
			Name: "leaf", Api: generic.API.String(), Model: model.String(),
		},
	})
	test.That(t, err, test.ShouldBeNil)

	testLocal(ctx, t, module, "branch", "trunk")

	// To play a trick, we add a `super` resource that will depend on each of `leaf`, `branch` and
	// `trunk`. Reconfiguring leaf ought to reconfigure `super` three times. Once for each of its
	// dependencies.
	superImplicitDependsOn, err := protoutils.StructToStructPb(&doCommandDependerConfig{
		DependsOn: []string{"leaf", "branch", "trunk"},
	})
	test.That(t, err, test.ShouldBeNil)

	_, err = module.AddResource(ctx, &pb.AddResourceRequest{
		Dependencies: []string{
			generic.Named("leaf").String(),
			generic.Named("branch").String(),
			generic.Named("trunk").String(),
		},
		Config: &v1.ComponentConfig{
			Name: "super", Api: generic.API.String(), Model: model.String(),
			Attributes: superImplicitDependsOn,
		},
	})
	test.That(t, err, test.ShouldBeNil)
	testLocal(ctx, t, module, "super")

	// Get a handle on the `super` resource that we are going to invalidate.
	staleSuperRes, err := module.getLocalResource(ctx, generic.Named("super"))
	test.That(t, err, test.ShouldBeNil)

	// Reconfigure the leaf _yet_ again. Assert that the `staleSuperRes` does not work.
	logger.Info("Reconfiguring leaf. Super resource should reconfigure 4 times.")
	_, err = module.ReconfigureResource(ctx, &pb.ReconfigureResourceRequest{
		Config: &v1.ComponentConfig{ // Same config.
			Name: "leaf", Api: generic.API.String(), Model: model.String(),
		},
	})
	test.That(t, err, test.ShouldBeNil)

	_, err = staleSuperRes.DoCommand(ctx, map[string]any{})
	test.That(t, err, test.ShouldNotBeNil)

	// Assert that super continues to work with all of its dependencies. Along with all the others
	// for good measure.
	testLocal(ctx, t, module, "super", "trunk", "branch", "leaf")

	// Assert that `super` was constructed once initially plus three times due to cascading
	// reconfigures. This is not a correctness assertion, but just a demonstration of current
	// behavior. By all means optimize this away with some topological sorting.
	test.That(t, superConstructCount, test.ShouldEqual, 4)
}
