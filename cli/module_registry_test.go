package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestUpdateModelsAction(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	test.That(t, ok, test.ShouldBeTrue)

	dir := filepath.Dir(filename)
	binaryPath := testutils.BuildTempModule(t, "./module/testmodule")
	metaPath := dir + "/../module/testmodule/test_meta.json"
	expectedMetaPath := dir + "/../module/testmodule/expected_meta.json"

	// create a temporary file where we can write the module's metadata
	metaFile, err := os.OpenFile(metaPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	test.That(t, err, test.ShouldBeNil)

	defer func() {
		test.That(t, metaFile.Close(), test.ShouldBeNil)
		test.That(t, os.Remove(metaPath), test.ShouldBeNil)
	}()

	_, err = metaFile.WriteString("{}")
	test.That(t, err, test.ShouldBeNil)

	flags := map[string]any{"binary": binaryPath, "module": metaPath}
	cCtx, _, _, errOut := setup(&inject.AppServiceClient{}, nil, nil, nil, flags, "")
	test.That(t, UpdateModelsAction(cCtx, parseStructFromCtx[updateModelsArgs](cCtx)), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)

	// verify that models added to meta.json are equivalent to those defined in expected_meta.json
	metaModels, err := loadManifest(metaPath)
	test.That(t, err, test.ShouldBeNil)

	expectedMetaModels, err := loadManifest(expectedMetaPath)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, sameModels(metaModels.Models, expectedMetaModels.Models), test.ShouldBeTrue)
}

func TestValidateModelAPI(t *testing.T) {
	err := validateModelAPI("rdk:component:x")
	test.That(t, err, test.ShouldBeNil)
	err = validateModelAPI("rdk:service:x")
	test.That(t, err, test.ShouldBeNil)
	err = validateModelAPI("rdk:unknown:x")
	test.That(t, err, test.ShouldHaveSameTypeAs, unknownRdkAPITypeError{})
	err = validateModelAPI("other:unknown:x")
	test.That(t, err, test.ShouldHaveSameTypeAs, unknownRdkAPITypeError{})
	err = validateModelAPI("rdk:component")
	test.That(t, err, test.ShouldNotBeNil)
	err = validateModelAPI("other:component:$x")
	test.That(t, err, test.ShouldNotBeNil)
	err = validateModelAPI("other:component:x_")
	test.That(t, err, test.ShouldBeNil)
}
