package slam

import (
	"context"
	"image"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
)

func createSLAMService(t *testing.T, attrCfg *AttrConfig) (Service, error) {
	t.Helper()
	cfgService := config.Service{Name: "test", Type: "slam"}
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	r := &inject.Robot{}
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, error) {
		return nil, rdkutils.NewResourceNotFoundError(n)
	}

	cfgService.ConvertedAttributes = attrCfg

	return New(ctx, r, cfgService, logger)
}

// General SLAM Tests.
func TestGeneralSLAMService(t *testing.T) {
	_, err := createSLAMService(t, &AttrConfig{})
	test.That(t, err, test.ShouldBeNil)

	name1, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	attrCfg := &AttrConfig{
		Algorithm:        "cartographer",
		Sensors:          []string{"rplidar"},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name1,
		InputFilePattern: "100:300:5",
	}

	_, err = createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeError,
		errors.Errorf("error with get camera for slam service: \"resource \\\"rdk:component:camera/%v\\\" not found\"", attrCfg.Sensors[0]))

	attrCfg = &AttrConfig{
		Algorithm:        "cartographer",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name1,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
		MapRateSec:       5,
	}

	slams, err := createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, slams.getSLAMServiceData().dataRateSec, test.ShouldEqual, 100)
	test.That(t, slams.getSLAMServiceData().mapRateSec, test.ShouldEqual, 5)

	attrCfg.DataRateMs = 0
	attrCfg.MapRateSec = 0
	slams, err = createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, slams.getSLAMServiceData().dataRateSec, test.ShouldEqual, 200)
	test.That(t, slams.getSLAMServiceData().mapRateSec, test.ShouldEqual, 60)

	slamSvc := slams.getSLAMServiceData()

	err = slamSvc.Close(slamSvc.cancelCtx)
	test.That(t, err, test.ShouldBeNil)

	err = resetFolder(name1)
	test.That(t, err, test.ShouldBeNil)
}

// Validate Tests.
func TestConfigValidation(t *testing.T) {
	logger := golog.NewTestLogger(t)

	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pointcloud.New(), nil
	}

	// Runtime Validation Tests
	name1, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	cfg := &AttrConfig{
		Algorithm:        "cartographer",
		Sensors:          []string{"rplidar"},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name1,
		InputFilePattern: "100:300:5",
	}

	err = runtimeConfigValidation(cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	// No sensor test
	t.Run("run test of config with no sensor", func(t *testing.T) {
		cfg.Sensors = []string{}
		err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeNil)
		cfg.Sensors = []string{"rplidar"}
	})

	// Mode SLAM Library test
	t.Run("SLAM config mode tests", func(t *testing.T) {
		cfg.ConfigParams["mode"] = ""
		err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeNil)

		testMetadata := metadata{
			AlgoName: "test",
			SlamMode: map[string]mode{},
		}

		slamLibraries["test"] = testMetadata
		cfg.Algorithm = "test"
		cfg.Sensors = []string{"test_sensor"}
		cfg.ConfigParams["mode"] = "test1"
		err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("getting data with specified algorithm, %v, and desired mode %v", cfg.Algorithm, cfg.ConfigParams["mode"]))

		cfg.Algorithm = "cartographer"
		cfg.Sensors = []string{"rplidar"}
		cfg.ConfigParams["mode"] = "2d"

		delete(slamLibraries, "test")

		cfg.ConfigParams["mode"] = "rgbd"
		err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("getting data with specified algorithm, %v, and desired mode %v", cfg.Algorithm, cfg.ConfigParams["mode"]))
	})

	// Input File Pattern test
	t.Run("SLAM config input file pattern tests", func(t *testing.T) {
		cfg.ConfigParams["mode"] = "2d"
		cfg.InputFilePattern = "dd:300:3"
		err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("input_file_pattern (%v) does not match the regex pattern (\\d+):(\\d+):(\\d+)", cfg.InputFilePattern))

		cfg.InputFilePattern = "500:300:3"
		err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("second value in input file pattern must be larger than the first [%v]", cfg.InputFilePattern))

		err = resetFolder(name1)
		test.That(t, err, test.ShouldBeNil)
	})

	// TODO: sensor and saving data checks once GetAndSaveData is implemented

	// Check if valid algorithm
	t.Run("SLAM config check on specified algorithm", func(t *testing.T) {
		cfg.Algorithm = "wrong_algo"
		err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError, errors.Errorf("%v algorithm specified not in implemented list", cfg.Algorithm))
	})
}

// Cartographer Specific Tests (config).
func TestCartographerData(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	attrCfg := &AttrConfig{
		Algorithm:        "cartographer",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
	}

	slamS, err := createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeNil)

	slamSvc := slamS.getSLAMServiceData()
	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pointcloud.New(), nil
	}
	slamSvc.camera = cam

	err = runtimeServiceValidation(&slamSvc)
	test.That(t, err, test.ShouldBeNil)

	_, _ = slamSvc.getAndSaveDataDense()
	test.That(t, err, test.ShouldBeNil)

	slamSvc.Close(slamSvc.cancelCtx)
	test.That(t, err, test.ShouldBeNil)

	err = resetFolder(name)
	test.That(t, err, test.ShouldBeNil)
}

// GetAndSaveDataDense Tests for poitncloud data.
func TestGetAndSaveDataCartographer(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	attrCfg := &AttrConfig{
		Algorithm:        "cartographer",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
	}

	slamS, err := createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeNil)

	slamSvc := slamS.getSLAMServiceData()

	err = slamSvc.startSLAMProcess(slamSvc.cancelCtx)
	test.That(t, err, test.ShouldBeNil)

	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pointcloud.New(), nil
	}

	slamSvc.camera = cam

	filename, err := slamSvc.getAndSaveDataDense()
	test.That(t, err, test.ShouldBeNil)

	_, err = os.Stat(filename)
	test.That(t, err, test.ShouldBeNil)

	ddTemp := slamSvc.dataDirectory
	slamSvc.dataDirectory = "gibberish"
	filename, err = slamSvc.getAndSaveDataDense()
	test.That(t, err, test.ShouldBeError, errors.Errorf("open %v: no such file or directory", filename))

	_, err = os.Stat(filename)
	test.That(t, err, test.ShouldBeError, errors.Errorf("stat %v: no such file or directory", filename))

	slamSvc.dataDirectory = ddTemp
	camErr := &inject.Camera{}
	camErr.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pointcloud.New(), errors.New("camera data error")
	}

	slamSvc.camera = camErr
	filename, err = slamSvc.getAndSaveDataDense()
	test.That(t, err, test.ShouldBeError, errors.New("camera data error"))
	test.That(t, filename, test.ShouldBeEmpty)

	err = slamSvc.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	err = resetFolder(name)
	test.That(t, err, test.ShouldBeNil)
}

// Cartographer data process tests.
func TestDataProcessCartographer(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	attrCfg := &AttrConfig{
		Algorithm:        "cartographer",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
	}

	slamS, err := createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeNil)

	slamSvc := slamS.getSLAMServiceData()

	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pointcloud.New(), nil
	}

	slamSvc.camera = cam

	n := 5
	err = slamSvc.startDataProcess(context.Background())
	test.That(t, err, test.ShouldBeNil)

	time.Sleep(time.Millisecond * time.Duration((n)*(slamSvc.dataRateSec+5)))
	err = slamSvc.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	files, err := ioutil.ReadDir(slamSvc.dataDirectory + "/data/")
	test.That(t, len(files), test.ShouldEqual, n)
	test.That(t, err, test.ShouldBeNil)

	err = resetFolder(name)
	test.That(t, err, test.ShouldBeNil)
}

// OrbSLAMv3 Specific Tests (config).
func TestORBSLAMData(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	attrCfg := &AttrConfig{
		Algorithm:        "orbslamv3",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "mono"},
		DataDirectory:    name,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
	}

	slamS, err := createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeNil)

	slamSvc := slamS.getSLAMServiceData()
	cam := &inject.Camera{}

	cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return image.NewNRGBA(image.Rect(0, 0, 501024, 1024)), nil, nil
	}
	slamSvc.camera = cam

	cam = &inject.Camera{}
	cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, nil
	}
	slamSvc.camera = cam

	err = runtimeServiceValidation(&slamSvc)
	test.That(t, err, test.ShouldBeNil)

	// TODO: image with depth test

	slamSvc.cancelCtx = context.Background()

	slamSvc.Close(slamSvc.cancelCtx)
	test.That(t, err, test.ShouldBeNil)

	err = resetFolder(name)
	test.That(t, err, test.ShouldBeNil)
}

// GetAndSaveDataSparse Tests for image data.
func TestGetAndSaveDataORBSLAM(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	attrCfg := &AttrConfig{
		Algorithm:        "orbslamv3",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "rgbd"},
		DataDirectory:    name,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
	}

	slamS, err := createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeNil)

	slamSvc := slamS.getSLAMServiceData()

	err = slamSvc.startSLAMProcess(slamSvc.cancelCtx)
	test.That(t, err, test.ShouldBeNil)

	cam := &inject.Camera{}
	cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, nil
	}

	slamSvc.camera = cam

	_, err = slamSvc.getAndSaveDataSparse()
	test.That(t, err, test.ShouldBeError, errors.New("want image/both but don't have *rimage.ImageWithDepth"))

	slamSvc.slamMode = mono

	filename, err := slamSvc.getAndSaveDataSparse()
	test.That(t, err, test.ShouldBeNil)

	_, err = os.Stat(filename)
	test.That(t, err, test.ShouldBeNil)

	slamSvc.dataDirectory = "gibberish"
	filename, err = slamSvc.getAndSaveDataSparse()
	test.That(t, err, test.ShouldBeError, errors.Errorf("open %v: no such file or directory", filename))

	_, err = os.Stat(filename)
	test.That(t, err, test.ShouldBeError, errors.Errorf("stat %v: no such file or directory", filename))

	camErr := &inject.Camera{}
	camErr.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, errors.New("camera data error")
	}

	slamSvc.camera = camErr
	filename, err = slamSvc.getAndSaveDataSparse()
	test.That(t, err, test.ShouldBeError, errors.New("camera data error"))
	test.That(t, filename, test.ShouldBeEmpty)

	err = slamSvc.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	err = resetFolder(name)
	test.That(t, err, test.ShouldBeNil)
}

// ORBSLAM data process tests.s.
func TestDataProcessORBSLAM(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	attrCfg := &AttrConfig{
		Algorithm:        "orbslamv3",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "mono"},
		DataDirectory:    name,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
	}

	slamS, err := createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeNil)

	slamSvc := slamS.getSLAMServiceData()

	cam := &inject.Camera{}
	cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, nil
	}

	slamSvc.camera = cam

	n := 5
	err = slamSvc.startDataProcess(context.Background())
	test.That(t, err, test.ShouldBeNil)

	time.Sleep(time.Millisecond * time.Duration((n)*(slamSvc.dataRateSec+5)))
	err = slamSvc.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	files, err := ioutil.ReadDir(slamSvc.dataDirectory + "/data/")
	test.That(t, len(files), test.ShouldEqual, n)
	test.That(t, err, test.ShouldBeNil)

	err = resetFolder(name)
	test.That(t, err, test.ShouldBeNil)
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
