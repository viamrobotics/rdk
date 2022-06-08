// Package slam_test tests the functions that required injected components (such as robot and camera)
// in order to be run. It utilizes the internal package located in slam_test_helper.go to access
// certain exported functions which we do not want to make available to the user.
package slam_test

import (
	"context"
	"fmt"
	"image"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/internal"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	timePadding = 5
)

var cam = &inject.Camera{}

func createFakeSLAMLibraries() {
	for _, s := range slam.SLAMLibraries {
		slam.SLAMLibraries["fake_"+s.AlgoName] = slam.LibraryMetadata{
			AlgoName:       "fake_" + s.AlgoName,
			AlgoType:       s.AlgoType,
			SlamMode:       s.SlamMode,
			BinaryLocation: "true",
		}
	}
}

func deleteFakeSLAMLibraries() {
	for k := range slam.SLAMLibraries {
		if strings.Contains(k, "fake") {
			delete(slam.SLAMLibraries, k)
		}
	}
}

func closeOutSLAMService(t *testing.T, name string) {
	t.Helper()

	if name != "" {
		err := resetFolder(name)
		test.That(t, err, test.ShouldBeNil)
	}

	deleteFakeSLAMLibraries()
}

func setupInjectRobot() *inject.Robot {
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case camera.Named("good_lidar"):
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return pointcloud.New(), nil
			}
			cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
				return nil, nil, errors.New("lidar not camera")
			}
			return cam, nil
		case camera.Named("bad_lidar"):
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("bad_lidar")
			}
			cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
				return nil, nil, errors.New("lidar not camera")
			}
			return cam, nil
		case camera.Named("good_camera"):
			cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
				return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, nil
			}
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("camera not lidar")
			}
			return cam, nil
		case camera.Named("good_depth_camera"):
			cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
				img, err := rimage.NewImageWithDepth(artifact.MustPath("rimage/board1.png"),
					artifact.MustPath("rimage/board1.dat.gz"), true)
				return img, nil, err
			}
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("camera not lidar")
			}
			return cam, nil
		case camera.Named("bad_camera"):
			cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
				return nil, nil, errors.New("bad_camera")
			}
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("camera not lidar")
			}
			return cam, nil
		default:
			return nil, rdkutils.NewResourceNotFoundError(name)
		}
	}
	return r
}

func createSLAMService(t *testing.T, attrCfg *slam.AttrConfig, logger golog.Logger, success bool) (slam.Service, error) {
	t.Helper()

	ctx := context.Background()
	cfgService := config.Service{Name: "test", Type: "slam"}
	cfgService.ConvertedAttributes = attrCfg

	r := setupInjectRobot()

	svc, err := slam.New(ctx, r, cfgService, logger)

	if success {
		test.That(t, svc, test.ShouldNotBeNil)
	} else {
		test.That(t, svc, test.ShouldBeNil)
	}

	return svc, err
}

func TestGeneralNew(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	t.Run("New slam service blank config", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		attrCfg := &slam.AttrConfig{}
		_, err := createSLAMService(t, attrCfg, logger, false)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("runtime slam config error: "+
				"%v algorithm specified not in implemented list", attrCfg.Algorithm))
	})

	t.Run("New slam service with no camera", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_cartographer",
			Sensors:       []string{},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger, true)
		test.That(t, err, test.ShouldBeNil)

		if svc != nil {
			svc.Close()
		}
	})

	t.Run("New slam service with bad camera", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_cartographer",
			Sensors:       []string{"gibberish"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    100,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger, false)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("configuring camera error: error getting camera for slam service: "+
				"\"resource \\\"rdk:component:camera/%v\\\" not found\"", attrCfg.Sensors[0]))

		if svc != nil {
			svc.Close()
		}
	})

	t.Run("New slam service with invalid slam algo type", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "test",
			Sensors:       []string{},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    100,
		}

		slam.SLAMLibraries["test"] = slam.LibraryMetadata{
			AlgoName:       "test",
			AlgoType:       99,
			SlamMode:       slam.SLAMLibraries["cartographer"].SlamMode,
			BinaryLocation: "",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger, false)
		test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, errors.New("error with slam service slam process:"))

		if svc != nil {
			svc.Close()
		}

		delete(slam.SLAMLibraries, "test")
	})

	t.Run("New slam service the fails at slam process due to binary location", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "orbslamv3",
			Sensors:       []string{"good_camera"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    100,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger, false)
		test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, errors.New("error with slam service slam process:"))

		if svc != nil {
			svc.Close()
		}
	})

	closeOutSLAMService(t, name)
}

func TestCartographerNew(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	t.Run("New cartographer service with good lidar in slam mode 2d", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_cartographer",
			Sensors:       []string{"good_lidar"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    100,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger, true)
		test.That(t, err, test.ShouldBeNil)

		if svc != nil {
			svc.Close()
		}
	})

	t.Run("New cartographer service with lidar that errors during call to NextPointCloud", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_cartographer",
			Sensors:       []string{"bad_lidar"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    100,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger, false)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("runtime slam service error: error getting data in desired mode: %v", attrCfg.Sensors[0]))

		if svc != nil {
			svc.Close()
		}
	})

	t.Run("New cartographer service with camera without NextPointCloud implementation", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_cartographer",
			Sensors:       []string{"good_camera"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    100,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger, false)

		test.That(t, err, test.ShouldBeError,
			errors.New("runtime slam service error: error getting data in desired mode: camera not lidar"))

		if svc != nil {
			svc.Close()
		}
	})
	closeOutSLAMService(t, name)
}

func TestORBSLAMNew(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	t.Run("New orbslamv3 service with good camera in slam mode rgbd", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_orbslamv3",
			Sensors:       []string{"good_depth_camera"},
			ConfigParams:  map[string]string{"mode": "rgbd"},
			DataDirectory: name,
			DataRateMs:    100,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger, true)
		test.That(t, err, test.ShouldBeNil)

		if svc != nil {
			svc.Close()
		}
	})

	t.Run("New orbslamv3 service with good camera in slam mode mono", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_orbslamv3",
			Sensors:       []string{"good_camera"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    100,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger, true)
		test.That(t, err, test.ShouldBeNil)

		if svc != nil {
			svc.Close()
		}
	})

	t.Run("New orbslamv3 service with camera that errors during call to Next", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_orbslamv3",
			Sensors:       []string{"bad_camera"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    100,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger, false)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("runtime slam service error: "+
				"error getting data in desired mode: %v", attrCfg.Sensors[0]))

		if svc != nil {
			svc.Close()
		}
	})

	t.Run("New orbslamv3 service with lidar without Next implementation", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_orbslamv3",
			Sensors:       []string{"good_lidar"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    100,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger, false)
		test.That(t, err, test.ShouldBeError,
			errors.New("runtime slam service error: error getting data in desired mode: lidar not camera"))

		if svc != nil {
			svc.Close()
		}
	})
	closeOutSLAMService(t, name)
}

func TestCartographerDataProcess(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	dataRateMs := 100
	attrCfg := &slam.AttrConfig{
		Algorithm:     "fake_cartographer",
		Sensors:       []string{"good_lidar"},
		ConfigParams:  map[string]string{"mode": "2d"},
		DataDirectory: name,
		DataRateMs:    dataRateMs,
	}

	// Create slam service
	logger, obs := golog.NewObservedTestLogger(t)
	svc, err := createSLAMService(t, attrCfg, logger, true)
	test.That(t, err, test.ShouldBeNil)

	if svc != nil {
		svc.Close()
	}

	slamSvc := svc.(internal.Service)

	t.Run("Cartographer Data Process with lidar in slam mode 2d", func(t *testing.T) {
		goodCam := &inject.Camera{}
		goodCam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
			return pointcloud.New(), nil
		}

		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		slamSvc.StartDataProcess(cancelCtx, goodCam)

		n := 5
		// Note: timePadding is required to allow the sub processes to be fully completed during test
		time.Sleep(time.Millisecond * time.Duration((n)*(dataRateMs+timePadding)))
		cancelFunc()

		files, err := ioutil.ReadDir(name + "/data/")
		test.That(t, len(files), test.ShouldEqual, n)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("Cartographer Data Process with lidar that errors during call to NextPointCloud", func(t *testing.T) {
		badCam := &inject.Camera{}
		badCam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
			return nil, errors.New("bad_lidar")
		}

		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		slamSvc.StartDataProcess(cancelCtx, badCam)

		time.Sleep(time.Millisecond * time.Duration(dataRateMs*2))
		cancelFunc()

		latestLoggedEntry := obs.All()[len(obs.All())-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "bad_lidar")
	})

	closeOutSLAMService(t, name)
}

func TestORBSLAMDataProcess(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	dataRateMs := 100
	attrCfg := &slam.AttrConfig{
		Algorithm:     "fake_orbslamv3",
		Sensors:       []string{"good_camera"},
		ConfigParams:  map[string]string{"mode": "mono"},
		DataDirectory: name,
		DataRateMs:    dataRateMs,
	}

	// Create slam service
	logger, obs := golog.NewObservedTestLogger(t)
	svc, err := createSLAMService(t, attrCfg, logger, true)
	test.That(t, err, test.ShouldBeNil)

	if svc != nil {
		svc.Close()
	}

	slamSvc := svc.(internal.Service)

	t.Run("ORBSLAM3 Data Process with camera in slam mode mono", func(t *testing.T) {
		goodCam := &inject.Camera{}
		goodCam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
			return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, nil
		}

		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		slamSvc.StartDataProcess(cancelCtx, goodCam)

		n := 5
		// Note: timePadding is required to allow the sub processes to be fully completed during test
		time.Sleep(time.Millisecond * time.Duration((n)*(dataRateMs+timePadding)))
		cancelFunc()

		files, err := ioutil.ReadDir(name + "/data/")
		test.That(t, len(files), test.ShouldEqual, n)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ORBSLAM3 Data Process with camera that errors during call to Next", func(t *testing.T) {
		badCam := &inject.Camera{}
		badCam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
			return nil, nil, errors.New("bad_camera")
		}

		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		slamSvc.StartDataProcess(cancelCtx, badCam)

		time.Sleep(time.Millisecond * time.Duration(dataRateMs*2))
		cancelFunc()

		latestLoggedEntry := obs.All()[len(obs.All())-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "bad_camera")
	})

	if slamSvc != nil {
		slamSvc.Close()
	}

	closeOutSLAMService(t, name)
}

func TestSLAMProcessSuccess(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	attrCfg := &slam.AttrConfig{
		Algorithm:        "fake_orbslamv3",
		Sensors:          []string{"good_camera"},
		ConfigParams:     map[string]string{"mode": "mono", "test_param": "viam"},
		DataDirectory:    name,
		MapRateSec:       200,
		DataRateMs:       100,
		InputFilePattern: "10:200:1",
	}

	// Create slam service
	logger := golog.NewTestLogger(t)
	svc, err := createSLAMService(t, attrCfg, logger, true)
	test.That(t, err, test.ShouldBeNil)

	slamSvc := svc.(internal.Service)

	t.Run("Run valid SLAM process with argument checks", func(t *testing.T) {
		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		cmd, err := slamSvc.StartSLAMProcess(cancelCtx)

		test.That(t, err, test.ShouldBeNil)

		cmdResult := [][]string{
			{slam.SLAMLibraries["fake_orbslamv3"].BinaryLocation},
			{"-sensors=good_camera"},
			{"-config_param={mode=mono,test_param=viam}", "-config_param={test_param=viam,mode=mono}"},
			{"-data_rate_ms=100"},
			{"-map_rate_sec=200"},
			{"-data_dir=" + name},
			{"-input_file_pattern=10:200:1"},
		}

		for i, s := range cmd {
			test.That(t, s, test.ShouldBeIn, cmdResult[i])
		}

		cancelFunc()

		err = slamSvc.StopSLAMProcess()
		test.That(t, err, test.ShouldBeNil)
	})

	if svc != nil {
		svc.Close()
		slamSvc.Close()
	}

	closeOutSLAMService(t, name)
}

func TestSLAMProcessFail(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	attrCfg := &slam.AttrConfig{
		Algorithm:        "fake_orbslamv3",
		Sensors:          []string{"good_camera"},
		ConfigParams:     map[string]string{"mode": "mono", "test_param": "viam"},
		DataDirectory:    name,
		MapRateSec:       200,
		DataRateMs:       100,
		InputFilePattern: "10:200:1",
	}

	// Create slam service
	logger := golog.NewTestLogger(t)
	svc, err := createSLAMService(t, attrCfg, logger, true)
	test.That(t, err, test.ShouldBeNil)

	slamSvc := svc.(internal.Service)

	t.Run("Run SLAM process that errors out due to invalid binary location", func(t *testing.T) {
		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		_, err := slamSvc.StartSLAMProcess(cancelCtx)

		test.That(t, err, test.ShouldBeNil)

		delete(slam.SLAMLibraries, "fake_orbslamv3")

		slam.SLAMLibraries["fake_orbslamv3"] = slam.LibraryMetadata{
			AlgoName:       "fake_" + slam.SLAMLibraries["orbslamv3"].AlgoName,
			AlgoType:       slam.SLAMLibraries["orbslamv3"].AlgoType,
			SlamMode:       slam.SLAMLibraries["orbslamv3"].SlamMode,
			BinaryLocation: "fail",
		}

		errCheck := fmt.Sprintf("\"%v\": executable file not found in $PATH", "fail")
		cmd, err := slamSvc.StartSLAMProcess(cancelCtx)
		test.That(t, cmd, test.ShouldResemble, []string{})
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("problem adding slam process: error running process \"%v\": exec: %v", "fail", errCheck))

		cancelFunc()

		err = slamSvc.StopSLAMProcess()
		test.That(t, err, test.ShouldBeNil)
	})

	if svc != nil {
		slamSvc.Close()
		svc.Close()
	}

	closeOutSLAMService(t, name)
}

// nolint:unparam
func createTempFolderArchitecture(validArch bool) (string, error) {
	name, err := ioutil.TempDir("", "*")
	if err != nil {
		return "nil", err
	}

	if validArch {
		if err := os.Mkdir(name+"/map", os.ModePerm); err != nil {
			return "", err
		}
		if err := os.Mkdir(name+"/data", os.ModePerm); err != nil {
			return "", err
		}
		if err := os.Mkdir(name+"/config", os.ModePerm); err != nil {
			return "", err
		}
	}
	return name, nil
}

func resetFolder(path string) error {
	err := os.RemoveAll(path)
	return err
}
