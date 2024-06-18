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
	if !ok {
		t.FailNow()
	}
	dir := filepath.Dir(filename)
	binaryPath := testutils.BuildTempModule(t, "./module/testmodule")
	modulePath := dir + "/../module/testmodule/module.json"
	metaPath := dir + "/../module/testmodule/meta.json"

	asc := &inject.AppServiceClient{}
	flags := map[string]any{"binary": binaryPath, "module": metaPath}
	cCtx, _, _, _ := setup(asc, nil, nil, nil, flags, "")
	err := UpdateModelsAction(cCtx)
	if err != nil {
		t.FailNow()
	}

	// verify that the models added to meta.json are equivalent to those in module.json
	moduleModels, err1 := loadManifest(modulePath)
	metaModels, err2 := loadManifest(metaPath)
	if err1 != nil || err2 != nil {
		t.FailNow()
	}
	test.That(t, sameModels(moduleModels.Models, metaModels.Models), test.ShouldBeTrue)
}
