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

const (
	cartoSleepMS = 100
)

// Creates the lua files required by the cartographer binary.
func createLuaFiles(name string) error {
	if err := os.Mkdir(name+"/config/lua_files", os.ModePerm); err != nil {
		return err
	}
	for _, luaFile := range []string{"locating_in_map.lua", "mapping_new_map.lua",
		"updating_a_map.lua", "map_builder.lua", "map_builder_server.lua",
		"pose_graph.lua", "trajectory_builder_2d.lua", "trajectory_builder_3d.lua",
		"trajectory_builder.lua"} {

		source, err := os.Open(artifact.MustPath("slam/" + luaFile))
		if err != nil {
			return err
		}
		defer source.Close()
		destination, err := os.Create(name + "/config/lua_files/" + luaFile)
		if err != nil {
			return err
		}
		defer destination.Close()
		_, err = io.Copy(destination, source)
		if err != nil {
			return err
		}
	}
	return nil
}

// Checks the cartographer position and map.
func testCartographerPositionAndMap(t *testing.T, svc slam.Service) {
	t.Helper()

	actualMIME, _, pointcloud, err := svc.GetMap(context.Background(), "test", "pointcloud/pcd", nil, false, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMIME, test.ShouldResemble, "pointcloud/pcd")
	test.That(t, pointcloud.Size(), test.ShouldBeGreaterThanOrEqualTo, 100)

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
}

func TestCartographerIntegration(t *testing.T) {
	_, err := exec.LookPath("carto_grpc_server")
	if err != nil {
		t.Log("Skipping test because carto_grpc_server binary was not found")
		t.Skip()
	}

	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)
	createLuaFiles(name)

	t.Log("Testing online mode")

	mapRate := 1

	attrCfg := &builtin.AttrConfig{
		Algorithm: "cartographer",
		Sensors:   []string{"cartographer_int_lidar"},
		ConfigParams: map[string]string{
			"mode": "2d",
			"v":    "1",
		},
		MapRateSec:    &mapRate,
		DataDirectory: name,
	}

	// Release point cloud for service validation
	cartographerIntLidarReleasePointCloudChan <- 1
	// Create slam service using a real cartographer binary
	svc, err := createSLAMService(t, attrCfg, golog.NewTestLogger(t), true, true)
	test.That(t, err, test.ShouldBeNil)

	// Release point cloud, since cartographer looks for the second most recent point cloud
	cartographerIntLidarReleasePointCloudChan <- 1
	// Wait for cartographer to finish processing data
	logReader := svc.(internal.Service).GetSLAMProcessBufferedLogReader()
	for i := 0; i < numCartographerPointClouds-2; i++ {
		t.Logf("Find log line for point cloud %v", i)
		cartographerIntLidarReleasePointCloudChan <- 1
		for {
			line, err := logReader.ReadString('\n')
			test.That(t, err, test.ShouldBeNil)
			if strings.Contains(line, "Passed sensor data to SLAM") {
				break
			}
		}
	}

	testCartographerPositionAndMap(t, svc)

	// Close out slam service
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	// Don't clear out the directory, since we will re-use the data for the next run
	closeOutSLAMService(t, "")

	// added sleep to ensure cartographer stops
	time.Sleep(time.Millisecond * cartoSleepMS)

	// Delete the last .pcd file in the data directory, so that offline mode runs on the
	// same data as online mode. (Online mode will not read the last .pcd file, since it
	// always processes the second-most-recent .pcd file, in case the most-recent .pcd
	// file is currently being written.)
	files, err := ioutil.ReadDir(name + "/data/")
	test.That(t, err, test.ShouldBeNil)
	lastFileName := files[len(files)-1].Name()
	test.That(t, os.Remove(name+"/data/"+lastFileName), test.ShouldBeNil)

	// Remove maps so that testing in offline mode will run in mapping mode,
	// as opposed to updating mode.
	test.That(t, resetFolder(name+"/map"), test.ShouldBeNil)

	// Test offline mode using the data generated in the online test
	t.Log("Testing offline mode")

	attrCfg = &builtin.AttrConfig{
		Algorithm: "cartographer",
		Sensors:   []string{},
		ConfigParams: map[string]string{
			"mode": "2d",
			"v":    "1",
		},
		MapRateSec:    &mapRate,
		DataDirectory: name,
	}

	// Create slam service using a real cartographer binary
	svc, err = createSLAMService(t, attrCfg, golog.NewTestLogger(t), true, true)
	test.That(t, err, test.ShouldBeNil)

	// Wait for cartographer to finish processing data
	logReader = svc.(internal.Service).GetSLAMProcessBufferedLogReader()
	for {
		line, err := logReader.ReadString('\n')
		test.That(t, err, test.ShouldBeNil)
		if strings.Contains(line, "Finished optimizing final map") {
			break
		}
	}

	testCartographerPositionAndMap(t, svc)

	// Close out slam service
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	// Don't clear out the directory, since we will re-use the maps for the next run
	closeOutSLAMService(t, "")

	// added sleep to ensure cartographer stops
	time.Sleep(time.Millisecond * cartoSleepMS)

	// Remove existing pointclouds, but leave maps and config (so we keep the lua files).
	test.That(t, resetFolder(name+"/data"), test.ShouldBeNil)
	// Count the initial number of maps in the map directory
	numMaps, err := ioutil.ReadDir(name + "/map/")
	test.That(t, err, test.ShouldBeNil)
	// Test online mode using the map generated in the offline test
	t.Log("Testing online mode in localization mode")

	mapRate = 0

	attrCfg = &builtin.AttrConfig{
		Algorithm: "cartographer",
		Sensors:   []string{"cartographer_int_lidar"},
		ConfigParams: map[string]string{
			"mode": "2d",
			"v":    "1",
		},
		MapRateSec:    &mapRate,
		DataDirectory: name,
	}

	// Release point cloud for service validation
	cartographerIntLidarReleasePointCloudChan <- 1
	// Create slam service using a real orbslam binary
	svc, err = createSLAMService(t, attrCfg, golog.NewTestLogger(t), true, true)
	test.That(t, err, test.ShouldBeNil)

	// Make sure we initialize from a saved map
	logReader = svc.(internal.Service).GetSLAMProcessBufferedLogReader()
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
	// Check if orbslam hangs and needs to be shut down
	// Wait for orbslam to finish processing images
	for i := 0; i < numCartographerPointClouds-2; i++ {
		t.Logf("Find log line for point cloud %v", i)
		cartographerIntLidarReleasePointCloudChan <- 1
		for {
			line, err := logReader.ReadString('\n')
			test.That(t, err, test.ShouldBeNil)
			if strings.Contains(line, "Passed sensor data to SLAM") {
				break
			}
		}
	}

	testCartographerPositionAndMap(t, svc)

	// Close out slam service
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	// Test that no new maps were generated
	numMapsLocalize, err := ioutil.ReadDir(name + "/map/")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(numMapsLocalize), test.ShouldEqual, len(numMaps))

	// Don't clear out the directory, since we will re-use the maps for the next run
	closeOutSLAMService(t, "")

	// added sleep to ensure cartographer stops
	time.Sleep(time.Millisecond * cartoSleepMS)

	// Remove existing pointclouds, but leave maps and config (so we keep the lua files).
	// Orbslam will use the most recent config.
	test.That(t, resetFolder(name+"/data"), test.ShouldBeNil)

	// Test online mode using the map generated in the offline test
	t.Log("Testing online mode with saved map")

	mapRate = 9999

	attrCfg = &builtin.AttrConfig{
		Algorithm: "cartographer",
		Sensors:   []string{"cartographer_int_lidar"},
		ConfigParams: map[string]string{
			"mode": "2d",
			"v":    "1",
		},
		MapRateSec:    &mapRate,
		DataDirectory: name,
	}

	// Release point cloud for service validation
	cartographerIntLidarReleasePointCloudChan <- 1
	// Create slam service using a real orbslam binary
	svc, err = createSLAMService(t, attrCfg, golog.NewTestLogger(t), true, true)
	test.That(t, err, test.ShouldBeNil)

	// Make sure we initialize from a saved map
	logReader = svc.(internal.Service).GetSLAMProcessBufferedLogReader()
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
	// Check if orbslam hangs and needs to be shut down
	// Wait for orbslam to finish processing images
	for i := 0; i < numCartographerPointClouds-2; i++ {
		t.Logf("Find log line for point cloud %v", i)
		cartographerIntLidarReleasePointCloudChan <- 1
		for {
			line, err := logReader.ReadString('\n')
			test.That(t, err, test.ShouldBeNil)
			if strings.Contains(line, "Passed sensor data to SLAM") {
				break
			}
		}
	}

	testCartographerPositionAndMap(t, svc)

	// Close out slam service
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	// Clear out directory
	closeOutSLAMService(t, name)
}
