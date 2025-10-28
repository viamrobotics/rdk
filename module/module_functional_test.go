package module

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	v1 "go.viam.com/api/app/v1"
	pb "go.viam.com/api/module/v1"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/shell"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"
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
	dcd.closed = true
	return nil
}

func (dcd *doCommandDepender) DoCommand(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	if dcd.closed {
		return nil, errors.New("was closed")
	}

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

	// Add a child resource that depends on nothing.
	_, err := module.AddResource(ctx, &pb.AddResourceRequest{Config: &v1.ComponentConfig{
		Name: "child", Api: generic.API.String(), Model: model.String(),
	}})
	test.That(t, err, test.ShouldBeNil)

	childRes, err := module.getLocalResource(ctx, generic.Named("child"))
	test.That(t, err, test.ShouldBeNil)

	resp, err := childRes.DoCommand(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	logger.Info("ChildResp:", resp)

	// Build up and add a parent resource that depends on the child resource.
	attrsBuf, err := protoutils.StructToStructPb(&doCommandDependerConfig{
		DependsOn: "child",
	})
	test.That(t, err, test.ShouldBeNil)

	_, err = module.AddResource(ctx, &pb.AddResourceRequest{
		Dependencies: []string{generic.Named("child").String()},
		Config: &v1.ComponentConfig{
			Name: "parent", Api: generic.API.String(), Model: model.String(),
			DependsOn:  []string{"child"}, // unnecessary but mimics reality?
			Attributes: attrsBuf,
		},
	})
	test.That(t, err, test.ShouldBeNil)

	parentRes, err := module.getLocalResource(ctx, generic.Named("parent"))
	test.That(t, err, test.ShouldBeNil)

	resp, err = parentRes.DoCommand(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	logger.Info("ParentResp:", resp)

	// Reconfigure the child. This results in a `Close` -> `Constructor`.
	_, err = module.ReconfigureResource(ctx, &pb.ReconfigureResourceRequest{
		Config: &v1.ComponentConfig{ // Same config.
			Name: "child", Api: generic.API.String(), Model: model.String(),
		},
	})
	test.That(t, err, test.ShouldBeNil)

	// Check if the `parentRes` has its dependency invalidated.
	resp, err = parentRes.DoCommand(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	logger.Info("ParentResp:", resp)
}
