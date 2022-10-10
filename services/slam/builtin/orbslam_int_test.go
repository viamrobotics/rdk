package builtin_test

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/builtin"
	"go.viam.com/rdk/services/slam/internal"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"
)

// Creates the vocabulary file required by the orbslam binary.
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

// Releases an image pair to be served by the mock camera. The pair is released under a mutex,
// so that they will be consumed in the same call to getSimultaneousColorAndDepth().
func releaseImages() {
	for {
		orbslamIntCameraMutex.Lock()
		if len(orbslamIntCameraReleaseImagesChan) == cap(orbslamIntCameraReleaseImagesChan) {
			orbslamIntCameraMutex.Unlock()
			time.Sleep(10 * time.Millisecond)
		} else {
			orbslamIntCameraReleaseImagesChan <- 1
			orbslamIntCameraReleaseImagesChan <- 1
			orbslamIntCameraMutex.Unlock()
			return
		}
	}
}

// Checks that we can get position and map, and that there are more than zero map points.
// Doesn't check precise values due to variations in orbslam results.
func testPositionAndMap(t *testing.T, svc slam.Service) {
	t.Helper()

	position, err := svc.Position(context.Background(), "test")
	test.That(t, err, test.ShouldBeNil)
	// Typical values are around (0.001, -0.001, 0.007)
	t.Logf("Position point: (%v, %v, %v)",
		position.Pose().Point().X, position.Pose().Point().Y, position.Pose().Point().Z)
	// Typical values are around (-0.73, 0.34, 0.58), theta=2.6
	t.Logf("Position orientation: RX: %v, RY: %v, RY: %v, Theta: %v",
		position.Pose().Orientation().AxisAngles().RX,
		position.Pose().Orientation().AxisAngles().RY,
		position.Pose().Orientation().AxisAngles().RZ,
		position.Pose().Orientation().AxisAngles().Theta)
	actualMIME, _, pointcloud, err := svc.GetMap(context.Background(), "test", "pointcloud/pcd", nil, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMIME, test.ShouldResemble, "pointcloud/pcd")
	// Typical value is 329
	t.Logf("Pointcloud points: %v", pointcloud.Size())
	test.That(t, pointcloud.Size(), test.ShouldBeGreaterThan, 0)
}

func TestOrbslamIntegration(t *testing.T) {
	_, err := exec.LookPath("orb_grpc_server")
	if err != nil {
		t.Log("Skipping test because orb_grpc_server binary was not found")
		t.Skip()
	}

	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)
	createVocabularyFile(name)

	t.Log("Testing online mode")

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
		// Even though we don't use the maps saved in this run, indicate in the config that
		// we want to save maps because the same yaml config gets used for the next run.
		MapRateSec: 1,
	}

	// Release a pair of camera images for service validation
	releaseImages()
	// Create slam service using a real orbslam binary
	svc, err := createSLAMService(t, attrCfg, golog.NewTestLogger(t), true, true)
	test.That(t, err, test.ShouldBeNil)

	// Release a pair of camera images, since orbslam looks for the second most recent pair
	releaseImages()
	// Wait for orbslam to finish processing images
	logReader := svc.(internal.Service).GetSLAMProcessBufferedLogReader()
	for i := 0; i < numOrbslamImages-2; i++ {
		t.Logf("Find log line for image %v", i)
		releaseImages()
		for {
			line, err := logReader.ReadString('\n')
			test.That(t, err, test.ShouldBeNil)
			if strings.Contains(line, "Passed image to SLAM") {
				break
			}
			test.That(t, strings.Contains(line, "Fail to track local map!"), test.ShouldBeFalse)
		}
	}

	testPositionAndMap(t, svc)

	// Close out slam service
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	// Don't clear out the directory, since we will re-use the config and data for the next run
	closeOutSLAMService(t, "")

	// Delete the last image pair, so that offline mode runs on the same set of images
	for _, directoryName := range [2]string{"rgb/", "depth/"} {
		files, err := ioutil.ReadDir(name + "/data/" + directoryName)
		test.That(t, err, test.ShouldBeNil)
		lastFileName := files[len(files)-1].Name()
		test.That(t, os.Remove(name+"/data/"+directoryName+lastFileName), test.ShouldBeNil)
	}

	// Remove any maps
	test.That(t, os.RemoveAll(name+"/map"), test.ShouldBeNil)

	// Test offline mode using the config and data generated in the online test
	t.Log("Testing offline mode")

	attrCfg = &builtin.AttrConfig{
		Algorithm: "orbslamv3",
		Sensors:   []string{},
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
		MapRateSec:    1,
	}

	// Create slam service using a real orbslam binary
	svc, err = createSLAMService(t, attrCfg, golog.NewTestLogger(t), true, true)
	test.That(t, err, test.ShouldBeNil)

	// Wait for orbslam to finish processing images
	logReader = svc.(internal.Service).GetSLAMProcessBufferedLogReader()
	for {
		line, err := logReader.ReadString('\n')
		test.That(t, err, test.ShouldBeNil)
		if strings.Contains(line, "Finished processing offline images") {
			break
		}
		test.That(t, strings.Contains(line, "Fail to track local map!"), test.ShouldBeFalse)
	}

	testPositionAndMap(t, svc)

	// Wait for the final map to be saved
	for {
		line, err := logReader.ReadString('\n')
		test.That(t, err, test.ShouldBeNil)
		if strings.Contains(line, "Finished saving final map") {
			break
		}
	}

	// Close out slam service
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	// Don't clear out the directory, since we will re-use the maps for the next run
	closeOutSLAMService(t, "")

	// Remove existing images, but leave maps and config (so we keep the vocabulary file).
	// Orbslam will use the most recent config.
	test.That(t, os.RemoveAll(name+"/data"), test.ShouldBeNil)

	// Test online mode using the map generated in the offline test
	t.Log("Testing online mode with saved map")

	attrCfg = &builtin.AttrConfig{
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
		MapRateSec:    -1,
	}

	// Release a pair of camera images for service validation
	releaseImages()
	// Create slam service using a real orbslam binary
	svc, err = createSLAMService(t, attrCfg, golog.NewTestLogger(t), true, true)
	test.That(t, err, test.ShouldBeNil)

	// Make sure we initialize from a saved map
	logReader = svc.(internal.Service).GetSLAMProcessBufferedLogReader()
	for {
		line, err := logReader.ReadString('\n')
		test.That(t, err, test.ShouldBeNil)
		if strings.Contains(line, "Initialization of Atlas from file") {
			break
		}
		test.That(t, strings.Contains(line, "Initialization of Atlas from scratch"), test.ShouldBeFalse)
	}

	// Release a pair of camera images, since orbslam looks for the second most recent pair
	releaseImages()
	// Wait for orbslam to finish processing images
	for i := 0; i < numOrbslamImages-2; i++ {
		t.Logf("Find log line for image %v", i)
		releaseImages()
		for {
			line, err := logReader.ReadString('\n')
			test.That(t, err, test.ShouldBeNil)
			if strings.Contains(line, "Passed image to SLAM") {
				break
			}
			test.That(t, strings.Contains(line, "Fail to track local map!"), test.ShouldBeFalse)
		}
	}

	testPositionAndMap(t, svc)

	// Close out slam service
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	// Clear out directory
	closeOutSLAMService(t, name)
}
