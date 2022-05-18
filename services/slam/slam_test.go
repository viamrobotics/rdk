package slam

import (
	"context"
	"fmt"
	"image"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	timePadding = 5
)

func createFakeLibs() []string {
	var newFakeLibs []string
	for _, s := range slamLibraries {
		slamLibraries["fake_"+s.AlgoName] = metadata{
			AlgoName:       s.AlgoName,
			AlgoType:       s.AlgoType,
			SlamMode:       s.SlamMode,
			BinaryLocation: "true",
		}
		newFakeLibs = append(newFakeLibs, "fake_"+s.AlgoName)
	}
	return newFakeLibs
}

func createSLAMService(t *testing.T, attrCfg *AttrConfig) (context.Context, []string, *slamService, error) {
	t.Helper()
	cfgService := config.Service{Name: "test", Type: "slam"}
	logger := golog.NewTestLogger(t)

	ctx := context.Background()

	r := &inject.Robot{}
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, error) {
		return nil, rdkutils.NewResourceNotFoundError(n)
	}

	newFakeLibs := createFakeLibs()

	cfgService.ConvertedAttributes = attrCfg

	svc, _ := New(ctx, r, cfgService, logger)

	if svc == nil {
		return nil, []string{}, nil, errors.New("error creating slam service")
	}

	slamSvc := svc.(*slamService)
	ctx = context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	slamSvc.cancelFunc = cancelFunc

	return cancelCtx, newFakeLibs, slamSvc, nil
}

func closeOutSLAMService(t *testing.T, slamSvc *slamService, name string, newFakeLibs []string) {
	t.Helper()

	slamSvc.Close()

	if name != "" {
		err := resetFolder(name)
		test.That(t, err, test.ShouldBeNil)
	}

	for _, s := range newFakeLibs {
		delete(slamLibraries, s)
	}
}

func TestGeneralSLAMService(t *testing.T) {
	ctx, _, _, err := createSLAMService(t, &AttrConfig{})
	test.That(t, err, test.ShouldBeError, errors.New("error creating slam service"))

	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	attrCfg := &AttrConfig{
		Algorithm:        "fake_cartographer",
		Sensors:          []string{"rplidar"},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name,
		InputFilePattern: "100:300:5",
	}

	_, _, slamSvc, err := createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeError, errors.New("error creating slam service"))
	test.That(t, slamSvc, test.ShouldBeNil)

	attrCfg = &AttrConfig{
		Algorithm:        "fake_cartographer",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
		MapRateSec:       5,
	}

	_, _, slamSvc, err = createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, slamSvc.dataRateMs, test.ShouldEqual, 100)
	test.That(t, slamSvc.mapRateSec, test.ShouldEqual, 5)

	attrCfg.DataRateMs = 0
	attrCfg.MapRateSec = 0
	_, newFakeLibs, slamSvc, err := createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, slamSvc.dataRateMs, test.ShouldEqual, 200)
	test.That(t, slamSvc.mapRateSec, test.ShouldEqual, 60)

	failMetadata := metadata{
		AlgoName: "test",
		AlgoType: slamLibrary(9),
	}

	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pointcloud.New(), nil
	}
	slamSvc.camera = cam
	slamSvc.slamLib = failMetadata

	err = runtimeServiceValidation(ctx, slamSvc)
	test.That(t, err, test.ShouldBeError, errors.Errorf("invalid slam algorithm %v", slamSvc.slamLib.AlgoName))

	closeOutSLAMService(t, slamSvc, name, newFakeLibs)
}

func TestConfigValidation(t *testing.T) {
	logger := golog.NewTestLogger(t)

	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pointcloud.New(), nil
	}

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

	t.Run("run test of config with no sensor", func(t *testing.T) {
		cfg.Sensors = []string{}
		err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeNil)
	})

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

		cfg.InputFilePattern = "1:15:0"
		err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError,
			errors.New("the file input pattern's interval must be greater than zero"))

		err = resetFolder(name1)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("SLAM config check on specified algorithm", func(t *testing.T) {
		cfg.Algorithm = "wrong_algo"
		err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError, errors.Errorf("%v algorithm specified not in implemented list", cfg.Algorithm))
	})
}

func TestCartographerData(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	attrCfg := &AttrConfig{
		Algorithm:        "fake_cartographer",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
	}

	cancelCtx, newFakeLibs, slamSvc, err := createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeNil)

	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pointcloud.New(), nil
	}
	slamSvc.camera = cam

	err = runtimeServiceValidation(cancelCtx, slamSvc)
	test.That(t, err, test.ShouldBeNil)

	_, err = slamSvc.getAndSaveDataDense(cancelCtx)
	test.That(t, err, test.ShouldBeNil)

	slamSvc.slamMode = mono
	err = runtimeServiceValidation(cancelCtx, slamSvc)
	errCheck := errors.Errorf("error getting data in desired mode: bad slamMode %v specified for this algorithm", slamSvc.slamMode)
	test.That(t, err, test.ShouldBeError, errCheck)

	closeOutSLAMService(t, slamSvc, name, newFakeLibs)
}

func TestGetAndSaveDataCartographer(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	attrCfg := &AttrConfig{
		Algorithm:        "fake_cartographer",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
	}

	cancelCtx, newFakeLibs, slamSvc, err := createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeNil)

	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pointcloud.New(), nil
	}

	slamSvc.camera = cam

	filename, err := slamSvc.getAndSaveDataDense(cancelCtx)
	test.That(t, err, test.ShouldBeNil)

	_, err = os.Stat(filename)
	test.That(t, err, test.ShouldBeNil)

	ddTemp := slamSvc.dataDirectory
	slamSvc.dataDirectory = "gibberish"
	filename, err = slamSvc.getAndSaveDataDense(cancelCtx)
	test.That(t, err, test.ShouldBeError, errors.Errorf("open %v: no such file or directory", filename))

	_, err = os.Stat(filename)
	test.That(t, err, test.ShouldBeError, errors.Errorf("stat %v: no such file or directory", filename))

	slamSvc.dataDirectory = ddTemp
	camErr := &inject.Camera{}
	camErr.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pointcloud.New(), errors.New("camera data error")
	}

	slamSvc.camera = camErr
	filename, err = slamSvc.getAndSaveDataDense(cancelCtx)
	test.That(t, err, test.ShouldBeError, errors.New("camera data error"))
	test.That(t, filename, test.ShouldBeEmpty)

	closeOutSLAMService(t, slamSvc, name, newFakeLibs)
}

func TestDataProcessCartographer(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	attrCfg := &AttrConfig{
		Algorithm:        "fake_cartographer",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
	}

	cancelCtx, newFakeLibs, slamSvc, err := createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeNil)

	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pointcloud.New(), nil
	}

	slamSvc.camera = cam

	n := 5
	slamSvc.startDataProcess(cancelCtx)

	// Note: timePadding is required to allow the sub processes to be fully completed during test
	time.Sleep(time.Millisecond * time.Duration((n)*(slamSvc.dataRateMs+timePadding)))
	slamSvc.Close()

	files, err := ioutil.ReadDir(slamSvc.dataDirectory + "/data/")
	test.That(t, len(files), test.ShouldEqual, n)
	test.That(t, err, test.ShouldBeNil)

	closeOutSLAMService(t, slamSvc, name, newFakeLibs)
}

func TestORBSLAMData(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	attrCfg := &AttrConfig{
		Algorithm:        "fake_orbslamv3",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "mono"},
		DataDirectory:    name,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
	}

	cancelCtx, newFakeLibs, slamSvc, err := createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeNil)

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

	err = runtimeServiceValidation(cancelCtx, slamSvc)
	test.That(t, err, test.ShouldBeNil)

	slamSvc.slamMode = twod
	err = runtimeServiceValidation(cancelCtx, slamSvc)
	errCheck := errors.Errorf("error getting data in desired mode: bad slamMode %v specified for this algorithm", slamSvc.slamMode)
	test.That(t, err, test.ShouldBeError, errCheck)

	closeOutSLAMService(t, slamSvc, name, newFakeLibs)
}

func TestGetAndSaveDataORBSLAM(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	attrCfg := &AttrConfig{
		Algorithm:        "fake_orbslamv3",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "rgbd"},
		DataDirectory:    name,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
	}

	cancelCtx, newFakeLibs, slamSvc, err := createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeNil)

	cam := &inject.Camera{}
	cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, nil
	}

	slamSvc.camera = cam

	_, err = slamSvc.getAndSaveDataSparse(cancelCtx)
	test.That(t, err, test.ShouldBeError, errors.New("want image/both but don't have *rimage.ImageWithDepth"))

	slamSvc.slamMode = mono

	filename, err := slamSvc.getAndSaveDataSparse(cancelCtx)
	test.That(t, err, test.ShouldBeNil)

	_, err = os.Stat(filename)
	test.That(t, err, test.ShouldBeNil)

	ddTemp := slamSvc.dataDirectory
	slamSvc.dataDirectory = "gibberish"
	filename, err = slamSvc.getAndSaveDataSparse(cancelCtx)
	test.That(t, err, test.ShouldBeError, errors.Errorf("open %v: no such file or directory", filename))

	_, err = os.Stat(filename)
	test.That(t, err, test.ShouldBeError, errors.Errorf("stat %v: no such file or directory", filename))

	camErr := &inject.Camera{}
	camErr.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, errors.New("camera data error")
	}

	slamSvc.camera = camErr
	filename, err = slamSvc.getAndSaveDataSparse(cancelCtx)
	test.That(t, err, test.ShouldBeError, errors.New("camera data error"))
	test.That(t, filename, test.ShouldBeEmpty)

	slamSvc.dataDirectory = ddTemp
	slamSvc.slamMode = rgbd
	cam = &inject.Camera{}
	cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		img, err := rimage.NewImageWithDepth(artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"), true)
		return img, nil, err
	}

	slamSvc.camera = cam

	filename, err = slamSvc.getAndSaveDataSparse(cancelCtx)
	test.That(t, err, test.ShouldBeNil)
	_, err = os.Stat(filename)
	test.That(t, err, test.ShouldBeNil)

	closeOutSLAMService(t, slamSvc, name, newFakeLibs)
}

func TestDataProcessORBSLAM(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	attrCfg := &AttrConfig{
		Algorithm:        "fake_orbslamv3",
		Sensors:          []string{},
		ConfigParams:     map[string]string{"mode": "mono"},
		DataDirectory:    name,
		InputFilePattern: "100:300:5",
		DataRateMs:       100,
	}

	cancelCtx, newFakeLibs, slamSvc, err := createSLAMService(t, attrCfg)
	test.That(t, err, test.ShouldBeNil)

	cam := &inject.Camera{}
	cam.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), nil, nil
	}

	slamSvc.camera = cam

	n := 5
	slamSvc.startDataProcess(cancelCtx)

	// Note: timePadding is required to allow the sub processes to be fully completed during test
	time.Sleep(time.Millisecond * time.Duration((n)*(slamSvc.dataRateMs+timePadding)))
	slamSvc.Close()

	files, err := ioutil.ReadDir(slamSvc.dataDirectory + "/data/")
	test.That(t, len(files), test.ShouldEqual, n)
	test.That(t, err, test.ShouldBeNil)

	closeOutSLAMService(t, slamSvc, name, newFakeLibs)
}

func TestSLAMProcess(t *testing.T) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	logger, obs := golog.NewObservedTestLogger(t)

	testMetadata := metadata{
		AlgoName:       "testLib",
		SlamMode:       map[string]mode{"mono": mono},
		BinaryLocation: "pwd",
	}

	slamSvc := &slamService{
		logger:                  logger,
		slamLib:                 testMetadata,
		slamProcess:             pexec.NewProcessManager(logger),
		cameraName:              "",
		configParams:            map[string]string{},
		dataRateMs:              0,
		mapRateSec:              0,
		dataDirectory:           "",
		inputFilePattern:        "",
		cancelFunc:              cancelFunc,
		activeBackgroundWorkers: &sync.WaitGroup{},
	}

	cmd, err := slamSvc.startSLAMProcess(cancelCtx)
	test.That(t, err, test.ShouldBeNil)

	// Check most recent log
	latestLoggedEntry := obs.All()[len(obs.All())-1]
	log := strings.TrimSuffix(latestLoggedEntry.Context[len(latestLoggedEntry.Context)-1].String, "\n")
	pwdResult, err := os.Getwd()
	test.That(t, err, test.ShouldBeNil)

	test.That(t, log, test.ShouldEqual, pwdResult)

	cmdResult := []string{
		testMetadata.BinaryLocation,
		"-sensors=" + slamSvc.cameraName,
		"-data_dir=" + slamSvc.dataDirectory,
		"-data_rate_ms=" + strconv.Itoa(slamSvc.dataRateMs),
		"-map_rate_sec=" + strconv.Itoa(slamSvc.mapRateSec),
		"-config_param=" + createKeyValuePairs(slamSvc.configParams),
		"-input_file_pattern=" + slamSvc.inputFilePattern,
	}

	for i, s := range cmd {
		test.That(t, s, test.ShouldEqual, cmdResult[i])
	}

	err = slamSvc.stopSLAMProcess()
	test.That(t, err, test.ShouldBeNil)

	testMetadataFail := metadata{
		AlgoName:       "testLib_fail",
		SlamMode:       map[string]mode{"mono": mono},
		BinaryLocation: "fail",
	}
	slamSvc.slamLib = testMetadataFail

	errCheck := fmt.Sprintf("\"%v\": executable file not found in $PATH", slamSvc.slamLib.BinaryLocation)
	_, err = slamSvc.startSLAMProcess(cancelCtx)
	test.That(t, err, test.ShouldBeError,
		errors.Errorf("problem adding slam process: error running process \"%v\": exec: %v", slamSvc.slamLib.BinaryLocation, errCheck))

	closeOutSLAMService(t, slamSvc, "", []string{})
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
