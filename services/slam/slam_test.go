// Package slam_test tests the functions that required injected components (such as robot and camera)
// in order to be run. It utilizes the internal package located in slam_test_helper.go to access
// certain exported functions which we do not want to make available to the user.
package slam_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"math"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/internal"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	timePadding     = 5
	validDataRateMS = 200
)

const (
	testSlamServiceName  = "slam1"
	testSlamServiceName2 = "slam2"
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

func setupTestGRPCServer(port string) *grpc.Server {
	listener2, _ := net.Listen("tcp", port)
	gServer2 := grpc.NewServer()
	go gServer2.Serve(listener2)

	return gServer2
}

func setupInjectRobot() *inject.Robot {
	r := &inject.Robot{}
	var projA rimage.Projector
	intrinsicsA := &transform.PinholeCameraIntrinsics{ // not the real camera parameters -- fake for test
		Width:  1280,
		Height: 720,
		Fx:     200,
		Fy:     200,
		Ppx:    100,
		Ppy:    100,
	}
	projA = intrinsicsA

	var projB rimage.Projector
	intrinsicsB := &transform.PinholeCameraIntrinsics{ // not the real camera parameters -- fake for test
		Width:  0,
		Height: 0,
		Fx:     0,
		Fy:     0,
		Ppx:    0,
		Ppy:    0,
	}
	projB = intrinsicsB

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		cam := &inject.Camera{}
		switch name {
		case camera.Named("good_lidar"):
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return pointcloud.New(), nil
			}
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				return nil, errors.New("lidar not camera")
			}
			cam.ProjectorFunc = func(ctx context.Context) (rimage.Projector, error) {
				return nil, transform.NewNoIntrinsicsError("")
			}
			return cam, nil
		case camera.Named("bad_lidar"):
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("bad_lidar")
			}
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				return nil, errors.New("lidar not camera")
			}
			cam.ProjectorFunc = func(ctx context.Context) (rimage.Projector, error) {
				return nil, transform.NewNoIntrinsicsError("")
			}
			return cam, nil
		case camera.Named("good_camera"):
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				return gostream.NewEmbeddedVideoStreamFromReader(gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
					return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, nil
				})), nil
			}
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("camera not lidar")
			}
			cam.ProjectorFunc = func(ctx context.Context) (rimage.Projector, error) {
				return projA, nil
			}
			return cam, nil
		case camera.Named("good_color_camera"):
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("camera not lidar")
			}
			cam.ProjectorFunc = func(ctx context.Context) (rimage.Projector, error) {
				return projA, nil
			}
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				imgBytes, err := os.ReadFile(artifact.MustPath("rimage/board1.png"))
				if err != nil {
					return nil, err
				}
				img, err := png.Decode(bytes.NewReader(imgBytes))
				if err != nil {
					return nil, err
				}
				lazy := rimage.NewLazyEncodedImage(imgBytes, rdkutils.MimeTypePNG, img.Bounds().Dx(), img.Bounds().Dy())
				return gostream.NewEmbeddedVideoStreamFromReader(gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
					return lazy, func() {}, nil
				})), nil
			}
			return cam, nil
		case camera.Named("good_depth_camera"):
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("camera not lidar")
			}
			cam.ProjectorFunc = func(ctx context.Context) (rimage.Projector, error) {
				return nil, transform.NewNoIntrinsicsError("")
			}
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				imgBytes, err := os.ReadFile(artifact.MustPath("rimage/board1_gray.png"))
				if err != nil {
					return nil, err
				}
				img, err := png.Decode(bytes.NewReader(imgBytes))
				if err != nil {
					return nil, err
				}
				lazy := rimage.NewLazyEncodedImage(imgBytes, rdkutils.MimeTypePNG, img.Bounds().Dx(), img.Bounds().Dy())
				return gostream.NewEmbeddedVideoStreamFromReader(gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
					return lazy, func() {}, nil
				})), nil
			}
			return cam, nil
		case camera.Named("bad_camera"):
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				return nil, errors.New("bad_camera")
			}
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("camera not lidar")
			}
			cam.ProjectorFunc = func(ctx context.Context) (rimage.Projector, error) {
				return nil, transform.NewNoIntrinsicsError("")
			}
			return cam, nil
		case camera.Named("bad_camera_intrinsics"):
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				return gostream.NewEmbeddedVideoStreamFromReader(gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
					return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, nil
				})), nil
			}
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("camera not lidar")
			}
			cam.ProjectorFunc = func(ctx context.Context) (rimage.Projector, error) {
				return projB, nil
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

	slam.SetCameraValidationMaxTimeoutSecForTesting(1)
	slam.SetDialMaxTimeoutSecForTesting(1)

	svc, err := slam.New(ctx, r, cfgService, logger)

	if success {
		if err != nil {
			return nil, err
		}
		test.That(t, svc, test.ShouldNotBeNil)
		return svc, nil
	}

	test.That(t, svc, test.ShouldBeNil)
	return nil, err
}

func TestGeneralNew(t *testing.T) {
	name, err := createTempFolderArchitecture()
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
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		grpcServer := setupTestGRPCServer(attrCfg.Port)
		svc, err := createSLAMService(t, attrCfg, logger, true)
		test.That(t, err, test.ShouldBeNil)

		grpcServer.Stop()
		test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	})

	t.Run("New slam service with bad camera", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_cartographer",
			Sensors:       []string{"gibberish"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, logger, false)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("configuring camera error: error getting camera %v for slam service: "+
				"resource \"rdk:component:camera/%v\" not found", attrCfg.Sensors[0], attrCfg.Sensors[0]))
	})

	t.Run("New slam service with invalid slam algo type", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "test",
			Sensors:       []string{},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
		}

		slam.SLAMLibraries["test"] = slam.LibraryMetadata{
			AlgoName:       "test",
			AlgoType:       99,
			SlamMode:       slam.SLAMLibraries["cartographer"].SlamMode,
			BinaryLocation: "",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, logger, false)
		test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "error with slam service slam process:")

		delete(slam.SLAMLibraries, "test")
	})

	t.Run("New slam service the fails at slam process due to binary location", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "orbslamv3",
			Sensors:       []string{"good_camera"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, logger, false)
		test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "error with slam service slam process:")
	})

	closeOutSLAMService(t, name)
}

func TestCartographerNew(t *testing.T) {
	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	t.Run("New cartographer service with good lidar in slam mode 2d", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_cartographer",
			Sensors:       []string{"good_lidar"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		grpcServer := setupTestGRPCServer(attrCfg.Port)
		svc, err := createSLAMService(t, attrCfg, logger, true)
		test.That(t, err, test.ShouldBeNil)

		grpcServer.Stop()
		test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	})

	t.Run("New cartographer service with lidar that errors during call to NextPointCloud", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_cartographer",
			Sensors:       []string{"bad_lidar"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, logger, false)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("runtime slam service error: error getting data in desired mode: %v", attrCfg.Sensors[0]))
	})

	t.Run("New cartographer service with camera without NextPointCloud implementation", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_cartographer",
			Sensors:       []string{"good_camera"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, logger, false)

		test.That(t, err, test.ShouldBeError,
			errors.New("runtime slam service error: error getting data in desired mode: camera not lidar"))
	})
	closeOutSLAMService(t, name)
}

func TestORBSLAMNew(t *testing.T) {
	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	t.Run("New orbslamv3 service with good camera in slam mode rgbd", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_orbslamv3",
			Sensors:       []string{"good_color_camera", "good_depth_camera"},
			ConfigParams:  map[string]string{"mode": "rgbd"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		grpcServer := setupTestGRPCServer(attrCfg.Port)
		svc, err := createSLAMService(t, attrCfg, logger, true)
		test.That(t, err, test.ShouldBeNil)

		grpcServer.Stop()
		test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	})

	t.Run("New orbslamv3 service in slam mode rgbd that errors due to a single camera", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_orbslamv3",
			Sensors:       []string{"good_color_camera"},
			ConfigParams:  map[string]string{"mode": "rgbd"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, logger, false)
		test.That(t, err.Error(), test.ShouldContainSubstring,
			errors.Errorf("expected 2 cameras for rgbd slam, found %v", len(attrCfg.Sensors)).Error())
	})

	t.Run("New orbslamv3 service in slam mode rgbd that errors due cameras in the wrong order", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_orbslamv3",
			Sensors:       []string{"good_depth_camera", "good_color_camera"},
			ConfigParams:  map[string]string{"mode": "rgbd"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, logger, false)
		test.That(t, err.Error(), test.ShouldContainSubstring,
			errors.New("Unable to get camera features for first camera, make sure the color camera is listed first").Error())
	})

	t.Run("New orbslamv3 service with good camera in slam mode mono", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_orbslamv3",
			Sensors:       []string{"good_camera"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		grpcServer := setupTestGRPCServer(attrCfg.Port)
		svc, err := createSLAMService(t, attrCfg, logger, true)
		test.That(t, err, test.ShouldBeNil)

		grpcServer.Stop()
		test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	})

	t.Run("New orbslamv3 service with camera that errors during call to Next", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_orbslamv3",
			Sensors:       []string{"bad_camera"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, logger, false)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("runtime slam service error: "+
				"error getting data in desired mode: %v", attrCfg.Sensors[0]))
	})

	t.Run("New orbslamv3 service with camera that errors from bad intrinsics", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_orbslamv3",
			Sensors:       []string{"bad_camera_intrinsics"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, logger, false)

		test.That(t, err.Error(), test.ShouldContainSubstring,
			transform.NewNoIntrinsicsError(fmt.Sprintf("Invalid size (%#v, %#v)", 0, 0)).Error())
	})

	t.Run("New orbslamv3 service with lidar without Next implementation", func(t *testing.T) {
		attrCfg := &slam.AttrConfig{
			Algorithm:     "fake_orbslamv3",
			Sensors:       []string{"good_lidar"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, logger, false)
		test.That(t, err, test.ShouldBeError,
			errors.New("runtime slam service error: error getting data in desired mode: lidar not camera"))
	})
	closeOutSLAMService(t, name)
}

func TestCartographerDataProcess(t *testing.T) {
	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	attrCfg := &slam.AttrConfig{
		Algorithm:     "fake_cartographer",
		Sensors:       []string{"good_lidar"},
		ConfigParams:  map[string]string{"mode": "2d"},
		DataDirectory: name,
		DataRateMs:    validDataRateMS,
		Port:          "localhost:4445",
	}

	// Create slam service
	logger, obs := golog.NewObservedTestLogger(t)
	grpcServer := setupTestGRPCServer(attrCfg.Port)
	svc, err := createSLAMService(t, attrCfg, logger, true)
	test.That(t, err, test.ShouldBeNil)

	grpcServer.Stop()
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	slamSvc := svc.(internal.Service)

	t.Run("Cartographer Data Process with lidar in slam mode 2d", func(t *testing.T) {
		goodCam := &inject.Camera{}
		goodCam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
			return pointcloud.New(), nil
		}
		cams := []camera.Camera{goodCam}
		camStreams := []gostream.VideoStream{gostream.NewEmbeddedVideoStream(goodCam)}
		defer func() {
			for _, stream := range camStreams {
				test.That(t, stream.Close(context.Background()), test.ShouldBeNil)
			}
		}()

		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		slamSvc.StartDataProcess(cancelCtx, cams, camStreams)

		n := 5
		// Note: timePadding is required to allow the sub processes to be fully completed during test
		time.Sleep(time.Millisecond * time.Duration((n)*(validDataRateMS+timePadding)))
		cancelFunc()

		files, err := os.ReadDir(name + "/data/")
		test.That(t, len(files), test.ShouldEqual, n)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, utils.TryClose(context.Background(), slamSvc), test.ShouldBeNil)
	})

	t.Run("Cartographer Data Process with lidar that errors during call to NextPointCloud", func(t *testing.T) {
		badCam := &inject.Camera{}
		badCam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
			return nil, errors.New("bad_lidar")
		}
		cams := []camera.Camera{badCam}
		camStreams := []gostream.VideoStream{gostream.NewEmbeddedVideoStream(badCam)}
		defer func() {
			for _, stream := range camStreams {
				test.That(t, stream.Close(context.Background()), test.ShouldBeNil)
			}
		}()

		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		slamSvc.StartDataProcess(cancelCtx, cams, camStreams)

		time.Sleep(time.Millisecond * time.Duration(validDataRateMS*2))
		cancelFunc()

		latestLoggedEntry := obs.All()[len(obs.All())-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "bad_lidar")
	})

	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	closeOutSLAMService(t, name)
}

func TestORBSLAMDataProcess(t *testing.T) {
	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	attrCfg := &slam.AttrConfig{
		Algorithm:     "fake_orbslamv3",
		Sensors:       []string{"good_camera"},
		ConfigParams:  map[string]string{"mode": "mono"},
		DataDirectory: name,
		DataRateMs:    validDataRateMS,
		Port:          "localhost:4445",
	}

	// Create slam service
	logger, obs := golog.NewObservedTestLogger(t)
	grpcServer := setupTestGRPCServer(attrCfg.Port)
	svc, err := createSLAMService(t, attrCfg, logger, true)
	test.That(t, err, test.ShouldBeNil)

	grpcServer.Stop()
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	slamSvc := svc.(internal.Service)

	t.Run("ORBSLAM3 Data Process with camera in slam mode mono", func(t *testing.T) {
		goodCam := &inject.Camera{}
		goodCam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
			return gostream.NewEmbeddedVideoStreamFromReader(gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
				return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, nil
			})), nil
		}
		cams := []camera.Camera{goodCam}
		camStreams := []gostream.VideoStream{gostream.NewEmbeddedVideoStream(goodCam)}
		defer func() {
			for _, stream := range camStreams {
				test.That(t, stream.Close(context.Background()), test.ShouldBeNil)
			}
		}()

		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		slamSvc.StartDataProcess(cancelCtx, cams, camStreams)

		n := 5
		// Note: timePadding is required to allow the sub processes to be fully completed during test
		time.Sleep(time.Millisecond * time.Duration((n)*(validDataRateMS+timePadding)))
		cancelFunc()

		files, err := os.ReadDir(name + "/data/")
		test.That(t, len(files), test.ShouldEqual, n)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, utils.TryClose(context.Background(), slamSvc), test.ShouldBeNil)
	})

	t.Run("ORBSLAM3 Data Process with camera that errors during call to Next", func(t *testing.T) {
		badCam := &inject.Camera{}
		badCam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
			return nil, errors.New("bad_camera")
		}
		cams := []camera.Camera{badCam}
		camStreams := []gostream.VideoStream{gostream.NewEmbeddedVideoStream(badCam)}
		defer func() {
			for _, stream := range camStreams {
				test.That(t, stream.Close(context.Background()), test.ShouldBeNil)
			}
		}()

		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		slamSvc.StartDataProcess(cancelCtx, cams, camStreams)

		time.Sleep(time.Millisecond * time.Duration(validDataRateMS*2))
		cancelFunc()

		latestLoggedEntry := obs.All()[len(obs.All())-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "bad_camera")
	})

	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	closeOutSLAMService(t, name)
}

func TestGetMapAndPosition(t *testing.T) {
	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	attrCfg := &slam.AttrConfig{
		Algorithm:        "fake_orbslamv3",
		Sensors:          []string{"good_camera"},
		ConfigParams:     map[string]string{"mode": "mono", "test_param": "viam"},
		DataDirectory:    name,
		MapRateSec:       200,
		DataRateMs:       validDataRateMS,
		InputFilePattern: "10:200:1",
		Port:             "localhost:4445",
	}

	// Create slam service
	logger := golog.NewTestLogger(t)
	grpcServer := setupTestGRPCServer(attrCfg.Port)
	svc, err := createSLAMService(t, attrCfg, logger, true)
	test.That(t, err, test.ShouldBeNil)

	p, err := svc.GetPosition(context.Background(), "hi")
	test.That(t, p, test.ShouldBeNil)
	test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "error getting SLAM position")

	pose := spatial.NewPoseFromOrientation(r3.Vector{1, 2, 3}, &spatial.OrientationVector{math.Pi / 2, 0, 0, -1})
	cp := referenceframe.NewPoseInFrame("frame", pose)

	mimeType, im, pc, err := svc.GetMap(context.Background(), "hi", rdkutils.MimeTypePCD, cp, true)
	test.That(t, mimeType, test.ShouldResemble, "")
	test.That(t, im, test.ShouldBeNil)
	test.That(t, pc, test.ShouldBeNil)
	test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "error getting SLAM map")

	grpcServer.Stop()
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	closeOutSLAMService(t, name)
}

func TestSLAMProcessSuccess(t *testing.T) {
	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	attrCfg := &slam.AttrConfig{
		Algorithm:        "fake_orbslamv3",
		Sensors:          []string{"good_camera"},
		ConfigParams:     map[string]string{"mode": "mono", "test_param": "viam"},
		DataDirectory:    name,
		MapRateSec:       200,
		DataRateMs:       validDataRateMS,
		InputFilePattern: "10:200:1",
		Port:             "localhost:4445",
	}

	// Create slam service
	logger := golog.NewTestLogger(t)
	grpcServer := setupTestGRPCServer(attrCfg.Port)
	svc, err := createSLAMService(t, attrCfg, logger, true)
	test.That(t, err, test.ShouldBeNil)

	slamSvc := svc.(internal.Service)
	processCfg := slamSvc.GetSLAMProcessConfig()
	cmd := append([]string{processCfg.Name}, processCfg.Args...)

	cmdResult := [][]string{
		{slam.SLAMLibraries["fake_orbslamv3"].BinaryLocation},
		{"-sensors=good_camera"},
		{"-config_param={mode=mono,test_param=viam}", "-config_param={test_param=viam,mode=mono}"},
		{"-data_rate_ms=200"},
		{"-map_rate_sec=200"},
		{"-data_dir=" + name},
		{"-input_file_pattern=10:200:1"},
		{"-port=localhost:4445"},
	}

	for i, s := range cmd {
		t.Run(fmt.Sprintf("Test command argument %v at index %v", s, i), func(t *testing.T) {
			test.That(t, s, test.ShouldBeIn, cmdResult[i])
		})
	}

	grpcServer.Stop()
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	closeOutSLAMService(t, name)
}

func TestSLAMProcessFail(t *testing.T) {
	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	attrCfg := &slam.AttrConfig{
		Algorithm:        "fake_orbslamv3",
		Sensors:          []string{"good_camera"},
		ConfigParams:     map[string]string{"mode": "mono", "test_param": "viam"},
		DataDirectory:    name,
		MapRateSec:       200,
		DataRateMs:       validDataRateMS,
		InputFilePattern: "10:200:1",
		Port:             "localhost:4445",
	}

	// Create slam service
	logger := golog.NewTestLogger(t)
	grpcServer := setupTestGRPCServer(attrCfg.Port)
	svc, err := createSLAMService(t, attrCfg, logger, true)
	test.That(t, err, test.ShouldBeNil)

	slamSvc := svc.(internal.Service)

	t.Run("Run SLAM process that errors out due to invalid binary location", func(t *testing.T) {
		cancelCtx, cancelFunc := context.WithCancel(context.Background())

		delete(slam.SLAMLibraries, "fake_orbslamv3")

		slam.SLAMLibraries["fake_orbslamv3"] = slam.LibraryMetadata{
			AlgoName:       "fake_" + slam.SLAMLibraries["orbslamv3"].AlgoName,
			AlgoType:       slam.SLAMLibraries["orbslamv3"].AlgoType,
			SlamMode:       slam.SLAMLibraries["orbslamv3"].SlamMode,
			BinaryLocation: "fail",
		}

		err := slamSvc.StartSLAMProcess(cancelCtx)
		test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "problem adding slam process:")

		cancelFunc()

		err = slamSvc.StopSLAMProcess()
		test.That(t, err, test.ShouldBeNil)
	})

	grpcServer.Stop()
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	closeOutSLAMService(t, name)
}

func TestGRPCConnection(t *testing.T) {
	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	attrCfg := &slam.AttrConfig{
		Algorithm:        "fake_orbslamv3",
		Sensors:          []string{"good_camera"},
		ConfigParams:     map[string]string{"mode": "mono", "test_param": "viam"},
		DataDirectory:    name,
		MapRateSec:       200,
		DataRateMs:       validDataRateMS,
		InputFilePattern: "10:200:1",
		Port:             "localhost:-1",
	}

	// Create slam service
	logger := golog.NewTestLogger(t)
	_, err = createSLAMService(t, attrCfg, logger, false)
	test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "error with initial grpc client to slam algorithm")

	closeOutSLAMService(t, name)
}

func createTempFolderArchitecture() (string, error) {
	name, err := os.MkdirTemp("", "*")
	if err != nil {
		return "nil", err
	}

	if err := os.Mkdir(name+"/map", os.ModePerm); err != nil {
		return "", err
	}
	if err := os.Mkdir(name+"/data", os.ModePerm); err != nil {
		return "", err
	}
	if err := os.Mkdir(name+"/config", os.ModePerm); err != nil {
		return "", err
	}
	return name, nil
}

var (
	testSvcName1 = "svc1"
	testSvcName2 = "svc2"
)

func TestRegisteredReconfigurable(t *testing.T) {
	s := registry.ResourceSubtypeLookup(slam.Subtype)
	test.That(t, s, test.ShouldNotBeNil)
	r := s.Reconfigurable
	test.That(t, r, test.ShouldNotBeNil)
}

func TestWrapWithReconfigurable(t *testing.T) {
	svc := &mock{name: testSvcName1}
	reconfSvc1, err := slam.WrapWithReconfigurable(svc)
	test.That(t, err, test.ShouldBeNil)

	_, err = slam.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rdkutils.NewUnimplementedInterfaceError("slam.Service", nil))

	reconfSvc2, err := slam.WrapWithReconfigurable(reconfSvc1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc2, test.ShouldEqual, reconfSvc1)
}

func TestReconfigurable(t *testing.T) {
	actualSvc1 := &mock{name: testSvcName1}
	reconfSvc1, err := slam.WrapWithReconfigurable(actualSvc1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc1, test.ShouldNotBeNil)

	actualArm2 := &mock{name: testSvcName2}
	reconfSvc2, err := slam.WrapWithReconfigurable(actualArm2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc2, test.ShouldNotBeNil)
	test.That(t, actualSvc1.reconfCount, test.ShouldEqual, 0)

	err = reconfSvc1.Reconfigure(context.Background(), reconfSvc2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc1, test.ShouldResemble, reconfSvc2)
	test.That(t, actualSvc1.reconfCount, test.ShouldEqual, 1)

	err = reconfSvc1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, rdkutils.NewUnexpectedTypeError(reconfSvc1, nil))
}

type mock struct {
	slam.Service
	name        string
	reconfCount int
}

func (m *mock) Close(ctx context.Context) error {
	m.reconfCount++
	return nil
}

func resetFolder(path string) error {
	err := os.RemoveAll(path)
	return err
}
