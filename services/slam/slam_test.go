package slam_test

import (
	"context"
	"fmt"
	"image"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"google.golang.org/grpc"

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

func setupTestGRPCServer(port string) *grpc.Server {

	listener2, _ := net.Listen("tcp", "localhost:"+port)
	gServer2 := grpc.NewServer()
	go gServer2.Serve(listener2)

	return gServer2
}

var cam = &inject.Camera{}

func setupInjectRobot() *inject.Robot {
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case camera.Named("good_lidar"):
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return pointcloud.New(), nil
			}
			return cam, nil
		case camera.Named("bad_lidar"):
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("bad_lidar")
			}
			return cam, nil
		case camera.Named("good_camera"):
			cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
				return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, nil
			}
			return cam, nil
		case camera.Named("good_depth_camera"):
			cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
				img, err := rimage.NewImageWithDepth(artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"), true)
				return img, nil, err
			}
			return cam, nil
		case camera.Named("bad_camera"):
			cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
				return nil, nil, errors.New("bad_camera")
			}
			return cam, nil
		default:
			return nil, rdkutils.NewResourceNotFoundError(name)
		}
	}
	return r
}

func createSLAMService(t *testing.T, attrCfg *slam.AttrConfig, logger golog.Logger) (slam.Service, error) {
	t.Helper()

	ctx := context.Background()
	cfgService := config.Service{Name: "test", Type: "slam"}
	cfgService.ConvertedAttributes = attrCfg

	r := setupInjectRobot()

	svc, _ := slam.New(ctx, r, cfgService, logger)

	if svc == nil {
		return nil, errors.New("error creating slam service")
	}

	return svc, nil
}

func closeOutSLAMService(t *testing.T, name string) {
	t.Helper()

	if name != "" {
		err := resetFolder(name)
		test.That(t, err, test.ShouldBeNil)
	}

	for k := range slam.SLAMLibraries {
		if strings.Contains(k, "fake") {
			delete(slam.SLAMLibraries, k)
		}
	}
}

func TestGeneralNew(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	t.Run("New slam service blank config", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, &slam.AttrConfig{}, logger)
		test.That(t, svc, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("error creating slam service"))
	})

	t.Run("New slam service with no camera", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_cartographer",
			Sensors:       []string{},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			Port:          "4444",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		grpcServer := setupTestGRPCServer(attrCfg.Port)
		svc, err := createSLAMService(t, attrCfg, logger)
		test.That(t, svc, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeNil)

		grpcServer.Stop()
		if svc != nil {
			svc.Close()
		}

	})

	t.Run("New slam service with no camera", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_cartographer",
			Sensors:       []string{"gibberish"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    100,
			Port:          "4444",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger)
		test.That(t, svc, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("error creating slam service"))

		if svc != nil {
			svc.Close()
		}
	})

	t.Run("New slam service with invalid slam algo type", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "test",
			Sensors:       []string{"good_lidar"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    100,
			Port:          "4444",
		}

		slam.SLAMLibraries["test"] = slam.LibraryMetadata{
			AlgoName:       "test",
			AlgoType:       99,
			SlamMode:       slam.SLAMLibraries["cartographer"].SlamMode,
			BinaryLocation: "",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger)
		test.That(t, svc, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("error creating slam service"))

		if svc != nil {
			svc.Close()
		}

		delete(slam.SLAMLibraries, "test")
	})

	t.Run("New slam service with invalid slam algo type", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "orbslamv3",
			Sensors:       []string{"good_camera"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    100,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger)
		test.That(t, svc, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("error creating slam service"))

		if svc != nil {
			svc.Close()
		}

		delete(slam.SLAMLibraries, "test")
	})

	closeOutSLAMService(t, name)
}

func TestCartographerNew(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	t.Run("New cartographer service with good lidar", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_cartographer",
			Sensors:       []string{"good_lidar"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    100,
			Port:          "4444",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		grpcServer := setupTestGRPCServer(attrCfg.Port)
		svc, err := createSLAMService(t, attrCfg, logger)
		test.That(t, svc, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeNil)

		if svc != nil {
			svc.Close()
		}

		grpcServer.Stop()
	})

	t.Run("New cartographer service with bad lidar", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_cartographer",
			Sensors:       []string{"bad_lidar"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    100,
			Port:          "4444",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger)
		test.That(t, svc, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("error creating slam service"))

		if svc != nil {
			svc.Close()
		}
	})

	t.Run("New cartographer service with camera", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_cartographer",
			Sensors:       []string{"good_camera"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    100,
			Port:          "4444",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger)
		test.That(t, svc, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("error creating slam service"))

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

	t.Run("New orbslamv3 service with good camera", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_orbslamv3",
			Sensors:       []string{"good_depth_camera"},
			ConfigParams:  map[string]string{"mode": "rgbd"},
			DataDirectory: name,
			DataRateMs:    100,
			Port:          "4444",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		grpcServer := setupTestGRPCServer(attrCfg.Port)
		svc, err := createSLAMService(t, attrCfg, logger)
		test.That(t, svc, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeNil)

		if svc != nil {
			svc.Close()
		}
		grpcServer.Stop()
	})

	t.Run("New orbslamv3 service with good camera mono", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_orbslamv3",
			Sensors:       []string{"good_camera"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    100,
			Port:          "4444",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		grpcServer := setupTestGRPCServer(attrCfg.Port)
		svc, err := createSLAMService(t, attrCfg, logger)
		test.That(t, svc, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeNil)

		if svc != nil {
			svc.Close()
		}
		grpcServer.Stop()
	})

	t.Run("New orbslamv3 service with bad camera", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_orbslamv3",
			Sensors:       []string{"bad_camera"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    100,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger)
		test.That(t, svc, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("error creating slam service"))

		if svc != nil {
			svc.Close()
		}
	})

	t.Run("New orbslamv3 service with lidar", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_orbslamv3",
			Sensors:       []string{"good_lidar"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    100,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		svc, err := createSLAMService(t, attrCfg, logger)
		test.That(t, svc, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("error creating slam service"))

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
		Port:          "4444",
	}

	// Create slam service
	logger, obs := golog.NewObservedTestLogger(t)
	grpcServer := setupTestGRPCServer(attrCfg.Port)
	svc, err := createSLAMService(t, attrCfg, logger)
	test.That(t, svc, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	if svc != nil {
		svc.Close()
	}

	slamSvc := svc.(internal.Service)

	t.Run("Cartographer Data Process with good lidar", func(t *testing.T) {
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

	t.Run("Cartographer Data Process with bad lidar", func(t *testing.T) {
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

	grpcServer.Stop()
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
		Port:          "4444",
	}

	// Create slam service
	logger, obs := golog.NewObservedTestLogger(t)
	grpcServer := setupTestGRPCServer(attrCfg.Port)
	svc, err := createSLAMService(t, attrCfg, logger)
	test.That(t, svc, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	if svc != nil {
		svc.Close()
	}

	slamSvc := svc.(internal.Service)

	t.Run("ORBSLAM3 Data Process with good camera", func(t *testing.T) {
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

	t.Run("ORBSLAM3 Data Process with bad camera", func(t *testing.T) {
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

	grpcServer.Stop()
	closeOutSLAMService(t, name)
}

func TestSLAMProcess(t *testing.T) {
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
		Port:             "4444",
	}

	// Create slam service
	logger := golog.NewTestLogger(t)
	grpcServer := setupTestGRPCServer(attrCfg.Port)
	svc, err := createSLAMService(t, attrCfg, logger)
	test.That(t, svc, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	slamSvc := svc.(internal.Service)

	t.Run("Run good SLAM process with argument checks", func(t *testing.T) {
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
	})

	t.Run("Run bad SLAM process with error check", func(t *testing.T) {
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
	})

	err = slamSvc.StopSLAMProcess()
	test.That(t, err, test.ShouldBeNil)

	if svc != nil {
		svc.Close()
	}
	grpcServer.Stop()
	closeOutSLAMService(t, name)
}

// nolint:unparam
func createTempFolderArchitecture(validArch bool) (string, error) {
	name, err := ioutil.TempDir("/tmp", "*")
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
