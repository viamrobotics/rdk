// Package builtin_test tests the functions that required injected components (such as robot and camera)
// in order to be run. It utilizes the internal package located in slam_test_helper.go to access
// certain exported functions which we do not want to make available to the user.
package builtin_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io/ioutil"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/builtin"
	slamConfig "go.viam.com/rdk/services/slam/internal/config"
	"go.viam.com/rdk/services/slam/internal/testhelper"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	validDataRateMS            = 200
	numCartographerPointClouds = 15
)

var (
	orbslamIntCameraMutex                     sync.Mutex
	orbslamIntCameraReleaseImagesChan         chan int = make(chan int, 2)
	orbslamIntWebcamReleaseImageChan          chan int = make(chan int, 1)
	orbslamIntSynchronizeCamerasChan          chan int = make(chan int)
	cartographerIntLidarReleasePointCloudChan chan int = make(chan int, 1)
	validMapRate                                       = 200
)

func getNumOrbslamImages(mode slam.Mode) int {
	switch mode {
	case slam.Mono:
		return 15
	case slam.Rgbd:
		return 29
	default:
		return 0
	}
}

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

func setupDeps(attr *builtin.AttrConfig) registry.Dependencies {
	deps := make(registry.Dependencies)
	var projA transform.Projector
	intrinsicsA := &transform.PinholeCameraIntrinsics{ // not the real camera parameters -- fake for test
		Width:  1280,
		Height: 720,
		Fx:     200,
		Fy:     200,
		Ppx:    640,
		Ppy:    360,
	}
	distortionsA := &transform.BrownConrady{RadialK1: 0.001, RadialK2: 0.00004}
	projA = intrinsicsA

	var projRealSense transform.Projector
	intrinsicsRealSense := &transform.PinholeCameraIntrinsics{
		Width:  1280,
		Height: 720,
		Fx:     900.538,
		Fy:     900.818,
		Ppx:    648.934,
		Ppy:    367.736,
	}
	distortionsRealSense := &transform.BrownConrady{
		RadialK1:     0.158701,
		RadialK2:     -0.485405,
		RadialK3:     0.435342,
		TangentialP1: -0.00143327,
		TangentialP2: -0.000705919}
	projRealSense = intrinsicsRealSense

	var projWebcam transform.Projector
	intrinsicsWebcam := &transform.PinholeCameraIntrinsics{
		Width:  640,
		Height: 480,
		Fx:     939.2693584627577,
		Fy:     940.2928257873841,
		Ppx:    320.6075282958033,
		Ppy:    239.14408757087756,
	}
	distortionsWebcam := &transform.BrownConrady{
		RadialK1:     0.046535971648456166,
		RadialK2:     0.8002516496932317,
		RadialK3:     -5.408034254951954,
		TangentialP1: -8.996658362365533e-06,
		TangentialP2: -0.002828504714921335}
	projWebcam = intrinsicsWebcam

	for _, sensor := range attr.Sensors {
		cam := &inject.Camera{}
		switch sensor {
		case "good_lidar":
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return pointcloud.New(), nil
			}
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				return nil, errors.New("lidar not camera")
			}
			cam.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
				return nil, transform.NewNoIntrinsicsError("")
			}
			cam.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
				return camera.Properties{}, nil
			}
			deps[camera.Named(sensor)] = cam
		case "bad_lidar":
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("bad_lidar")
			}
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				return nil, errors.New("lidar not camera")
			}
			cam.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
				return nil, transform.NewNoIntrinsicsError("")
			}
			deps[camera.Named(sensor)] = cam
		case "good_camera":
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				return gostream.NewEmbeddedVideoStreamFromReader(
					gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
						return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, nil
					}),
				), nil
			}
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("camera not lidar")
			}
			cam.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
				return projA, nil
			}
			cam.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
				return camera.Properties{IntrinsicParams: intrinsicsA, DistortionParams: distortionsA}, nil
			}
			deps[camera.Named(sensor)] = cam
		case "good_color_camera":
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("camera not lidar")
			}
			cam.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
				return projA, nil
			}
			cam.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
				return camera.Properties{IntrinsicParams: intrinsicsA, DistortionParams: distortionsA}, nil
			}
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				imgBytes, err := os.ReadFile(artifact.MustPath("rimage/board1.png"))
				if err != nil {
					return nil, err
				}
				lazy := rimage.NewLazyEncodedImage(imgBytes, rdkutils.MimeTypePNG)
				return gostream.NewEmbeddedVideoStreamFromReader(
					gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
						return lazy, func() {}, nil
					}),
				), nil
			}
			deps[camera.Named(sensor)] = cam
		case "good_depth_camera":
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("camera not lidar")
			}
			cam.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
				return nil, transform.NewNoIntrinsicsError("")
			}
			cam.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
				return camera.Properties{}, nil
			}
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				imgBytes, err := os.ReadFile(artifact.MustPath("rimage/board1_gray.png"))
				if err != nil {
					return nil, err
				}
				lazy := rimage.NewLazyEncodedImage(imgBytes, rdkutils.MimeTypePNG)
				return gostream.NewEmbeddedVideoStreamFromReader(
					gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
						return lazy, func() {}, nil
					}),
				), nil
			}
			deps[camera.Named(sensor)] = cam
		case "bad_camera":
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				return nil, errors.New("bad_camera")
			}
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("camera not lidar")
			}
			cam.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
				return nil, transform.NewNoIntrinsicsError("")
			}
			deps[camera.Named(sensor)] = cam
		case "bad_camera_intrinsics":
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				return gostream.NewEmbeddedVideoStreamFromReader(
					gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
						return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, nil
					}),
				), nil
			}
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("camera not lidar")
			}
			cam.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
				return &transform.PinholeCameraIntrinsics{}, nil
			}
			cam.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
				return camera.Properties{
					IntrinsicParams:  &transform.PinholeCameraIntrinsics{},
					DistortionParams: &transform.BrownConrady{},
				}, nil
			}
			deps[camera.Named(sensor)] = cam
		case "orbslam_int_color_camera":
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("camera not lidar")
			}
			cam.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
				return projRealSense, nil
			}
			cam.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
				return camera.Properties{IntrinsicParams: intrinsicsRealSense, DistortionParams: distortionsRealSense}, nil
			}
			var index uint64
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				defer func() {
					orbslamIntSynchronizeCamerasChan <- 1
				}()
				// Ensure the StreamFunc functions for orbslam_int_color_camera and orbslam_int_depth_camera run under
				// the lock so that they release images in the same call to getSimultaneousColorAndDepth().
				orbslamIntCameraMutex.Lock()
				select {
				case <-orbslamIntCameraReleaseImagesChan:
					i := atomic.AddUint64(&index, 1) - 1
					if i >= uint64(getNumOrbslamImages(slam.Rgbd)) {
						return nil, errors.New("No more orbslam color images")
					}
					imgBytes, err := os.ReadFile(artifact.MustPath("slam/mock_camera_short/rgb/" + strconv.FormatUint(i, 10) + ".png"))
					if err != nil {
						return nil, err
					}
					lazy := rimage.NewLazyEncodedImage(imgBytes, rdkutils.MimeTypePNG)
					return gostream.NewEmbeddedVideoStreamFromReader(
						gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
							return lazy, func() {}, nil
						}),
					), nil
				default:
					return nil, errors.Errorf("Color camera not ready to return image %v", index)
				}
			}
			deps[camera.Named(sensor)] = cam
		case "orbslam_int_depth_camera":
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("camera not lidar")
			}
			cam.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
				return nil, transform.NewNoIntrinsicsError("")
			}
			cam.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
				return camera.Properties{}, nil
			}
			var index uint64
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				defer func() {
					orbslamIntCameraMutex.Unlock()
				}()
				// Ensure StreamFunc for orbslam_int_color_camera runs first, so that we lock orbslamIntCameraMutex before
				// unlocking it
				<-orbslamIntSynchronizeCamerasChan
				select {
				case <-orbslamIntCameraReleaseImagesChan:
					i := atomic.AddUint64(&index, 1) - 1
					if i >= uint64(getNumOrbslamImages(slam.Rgbd)) {
						return nil, errors.New("No more orbslam depth images")
					}
					imgBytes, err := os.ReadFile(artifact.MustPath("slam/mock_camera_short/depth/" + strconv.FormatUint(i, 10) + ".png"))
					if err != nil {
						return nil, err
					}
					lazy := rimage.NewLazyEncodedImage(imgBytes, rdkutils.MimeTypePNG)
					return gostream.NewEmbeddedVideoStreamFromReader(
						gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
							return lazy, func() {}, nil
						}),
					), nil
				default:
					return nil, errors.Errorf("Depth camera not ready to return image %v", index)
				}
			}
			deps[camera.Named(sensor)] = cam
		case "orbslam_int_webcam":
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				return nil, errors.New("camera not lidar")
			}
			cam.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
				return projWebcam, nil
			}
			cam.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
				return camera.Properties{IntrinsicParams: intrinsicsWebcam, DistortionParams: distortionsWebcam}, nil
			}
			var index uint64
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				select {
				case <-orbslamIntWebcamReleaseImageChan:
					i := atomic.AddUint64(&index, 1) - 1
					if i >= uint64(getNumOrbslamImages(slam.Mono)) {
						return nil, errors.New("No more orbslam webcam images")
					}
					imgBytes, err := os.ReadFile(artifact.MustPath("slam/mock_mono_camera/rgb/" + strconv.FormatUint(i, 10) + ".png"))
					if err != nil {
						return nil, err
					}
					img, _, err := image.Decode(bytes.NewReader(imgBytes))
					if err != nil {
						return nil, err
					}
					var ycbcrImg image.YCbCr
					rimage.ImageToYCbCrForTesting(&ycbcrImg, img)
					return gostream.NewEmbeddedVideoStreamFromReader(
						gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
							return &ycbcrImg, func() {}, nil
						}),
					), nil
				default:
					return nil, errors.Errorf("Webcam not ready to return image %v", index)
				}
			}
			deps[camera.Named(sensor)] = cam
		case "gibberish":
			return deps
		case "cartographer_int_lidar":
			var index uint64
			cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
				select {
				case <-cartographerIntLidarReleasePointCloudChan:
					i := atomic.AddUint64(&index, 1) - 1
					if i >= numCartographerPointClouds {
						return nil, errors.New("No more cartographer point clouds")
					}
					file, err := os.Open(artifact.MustPath("slam/mock_lidar/" + strconv.FormatUint(i, 10) + ".pcd"))
					if err != nil {
						return nil, err
					}
					pointCloud, err := pointcloud.ReadPCD(file)
					if err != nil {
						return nil, err
					}
					return pointCloud, nil
				default:
					return nil, errors.Errorf("Lidar not ready to return point cloud %v", index)
				}
			}
			cam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
				return nil, errors.New("lidar not camera")
			}
			cam.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
				return nil, transform.NewNoIntrinsicsError("")
			}
			cam.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
				return camera.Properties{}, nil
			}
			deps[camera.Named(sensor)] = cam
		default:
			continue
		}
	}
	return deps
}

func createSLAMService(
	t *testing.T,
	attrCfg *builtin.AttrConfig,
	model string,
	logger golog.Logger,
	bufferSLAMProcessLogs bool,
	success bool,
) (slam.Service, error) {
	t.Helper()

	ctx := context.Background()
	cfgService := config.Service{Name: "test", Type: "slam", Model: resource.NewDefaultModel(resource.ModelName(model))}
	cfgService.ConvertedAttributes = attrCfg

	deps := setupDeps(attrCfg)

	sensorDeps, err := attrCfg.Validate("path")
	if err != nil {
		return nil, err
	}
	test.That(t, sensorDeps, test.ShouldResemble, attrCfg.Sensors)

	builtin.SetCameraValidationMaxTimeoutSecForTesting(1)
	builtin.SetDialMaxTimeoutSecForTesting(1)

	svc, err := builtin.NewBuiltIn(ctx, deps, cfgService, logger, bufferSLAMProcessLogs)

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
		attrCfg := &builtin.AttrConfig{}
		_, err := createSLAMService(t, attrCfg, "test", logger, false, false)
		test.That(t, err, test.ShouldBeError, utils.NewConfigValidationFieldRequiredError("path", "config_params[mode]"))
	})

	t.Run("New slam service with no data dir", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		attrCfg := &builtin.AttrConfig{ConfigParams: map[string]string{"mode": "2d"}}
		_, err := createSLAMService(t, attrCfg, "test", logger, false, false)
		test.That(t, err, test.ShouldBeError, utils.NewConfigValidationFieldRequiredError("path", "data_dir"))
	})

	t.Run("New slam service with no mode", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		attrCfg := &builtin.AttrConfig{DataDirectory: "test"}
		_, err := createSLAMService(t, attrCfg, "test", logger, false, false)
		test.That(t, err, test.ShouldBeError, utils.NewConfigValidationFieldRequiredError("path", "config_params[mode]"))
	})

	t.Run("New slam service with no camera", func(t *testing.T) {
		attrCfg := &builtin.AttrConfig{
			Sensors:       []string{},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		grpcServer := setupTestGRPCServer(attrCfg.Port)
		svc, err := createSLAMService(t, attrCfg, "fake_cartographer", logger, false, true)
		test.That(t, err, test.ShouldBeNil)

		grpcServer.Stop()
		test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	})

	t.Run("New slam service with bad camera", func(t *testing.T) {
		attrCfg := &builtin.AttrConfig{
			Sensors:       []string{"gibberish"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, "fake_cartographer", logger, false, false)
		test.That(t, err, test.ShouldBeError,
			errors.New("configuring camera error: error getting camera gibberish for slam service: \"gibberish\" missing from dependencies"))

	})

	t.Run("New slam service with invalid slam algo type", func(t *testing.T) {
		attrCfg := &builtin.AttrConfig{
			Sensors:       []string{"good_camera"},
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
		_, err := createSLAMService(t, attrCfg, "test", logger, false, false)
		test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "runtime slam service error: invalid slam algorithm \"test\"")

		delete(slam.SLAMLibraries, "test")
	})

	closeOutSLAMService(t, name)
}

func TestArgumentInputs(t *testing.T) {

	logger := golog.NewTestLogger(t)

	t.Run("Testing delete_data_process evaluation", func(t *testing.T) {
		// No delete_processed_data
		deleteProcessedData := slamConfig.DetermineDeleteProcessedData(logger, nil, true)
		test.That(t, deleteProcessedData, test.ShouldBeFalse)

		deleteProcessedData = slamConfig.DetermineDeleteProcessedData(logger, nil, false)
		test.That(t, deleteProcessedData, test.ShouldBeTrue)

		// False delete_processed_data
		delete := false
		deleteProcessedData = slamConfig.DetermineDeleteProcessedData(logger, &delete, true)
		test.That(t, deleteProcessedData, test.ShouldBeFalse)

		deleteProcessedData = slamConfig.DetermineDeleteProcessedData(logger, &delete, false)
		test.That(t, deleteProcessedData, test.ShouldBeFalse)

		// True delete_processed_data
		delete = true
		deleteProcessedData = slamConfig.DetermineDeleteProcessedData(logger, &delete, true)
		test.That(t, deleteProcessedData, test.ShouldBeFalse)

		deleteProcessedData = slamConfig.DetermineDeleteProcessedData(logger, &delete, false)
		test.That(t, deleteProcessedData, test.ShouldBeTrue)
	})
}

func TestCartographerNew(t *testing.T) {
	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	t.Run("New cartographer service with good lidar in slam mode 2d", func(t *testing.T) {
		attrCfg := &builtin.AttrConfig{
			Sensors:       []string{"good_lidar"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		grpcServer := setupTestGRPCServer(attrCfg.Port)
		svc, err := createSLAMService(t, attrCfg, "fake_cartographer", logger, false, true)
		test.That(t, err, test.ShouldBeNil)

		grpcServer.Stop()
		test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	})

	t.Run("New cartographer service with lidar that errors during call to NextPointCloud", func(t *testing.T) {
		attrCfg := &builtin.AttrConfig{
			Sensors:       []string{"bad_lidar"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, "fake_cartographer", logger, false, false)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("runtime slam service error: error getting data in desired mode: %v", attrCfg.Sensors[0]))
	})

	t.Run("New cartographer service with camera without NextPointCloud implementation", func(t *testing.T) {
		attrCfg := &builtin.AttrConfig{
			Sensors:       []string{"good_camera"},
			ConfigParams:  map[string]string{"mode": "2d"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, "fake_cartographer", logger, false, false)

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
		attrCfg := &builtin.AttrConfig{
			Sensors:       []string{"good_color_camera", "good_depth_camera"},
			ConfigParams:  map[string]string{"mode": "rgbd"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		grpcServer := setupTestGRPCServer(attrCfg.Port)
		svc, err := createSLAMService(t, attrCfg, "fake_orbslamv3", logger, false, true)
		test.That(t, err, test.ShouldBeNil)

		grpcServer.Stop()
		test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	})

	t.Run("New orbslamv3 service in slam mode rgbd that errors due to a single camera", func(t *testing.T) {
		attrCfg := &builtin.AttrConfig{
			Sensors:       []string{"good_color_camera"},
			ConfigParams:  map[string]string{"mode": "rgbd"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, "fake_orbslamv3", logger, false, false)
		test.That(t, err.Error(), test.ShouldContainSubstring,
			errors.Errorf("expected 2 cameras for Rgbd slam, found %v", len(attrCfg.Sensors)).Error())
	})

	t.Run("New orbslamv3 service in slam mode rgbd that errors due cameras in the wrong order", func(t *testing.T) {
		attrCfg := &builtin.AttrConfig{
			Sensors:       []string{"good_depth_camera", "good_color_camera"},
			ConfigParams:  map[string]string{"mode": "rgbd"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, "fake_orbslamv3", logger, false, false)
		test.That(t, err.Error(), test.ShouldContainSubstring,
			errors.New("Unable to get camera features for first camera, make sure the color camera is listed first").Error())
	})

	t.Run("New orbslamv3 service with good camera in slam mode mono", func(t *testing.T) {
		attrCfg := &builtin.AttrConfig{
			Sensors:       []string{"good_color_camera"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
			Port:          "localhost:4445",
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		grpcServer := setupTestGRPCServer(attrCfg.Port)
		svc, err := createSLAMService(t, attrCfg, "fake_orbslamv3", logger, false, true)
		test.That(t, err, test.ShouldBeNil)

		grpcServer.Stop()
		test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	})

	t.Run("New orbslamv3 service with camera that errors during call to Next", func(t *testing.T) {
		attrCfg := &builtin.AttrConfig{
			Sensors:       []string{"bad_camera"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, "fake_orbslamv3", logger, false, false)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("runtime slam service error: "+
				"error getting data in desired mode: %v", attrCfg.Sensors[0]))
	})

	t.Run("New orbslamv3 service with camera that errors from bad intrinsics", func(t *testing.T) {
		attrCfg := &builtin.AttrConfig{
			Sensors:       []string{"bad_camera_intrinsics"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, "fake_orbslamv3", logger, false, false)

		test.That(t, err.Error(), test.ShouldContainSubstring,
			transform.NewNoIntrinsicsError(fmt.Sprintf("Invalid size (%#v, %#v)", 0, 0)).Error())
	})

	t.Run("New orbslamv3 service with lidar without Next implementation", func(t *testing.T) {
		attrCfg := &builtin.AttrConfig{
			Sensors:       []string{"good_lidar"},
			ConfigParams:  map[string]string{"mode": "mono"},
			DataDirectory: name,
			DataRateMs:    validDataRateMS,
		}

		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfg, "fake_orbslamv3", logger, false, false)
		test.That(t, err, test.ShouldBeError,
			errors.New("runtime slam service error: error getting data in desired mode: lidar not camera"))
	})
	closeOutSLAMService(t, name)
}

func TestCartographerDataProcess(t *testing.T) {
	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	attrCfg := &builtin.AttrConfig{
		Sensors:       []string{"good_lidar"},
		ConfigParams:  map[string]string{"mode": "2d"},
		DataDirectory: name,
		DataRateMs:    validDataRateMS,
		Port:          "localhost:4445",
	}

	// Create slam service
	logger, obs := golog.NewObservedTestLogger(t)
	grpcServer := setupTestGRPCServer(attrCfg.Port)
	svc, err := createSLAMService(t, attrCfg, "fake_cartographer", logger, false, true)
	test.That(t, err, test.ShouldBeNil)

	grpcServer.Stop()
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	slamSvc := svc.(testhelper.Service)

	t.Run("Cartographer Data Process with lidar in slam mode 2d", func(t *testing.T) {
		goodCam := &inject.Camera{}
		goodCam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
			return pointcloud.New(), nil
		}
		goodCam.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
			return camera.Properties{}, nil
		}
		cams := []camera.Camera{goodCam}
		camStreams := []gostream.VideoStream{gostream.NewEmbeddedVideoStream(goodCam)}
		defer func() {
			for _, stream := range camStreams {
				test.That(t, stream.Close(context.Background()), test.ShouldBeNil)
			}
		}()

		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		c := make(chan int, 100)
		slamSvc.StartDataProcess(cancelCtx, cams, camStreams, c)

		<-c
		cancelFunc()
		files, err := os.ReadDir(name + "/data/")
		test.That(t, len(files), test.ShouldBeGreaterThanOrEqualTo, 1)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("Cartographer Data Process with lidar that errors during call to NextPointCloud", func(t *testing.T) {
		badCam := &inject.Camera{}
		badCam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
			return nil, errors.New("bad_lidar")
		}
		badCam.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
			return camera.Properties{}, nil
		}
		cams := []camera.Camera{badCam}
		camStreams := []gostream.VideoStream{gostream.NewEmbeddedVideoStream(badCam)}
		defer func() {
			for _, stream := range camStreams {
				test.That(t, stream.Close(context.Background()), test.ShouldBeNil)
			}
		}()

		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		c := make(chan int, 100)
		slamSvc.StartDataProcess(cancelCtx, cams, camStreams, c)

		<-c
		allObs := obs.All()
		latestLoggedEntry := allObs[len(allObs)-1]
		cancelFunc()
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "bad_lidar")
	})

	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	closeOutSLAMService(t, name)
}

func TestORBSLAMDataProcess(t *testing.T) {
	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	attrCfg := &builtin.AttrConfig{
		Sensors:       []string{"good_color_camera"},
		ConfigParams:  map[string]string{"mode": "mono"},
		DataDirectory: name,
		DataRateMs:    validDataRateMS,
		Port:          "localhost:4445",
	}

	// Create slam service
	logger, obs := golog.NewObservedTestLogger(t)
	grpcServer := setupTestGRPCServer(attrCfg.Port)
	svc, err := createSLAMService(t, attrCfg, "fake_orbslamv3", logger, false, true)
	test.That(t, err, test.ShouldBeNil)

	grpcServer.Stop()
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	slamSvc := svc.(testhelper.Service)

	t.Run("ORBSLAM3 Data Process with camera in slam mode mono", func(t *testing.T) {
		goodCam := &inject.Camera{}
		goodCam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
			imgBytes, err := os.ReadFile(artifact.MustPath("rimage/board1.png"))
			if err != nil {
				return nil, err
			}
			lazy := rimage.NewLazyEncodedImage(imgBytes, rdkutils.MimeTypePNG)
			return gostream.NewEmbeddedVideoStreamFromReader(
				gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
					return lazy, func() {}, nil
				}),
			), nil
		}

		cams := []camera.Camera{goodCam}
		camStreams := []gostream.VideoStream{gostream.NewEmbeddedVideoStream(goodCam)}
		defer func() {
			for _, stream := range camStreams {
				test.That(t, stream.Close(context.Background()), test.ShouldBeNil)
			}
		}()

		cancelCtx, cancelFunc := context.WithCancel(context.Background())

		c := make(chan int, 100)
		slamSvc.StartDataProcess(cancelCtx, cams, camStreams, c)

		<-c
		cancelFunc()
		files, err := os.ReadDir(name + "/data/rgb/")
		test.That(t, len(files), test.ShouldBeGreaterThanOrEqualTo, 1)
		test.That(t, err, test.ShouldBeNil)
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
		c := make(chan int, 100)
		slamSvc.StartDataProcess(cancelCtx, cams, camStreams, c)

		<-c
		obsAll := obs.All()
		latestLoggedEntry := obsAll[len(obsAll)-1]
		cancelFunc()
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "bad_camera")
	})

	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	closeOutSLAMService(t, name)
}

func TestGetMapAndPosition(t *testing.T) {
	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	attrCfg := &builtin.AttrConfig{
		Sensors:          []string{"good_color_camera"},
		ConfigParams:     map[string]string{"mode": "mono", "test_param": "viam"},
		DataDirectory:    name,
		MapRateSec:       &validMapRate,
		DataRateMs:       validDataRateMS,
		InputFilePattern: "10:200:1",
		Port:             "localhost:4445",
	}

	// Create slam service
	logger := golog.NewTestLogger(t)
	grpcServer := setupTestGRPCServer(attrCfg.Port)
	svc, err := createSLAMService(t, attrCfg, "fake_orbslamv3", logger, false, true)
	test.That(t, err, test.ShouldBeNil)

	p, err := svc.Position(context.Background(), "hi", map[string]interface{}{})
	test.That(t, p, test.ShouldBeNil)
	test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "error getting SLAM position")

	pose := spatial.NewPoseFromOrientation(r3.Vector{X: 1, Y: 2, Z: 3},
		&spatial.OrientationVector{Theta: math.Pi / 2, OX: 0, OY: 0, OZ: -1})
	cp := referenceframe.NewPoseInFrame("frame", pose)

	mimeType, im, pc, err := svc.GetMap(context.Background(), "hi", rdkutils.MimeTypePCD, cp, true, map[string]interface{}{})
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

	logger := golog.NewTestLogger(t)

	t.Run("Test online SLAM process with default parameters", func(t *testing.T) {

		attrCfg := &builtin.AttrConfig{
			Sensors:       []string{"good_lidar"},
			ConfigParams:  map[string]string{"mode": "2d", "test_param": "viam"},
			DataDirectory: name,
			Port:          "localhost:4445",
		}

		// Create slam service
		grpcServer := setupTestGRPCServer(attrCfg.Port)
		svc, err := createSLAMService(t, attrCfg, "fake_cartographer", logger, false, true)
		test.That(t, err, test.ShouldBeNil)

		slamSvc := svc.(testhelper.Service)
		processCfg := slamSvc.GetSLAMProcessConfig()
		cmd := append([]string{processCfg.Name}, processCfg.Args...)

		cmdResult := [][]string{
			{slam.SLAMLibraries["fake_cartographer"].BinaryLocation},
			{"-sensors=good_lidar"},
			{"-config_param={test_param=viam,mode=2d}", "-config_param={mode=2d,test_param=viam}"},
			{"-data_rate_ms=200"},
			{"-map_rate_sec=60"},
			{"-data_dir=" + name},
			{"-input_file_pattern="},
			{"-delete_processed_data=true"},
			{"-port=localhost:4445"},
			{"--aix-auto-update"},
		}

		for i, s := range cmd {
			t.Run(fmt.Sprintf("Test command argument %v at index %v", s, i), func(t *testing.T) {
				test.That(t, s, test.ShouldBeIn, cmdResult[i])
			})
		}

		grpcServer.Stop()
		test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	})

	t.Run("Test offline SLAM process with default parameters", func(t *testing.T) {

		attrCfg := &builtin.AttrConfig{
			Sensors:       []string{},
			ConfigParams:  map[string]string{"mode": "mono", "test_param": "viam"},
			DataDirectory: name,
			Port:          "localhost:4445",
		}

		// Create slam service
		grpcServer := setupTestGRPCServer(attrCfg.Port)
		svc, err := createSLAMService(t, attrCfg, "fake_orbslamv3", logger, false, true)
		test.That(t, err, test.ShouldBeNil)

		slamSvc := svc.(testhelper.Service)
		processCfg := slamSvc.GetSLAMProcessConfig()
		cmd := append([]string{processCfg.Name}, processCfg.Args...)

		cmdResult := [][]string{
			{slam.SLAMLibraries["fake_orbslamv3"].BinaryLocation},
			{"-sensors="},
			{"-config_param={mode=mono,test_param=viam}", "-config_param={test_param=viam,mode=mono}"},
			{"-data_rate_ms=200"},
			{"-map_rate_sec=60"},
			{"-data_dir=" + name},
			{"-input_file_pattern="},
			{"-delete_processed_data=false"},
			{"-port=localhost:4445"},
			{"--aix-auto-update"},
		}

		for i, s := range cmd {
			t.Run(fmt.Sprintf("Test command argument %v at index %v", s, i), func(t *testing.T) {
				test.That(t, s, test.ShouldBeIn, cmdResult[i])
			})
		}

		grpcServer.Stop()
		test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	})

	closeOutSLAMService(t, name)
}

func TestSLAMProcessFail(t *testing.T) {
	name, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	attrCfg := &builtin.AttrConfig{
		Sensors:          []string{"good_color_camera"},
		ConfigParams:     map[string]string{"mode": "mono", "test_param": "viam"},
		DataDirectory:    name,
		MapRateSec:       &validMapRate,
		DataRateMs:       validDataRateMS,
		InputFilePattern: "10:200:1",
		Port:             "localhost:4445",
	}

	// Create slam service
	logger := golog.NewTestLogger(t)
	grpcServer := setupTestGRPCServer(attrCfg.Port)
	svc, err := createSLAMService(t, attrCfg, "fake_orbslamv3", logger, false, true)
	test.That(t, err, test.ShouldBeNil)

	slamSvc := svc.(testhelper.Service)

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

	attrCfg := &builtin.AttrConfig{
		Sensors:          []string{"good_color_camera"},
		ConfigParams:     map[string]string{"mode": "mono", "test_param": "viam"},
		DataDirectory:    name,
		MapRateSec:       &validMapRate,
		DataRateMs:       validDataRateMS,
		InputFilePattern: "10:200:1",
		Port:             "localhost:-1",
	}

	// Create slam service
	logger := golog.NewTestLogger(t)
	_, err = createSLAMService(t, attrCfg, "fake_orbslamv3", logger, false, false)
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

func resetFolder(path string) error {
	err := os.RemoveAll(path)
	if err != nil {
		return err
	}
	err = os.Mkdir(path, os.ModePerm)
	return err
}

func checkDeleteProcessedData(t *testing.T, mode slam.Mode, dir string, prev int, deleteProcessedData, online bool) int {
	var numFiles int

	switch mode {
	case slam.Mono:
		numFilesRGB, err := checkDataDirForExpectedFiles(t, dir+"/data/rgb", prev, online, deleteProcessedData)
		test.That(t, err, test.ShouldBeNil)

		numFiles = numFilesRGB
	case slam.Rgbd:
		numFilesRGB, err := checkDataDirForExpectedFiles(t, dir+"/data/rgb", prev, online, deleteProcessedData)
		test.That(t, err, test.ShouldBeNil)

		numFilesDepth, err := checkDataDirForExpectedFiles(t, dir+"/data/depth", prev, online, deleteProcessedData)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, numFilesRGB, test.ShouldEqual, numFilesDepth)
		numFiles = numFilesRGB
	case slam.Dim2d:
		numFiles2D, err := checkDataDirForExpectedFiles(t, dir+"/data", prev, online, deleteProcessedData)
		test.That(t, err, test.ShouldBeNil)
		numFiles = numFiles2D
	default:
	}
	return numFiles
}

// Compares the number of files found in a specified data directory with the previous number found and uses
// the online state and delete_processed_data value to evaluate this comparison.
func checkDataDirForExpectedFiles(t *testing.T, dir string, prev int, delete_processed_data, online bool) (int, error) {

	files, err := ioutil.ReadDir(dir)
	test.That(t, err, test.ShouldBeNil)

	if prev == 0 {
		return len(files), nil
	}
	if delete_processed_data && online {
		test.That(t, prev, test.ShouldEqual, len(files))
	}
	if !delete_processed_data && online {
		test.That(t, prev, test.ShouldBeLessThan, len(files))
	}
	if delete_processed_data && !online {
		return 0, errors.New("the delete_processed_data value cannot be true when running SLAM in offline mode")
	}
	if !delete_processed_data && !online {
		test.That(t, prev, test.ShouldEqual, len(files))
	}
	return len(files), nil
}
