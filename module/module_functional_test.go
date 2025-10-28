package module

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	v1 "go.viam.com/api/app/v1"
	pb "go.viam.com/api/module/v1"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/test"
	"go.viam.com/utils"
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

func TestOptimizedModuleCommunication(t *testing.T) {
	ctx := t.Context()
	logger := logging.NewTestLogger(t)

	modelName := utils.RandomAlphaString(20)
	model := resource.DefaultModelFamily.WithModel(modelName)
	logger.Info("Randomized model name:", modelName)

	module := setupLocalModule(t, ctx, logging.NewTestLogger(t))
	test.That(t, module.AddModelFromRegistry(ctx, generic.API, model), test.ShouldBeNil)

	componentConfig := v1.ComponentConfig{
		Name: "component", Api: generic.API.String(), Model: model.String(),
	}

	_, err := module.AddResource(ctx, &pb.AddResourceRequest{Config: &componentConfig})
	test.That(t, err, test.ShouldBeNil)

}
