// Package slam implements simultaneous localization and mapping
package slam

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

const (
	defaultDataRateMs = 200
	defaultMapRateSec = 60
)

// TBD 05/04/2022: Needs more work once GRPC is included (future PR).
func init() {
	registry.RegisterService(Subtype, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			svc, err := New(ctx, r, c, logger)
			if err != nil {
				logger.Warn(err)
			}
			return svc, nil
		},
	})
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("slam")

// Subtype is a constant that identifies the slam resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Name is the slam service's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

// runtimeConfigValidation ensures all parts of the config are valid at runtime but will not close out server. These primarily focus
// on parameters not in the config_param, however, the slam mode is required and thus is looked for.
func runtimeConfigValidation(svcConfig *AttrConfig, logger golog.Logger) error {
	slamLib, ok := slamLibraries[svcConfig.Algorithm]
	if !ok {
		return errors.Errorf("%v algorithm specified not in implemented list", svcConfig.Algorithm)
	}

	if svcConfig.ConfigParams["mode"] != "" {
		if _, ok := slamLib.SlamMode[svcConfig.ConfigParams["mode"]]; !ok {
			return errors.Errorf("getting data with specified algorithm, %v, and desired mode %v",
				svcConfig.Algorithm, svcConfig.ConfigParams["mode"])
		}
	}

	for _, directoryName := range [4]string{"", "data", "map", "config"} {
		directoryPath := filepath.Join(svcConfig.DataDirectory, directoryName)
		if _, err := os.Stat(directoryPath); os.IsNotExist(err) {
			logger.Warnf("%v directory does not exist", directoryPath)
			if err := os.Mkdir(directoryPath, os.ModePerm); err != nil {
				return errors.Errorf("issue creating directory at %v: %v", directoryPath, err)
			}
		}
	}

	// Confirms that input file pattern abides by the format n1:n2:n3 where n1, n2 and n3 are all positive integers and n1 <= n2
	// and n3 must be non-zero
	if svcConfig.InputFilePattern != "" {
		pattern := `(\d+):(\d+):(\d+)`
		re := regexp.MustCompile(pattern)
		res := re.MatchString(svcConfig.InputFilePattern)
		if !res {
			return errors.Errorf("input_file_pattern (%v) does not match the regex pattern %v", svcConfig.InputFilePattern, pattern)
		}

		re = regexp.MustCompile(`(\d+)`)
		res2 := re.FindAllString(svcConfig.InputFilePattern, 3)
		X, err := strconv.Atoi(res2[0])
		if err != nil {
			return err
		}
		Y, err := strconv.Atoi(res2[1])
		if err != nil {
			return err
		}

		D, err := strconv.Atoi(res2[2])
		if err != nil {
			return err
		}

		if D == 0 {
			return errors.New("the file input pattern's interval must be greater than zero")
		}

		if X > Y {
			return errors.Errorf("second value in input file pattern must be larger than the first [%v]", svcConfig.InputFilePattern)
		}
	}

	if svcConfig.ConfigParams["mode"] != "" {
		_, ok := slamLib.SlamMode[svcConfig.ConfigParams["mode"]]
		if !ok {
			return errors.Errorf("invalid mode (%v) specified for algorithm [%v]", svcConfig.ConfigParams["mode"], svcConfig.Algorithm)
		}
	}

	return nil
}

// runtimeServiceValidation ensures the service's data processing and saving is valid for the mode and cam given.
func runtimeServiceValidation(ctx context.Context, slamSvc *slamService) error {
	if slamSvc.camera != nil {
		var err error
		var path string

		// TODO 05/05/2022: This will be removed once GRPC data transfer is available as the responsibility for
		// calling the right algorithms (Next vs NextPointCloud) will be held by the slam libraries themselves
		// Note: if GRPC data transfer is delayed to after other algorithms (or user custom algos) are being
		// added this point will be revisited
		switch slamSvc.slamLib.AlgoType {
		case sparse:
			path, err = slamSvc.getAndSaveDataSparse(ctx)
		case dense:
			path, err = slamSvc.getAndSaveDataDense(ctx)
		default:
			return errors.Errorf("invalid slam algorithm %v", slamSvc.slamLib.AlgoName)
		}
		if err != nil {
			return errors.Errorf("error getting data in desired mode: %v", err)
		}
		err = os.RemoveAll(path)
		if err != nil {
			return errors.New("removing generated file during validation")
		}
	}
	return nil
}

// AttrConfig describes how to configure the service.
type AttrConfig struct {
	Sensors          []string          `json:"sensors"`
	Algorithm        string            `json:"algorithm"`
	ConfigParams     map[string]string `json:"config_params"`
	DataRateMs       int               `json:"data_rate_ms"`
	MapRateSec       int               `json:"map_rate_sec"`
	DataDirectory    string            `json:"data_dir"`
	InputFilePattern string            `json:"input_file_pattern"`
}

// Service describes the functions that are available to the service.
type Service interface {
	startDataProcess(ctx context.Context)
	startSLAMProcess(ctx context.Context) ([]string, error)
	stopSLAMProcess() error
	Close()
}

// SlamService is the structure of the slam service.
type slamService struct {
	cameraName       string
	camera           camera.Camera
	slamLib          metadata
	slamMode         mode
	slamProcess      pexec.ProcessManager
	configParams     map[string]string
	dataDirectory    string
	inputFilePattern string

	port       int
	dataRateMs int
	mapRateSec int

	cancelFunc              func()
	logger                  golog.Logger
	activeBackgroundWorkers *sync.WaitGroup
}

// configureCamera will check the config to see if a camera is desired and if so, grab the camera from
// the robot as well as get the intrinsic associated with it.
func configureCamera(svcConfig *AttrConfig, r robot.Robot, logger golog.Logger) (string, camera.Camera, error) {
	var cam camera.Camera
	var cameraName string
	var err error
	if len(svcConfig.Sensors) > 0 {
		logger.Debug("Running in live mode")
		cameraName := svcConfig.Sensors[0]
		cam, err = camera.FromRobot(r, cameraName)
		if err != nil {
			return "", nil, errors.Errorf("error getting camera for slam service: %q", err)
		}

		proj := camera.Projector(cam) // will be nil if no intrinsics
		if proj != nil {
			_, ok := proj.(*transform.PinholeCameraIntrinsics)
			if !ok {
				return "", nil, errors.New("error camera intrinsics were not defined properly")
			}
		}
	} else {
		logger.Debug("Running in non-live mode")
		cameraName = ""
		cam = nil
	}

	return cameraName, cam, nil
}

// New returns a new slam service for the given robot. Will not error out as to prevent server shutdown.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	svcConfig, ok := config.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(svcConfig, config.ConvertedAttributes)
	}

	cameraName, cam, err := configureCamera(svcConfig, r, logger)
	if err != nil {
		return nil, errors.Errorf("configuring camera error: %v", err)
	}

	if err := runtimeConfigValidation(svcConfig, logger); err != nil {
		return nil, errors.Errorf("runtime slam config error: %v", err)
	}

	p, err := goutils.TryReserveRandomPort()
	if err != nil {
		return nil, errors.Errorf("error trying to return a random port: %v", err)
	}

	slamLib := slamLibraries[svcConfig.Algorithm]
	slamModeName := strings.ToLower(svcConfig.ConfigParams["mode"])
	slamMode := slamLib.SlamMode[slamModeName]

	var dataRate int
	if svcConfig.DataRateMs == 0 {
		dataRate = defaultDataRateMs
	} else {
		dataRate = svcConfig.DataRateMs
	}

	var mapRate int
	if svcConfig.MapRateSec == 0 {
		mapRate = defaultMapRateSec
	} else {
		mapRate = svcConfig.MapRateSec
	}

	cancelCtx, cancelFunc := context.WithCancel(ctx)

	// SLAM Service Object
	slamSvc := &slamService{
		cameraName:              cameraName,
		camera:                  cam,
		slamLib:                 slamLibraries[svcConfig.Algorithm],
		slamMode:                slamMode,
		slamProcess:             pexec.NewProcessManager(logger),
		configParams:            svcConfig.ConfigParams,
		dataDirectory:           svcConfig.DataDirectory,
		inputFilePattern:        svcConfig.InputFilePattern,
		port:                    p,
		dataRateMs:              dataRate,
		mapRateSec:              mapRate,
		cancelFunc:              cancelFunc,
		logger:                  logger,
		activeBackgroundWorkers: &sync.WaitGroup{},
	}

	if err := runtimeServiceValidation(cancelCtx, slamSvc); err != nil {
		slamSvc.Close()
		return nil, errors.Errorf("runtime slam service error: %v", err)
	}

	slamSvc.startDataProcess(cancelCtx)

	if _, err := slamSvc.startSLAMProcess(ctx); err != nil {
		// logger.Warnw("error with slam service slam process", "error", err)
		slamSvc.Close()
		return nil, errors.Errorf("error with slam service slam process: %v", err)
	}

	return slamSvc, nil
}

// Close out of all slam related processes.
func (slamSvc *slamService) Close() {
	slamSvc.cancelFunc()
	if err := slamSvc.stopSLAMProcess(); err != nil {
		slamSvc.logger.Warnw("error occurred during closeout of process", "error", err)
	}
	slamSvc.activeBackgroundWorkers.Wait()
}

// TODO 05/10/2022: Remove from SLAM service once GRPC data transfer is available.
// startDataProcess is the background control loop for sending data from camera to the data directory for processing.
func (slamSvc *slamService) startDataProcess(cancelCtx context.Context) {
	if slamSvc.camera == nil {
		return
	}

	slamSvc.activeBackgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		ticker := time.NewTicker(time.Millisecond * time.Duration(slamSvc.dataRateMs))
		defer ticker.Stop()
		defer slamSvc.activeBackgroundWorkers.Done()

		dataWorker := &sync.WaitGroup{}

		for {
			if err := cancelCtx.Err(); err != nil {
				if !errors.Is(err, context.Canceled) {
					slamSvc.logger.Errorw("unexpected error in SLAM data process", "error", err)
				}
				dataWorker.Wait()
				return
			}

			select {
			case <-cancelCtx.Done():
				dataWorker.Wait()
				return
			case <-ticker.C:
				dataWorker.Add(1)

				// Split off go routine to handle data processing without affecting timing
				goutils.PanicCapturingGo(func() {
					defer dataWorker.Done()
					switch slamSvc.slamLib.AlgoType {
					case dense:
						if _, err := slamSvc.getAndSaveDataDense(cancelCtx); err != nil {
							slamSvc.logger.Warn(err)
						}
					case sparse:
						if _, err := slamSvc.getAndSaveDataSparse(cancelCtx); err != nil {
							slamSvc.logger.Warn(err)
						}
					default:
						slamSvc.logger.Warn("warning invalid algorithm specified")
					}
				})
			}
		}
	})
}

// startSLAMProcess starts up the SLAM library process by calling the executable binary and giving it the necessary arguments.
func (slamSvc *slamService) startSLAMProcess(ctx context.Context) ([]string, error) {
	var args []string

	if slamSvc.cameraName != "" {
		args = append(args, "-sensors="+slamSvc.cameraName)
	}
	if len(slamSvc.configParams) != 0 {
		args = append(args, "-config_param="+createKeyValuePairs(slamSvc.configParams))
	}
	if slamSvc.dataRateMs != 0 {
		args = append(args, "-data_rate_ms="+strconv.Itoa(slamSvc.dataRateMs))
	}
	if slamSvc.dataRateMs != 0 {
		args = append(args, "-map_rate_sec="+strconv.Itoa(slamSvc.mapRateSec))
	}
	if slamSvc.dataDirectory != "" {
		args = append(args, "-data_dir="+slamSvc.dataDirectory)
	}
	if slamSvc.inputFilePattern != "" {
		args = append(args, "-input_file_pattern="+slamSvc.inputFilePattern)
	}

	processCfg := pexec.ProcessConfig{
		ID:      "slam_" + slamSvc.slamLib.AlgoName,
		Name:    slamSvc.slamLib.BinaryLocation,
		Args:    args,
		Log:     true,
		OneShot: true,
	}

	_, err := slamSvc.slamProcess.AddProcessFromConfig(ctx, processCfg)
	if err != nil {
		return []string{}, errors.Errorf("problem adding slam process: %v", err)
	}

	slamSvc.logger.Info("starting slam process")

	if err = slamSvc.slamProcess.Start(ctx); err != nil {
		return []string{}, errors.Errorf("problem starting slam process: %v", err)
	}

	cmd := append([]string{processCfg.Name}, processCfg.Args...)

	return cmd, nil
}

// stopSLAMProcess uses the process manager to stop the created slam process from running.
func (slamSvc *slamService) stopSLAMProcess() error {
	if err := slamSvc.slamProcess.Stop(); err != nil {
		return errors.Errorf("problem stopping slam process: %v", err)
	}
	return nil
}

// getAndSaveDataSparse implements the data extraction for sparse algos and saving to the directory path (data subfolder) specified in
// the config. It returns the full filepath for each file saved along with any error associated with the data creation or saving.
func (slamSvc *slamService) getAndSaveDataSparse(ctx context.Context) (string, error) {
	img, _, err := slamSvc.camera.Next(ctx)
	if err != nil {
		if err.Error() == "bad scan: OpTimeout" {
			slamSvc.logger.Warnw("Skipping this scan due to error", "error", err)
			return "", nil
		}
		return "", err
	}

	var fileType string
	switch slamSvc.slamMode {
	case mono:
		fileType = ".jpeg"
	case rgbd:
		// TODO 05/12/2022: Soon wil be deprecated into pointcloud files or rgb and monochromatic depth file. We will want picture pair.
		fileType = ".both"
	case twod, threed:
		return "", errors.Errorf("bad slamMode %v specified for this algorithm", slamSvc.slamMode)
	}

	filename := createTimestampFilename(slamSvc.cameraName, slamSvc.dataDirectory, fileType)
	f, err := os.Create(filename)
	if err != nil {
		return filename, err
	}

	w := bufio.NewWriter(f)

	switch slamSvc.slamMode {
	case mono:
		if err := jpeg.Encode(w, img, nil); err != nil {
			return filename, err
		}
	case rgbd:
		// TODO 05/10/2022: the file type saving may change here based on John N.'s recommendation (whether to use poitntcloud or two images).
		// Both file types soon will be deprecated.
		// https://docs.google.com/document/d/1Fa8DY-a2dPhoGNLaUlsEgQ28kbgVexaacBtJrkwnwQQ/edit#heading=h.rhjz058xy3j5
		iwd, ok := img.(*rimage.ImageWithDepth)
		if !ok {
			return filename, errors.Errorf("want %s but don't have %T", utils.MimeTypeBoth, iwd)
		}
		if err := rimage.EncodeBoth(iwd, w); err != nil {
			return filename, err
		}
	case twod, threed:
		return "", errors.Errorf("bad slamMode %v specified for this algorithm", slamSvc.slamMode)
	}
	if err = w.Flush(); err != nil {
		return filename, err
	}
	return filename, f.Close()
}

// getAndSaveDataDense implements the data extraction for dense algos and saving to the directory path (data subfolder) specified in
// the config. It returns the full filepath for each file saved along with any error associated with the data creation or saving.
func (slamSvc *slamService) getAndSaveDataDense(ctx context.Context) (string, error) {
	pointcloud, err := slamSvc.camera.NextPointCloud(ctx)
	if err != nil {
		if err.Error() == "bad scan: OpTimeout" {
			slamSvc.logger.Warnw("Skipping this scan due to error", "error", err)
			return "", nil
		}
		return "", err
	}

	var fileType string
	switch slamSvc.slamMode {
	case twod, threed:
		fileType = ".pcd"
	case rgbd, mono:
		return "", errors.Errorf("bad slamMode %v specified for this algorithm", slamSvc.slamMode)
	}
	filename := createTimestampFilename(slamSvc.cameraName, slamSvc.dataDirectory, fileType)
	f, err := os.Create(filename)
	if err != nil {
		return filename, err
	}

	w := bufio.NewWriter(f)

	if err = pc.ToPCD(pointcloud, w, 1); err != nil {
		return filename, err
	}
	if err = w.Flush(); err != nil {
		return filename, err
	}
	return filename, f.Close()
}

// Creates a file for camera data with sensor and timestamp included in name.
func createTimestampFilename(cameraName, dataDirectory, fileType string) string {
	timeStamp := time.Now()
	filename := filepath.Join(dataDirectory, "data", cameraName+"_data_"+timeStamp.UTC().Format("2006-01-02T15_04_05.0000")+fileType)

	return filename
}

// Converts a dictionary to a string for so that it can be loaded into an arg for the slam process.
func createKeyValuePairs(m map[string]string) string {
	b := new(bytes.Buffer)
	for key, value := range m {
		fmt.Fprintf(b, "%s=%s,", key, value)
	}

	s := "{" + b.String() + "}"
	return s
}
