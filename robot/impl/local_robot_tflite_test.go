//go:build !no_tflite

package robotimpl_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	// registers all components.

	"go.viam.com/test"
	"go.viam.com/utils"

	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	putils "go.viam.com/rdk/robot/packages/testutils"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/services/mlmodel/tflitecpu"
	_ "go.viam.com/rdk/services/register"
)

func TestConfigPackageReferenceReplacement(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	fakePackageServer, err := putils.NewFakePackageServer(ctx, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(fakePackageServer.Shutdown)

	packageDir := t.TempDir()
	labelPath := "${packages.orgID/some-name-2}/labels.txt"

	robotConfig := &config.Config{
		Packages: []config.PackageConfig{
			{
				Name:    "some-name-1",
				Package: "package-1",
				Version: "v1",
			},
			{
				Name:    "orgID/some-name-2",
				Package: "package-2",
				Version: "latest",
			},
			{
				Name:    "my-module",
				Package: "orgID/my-module",
				Type:    config.PackageTypeModule,
				Version: "1.2",
			},
			{
				Name:    "my-ml-model",
				Package: "orgID/my-ml-model",
				Type:    config.PackageTypeMlModel,
				Version: "latest",
			},
		},
		PackagePath: packageDir,
		Services: []resource.Config{
			{
				Name:  "ml-model-service",
				API:   mlmodel.API,
				Model: resource.DefaultModelFamily.WithModel("tflite_cpu"),
				ConvertedAttributes: &tflitecpu.TFLiteConfig{
					ModelPath:  "${packages.some-name-1}/model.tflite",
					LabelPath:  labelPath,
					NumThreads: 1,
				},
			},
			{
				Name:  "my-ml-model",
				API:   mlmodel.API,
				Model: resource.DefaultModelFamily.WithModel("tflite_cpu"),
				ConvertedAttributes: &tflitecpu.TFLiteConfig{
					ModelPath:  "${packages.ml_models.my-ml-model}/model.tflite",
					LabelPath:  labelPath,
					NumThreads: 2,
				},
			},
		},
		Modules: []config.Module{
			{
				Name:    "my-module",
				ExePath: "${packages.modules.my-module}/exec.sh",
			},
		},
	}

	fakePackageServer.StorePackage(robotConfig.Packages...)

	r, err := robotimpl.New(ctx, robotConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}
