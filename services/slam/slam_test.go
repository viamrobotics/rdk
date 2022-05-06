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

	name1, err := createTempFolderArchiecture(true)
	test.That(t, err, test.ShouldBeNil)

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

	ss := slamSvc.getSLAMServiceData()

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
	logger := golog.NewTestLogger(t)
	cfg := &AttrConfig{Algorithm: "test_algo"}

	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pointcloud.New(), nil
	}

	p := "path"
	err := cfg.Validate(p)
	test.That(t, err, test.ShouldBeError, errors.Errorf("error validating \"%v\": algorithm specified not in implemented list", p))

	cfg.Algorithm = "cartographer"
	err = cfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)

	// Runtime Validation Tests
	name1, err := createTempFolderArchiecture(true)
	test.That(t, err, test.ShouldBeNil)

	cfg = &AttrConfig{
		Algorithm:        "cartographer",
		Sensors:          []string{"rplidar"},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name1,
		InputFilePattern: "100:300:5",
	}

	err = runtimeConfigValidation(cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	// No sesnor test
	cfg.Sensors = []string{}
	err = runtimeConfigValidation(cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	cfg.Sensors = []string{"rplidar"}

	// Mode SLAM Library test
	cfg.ConfigParams["mode"] = ""
	err = runtimeConfigValidation(cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	testMetadata := metadata{
		AlgoName: "test",
		SLAMType: denseSLAM,
		SlamMode: map[string]bool{"test2": false},
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

	// Input File Pattern test
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

	// Sensor Mode test
	cfg.ConfigParams["mode"] = "rgbd"
	err = runtimeConfigValidation(cfg, logger)
	test.That(t, err, test.ShouldBeError,
		errors.Errorf("getting data with specified algorithm, %v, and desired mode %v", cfg.Algorithm, cfg.ConfigParams["mode"]))

	// TODO: sensor and saving data checks once GetAndSaveData is implemented

	// Valid Algo
	cfg.Algorithm = "wrong_algo"
	err = runtimeConfigValidation(cfg, logger)
	test.That(t, err, test.ShouldBeError, errors.Errorf("%v algorithm specified not in implemented list", cfg.Algorithm))
}

func TestCartographerData(t *testing.T) {
	cfgService := config.Service{Name: "test", Type: "slam"}

	name, err := createTempFolderArchiecture(true)
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

	ss := slamSvc.getSLAMServiceData()
	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pointcloud.New(), nil
	}
	ss.camera = cam

	err = runtimeServiceValidation(&ss)
	test.That(t, err, test.ShouldBeNil)

	_, _ = ss.getAndSaveDataDense()
	test.That(t, err, test.ShouldBeNil)

	ss.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = resetFolder(name)
	test.That(t, err, test.ShouldBeNil)
}

func TestOrbSLAMData(t *testing.T) {
	cfgService := config.Service{Name: "test", Type: "slam"}

	name, err := createTempFolderArchiecture(true)
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

	ss := slamSvc.getSLAMServiceData()

	cam := &inject.Camera{}
	cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return image.NewNRGBA(image.Rect(0, 0, 501024, 1024)), nil, nil
	}
	ss.camera = cam

	cam = &inject.Camera{}
	cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, nil
	}
	ss.camera = cam

	err = runtimeServiceValidation(&ss)
	test.That(t, err, test.ShouldBeNil)

	ss.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = resetFolder(name)
	test.That(t, err, test.ShouldBeNil)
	// TODO: image with depth test
}

// nolint:unparam
func createTempFolderArchiecture(validArch bool) (string, error) {
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
