package builtin_test

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
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

const (
	dataInsertionMaxTimeoutMin = 3
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

// Releases an image or image pair to be served by the mock camera(s). If a pair of images is
// released, it is released under a mutex, so that the images will be consumed in the same call
// to getSimultaneousColorAndDepth().
func releaseImages(t *testing.T, mode slam.Mode) {
	switch mode {
	case slam.Mono:
		orbslamIntWebcamReleaseImageChan <- 1
	case slam.Rgbd:
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
	default:
		t.FailNow()
	}
}

// Checks that we can get position and map, and that there are more than zero map points.
// Doesn't check precise values due to variations in orbslam results.
func testOrbslamPositionAndMap(t *testing.T, svc slam.Service) {
	t.Helper()

	position, err := svc.Position(context.Background(), "test", map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	// Typical values for RGBD are around (-0.001, -0.004, -0.008)
	// Typical values for Mono without an existing map are around (0.020, -0.032, -0.053)
	// Typical values for Mono with an existing map are around (0.023, -0.036, -0.040)
	t.Logf("Position point: (%v, %v, %v)",
		position.Pose().Point().X, position.Pose().Point().Y, position.Pose().Point().Z)
	// Typical values for RGBD are around (0.602, -0.772, -0.202), theta=0.002
	// Typical values for Mono without an existing map are around (0.144, 0.980, -0.137), theta=0.104
	// Typical values for Mono with an existing map are around ( 0.092, 0.993, -0.068), theta=0.099
	t.Logf("Position orientation: RX: %v, RY: %v, RZ: %v, Theta: %v",
		position.Pose().Orientation().AxisAngles().RX,
		position.Pose().Orientation().AxisAngles().RY,
		position.Pose().Orientation().AxisAngles().RZ,
		position.Pose().Orientation().AxisAngles().Theta)
	actualMIME, _, pointcloud, err := svc.GetMap(context.Background(), "test", "pointcloud/pcd", nil, false, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMIME, test.ShouldResemble, "pointcloud/pcd")
	// Typical value for RGBD is 329
	// Values for Mono vary
	t.Logf("Pointcloud points: %v", pointcloud.Size())
	test.That(t, pointcloud.Size(), test.ShouldBeGreaterThan, 0)
}

func integrationTestHelper(t *testing.T, mode slam.Mode) {
	_, err := exec.LookPath("orb_grpc_server")
	if err != nil {
		t.Skip("Skipping test because orb_grpc_server binary was not found")
	}

	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)
	createVocabularyFile(name)

	t.Log("Testing online mode")

	var sensors []string
	switch mode {
	case slam.Mono:
		sensors = []string{"orbslam_int_webcam"}
	case slam.Rgbd:
		sensors = []string{"orbslam_int_color_camera", "orbslam_int_depth_camera"}
	default:
		t.FailNow()
	}

	mapRate := 1

	attrCfg := &builtin.AttrConfig{
		Sensors: sensors,
		ConfigParams: map[string]string{
			"mode":              reflect.ValueOf(mode).String(),
			"orb_n_features":    "1250",
			"orb_scale_factor":  "1.2",
			"orb_n_levels":      "8",
			"orb_n_ini_th_fast": "20",
			"orb_n_min_th_fast": "7",
			"debug":             "true",
		},
		DataDirectory: name,
		// Even though we don't use the maps saved in this run, indicate in the config that
		// we want to save maps because the same yaml config gets used for the next run.
		MapRateSec: &mapRate,
	}

	// Release camera image(s) for service validation
	releaseImages(t, mode)
	// Create slam service using a real orbslam binary
	svc, err := createSLAMService(t, attrCfg, "orbslamv3", golog.NewTestLogger(t), true, true)
	test.That(t, err, test.ShouldBeNil)

	// Release camera image(s), since orbslam looks for the second most recent image(s)
	releaseImages(t, mode)
	// Check if orbslam hangs and needs to be shut down
	orbslam_hangs := false
	// Wait for orbslam to finish processing images
	logReader := svc.(internal.Service).GetSLAMProcessBufferedLogReader()
	for i := 0; i < getNumOrbslamImages(mode)-2; i++ {
		start_time_sent_image := time.Now()
		t.Logf("Find log line for image %v", i)
		releaseImages(t, mode)
		for {
			line, err := logReader.ReadString('\n')
			test.That(t, err, test.ShouldBeNil)
			if strings.Contains(line, "Passed image to SLAM") {
				break
			}
			test.That(t, strings.Contains(line, "Fail to track local map!"), test.ShouldBeFalse)
			if time.Since(start_time_sent_image) > time.Duration(dataInsertionMaxTimeoutMin)*time.Minute {
				orbslam_hangs = true
				t.Log("orbslam hangs: exiting the data loop")
				break
			}
		}
		if orbslam_hangs {
			break
		}
	}

	testOrbslamPositionAndMap(t, svc)

	// Close out slam service
	err = utils.TryClose(context.Background(), svc)
	if !orbslam_hangs {
		test.That(t, err, test.ShouldBeNil)
	} else if err != nil {
		t.Skip("Skipping test because orbslam hangs and failed to shut down")
	}

	// Don't clear out the directory, since we will re-use the config and data for the next run
	closeOutSLAMService(t, "")

	// Delete the last image (or image pair) in the data directory, so that offline mode runs on
	// the same data as online mode. (Online mode will not read the last image (or image pair),
	// since it always processes the second-most-recent image (or image pair), in case the
	// most-recent image (or image pair) is currently being written.)
	var directories []string
	switch mode {
	case slam.Mono:
		directories = []string{"rgb/"}
	case slam.Rgbd:
		directories = []string{"rgb/", "depth/"}
	default:
		t.FailNow()
	}
	for _, directoryName := range directories {
		files, err := ioutil.ReadDir(name + "/data/" + directoryName)
		test.That(t, err, test.ShouldBeNil)
		lastFileName := files[len(files)-1].Name()
		test.That(t, os.Remove(name+"/data/"+directoryName+lastFileName), test.ShouldBeNil)
	}

	// Remove any maps
	test.That(t, resetFolder(name+"/map"), test.ShouldBeNil)

	// Test offline mode using the config and data generated in the online test
	t.Log("Testing offline mode")

	mapRate = 1

	attrCfg = &builtin.AttrConfig{
		Sensors: []string{},
		ConfigParams: map[string]string{
			"mode":              reflect.ValueOf(mode).String(),
			"orb_n_features":    "1250",
			"orb_scale_factor":  "1.2",
			"orb_n_levels":      "8",
			"orb_n_ini_th_fast": "20",
			"orb_n_min_th_fast": "7",
			"debug":             "true",
		},
		DataDirectory: name,
		MapRateSec:    &mapRate,
	}

	// Create slam service using a real orbslam binary
	svc, err = createSLAMService(t, attrCfg, "orbslamv3", golog.NewTestLogger(t), true, true)
	test.That(t, err, test.ShouldBeNil)

	// Check if orbslam hangs and needs to be shut down
	orbslam_hangs = false
	start_time_sent_image := time.Now()
	// Wait for orbslam to finish processing images
	logReader = svc.(internal.Service).GetSLAMProcessBufferedLogReader()
	for {
		line, err := logReader.ReadString('\n')
		test.That(t, err, test.ShouldBeNil)
		if strings.Contains(line, "Passed image to SLAM") {
			start_time_sent_image = time.Now()
		}
		if strings.Contains(line, "Finished processing offline images") {
			break
		}
		test.That(t, strings.Contains(line, "Fail to track local map!"), test.ShouldBeFalse)
		if time.Since(start_time_sent_image) > time.Duration(dataInsertionMaxTimeoutMin)*time.Minute {
			orbslam_hangs = true
			t.Log("orbslam hangs: exiting the data loop")
			break
		}
	}

	testOrbslamPositionAndMap(t, svc)

	if !orbslam_hangs {
		// Wait for the final map to be saved
		for {
			line, err := logReader.ReadString('\n')
			test.That(t, err, test.ShouldBeNil)
			if strings.Contains(line, "Finished saving final map") {
				break
			}
		}
	}

	// Close out slam service
	err = utils.TryClose(context.Background(), svc)
	if !orbslam_hangs {
		test.That(t, err, test.ShouldBeNil)
	} else if err != nil {
		t.Skip("Skipping test because orbslam hangs and failed to shut down")
	}

	// Don't clear out the directory, since we will re-use the maps for the next run
	closeOutSLAMService(t, "")

	// Remove existing images, but leave maps and config (so we keep the vocabulary file).
	// Orbslam will use the most recent config.
	test.That(t, resetFolder(name+"/data"), test.ShouldBeNil)

	// Test online mode using the map generated in the offline test
	t.Log("Testing online mode with saved map")

	mapRate = 9999

	attrCfg = &builtin.AttrConfig{
		Sensors: sensors,
		ConfigParams: map[string]string{
			"mode":              reflect.ValueOf(mode).String(),
			"orb_n_features":    "1250",
			"orb_scale_factor":  "1.2",
			"orb_n_levels":      "8",
			"orb_n_ini_th_fast": "20",
			"orb_n_min_th_fast": "7",
			"debug":             "true",
		},
		DataDirectory: name,
		MapRateSec:    &mapRate,
	}

	// Release camera image(s) for service validation
	releaseImages(t, mode)
	// Create slam service using a real orbslam binary
	svc, err = createSLAMService(t, attrCfg, "orbslamv3", golog.NewTestLogger(t), true, true)
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

	// Release camera image(s), since orbslam looks for the second most recent image(s)
	releaseImages(t, mode)
	// Check if orbslam hangs and needs to be shut down
	orbslam_hangs = false
	// Wait for orbslam to finish processing images
	for i := 0; i < getNumOrbslamImages(mode)-2; i++ {
		start_time_sent_image = time.Now()
		t.Logf("Find log line for image %v", i)
		releaseImages(t, mode)
		for {
			line, err := logReader.ReadString('\n')
			test.That(t, err, test.ShouldBeNil)
			if strings.Contains(line, "Passed image to SLAM") {
				break
			}
			test.That(t, strings.Contains(line, "Fail to track local map!"), test.ShouldBeFalse)
			if time.Since(start_time_sent_image) > time.Duration(dataInsertionMaxTimeoutMin)*time.Minute {
				orbslam_hangs = true
				t.Log("orbslam hangs: exiting the data loop")
				break
			}
		}
		if orbslam_hangs {
			break
		}
	}

	testOrbslamPositionAndMap(t, svc)

	// Close out slam service
	err = utils.TryClose(context.Background(), svc)
	if !orbslam_hangs {
		test.That(t, err, test.ShouldBeNil)
	} else if err != nil {
		t.Skip("Skipping test because orbslam hangs and failed to shut down")
	}

	// Clear out directory
	closeOutSLAMService(t, name)

}

func TestOrbslamIntegrationRGBD(t *testing.T) {
	integrationTestHelper(t, slam.Rgbd)
}

func TestOrbslamIntegrationMono(t *testing.T) {
	integrationTestHelper(t, slam.Mono)
}
