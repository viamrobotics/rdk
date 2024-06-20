package cli

import (
	"path/filepath"
	"runtime"
	"testing"

	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
)

func TestUpdateModelsAction(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
    test.That(t, ok, test.ShouldBeTrue)

	dir := filepath.Dir(filename)
	binaryPath := testutils.BuildTempModule(t, "./module/testmodule")
	modulePath := dir + "/../module/testmodule/module.json"
	metaPath := dir + "/../module/testmodule/meta.json"

	flags := map[string]any{"binary": binaryPath, "module": metaPath}
	cCtx, _, _, _ := setup(&inject.AppServiceClient{}, nil, nil, nil, flags, "")
    test.That(t, UpdateModelsAction(cCtx), test.ShouldBeNil)

	// verify that models added to meta.json are equivalent to those defined in module.json
	moduleModels, err := loadManifest(modulePath)
    test.That(t, err, test.ShouldBeNil)

	metaModels, err := loadManifest(metaPath)
    test.That(t, err, test.ShouldBeNil)

	test.That(t, sameModels(moduleModels.Models, metaModels.Models), test.ShouldBeTrue)
}
