// Package slam implements simultaneous localization and mapping
package slam

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

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

// TBD 05/04/2022: Needs more work wonce GRPC is included (future PR).
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

// runtimeConfigValidation ensures all parts of the config are valid at runtime but will not close out server.
func runtimeConfigValidation(svcConfig *AttrConfig, logger golog.Logger) error {
	slamLib, ok := slamLibraries[svcConfig.Algorithm]
	if !ok {
		return errors.Errorf("%v algorithm specified not in implemented list", svcConfig.Algorithm)
	}

	// Check sensor and mode combination
	if svcConfig.ConfigParams["mode"] != "" {
		mode := svcConfig.ConfigParams["mode"]
		modeCheck, ok := slamLib.SlamMode[svcConfig.ConfigParams["mode"]]
		if !ok || !modeCheck {
			return errors.Errorf("getting data with specified algorithm, %v, and desired mode %v", svcConfig.Algorithm, mode)
		}
	}

	// Check Data Directory Architecture - create new one if issue accessing folder
	for _, directoryName := range [4]string{"", "data", "map", "config"} {
		directoryPath := filepath.Join(svcConfig.DataDirectory, directoryName)
		if _, err := os.Stat(directoryPath); os.IsNotExist(err) {
			logger.Warnf("%v directory does not exist", directoryPath)
			if err := os.Mkdir(directoryPath, os.ModePerm); err != nil {
				return errors.Errorf("issue creating directory at %v: %v", directoryPath, err)
			}
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
		result, ok := slamLib.SlamMode[svcConfig.ConfigParams["mode"]]
		if !ok {
			return errors.Errorf("invalid mode (%v) specified for algorithm [%v]", svcConfig.ConfigParams["mode"], svcConfig.Algorithm)
		}
		if !result {
			return errors.Errorf("specified mode (%v) is not supported for algorithm [%v]", svcConfig.ConfigParams["mode"], svcConfig.Algorithm)
		}
	}

	return nil
}

// runtimeServiceValidation ensures the service's data processing and saving is valid for the mode and cam given.
func runtimeServiceValidation(slamSvc *slamService) error {
	if slamSvc.camera != nil {
		var err error
		var path string

		// TODO 05/05/2022: This will be removed once GRPC data transfer is available as the responsibility for
		// calling the right algorithms (Next vs NextPointCloud) will be held by the slam libararies themselves
		// Note: if GRPC data transfer is delayed to after other algorithms (or user custom algos) are being
		// added this point will be revisited
		switch slamSvc.slamLib.AlgoName {
		case "orbslamv3":
			path, err = slamSvc.getAndSaveDataSparse()
		case "cartographer":
			path, err = slamSvc.getAndSaveDataDense()
		default:
			return errors.Errorf("invalid slam algorithm %v", slamSvc.slamLib.AlgoName)
		}
		if err != nil {
			return errors.Errorf("getting data with specified sensor and desired mode %v", slamSvc.slamMode)
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
	getSLAMServiceData() slamService
	startDataProcess(ctx context.Context) error
	startSLAMProcess(ctx context.Context) error
	Close(ctx context.Context) error
}

// SlamService is the structure of the slam service.
type slamService struct {
	camera           camera.Camera
	slamLib          metadata
	slamMode         string
	configParams     map[string]string
	dataDirectory    string
	inputFilePattern string

	port       int
	dataFreqHz float64

	cancelCtx  context.Context
	cancelFunc func()
	logger     golog.Logger
}

// configureCamera will check the config to see if a camera is desired and if so, grab the camera from
// the robot as well as get the intrinsic assocated with it.
func configureCamera(svcConfig *AttrConfig, r robot.Robot, logger golog.Logger) (camera.Camera, error) {
	var cam camera.Camera
	var err error
	if len(svcConfig.Sensors) > 0 {
		logger.Debug("Running in live mode")
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
		logger.Debug("Running in non-live mode")
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

	cam, err := configureCamera(svcConfig, r, logger)
	if err != nil {
		return nil, err
	}

	if err := runtimeConfigValidation(svcConfig, logger); err != nil {
		logger.Warnf("runtime slam config error: %v", err)
		return &slamService{}, nil
	}

	p, err := goutils.TryReserveRandomPort()
	if err != nil {
		return nil, errors.Errorf("error trying to return a random port %v", err)
	}

	cancelCtx, cancelFunc := context.WithCancel(ctx)

	// SLAM Service Object
	slamSvc := &slamService{
		camera:           cam,
		slamLib:          slamLibraries[svcConfig.Algorithm],
		slamMode:         strings.ToLower(svcConfig.ConfigParams["mode"]),
		configParams:     svcConfig.ConfigParams,
		dataDirectory:    svcConfig.DataDirectory,
		inputFilePattern: svcConfig.InputFilePattern,
		port:             p,
		dataFreqHz:       float64(1000.0 / svcConfig.DataRateMs),
		cancelCtx:        cancelCtx,
		cancelFunc:       cancelFunc,
		logger:           logger,
	}

	if err := runtimeServiceValidation(slamSvc); err != nil {
		logger.Warnf("runtime slam service error: %v", err)
		return &slamService{}, nil
	}

	if err := slamSvc.startDataProcess(ctx); err != nil {
		return nil, errors.Errorf("error with slam service data process: %q", err)
	}

	if err := slamSvc.startSLAMProcess(ctx); err != nil {
		return nil, errors.Errorf("error with slam service slam process: %q", err)
	}

	return slamSvc, nil
}

// TBD 05/03/2022: Data processing loop in new PR (see slamlibarary.go GetandSaveData functions as well).
// startDataProcess is the main control loops for sending data from camera to the data directory for processing.
func (slamSvc *slamService) startDataProcess(ctx context.Context) error {
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

// TODO 05/03/2022: Implement closeout of slam service and subprocesses.
// Close out of all slam related processes.
func (slamSvc *slamService) Close(ctx context.Context) error {
	return nil
}

// TODO 05/06/2022: Data processing loop in new PR (see slam.go startDataProcessing function)
// getAndSaveData implements the data extraction for sparse algos and saving to the specified directory.
// nolint:unparam
func (slamSvc *slamService) getAndSaveDataSparse() (string, error) {
	return filepath.Join(slamSvc.dataDirectory, "temp.txt"), nil
}

// TODO 05/03/2022: Data processing loop in new PR (see slam.go startDataProcessing function)
// getAndSaveData implements the data extraction for dense algos and saving to the specified directory.
// nolint:unparam
func (slamSvc *slamService) getAndSaveDataDense() (string, error) {
	return filepath.Join(slamSvc.dataDirectory, "temp.txt"), nil
}
