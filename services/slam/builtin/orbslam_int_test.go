package builtin_test

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/internal/testhelper"
	"go.viam.com/rdk/spatialmath"
	slamConfig "go.viam.com/rdk/services/slam/slam_copy/config"
	slamTesthelper "go.viam.com/rdk/services/slam/slam_copy/testhelper"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"
)

const (
	dataInsertionMaxTimeoutMin = 3
	orbSleepMs                 = 100
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

// Checks the orbslam map and confirms there are more than zero map points.
func testOrbslamMap(t *testing.T, svc slam.Service) {
	pcd, err := slam.GetPointCloudMapFull(context.Background(), svc, "test")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pcd, test.ShouldNotBeNil)

	pointcloud, err := pointcloud.ReadPCD(bytes.NewReader(pcd))
	t.Logf("Pointcloud points: %v", pointcloud.Size())
	test.That(t, pointcloud.Size(), test.ShouldBeGreaterThanOrEqualTo, 100)
}

// Checks the orbslam position within a defined tolerance
func testOrbslamPosition(t *testing.T, svc slam.Service, mode, actionMode string, expectedComponentRef string) {
	var expectedPos r3.Vector
	expectedOri := &spatialmath.R4AA{}
	tolerancePos := 0.5
	toleranceOri := 0.5

	switch {
	case mode == "mono" && actionMode == "mapping":
		expectedPos = r3.Vector{X: 0.020, Y: -0.032, Z: -0.053}
		expectedOri = &spatialmath.R4AA{Theta: 0.104, RX: 0.144, RY: 0.980, RZ: -0.137}
	case mode == "mono" && actionMode == "updating":
		expectedPos = r3.Vector{X: 0.023, Y: -0.036, Z: -0.040}
		expectedOri = &spatialmath.R4AA{Theta: 0.099, RX: 0.092, RY: 0.993, RZ: -0.068}
	case mode == "rgbd":
		expectedPos = r3.Vector{X: -0.001, Y: -0.004, Z: -0.008}
		expectedOri = &spatialmath.R4AA{Theta: 0.002, RX: 0.602, RY: -0.772, RZ: -0.202}
	}

	position, componentRef, err := svc.GetPosition(context.Background(), "test")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, componentRef, test.ShouldEqual, expectedComponentRef)

	actualPos := position.Point()
	t.Logf("Position point: (%v, %v, %v)", actualPos.X, actualPos.Y, actualPos.Z)
	test.That(t, actualPos.X, test.ShouldBeBetween, expectedPos.X-tolerancePos, expectedPos.X+tolerancePos)
	test.That(t, actualPos.Y, test.ShouldBeBetween, expectedPos.Y-tolerancePos, expectedPos.Y+tolerancePos)
	test.That(t, actualPos.Z, test.ShouldBeBetween, expectedPos.Z-tolerancePos, expectedPos.Z+tolerancePos)

	actualOri := position.Orientation().AxisAngles()
	t.Logf("Position orientation: RX: %v, RY: %v, RZ: %v, Theta: %v", actualOri.RX, actualOri.RY, actualOri.RZ, actualOri.Theta)
	test.That(t, actualOri.RX, test.ShouldBeBetween, expectedOri.RX-toleranceOri, expectedOri.RX+toleranceOri)
	test.That(t, actualOri.RY, test.ShouldBeBetween, expectedOri.RY-toleranceOri, expectedOri.RY+toleranceOri)
	test.That(t, actualOri.RZ, test.ShouldBeBetween, expectedOri.RZ-toleranceOri, expectedOri.RZ+toleranceOri)
	test.That(t, actualOri.Theta, test.ShouldBeBetween, expectedOri.Theta-toleranceOri, expectedOri.Theta+toleranceOri)
}

// Checks the orbslam internal state.
func testOrbslamInternalState(t *testing.T, svc slam.Service, dataDir string) {
	internalState, err := slam.GetInternalStateFull(context.Background(), svc, "test")
	test.That(t, err, test.ShouldBeNil)

	// Save the data from the call to GetInternalState for use in next test.
	timeStamp := time.Now()
	filename := filepath.Join(dataDir, "map", "orbslam_int_color_camera_data_"+timeStamp.UTC().Format(slamTimeFormat)+".osa")
	err = os.WriteFile(filename, internalState, 0644)
	test.That(t, err, test.ShouldBeNil)
}

func integrationTestHelperOrbslam(t *testing.T, mode slam.Mode) {
	_, err := exec.LookPath("orb_grpc_server")
	if err != nil {
		t.Skip("Skipping test because orb_grpc_server binary was not found")
	}

	logger := golog.NewTestLogger(t)
	name, err := slamTesthelper.CreateTempFolderArchitecture(logger)
	test.That(t, err, test.ShouldBeNil)
	createVocabularyFile(name)
	prevNumFiles := 0

	t.Log("\n=== Testing online mode ===\n")

	var sensors []string
	var expectedMapsOnline, expectedMapsOffline, expectedMapsApriori int
	switch mode {
	case slam.Mono:
		sensors = []string{"orbslam_int_webcam"}
		expectedMapsOnline = 0
		expectedMapsOffline = 1
		expectedMapsApriori = expectedMapsOnline + 1
	case slam.Rgbd:
		sensors = []string{"orbslam_int_color_camera", "orbslam_int_depth_camera"}
		expectedMapsOnline = 5
		expectedMapsOffline = 1
		expectedMapsApriori = expectedMapsOnline + 1
	default:
		t.FailNow()
	}

	mapRate := 1
	deleteProcessedData := false
	useLiveData := true

	attrCfg := &slamConfig.AttrConfig{
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
		MapRateSec:          &mapRate,
		DeleteProcessedData: &deleteProcessedData,
		UseLiveData:         &useLiveData,
	}

	// Release camera image(s) for service validation
	releaseImages(t, mode)
	// Create slam service using a real orbslam binary
	svc, err := createSLAMService(t, attrCfg, "orbslamv3", logger, true, true)
	test.That(t, err, test.ShouldBeNil)

	// Release camera image(s), since orbslam looks for the second most recent image(s)
	releaseImages(t, mode)
	// Check if orbslam hangs and needs to be shut down
	orbslam_hangs := false

	// Wait for orbslam to finish processing images
	logReader := svc.(testhelper.Service).GetSLAMProcessBufferedLogReader()
	for i := 0; i < getNumOrbslamImages(mode)-2; i++ {
		start_time_sent_image := time.Now()
		t.Logf("Find log line for image %v", i)
		releaseImages(t, mode)
		for {
			line, err := logReader.ReadString('\n')
			test.That(t, err, test.ShouldBeNil)
			if strings.Contains(line, "Passed image to SLAM") {
				prevNumFiles = slamTesthelper.CheckDeleteProcessedData(t, mode, name, prevNumFiles, deleteProcessedData, useLiveData)
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

	testOrbslamPosition(t, svc, reflect.ValueOf(mode).String(), "mapping", attrCfg.Sensors[0])
	testOrbslamMap(t, svc)

	// Close out slam service
	err = utils.TryClose(context.Background(), svc)
	if !orbslam_hangs {
		test.That(t, err, test.ShouldBeNil)
	} else if err != nil {
		t.Skip("Skipping test because orbslam hangs and failed to shut down")
	}

	// Don't clear out the directory, since we will re-use the config and data for the next run
	closeOutSLAMService(t, "")

	// Added sleep to ensure orbslam stops
	time.Sleep(time.Millisecond * orbSleepMs)

	// test orbslam directory, should have 2 configs
	testOrbslamDir(t, name, expectedMapsOnline, 2)

	// Delete the last image (or image pair) in the data directory, so that offline mode runs on
	// the same data as online mode. (Online mode will not read the last image (or image pair),
	// since it always processes the second-most-recent image (or image pair), in case the
	// most-recent image (or image pair) is currently being written.
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
	prevNumFiles -= 1

	// Remove any maps
	test.That(t, slamTesthelper.ResetFolder(name+"/map"), test.ShouldBeNil)

	// Test offline mode using the config and data generated in the online test
	t.Log("\n=== Testing offline mode ===\n")

	mapRate = 1
	deleteProcessedData = false
	useLiveData = false

	attrCfg = &slamConfig.AttrConfig{
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
		DataDirectory:       name,
		MapRateSec:          &mapRate,
		DeleteProcessedData: &deleteProcessedData,
		UseLiveData:         &useLiveData,
	}

	// Create slam service using a real orbslam binary
	svc, err = createSLAMService(t, attrCfg, "orbslamv3", golog.NewTestLogger(t), true, true)
	test.That(t, err, test.ShouldBeNil)

	// Check if orbslam hangs and needs to be shut down
	orbslam_hangs = false

	start_time_sent_image := time.Now()
	// Wait for orbslam to finish processing images
	logReader = svc.(testhelper.Service).GetSLAMProcessBufferedLogReader()
	for {
		line, err := logReader.ReadString('\n')
		test.That(t, err, test.ShouldBeNil)
		if strings.Contains(line, "Passed image to SLAM") {
			prevNumFiles = slamTesthelper.CheckDeleteProcessedData(t, mode, name, prevNumFiles, deleteProcessedData, useLiveData)
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

	testOrbslamPosition(t, svc, reflect.ValueOf(mode).String(), "mapping", sensors[0]) // setting to sensors[0] because orbslam interprets the component reference in offline mode
	testOrbslamMap(t, svc)

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

	// Remove maps so that testing is done on the map generated by the internal map
	test.That(t, slamTesthelper.ResetFolder(name+"/map"), test.ShouldBeNil)

	testOrbslamInternalState(t, svc, name)

	// Close out slam service
	err = utils.TryClose(context.Background(), svc)
	if !orbslam_hangs {
		test.That(t, err, test.ShouldBeNil)
	} else if err != nil {
		t.Skip("Skipping test because orbslam hangs and failed to shut down")
	}

	// Don't clear out the directory, since we will re-use the maps for the next run
	closeOutSLAMService(t, "")

	// Added sleep to ensure orbslam stops
	time.Sleep(time.Millisecond * orbSleepMs)

	// test orbslam directory, should have 2 configs
	testOrbslamDir(t, name, expectedMapsOffline, 2)

	// Remove existing images, but leave maps and config (so we keep the vocabulary file).
	// Orbslam will use the most recent config.
	test.That(t, slamTesthelper.ResetFolder(name+"/data"), test.ShouldBeNil)
	prevNumFiles = 0

	// Test online mode using the map generated in the offline test
	t.Log("\n=== Testing online mode with saved map ===\n")

	mapRate = 1
	deleteProcessedData = true
	useLiveData = true

	attrCfg = &slamConfig.AttrConfig{
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
		DataDirectory:       name,
		MapRateSec:          &mapRate,
		DeleteProcessedData: &deleteProcessedData,
		UseLiveData:         &useLiveData,
	}

	// Release camera image(s) for service validation
	releaseImages(t, mode)
	// Create slam service using a real orbslam binary
	svc, err = createSLAMService(t, attrCfg, "orbslamv3", golog.NewTestLogger(t), true, true)
	test.That(t, err, test.ShouldBeNil)

	// Make sure we initialize from a saved map
	logReader = svc.(testhelper.Service).GetSLAMProcessBufferedLogReader()
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
				prevNumFiles = slamTesthelper.CheckDeleteProcessedData(t, mode, name, prevNumFiles, deleteProcessedData, useLiveData)
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

	testOrbslamPosition(t, svc, reflect.ValueOf(mode).String(), "updating", attrCfg.Sensors[0])
	testOrbslamMap(t, svc)

	// Close out slam service
	err = utils.TryClose(context.Background(), svc)
	if !orbslam_hangs {
		test.That(t, err, test.ShouldBeNil)
	} else if err != nil {
		t.Skip("Skipping test because orbslam hangs and failed to shut down")
	}

	// Added sleep to ensure orbslam stops
	time.Sleep(time.Millisecond * orbSleepMs)

	// test orbslam directory, should have 3 configs
	testOrbslamDir(t, name, expectedMapsApriori, 3)

	// Clear out directory
	closeOutSLAMService(t, name)

}

// Checks the current slam directory to see if the number of files is around the expected amount
// Because how orbslam runs, the number of maps is not the same between integration tests
func testOrbslamDir(t *testing.T, path string, expectedMaps int, expectedConfigs int) {
	mapsInDir, err := ioutil.ReadDir(path + "/map/")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(mapsInDir), test.ShouldBeGreaterThanOrEqualTo, expectedMaps)

	configsInDir, err := ioutil.ReadDir(path + "/config/")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(configsInDir), test.ShouldEqual, expectedConfigs)
}

func TestOrbslamIntegrationRGBD(t *testing.T) {
	integrationTestHelperOrbslam(t, slam.Rgbd)
}

func TestOrbslamIntegrationMono(t *testing.T) {
	integrationTestHelperOrbslam(t, slam.Mono)
}
