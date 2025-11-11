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
	DependsOn string
}

func (dcdc *doCommandDependerConfig) Validate(path string) ([]string, []string, error) {
	if len(dcdc.DependsOn) > 0 {
		return []string{}, []string{dcdc.DependsOn}, nil
	}

	return []string{}, []string{}, nil
}

type doCommandDepender struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable

	dependsOn resource.Resource
	closed    bool

	logger logging.Logger
}

func (dcd *doCommandDepender) Close(ctx context.Context) error {
	dcd.logger.Infof("Closing %v %p, depends: %p\n", dcd.Name(), dcd, dcd.dependsOn)
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
	if dcd.dependsOn != nil {
		if val, exists := cmd["outgoing"]; exists {
			cmd["outgoing"] = val.(int) + 1
		} else {
			cmd["outgoing"] = 1
		}

		res, err := dcd.dependsOn.DoCommand(ctx, cmd)
		if err != nil {
			return nil, err
		}

		res["incoming"] = res["incoming"].(int) + 1
		return res, nil
	}

	var outgoing any = int(0)
	if val, exists := cmd["outgoing"]; exists {
		outgoing = val
	}

	return map[string]any{
		"outgoing": outgoing,
		"incoming": 1,
	}, nil
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
	resource.RegisterComponent(generic.API, model, resource.Registration[resource.Resource, *doCommandDependerConfig]{
		Constructor: func(
			ctx context.Context, deps resource.Dependencies, rcfg resource.Config, logger logging.Logger,
		) (resource.Resource, error) {
			cfg, err := resource.NativeConfig[*doCommandDependerConfig](rcfg)
			if err != nil {
				return nil, err
			}

			ret := &doCommandDepender{
				Named:  rcfg.ResourceName().AsNamed(),
				logger: logger,
			}

			if len(cfg.DependsOn) > 0 {
				ret.dependsOn, err = generic.FromProvider(deps, cfg.DependsOn)
				logger.Infof("I %v (%p) depend on %p Config: %v\n",
					rcfg.ResourceName().Name, ret, ret.dependsOn, cfg.DependsOn)
				if err != nil {
					return nil, err
				}
			}

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

	leafRes, err := module.getLocalResource(ctx, generic.Named("leaf"))
	test.That(t, err, test.ShouldBeNil)

	_, err = leafRes.DoCommand(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)

	// Build up and add a branch resource that depends on the leaf resource.
	attrsBuf, err := protoutils.StructToStructPb(&doCommandDependerConfig{
		DependsOn: "leaf",
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

	branchRes, err := module.getLocalResource(ctx, generic.Named("branch"))
	test.That(t, err, test.ShouldBeNil)

	_, err = branchRes.DoCommand(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)

	// Reconfigure the leaf. This results in a `Close` -> `Constructor`.
	logger.Info("Reconfiguring leaf first time")
	_, err = module.ReconfigureResource(ctx, &pb.ReconfigureResourceRequest{
		Config: &v1.ComponentConfig{ // Same config.
			Name: "leaf", Api: generic.API.String(), Model: model.String(),
		},
	})
	test.That(t, err, test.ShouldBeNil)

	// Assert that the original `branchRes` has its dependency invalidated.
	_, err = branchRes.DoCommand(ctx, map[string]any{})
	test.That(t, err, test.ShouldNotBeNil)

	// Get a fresh value for the `branchRes`.
	branchRes, err = module.getLocalResource(ctx, generic.Named("branch"))
	test.That(t, err, test.ShouldBeNil)

	_, err = branchRes.DoCommand(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)

	// Build up and add a branch resource that depends on the leaf resource.
	trunkAttrsBuf, err := protoutils.StructToStructPb(&doCommandDependerConfig{
		DependsOn: "branch",
	})
	test.That(t, err, test.ShouldBeNil)
	// Add a trunk that depends on the branch resource.
	_, err = module.AddResource(ctx, &pb.AddResourceRequest{
		Dependencies: []string{generic.Named("branch").String()},
		Config: &v1.ComponentConfig{
			Name: "trunk", Api: generic.API.String(), Model: model.String(),
			DependsOn:  []string{"branch"}, // unnecessary but mimics reality?
			Attributes: trunkAttrsBuf,
		},
	})
	test.That(t, err, test.ShouldBeNil)

	trunkRes, err := module.getLocalResource(ctx, generic.Named("trunk"))
	test.That(t, err, test.ShouldBeNil)

	_, err = trunkRes.DoCommand(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)

	logger.Info("Reconfiguring leaf second time")
	// Reconfigure the leaf again. This results in a `Close` -> `Constructor` on both the leaf
	// _and_ the branch _and_ the trunk. Refetch the branch and trunk and assert they
	// both trunk can continue respond to `DoCommand`s that require dependencies to be valid.

	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = module.ReconfigureResource(canceledCtx, &pb.ReconfigureResourceRequest{
		Config: &v1.ComponentConfig{ // Same config.
			Name: "leaf", Api: generic.API.String(), Model: model.String(),
		},
	})
	test.That(t, err, test.ShouldBeNil)

	// Get a fresh value for the `branchRes`.
	branchRes, err = module.getLocalResource(ctx, generic.Named("branch"))
	test.That(t, err, test.ShouldBeNil)

	_, err = branchRes.DoCommand(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)

	trunkRes, err = module.getLocalResource(ctx, generic.Named("branch"))
	test.That(t, err, test.ShouldBeNil)

	_, err = trunkRes.DoCommand(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
}
