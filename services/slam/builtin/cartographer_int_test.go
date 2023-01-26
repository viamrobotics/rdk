package builtin_test

import (
	"context"
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
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/builtin"
	"go.viam.com/rdk/services/slam/internal/testhelper"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
	"go.viam.com/utils"
)

const (
	cartoSleepMs = 100
)

// Checks the cartographer map and confirms there at least 100 map points.
func testCartographerMap(t *testing.T, svc slam.Service) {
	actualMIME, _, pointcloud, err := svc.GetMap(context.Background(), "test", "pointcloud/pcd", nil, false, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMIME, test.ShouldResemble, "pointcloud/pcd")
	t.Logf("Pointcloud points: %v", pointcloud.Size())
	test.That(t, pointcloud.Size(), test.ShouldBeGreaterThanOrEqualTo, 100)
}

// Checks the cartographer position within a defined tolerance.
func testCartographerPosition(t *testing.T, svc slam.Service) {
	expectedPos := r3.Vector{X: -0.004, Y: 0.004, Z: 0}
	tolerancePos := 0.01
	expectedOri := &spatialmath.OrientationVector{Theta: 0, OX: 0, OY: 0, OZ: -1}
	toleranceOri := 0.5

	position, err := svc.Position(context.Background(), "test", map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)

	actualPos := position.Pose().Point()
	t.Logf("Position point: (%v, %v, %v)", actualPos.X, actualPos.Y, actualPos.Z)
	test.That(t, actualPos.X, test.ShouldBeBetween, expectedPos.X-tolerancePos, expectedPos.X+tolerancePos)
	test.That(t, actualPos.Y, test.ShouldBeBetween, expectedPos.Y-tolerancePos, expectedPos.Y+tolerancePos)
	test.That(t, actualPos.Z, test.ShouldBeBetween, expectedPos.Z-tolerancePos, expectedPos.Z+tolerancePos)

	actualOri := position.Pose().Orientation().AxisAngles()
	t.Logf("Position orientation: RX: %v, RY: %v, RZ: %v, Theta: %v", actualOri.RX, actualOri.RY, actualOri.RZ, actualOri.Theta)
	test.That(t, actualOri.RX, test.ShouldBeBetween, expectedOri.OX-toleranceOri, expectedOri.OX+toleranceOri)
	test.That(t, actualOri.RY, test.ShouldBeBetween, expectedOri.OY-toleranceOri, expectedOri.OY+toleranceOri)
	test.That(t, actualOri.RZ, test.ShouldBeBetween, expectedOri.OZ-toleranceOri, expectedOri.OZ+toleranceOri)
	test.That(t, actualOri.Theta, test.ShouldBeBetween, expectedOri.Theta-toleranceOri, expectedOri.Theta+toleranceOri)
}

// Checks the cartographer internal state.
func testCartographerInternalState(t *testing.T, svc slam.Service, dataDir string) {
	internalState, err := svc.GetInternalState(context.Background(), "test")
	test.That(t, err, test.ShouldBeNil)

	// Save the data from the call to GetInternalState for use in next test.
	timeStamp := time.Now()
	filename := filepath.Join(dataDir, "map", "map_data_"+timeStamp.UTC().Format(slamTimeFormat)+".pbstream")
	err = os.WriteFile(filename, internalState, 0644)
	test.That(t, err, test.ShouldBeNil)
}

func integrationtestHelperCartographer(t *testing.T, mode slam.Mode) {
	_, err := exec.LookPath("carto_grpc_server")
	if err != nil {
		t.Log("Skipping test because carto_grpc_server binary was not found")
		t.Skip()
	}

	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)

	prevNumFiles := 0

	t.Log("\n=== Testing online mode ===\n")

	mapRate := 1
	deleteProcessedData := false
	useLiveData := true

	attrCfg := &builtin.AttrConfig{
		Sensors: []string{"cartographer_int_lidar"},
		ConfigParams: map[string]string{
			"mode":  reflect.ValueOf(mode).String(),
			"v":     "1",
			"debug": "true",
		},
		MapRateSec:          &mapRate,
		DataDirectory:       name,
		DeleteProcessedData: &deleteProcessedData,
		UseLiveData:         &useLiveData,
	}

	// Release point cloud for service validation
	cartographerIntLidarReleasePointCloudChan <- 1
	// Create slam service using a real cartographer binary
	svc, err := createSLAMService(t, attrCfg, "cartographer", golog.NewTestLogger(t), true, true)
	test.That(t, err, test.ShouldBeNil)

	// Release point cloud, since cartographer looks for the second most recent point cloud
	cartographerIntLidarReleasePointCloudChan <- 1
	// Wait for cartographer to finish processing data
	logReader := svc.(testhelper.Service).GetSLAMProcessBufferedLogReader()
	for i := 0; i < numCartographerPointClouds-2; i++ {
		t.Logf("Find log line for point cloud %v", i)
		cartographerIntLidarReleasePointCloudChan <- 1
		for {
			line, err := logReader.ReadString('\n')
			test.That(t, err, test.ShouldBeNil)
			if strings.Contains(line, "Passed sensor data to SLAM") {
				prevNumFiles = checkDeleteProcessedData(t, mode, name, prevNumFiles, len(attrCfg.Sensors) != 0, deleteProcessedData)
				break
			}
		}
	}

	testCartographerPosition(t, svc)
	testCartographerMap(t, svc)

	// Close out slam service
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	// Don't clear out the directory, since we will re-use the data for the next run
	closeOutSLAMService(t, "")

	// Sleep to ensure cartographer stops
	time.Sleep(time.Millisecond * cartoSleepMs)

	// Delete the last .pcd file in the data directory, so that offline mode runs on the
	// same data as online mode. (Online mode will not read the last .pcd file, since it
	// always processes the second-most-recent .pcd file, in case the most-recent .pcd
	// file is currently being written.)
	files, err := ioutil.ReadDir(name + "/data/")
	test.That(t, err, test.ShouldBeNil)
	lastFileName := files[len(files)-1].Name()
	test.That(t, os.Remove(name+"/data/"+lastFileName), test.ShouldBeNil)
	prevNumFiles -= 1

	// Remove maps so that testing in offline mode will run in mapping mode,
	// as opposed to updating mode.
	test.That(t, resetFolder(name+"/map"), test.ShouldBeNil)

	// Test offline mode using the data generated in the online test
	t.Log("\n=== Testing offline mode ===\n")

	useLiveData = false

	attrCfg = &builtin.AttrConfig{
		Sensors: []string{},
		ConfigParams: map[string]string{
			"mode": reflect.ValueOf(mode).String(),
			"v":    "1",
		},
		MapRateSec:          &mapRate,
		DataDirectory:       name,
		DeleteProcessedData: &deleteProcessedData,
		UseLiveData:         &useLiveData,
	}

	// Create slam service using a real cartographer binary
	svc, err = createSLAMService(t, attrCfg, "cartographer", golog.NewTestLogger(t), true, true)
	test.That(t, err, test.ShouldBeNil)

	// Wait for cartographer to finish processing data
	logReader = svc.(testhelper.Service).GetSLAMProcessBufferedLogReader()
	for {
		line, err := logReader.ReadString('\n')
		test.That(t, err, test.ShouldBeNil)
		if strings.Contains(line, "Passed sensor data to SLAM") {
			prevNumFiles = checkDeleteProcessedData(t, mode, name, prevNumFiles, len(attrCfg.Sensors) != 0, deleteProcessedData)
		}
		if strings.Contains(line, "Finished optimizing final map") {
			break
		}
	}

	testCartographerPosition(t, svc)
	testCartographerMap(t, svc)

	// Sleep to ensure cartographer saves at least one map
	time.Sleep(time.Second * time.Duration(*attrCfg.MapRateSec))

	// Close out slam service
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	// Don't clear out the directory, since we will re-use the maps for the next run
	closeOutSLAMService(t, "")

	// Sleep to ensure cartographer stops
	time.Sleep(time.Millisecond * cartoSleepMs)

	// Remove existing pointclouds, but leave maps and config (so we keep the lua files).
	test.That(t, resetFolder(name+"/data"), test.ShouldBeNil)
	prevNumFiles = 0

	// Count the initial number of maps in the map directory (should equal 1)
	testCartographerDir(t, name, 1)

	// Test online mode using the map generated in the offline test
	t.Log("\n=== Testing online localization mode ===\n")

	mapRate = 0
	deleteProcessedData = true
	useLiveData = true

	attrCfg = &builtin.AttrConfig{
		Sensors: []string{"cartographer_int_lidar"},
		ConfigParams: map[string]string{
			"mode": reflect.ValueOf(mode).String(),
			"v":    "1",
		},
		MapRateSec:          &mapRate,
		DataDirectory:       name,
		DeleteProcessedData: &deleteProcessedData,
		UseLiveData:         &useLiveData,
	}

	// Release point cloud for service validation
	cartographerIntLidarReleasePointCloudChan <- 1
	// Create slam service using a real cartographer binary
	svc, err = createSLAMService(t, attrCfg, "cartographer", golog.NewTestLogger(t), true, true)
	test.That(t, err, test.ShouldBeNil)

	// Make sure we initialize from a saved map
	logReader = svc.(testhelper.Service).GetSLAMProcessBufferedLogReader()
	for {
		line, err := logReader.ReadString('\n')
		test.That(t, err, test.ShouldBeNil)
		if strings.Contains(line, "Running in localization only mode") {
			break
		}
		test.That(t, strings.Contains(line, "Running in updating mode"), test.ShouldBeFalse)
		test.That(t, strings.Contains(line, "Running in mapping mode"), test.ShouldBeFalse)
	}

	// Release point cloud, since cartographer looks for the second most recent point cloud
	cartographerIntLidarReleasePointCloudChan <- 1
	for i := 0; i < numCartographerPointClouds-2; i++ {
		t.Logf("Find log line for point cloud %v", i)
		cartographerIntLidarReleasePointCloudChan <- 1
		for {
			line, err := logReader.ReadString('\n')
			test.That(t, err, test.ShouldBeNil)
			if strings.Contains(line, "Passed sensor data to SLAM") {
				prevNumFiles = checkDeleteProcessedData(t, mode, name, prevNumFiles, len(attrCfg.Sensors) != 0, deleteProcessedData)
				break
			}
		}
	}

	testCartographerPosition(t, svc)
	testCartographerMap(t, svc)

	// Remove maps so that testing is done on the map generated by the internal map
	test.That(t, resetFolder(name+"/map"), test.ShouldBeNil)

	testCartographerInternalState(t, svc, name)

	// Close out slam service
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	// Test that only the map present is the one generated by the GetInternalState call
	testCartographerDir(t, name, 1)

	// Don't clear out the directory, since we will re-use the maps for the next run
	closeOutSLAMService(t, "")

	// Sleep to ensure cartographer stops
	time.Sleep(time.Millisecond * cartoSleepMs)

	// Remove existing pointclouds, but leave maps and config (so we keep the lua files).
	test.That(t, resetFolder(name+"/data"), test.ShouldBeNil)
	prevNumFiles = 0

	// Test online mode using the map generated in the offline test
	t.Log("\n=== Testing online mode with saved map ===\n")

	mapRate = 1

	attrCfg = &builtin.AttrConfig{
		Sensors: []string{"cartographer_int_lidar"},
		ConfigParams: map[string]string{
			"mode": reflect.ValueOf(mode).String(),
			"v":    "1",
		},
		MapRateSec:    &mapRate,
		DataDirectory: name,
		UseLiveData:   &useLiveData,
	}

	// Release point cloud for service validation
	cartographerIntLidarReleasePointCloudChan <- 1
	// Create slam service using a real cartographer binary
	svc, err = createSLAMService(t, attrCfg, "cartographer", golog.NewTestLogger(t), true, true)
	test.That(t, err, test.ShouldBeNil)

	// Make sure we initialize from a saved map
	logReader = svc.(testhelper.Service).GetSLAMProcessBufferedLogReader()
	for {
		line, err := logReader.ReadString('\n')
		test.That(t, err, test.ShouldBeNil)
		if strings.Contains(line, "Running in updating mode") {
			break
		}
		test.That(t, strings.Contains(line, "Running in localization only mode"), test.ShouldBeFalse)
		test.That(t, strings.Contains(line, "Running in mapping mode"), test.ShouldBeFalse)
	}

	// Release point cloud, since cartographer looks for the second most recent point cloud
	cartographerIntLidarReleasePointCloudChan <- 1
	for i := 0; i < numCartographerPointClouds-2; i++ {
		t.Logf("Find log line for point cloud %v", i)
		cartographerIntLidarReleasePointCloudChan <- 1
		for {
			line, err := logReader.ReadString('\n')
			test.That(t, err, test.ShouldBeNil)
			if strings.Contains(line, "Passed sensor data to SLAM") {
				prevNumFiles = checkDeleteProcessedData(t, mode, name, prevNumFiles, len(attrCfg.Sensors) != 0, deleteProcessedData)
				break
			}
			test.That(t, strings.Contains(line, "Failed to open proto stream"), test.ShouldBeFalse)
			test.That(t, strings.Contains(line, "Failed to read SerializationHeader"), test.ShouldBeFalse)
		}
	}

	testCartographerPosition(t, svc)
	testCartographerMap(t, svc)

	// Close out slam service
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	// Test that a new map was generated
	testCartographerDir(t, name, 2)

	// Clear out directory
	closeOutSLAMService(t, name)
}

// Checks the current slam directory to see if the number of files matches the expected amount
func testCartographerDir(t *testing.T, path string, expectedMaps int) {
	mapsInDir, err := ioutil.ReadDir(path + "/map/")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(mapsInDir), test.ShouldBeGreaterThanOrEqualTo, expectedMaps)
}

func TestCartographerIntegration2D(t *testing.T) {
	integrationtestHelperCartographer(t, slam.Dim2d)
}
