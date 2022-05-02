package slam

import (
	"context"
	"image"
	"io/ioutil"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
)

func TestGeneralSLAMService(t *testing.T) {
	cfgService := config.Service{Name: "test", Type: "slam"}
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	r := &inject.Robot{}
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, error) {
		return nil, rdkutils.NewResourceNotFoundError(n)
	}

	_, err := New(ctx, r, cfgService, logger)
	test.That(t, err, test.ShouldBeError, errors.Errorf("expected *slam.AttrConfig but got %v", cfgService.ConvertedAttributes))

	cfgService.ConvertedAttributes = &AttrConfig{}

	_, err = New(ctx, r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	name1, err := createFolderArchiecture("/tmp", true)
	attrCfg := &AttrConfig{
		Algorithm:        "cartographer",
		Sensors:          []string{"rplidar"},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name1,
		InputFilePattern: "100:300:5",
	}
	cfgService.ConvertedAttributes = attrCfg

	_, err = New(ctx, r, cfgService, logger)
	test.That(t, err, test.ShouldBeError,
		errors.Errorf("error with get camera for slam service: \"resource \\\"rdk:component:camera/%v\\\" not found\"", attrCfg.Sensors[0]))

	attrCfg = &AttrConfig{
		Algorithm:        "cartographer",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name1,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
	}
	cfgService.ConvertedAttributes = attrCfg

	slamSvc, err := New(ctx, r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	ss := slamSvc.GetSLAMServiceData()

	// cam := &fake.Camera{}
	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pointcloud.New(), nil
	}
	ss.camera = cam

	// err = ss.startDataProcess(ctx)
	// ss.cancelCtx.Done()
	// test.That(t, err, test.ShouldBeNil)
	err = resetFolder(name1)
	test.That(t, err, test.ShouldBeNil)
}

func TestConfigValidation(t *testing.T) {
	// Validate Tests
	cfg := &AttrConfig{Algorithm: "test_algo"}

	p := "path"
	err := cfg.Validate(p)
	test.That(t, err, test.ShouldBeError, errors.Errorf("error validating \"%v\": algorithm specified not in implemented list", p))

	cfg.Algorithm = "cartographer"
	err = cfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)

	// Runtime Validation Tests
	name1, err := createFolderArchiecture("/tmp", true)
	test.That(t, err, test.ShouldBeNil)

	cfg = &AttrConfig{
		Algorithm:        "cartographer",
		Sensors:          []string{"rplidar"},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name1,
		InputFilePattern: "100:300:5",
	}

	err = RunTimeConfigValidation(cfg)
	test.That(t, err, test.ShouldBeNil)

	// No sesnor test
	cfg.Sensors = []string{}
	err = RunTimeConfigValidation(cfg)
	test.That(t, err, test.ShouldBeNil)
	cfg.Sensors = []string{"rplidar"}

	// Mode SLAM Library test
	cfg.ConfigParams["mode"] = ""
	err = RunTimeConfigValidation(cfg)
	test.That(t, err, test.ShouldBeNil)

	testSlamType := slamType{
		SupportedCameras: map[string][]string{"test_sensor": {"test1", "test2"}},
		ModeFileType:     map[string]string{"2d": ".pcd", "3d": ".pcd"},
	}

	testMetadata := Metadata{
		AlgoName: "test",
		SlamType: testSlamType,
		SlamMode: map[string]bool{"test2": false},
	}

	slamLibraries["test"] = DenseSlamAlgo{Metadata: testMetadata}
	cfg.Algorithm = "test"
	cfg.Sensors = []string{"test_sensor"}
	cfg.ConfigParams["mode"] = "test1"
	err = RunTimeConfigValidation(cfg)
	test.That(t, err, test.ShouldBeError,
		errors.Errorf("invalid mode (%v) specified for algorithm [%v]", cfg.ConfigParams["mode"], cfg.Algorithm))

	cfg.ConfigParams["mode"] = "test2"
	err = RunTimeConfigValidation(cfg)
	test.That(t, err, test.ShouldBeError,
		errors.Errorf("specified mode (%v) is not supported for algorithm [%v]", cfg.ConfigParams["mode"], cfg.Algorithm))

	cfg.Algorithm = "cartographer"
	cfg.Sensors = []string{"rplidar"}
	cfg.ConfigParams["mode"] = "2d"

	delete(slamLibraries, "test")

	// Input File Pattern test
	cfg.InputFilePattern = "dd:300:3"
	err = RunTimeConfigValidation(cfg)
	test.That(t, err, test.ShouldBeError,
		errors.Errorf("input_file_pattern (%v) does not match the regex pattern (\\d+):(\\d+):(\\d+)", cfg.InputFilePattern))

	cfg.InputFilePattern = "500:300:3"
	err = RunTimeConfigValidation(cfg)
	test.That(t, err, test.ShouldBeError,
		errors.Errorf("second value in input file pattern must be larger than the first [%v]", cfg.InputFilePattern))

	// Directory test
	name2, err := createFolderArchiecture("/tmp", false)
	test.That(t, err, test.ShouldBeNil)

	cfg.DataDirectory = name2

	err = RunTimeConfigValidation(cfg)
	test.That(t, err, test.ShouldBeError, errors.Errorf("no data folder was found in [%v]", cfg.DataDirectory))

	// ---- Note: Test os.Stat / ioutil.ReadDir
	err = os.Mkdir(name2+"/data", os.ModePerm)
	test.That(t, err, test.ShouldBeNil)

	err = RunTimeConfigValidation(cfg)
	test.That(t, err, test.ShouldBeError, errors.Errorf("no map folder was found in [%v]", cfg.DataDirectory))

	err = os.Mkdir(name2+"/map", os.ModePerm)
	test.That(t, err, test.ShouldBeNil)

	err = RunTimeConfigValidation(cfg)
	test.That(t, err, test.ShouldBeError, errors.Errorf("no config folder was found in [%v]", cfg.DataDirectory))

	// Sensor Mode test
	cfg.ConfigParams["mode"] = "rgbd"
	err = RunTimeConfigValidation(cfg)
	test.That(t, err, test.ShouldBeError,
		errors.Errorf("specified mode (%v) is not supported for camera [%v]", cfg.ConfigParams["mode"], cfg.Sensors[0]))

	// Sensor test
	cfg.Sensors = []string{"intelrealsense"}
	err = RunTimeConfigValidation(cfg)
	test.That(t, err, test.ShouldBeError,
		errors.Errorf("%v is not one of the valid sensors for valid sensor for %v", cfg.Sensors[0], cfg.Algorithm))

	// Valid Algo
	cfg.Algorithm = "wrong_algo"
	err = RunTimeConfigValidation(cfg)
	test.That(t, err, test.ShouldBeError, errors.Errorf("%v algorithm specified not in implemented list", cfg.Algorithm))

	err = resetFolder(name1)
	test.That(t, err, test.ShouldBeNil)

	err = resetFolder(name2)
	test.That(t, err, test.ShouldBeNil)
}

func TestCartographerData(t *testing.T) {
	cfgService := config.Service{Name: "test", Type: "slam"}

	name, err := createFolderArchiecture("/tmp", true)
	test.That(t, err, test.ShouldBeNil)

	attrCfg := &AttrConfig{
		Algorithm:        "cartographer",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
	}
	cfgService.ConvertedAttributes = attrCfg

	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	r := &inject.Robot{}
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, error) {
		return nil, rdkutils.NewResourceNotFoundError(n)
	}

	slamSvc, err := New(ctx, r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	ss := slamSvc.GetSLAMServiceData()
	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pointcloud.New(), nil
	}
	ss.camera = cam

	_ = ss.slamLib.GetAndSaveData(ss.cancelCtx, ss.camera, ss.slamMode, ss.dataDirectory, ss.logger)
	test.That(t, err, test.ShouldBeNil)

	err = resetFolder(name)
	test.That(t, err, test.ShouldBeNil)
}

func TestOrbSLAMData(t *testing.T) {
	cfgService := config.Service{Name: "test", Type: "slam"}

	name, err := createFolderArchiecture("/tmp", true)
	test.That(t, err, test.ShouldBeNil)

	attrCfg := &AttrConfig{
		Algorithm:        "orbslamv3",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "mono"},
		DataDirectory:    name,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
	}
	cfgService.ConvertedAttributes = attrCfg

	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	r := &inject.Robot{}
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, error) {
		return nil, rdkutils.NewResourceNotFoundError(n)
	}

	slamSvc, err := New(ctx, r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	ss := slamSvc.GetSLAMServiceData()

	cam := &inject.Camera{}
	cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return image.NewNRGBA(image.Rect(0, 0, 501024, 1024)), nil, nil
	}
	ss.camera = cam

	err = ss.slamLib.GetAndSaveData(ss.cancelCtx, ss.camera, ss.slamMode, ss.dataDirectory, ss.logger)
	test.That(t, err, test.ShouldBeError, errors.New("jpeg: image is too large to encode"))

	cam = &inject.Camera{}
	cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, nil
	}
	ss.camera = cam

	err = ss.slamLib.GetAndSaveData(ss.cancelCtx, ss.camera, ss.slamMode, ss.dataDirectory, ss.logger)
	test.That(t, err, test.ShouldBeNil)

	err = ss.slamLib.GetAndSaveData(ss.cancelCtx, ss.camera, "rgbd", ss.dataDirectory, ss.logger)
	test.That(t, err, test.ShouldBeError, errors.New("want image/both but don't have *rimage.ImageWithDepth"))

	err = resetFolder(name)
	test.That(t, err, test.ShouldBeNil)
	// TODO: image with depth test
}

func createFolderArchiecture(path string, validArch bool) (string, error) {
	// if err := os.Mkdir(path, os.ModePerm); err != nil {
	// 	return err
	// }

	name, err := ioutil.TempDir(path, "*")
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
