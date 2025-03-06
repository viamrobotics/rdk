package capture

import (
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/datamanager"
)

func TestTargetDir(t *testing.T) {
	test.That(t, targetDir("/some/path", datamanager.DataCaptureConfig{
		Name:   arm.Named("arm1"),
		Method: "JointPositions",
	}, logging.Global()), test.ShouldResemble, "/some/path/rdk_component_arm/arm1/JointPositions")

	homeDir, err := os.UserHomeDir()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, targetDir("~/.viam/capture", datamanager.DataCaptureConfig{
		Name:   camera.Named("camera1"),
		Method: "ReadImage",
	}, logging.Global()), test.ShouldResemble, filepath.Join(homeDir, ".viam/capture/rdk_component_camera/camera1/ReadImage"))
}

func TestDefaultIfZeroVal(t *testing.T) {
	test.That(t, defaultIfZeroVal("non default", "default"), test.ShouldResemble, "non default")
	test.That(t, defaultIfZeroVal("", "default"), test.ShouldResemble, "default")

	nonDefaultInt := 1
	defaultValInt := 2
	test.That(t, defaultIfZeroVal(nonDefaultInt, defaultValInt), test.ShouldResemble, nonDefaultInt)
	test.That(t, defaultIfZeroVal(0, defaultValInt), test.ShouldResemble, defaultValInt)

	nonDefaultF64 := 1.0
	defaultValF64 := 2.0
	test.That(t, defaultIfZeroVal(nonDefaultF64, defaultValF64), test.ShouldAlmostEqual, nonDefaultF64)
	test.That(t, defaultIfZeroVal(0, defaultValF64), test.ShouldAlmostEqual, defaultValF64)
}
