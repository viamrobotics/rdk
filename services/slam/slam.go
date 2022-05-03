// Package slam implements simultaneous localization and mapping
package slam

import (
	"context"
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

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterService(Subtype, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
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

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) error {
	if _, ok := slamLibraries[config.Algorithm]; !ok {
		return goutils.NewConfigValidationError(path, errors.New("algorithm specified not in implemented list"))
	}

	return nil
}

// RunTimeConfigValidation ensures all parts of the config are valid at runtime but will not close out server.
func runtimeConfigValidation(svcConfig *AttrConfig) error {
	slamLib, ok := slamLibraries[svcConfig.Algorithm]
	if !ok {
		return errors.Errorf("%v algorithm specified not in implemented list", svcConfig.Algorithm)
	}

	slamAlgoMetadata := slamLib.getMetadata()

	// TODO 04/28/2022: Do camera checks not based on name but camera model. Currently not possible to get model type from camera object
	// See: https://viam.atlassian.net/jira/software/c/projects/PRODUCT/boards/16?modal=detail&selectedIssue=PRODUCT-61
	// Check Sensor File Type
	if len(svcConfig.Sensors) != 0 {
		for _, sensor := range svcConfig.Sensors {
			if _, ok := slamAlgoMetadata.SlamType.SupportedCameras[sensor]; !ok {
				return errors.Errorf("%v is not one of the valid sensors for valid sensor for %v", sensor, svcConfig.Algorithm)
			}

			// Check mode and camera
			if svcConfig.ConfigParams["mode"] != "" {
				cameraSupportedModes := slamAlgoMetadata.SlamType.SupportedCameras[sensor]
				var result bool
				for _, supportedModes := range cameraSupportedModes {
					if supportedModes == svcConfig.ConfigParams["mode"] {
						result = true
					}
				}
				if !result {
					return errors.Errorf("specified mode (%v) is not supported for camera [%v]", svcConfig.ConfigParams["mode"], sensor)
				}
			}
		}
	}

	// Check Data Directory Architecture
	if _, err := os.Stat(svcConfig.DataDirectory); err != nil {
		return errors.Errorf("file directory [%v] could not be found", svcConfig.DataDirectory)
	}

	for _, subdirectoryName := range [3]string{"data", "map", "config"} {
		subdirectoryPath := filepath.Join(svcConfig.DataDirectory, subdirectoryName)
		if _, err := os.Stat(subdirectoryPath); os.IsNotExist(err) {
			return errors.Errorf("directory does not exist [%v]", subdirectoryPath)
		}
	}

	// Check Input File Pattern
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

		if X > Y {
			return errors.Errorf("second value in input file pattern must be larger than the first [%v]", svcConfig.InputFilePattern)
		}
	}

	// Check Slam Mode Compatibility with Slam Library
	if svcConfig.ConfigParams["mode"] != "" {
		result, ok := slamAlgoMetadata.SlamMode[svcConfig.ConfigParams["mode"]]
		if !ok {
			return errors.Errorf("invalid mode (%v) specified for algorithm [%v]", svcConfig.ConfigParams["mode"], svcConfig.Algorithm)
		}
		if !result {
			return errors.Errorf("specified mode (%v) is not supported for algorithm [%v]", svcConfig.ConfigParams["mode"], svcConfig.Algorithm)
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
	getSLAMServiceData() slamService
	startDataProcess(ctx context.Context) error
	startSLAMProcess(ctx context.Context) error
	Close(ctx context.Context) error
}

// SlamService is the structure of the slam service.
type slamService struct {
	camera   camera.Camera
	slamLib  SLAM
	slamMode string
	// algoProcess      pexec.ProcessConfig
	configParams     map[string]string
	dataDirectory    string
	inputFilePattern string

	port       int
	dataFreqHz float64

	cancelCtx               context.Context
	cancelFunc              func()
	logger                  golog.Logger
	activeBackgroundWorkers *sync.WaitGroup
}

func configureCamera(svcConfig *AttrConfig, r robot.Robot, logger golog.Logger) (camera.Camera, error) {
	var cam camera.Camera
	var err error
	if len(svcConfig.Sensors) > 0 {
		logger.Info("Running in live mode")
		cam, err = camera.FromRobot(r, svcConfig.Sensors[0])
		if err != nil {
			return nil, errors.Errorf("error with get camera for slam service: %q", err)
		}

		proj := camera.Projector(cam) // will be nil if no intrinsics
		if proj != nil {
			_, ok := proj.(*transform.PinholeCameraIntrinsics)
			if !ok {
				return nil, errors.New("error camera intrinsics were not defined properly")
			}
		}
	} else {
		logger.Info("Running in non-live mode")
		cam = nil
	}

	return cam, nil
}

// New returns a new slam service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	svcConfig, ok := config.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(svcConfig, config.ConvertedAttributes)
	}

	// Runtime Validation Check
	if err := RuntimeConfigValidation(svcConfig); err != nil {
		logger.Warnf("runtime slam config error: %v", err)
		return &slamService{}, nil
	}

	// ------------- Get Camera ------------
	cam, err := configureCamera(svcConfig, r, logger)
	if err != nil {
		return nil, err
	}

	// ---------- Get Random Port ----------
	p, err := goutils.TryReserveRandomPort()
	if err != nil {
		return nil, errors.Errorf("error trying to return a random port %v", err)
	}

	// -------- Get Data Frequency ---------
	freq := float64(1000.0 / svcConfig.DataRateMs)

	// ------- Get Cancel Protocols --------
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	// ------- Get SLAM Lib --------
	slamLib := slamLibraries[svcConfig.Algorithm]

	// ------- Get SLAM Mode --------
	slamMode := strings.ToLower(svcConfig.ConfigParams["mode"])

	// SLAM Service Object
	slamSvc := &slamService{
		camera:           cam,
		slamLib:          slamLib,
		slamMode:         slamMode,
		configParams:     svcConfig.ConfigParams,
		dataDirectory:    svcConfig.DataDirectory,
		inputFilePattern: svcConfig.InputFilePattern,
		port:             p,
		dataFreqHz:       freq,
		cancelCtx:        cancelCtx,
		cancelFunc:       cancelFunc,
		logger:           logger,
	}

	// Data Process
	if err := slamSvc.startDataProcess(ctx); err != nil {
		return nil, errors.Errorf("error with slam service data process: %q", err)
	}

	// SLAM Process
	if err := slamSvc.startSLAMProcess(ctx); err != nil {
		return nil, errors.Errorf("error with slam service slam process: %q", err)
	}

	return slamSvc, nil
}

// TBD 05/03/2022: Data processing loop in new PR (see slamlibarary.go GetandSaveData functions as well)
// startDataProcess is the main control loops for sending data from camera to the data directory for processing.
func (slamSvc *slamService) startDataProcess(ctx context.Context) error {
	if slamSvc.camera == nil {
		return nil
	}

	slamSvc.activeBackgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer slamSvc.activeBackgroundWorkers.Done()

		for {
			select {
			case <-slamSvc.cancelCtx.Done():
				return
			default:
			}

			timer := time.NewTimer(time.Second / time.Duration(slamSvc.dataFreqHz))
			select {
			case <-slamSvc.cancelCtx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}

			// Get data from desired camera
			if err := slamSvc.slamLib.getAndSaveData(slamSvc.cancelCtx, slamSvc.camera, slamSvc.slamMode,
				slamSvc.dataDirectory, slamSvc.logger); err != nil {
				panic(err)
			}
		}
	})

	return nil
}

// TODO 05/03/2022: Implement SLAM starting and stopping processes (see JIRA ticket:
// https://viam.atlassian.net/jira/software/c/projects/DATA/boards/30?modal=detail&selectedIssue=DATA-104)
// startSLAMProcess starts up the SLAM library process by calling the executable binary.
func (slamSvc *slamService) startSLAMProcess(ctx context.Context) error {
	return nil
}

// GetSLAMServiceData returns the SLAM Service implementation and associated data.
func (slamSvc *slamService) getSLAMServiceData() slamService {
	return *slamSvc
}

// TODO 05/03/2022: Implement closeout of slam service and subprocesses
// Close out of all slam related processes.
func (slamSvc *slamService) Close(ctx context.Context) error {
	return nil
}
