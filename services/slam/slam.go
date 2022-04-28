// Package slam implements simulatenous localization and mapping
package slam

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"
)

func init() {
	registry.RegisterService(Subtype, registry.Service{Constructor: New})
	cType := config.ServiceType(SubtypeName)

	config.RegisterServiceAttributeMapConverter(cType, func(attributes config.AttributeMap) (interface{}, error) {
		var conf AttrConfig
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &conf})
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(attributes); err != nil {
			return nil, err
		}
		return &conf, nil
	}, &AttrConfig{})
}

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

	if _, ok := slam_map[config.Algorithm]; !ok {
		return goutils.NewConfigValidationError(path, errors.New("algorithm specified not in implemented list"))
	}

	return nil
}

// Validate ensures all parts of the config are valid.
func RunTimeConfigValidation(svcConfig *AttrConfig) error {

	slam_algo_metadata, err := slam_map[svcConfig.Algorithm].GetMetadata()
	if err != nil {
		return errors.Errorf("retrieving metadata from config algorthim [%v]", svcConfig.Algorithm)
	}

	// TODO 04/28/2022: Do camera checks not based on name but camera model. Currently not possible to get model type from camera object
	// See: https://viam.atlassian.net/jira/software/c/projects/PRODUCT/boards/16?modal=detail&selectedIssue=PRODUCT-61
	// Check Sensor File Type
	if len(svcConfig.Sensors) != 0 {
		for _, sensor := range svcConfig.Sensors {
			if _, ok := slam_algo_metadata.SlamType.SupportedCameras[sensor]; !ok {
				return errors.Errorf("%v is not one of the valid sensors for valid sensor for %v", sensor, svcConfig.Algorithm)
			}

			// Check mode and camera
			if svcConfig.ConfigParams["mode"] != "" {
				cameraSupportedModes := slam_algo_metadata.SlamType.SupportedCameras[sensor]
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

	files, err := ioutil.ReadDir(svcConfig.DataDirectory)
	if err != nil {
		return errors.Errorf("checking files in provided data driectory [%v]", svcConfig.DataDirectory)
	}

	var configBool bool
	var mapBool bool
	var dataBool bool
	for _, f := range files {
		if f.Name() == "data" {
			dataBool = true
		}
		if f.Name() == "map" {
			mapBool = true
		}
		if f.Name() == "config" {
			configBool = true
		}
	}
	if !dataBool {
		return errors.Errorf("no data folder was found in [%v]", svcConfig.DataDirectory)
	}

	if !mapBool {
		return errors.Errorf("no map folder was found in [%v]", svcConfig.DataDirectory)
	}

	if !configBool {
		return errors.Errorf("no config folder was found in [%v]", svcConfig.DataDirectory)
	}

	// Check Input File Pattern
	if svcConfig.InputFilePattern != "" {
		re := regexp.MustCompile("(\\d+):(\\d+):(\\d+)")
		res := re.MatchString(svcConfig.InputFilePattern)
		if res {
			return errors.Errorf("input_file_pattern in config (%V) does not match the valid regex pattern (\\d+):(\\d+):(\\d+)", svcConfig.InputFilePattern)
		}

		re = regexp.MustCompile("(\\d+)")
		res2 := re.FindAllString(svcConfig.InputFilePattern, 3)
		X, _ := strconv.Atoi(res2[0])
		Y, _ := strconv.Atoi(res2[1])

		if X < Y {
			return errors.Errorf("second valuen in input file pattern must be larger than the first [%v]", svcConfig.InputFilePattern)
		}
	}

	// Check Slam Mode Compatability with Slam Library
	if svcConfig.ConfigParams["mode"] != "" {
		result, ok := slam_algo_metadata.SlamMode[svcConfig.ConfigParams["mode"]]
		if !ok {
			return errors.Errorf("invalid mode type (%v) specified in config params for algorithm [%v]", svcConfig.ConfigParams["mode"], svcConfig.Algorithm)
		}
		if !result {
			return errors.Errorf("specified mode (%v) is not supported for algorithm [%v]", svcConfig.ConfigParams["mode"], svcConfig.Algorithm)
		}
	}

	return nil
}

// Config describes how to configure the service.
type AttrConfig struct {
	Sensors          []string          `json:"sensors"`
	Algorithm        string            `json:"algorithm"`
	ConfigParams     map[string]string `json:"config_params"`
	DataRateMs       int               `json:"data_rate_ms"`
	MapRateSec       int               `json:"map_rate_sec"`
	DataDirectory    string            `json:"data_dir"`
	InputFilePattern string            `json:"input_file_pattern"`
}

// SlamService is the structure of the slam service
type slamService struct {
	camera      camera.Camera
	slamLib     SLAM
	slamMode    string
	algoProcess pexec.ProcessConfig
	//algoClient			SlamServiceClient
	configParams     map[string]string
	dataDirectory    string
	inputFilePattern string

	port       int
	dataFreqHz float64

	cancelCtx               context.Context
	cancelFunc              func()
	logger                  golog.Logger
	activeBackgroundWorkers sync.WaitGroup
}

// New returns a new slam service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (interface{}, error) {

	svcConfig, ok := config.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(svcConfig, config.ConvertedAttributes)
	}

	// Runtime Validation Check
	if err := RunTimeConfigValidation(svcConfig); err != nil {
		logger.Warnf("runtime slam config error: %v", err)
		return nil, nil
	}

	// ------------- Get Camera ------------
	var cam camera.Camera
	var err error
	if len(svcConfig.Sensors) > 0 {
		fmt.Println("Running in live mode")
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
		fmt.Println("Running in non-live mode")
		cam = nil
	}

	// ---------- Get Random Port ----------
	p, err := goutils.TryReserveRandomPort()

	// -------- Get Data Frequency ---------
	freq := float64(1000.0 / svcConfig.DataRateMs)

	// ------- Get Cancel Protocols --------
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	// ------- Get SLAM Lib --------
	slam_lib := slam_map[svcConfig.Algorithm]

	// ------- Get SLAM Mode --------
	slam_mode := strings.ToLower(svcConfig.ConfigParams["mode"])

	// SLAM Service Object
	slamSvc := &slamService{
		camera:   cam,
		slamLib:  slam_lib,
		slamMode: slam_mode,
		//algoProcess:      nil,
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

// Start is the main control loops for sending events from controller to base.
func (slamSvc *slamService) startDataProcess(ctx context.Context) error {
	if slamSvc.camera == nil {
		return nil
	}

	slamSvc.activeBackgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer slamSvc.activeBackgroundWorkers.Done()

		i := 0
		for {
			fmt.Println(i)
			select {
			case <-slamSvc.cancelCtx.Done():
				return
			default:
			}

			timer := time.NewTimer(1)
			select {
			case <-slamSvc.cancelCtx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}

			// Get data from desired camera
			if err := slamSvc.slamLib.GetAndSaveData(slamSvc.cancelCtx, slamSvc.camera, slamSvc.slamMode, slamSvc.dataDirectory, slamSvc.logger); err != nil {
				panic(err)
			}

		}
	})

	return nil
}

func (slamSvc *slamService) startSLAMProcess(ctx context.Context) error {
	return nil
}

// Close out of all slam related processes.
func (slamSvc *slamService) Close(ctx context.Context) error {
	return nil
}
