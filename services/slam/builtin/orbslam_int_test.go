package builtin_test

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/services/slam/builtin"
	"go.viam.com/rdk/services/slam/internal"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"
)

func createVocabularyFile(name string) error {
	source, err := os.Open(artifact.MustPath("slam/ORBvoc.txt"))
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.Create(name + "/config/ORBvoc.txt")
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return err
}

func TestOrbslamIntegrationExample(t *testing.T) {
	// TODO DATA-364: remove this check
	_, err := exec.LookPath("orb_grpc_server")
	if err != nil {
		t.Log("Skipping test because orb_grpc_server binary was not found")
		t.Skip()
	}

	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)
	createVocabularyFile(name)

	attrCfg := &builtin.AttrConfig{
		Algorithm: "orbslamv3",
		Sensors:   []string{"orbslam_int_color_camera", "orbslam_int_depth_camera"},
		ConfigParams: map[string]string{
			"mode":              "rgbd",
			"orb_n_features":    "1000",
			"orb_scale_factor":  "1.2",
			"orb_n_levels":      "8",
			"orb_n_ini_th_fast": "20",
			"orb_n_min_th_fast": "7",
			"debug":             "true",
		},
		DataDirectory: name,
	}

	// Create slam service using a real orbslam binary
	logger := golog.NewTestLogger(t)
	svc, err := createSLAMService(t, attrCfg, logger, true, true)
	test.That(t, err, test.ShouldBeNil)

	// Wait for orbslam to finish processing images
	logReader := svc.(internal.Service).GetSLAMProcessBufferedLogReader()
	// The bug fixed in DATA-182 revealed an issue with this test framework that since
	// orbslam is running in online mode, it will only consume the most recent image.
	// TODO DATA-363: ensure all images are found
	t.Logf("Find log line for image 0")
	for {
		line, err := logReader.ReadString('\n')
		test.That(t, err, test.ShouldBeNil)
		if strings.Contains(line, "Passed image to SLAM") {
			break
		}
	}

	// Test position and map
	position, err := svc.Position(context.Background(), "test")
	test.That(t, err, test.ShouldBeNil)
	t.Logf("Position point: (%v, %v, %v)",
		position.Pose().Point().X, position.Pose().Point().Y, position.Pose().Point().Z)
	t.Logf("Position orientation: RX: %v, RY: %v, RY: %v, Theta: %v",
		position.Pose().Orientation().AxisAngles().RX,
		position.Pose().Orientation().AxisAngles().RY,
		position.Pose().Orientation().AxisAngles().RZ,
		position.Pose().Orientation().AxisAngles().Theta)
	actualMIME, _, pointcloud, err := svc.GetMap(context.Background(), "test", "pointcloud/pcd", nil, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMIME, test.ShouldResemble, "pointcloud/pcd")
	t.Logf("Pointcloud points: %v", pointcloud.Size())

	// Close out slam service
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	closeOutSLAMService(t, name)
}
