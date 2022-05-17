// Package slam implements simultaneous localization and mapping
package slam

import (
	"bufio"
	"context"
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
			return New(ctx, r, c, logger), nil
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
		if _, ok := slamLib.SlamMode[svcConfig.ConfigParams["mode"]]; !ok {
			return errors.Errorf("getting data with specified algorithm, %v, and desired mode %v",
				svcConfig.Algorithm, svcConfig.ConfigParams["mode"])
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
		_, ok := slamLib.SlamMode[svcConfig.ConfigParams["mode"]]
		if !ok {
			return errors.Errorf("invalid mode (%v) specified for algorithm [%v]", svcConfig.ConfigParams["mode"], svcConfig.Algorithm)
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
		// calling the right algorithms (Next vs NextPointCloud) will be held by the slam libraries themselves
		// Note: if GRPC data transfer is delayed to after other algorithms (or user custom algos) are being
		// added this point will be revisited
		switch slamSvc.slamLib.AlgoType {
		case sparse:
			path, err = slamSvc.getAndSaveDataSparse()
		case dense:
			path, err = slamSvc.getAndSaveDataDense()
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
	startDataProcess()
	startSLAMProcess(ctx context.Context) error
	Close()
}

// SlamService is the structure of the slam service.
type slamService struct {
	cameraName       string
	camera           camera.Camera
	slamLib          metadata
	slamMode         mode
	configParams     map[string]string
	dataDirectory    string
	inputFilePattern string

	port       int
	dataRateMs int
	mapRateSec int

	cancelCtx               context.Context
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
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) Service {
	svcConfig, ok := config.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil
	}

	cameraName, cam, err := configureCamera(svcConfig, r, logger)
	if err != nil {
		logger.Warnw("configuring camera error", "error", err)
		return nil
	}

	if err := runtimeConfigValidation(svcConfig, logger); err != nil {
		logger.Warnw("runtime slam config error", "error", err)
		return nil
	}

	p, err := goutils.TryReserveRandomPort()
	if err != nil {
		logger.Warnw("error trying to return a random port", "error", err)
		return nil
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
		configParams:            svcConfig.ConfigParams,
		dataDirectory:           svcConfig.DataDirectory,
		inputFilePattern:        svcConfig.InputFilePattern,
		port:                    p,
		dataRateMs:              dataRate,
		mapRateSec:              mapRate,
		cancelCtx:               cancelCtx,
		cancelFunc:              cancelFunc,
		logger:                  logger,
		activeBackgroundWorkers: &sync.WaitGroup{},
	}

	if err := runtimeServiceValidation(slamSvc); err != nil {
		logger.Warnw("runtime slam service error", "error", err)
		slamSvc.Close()
		return nil
	}

	slamSvc.startDataProcess()

	if err := slamSvc.startSLAMProcess(ctx); err != nil {
		logger.Warnw("error with slam service slam process", "error", err)
		slamSvc.Close()
		return nil
	}

	return slamSvc
}

// TODO 05/10/2022: Remove from SLAM service once GRPC data transfer is available.
// startDataProcess is the background control loop for sending data from camera to the data directory for processing.
func (slamSvc *slamService) startDataProcess() {
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
			if err := slamSvc.cancelCtx.Err(); err != nil {
				if !errors.Is(err, context.Canceled) {
					slamSvc.logger.Errorw("unexpected error in SLAM data process", "error", err)
				}
				dataWorker.Wait()
				return
			}

			select {
			case <-slamSvc.cancelCtx.Done():
				dataWorker.Wait()
				return
			case <-ticker.C:
				dataWorker.Add(1)

				// Split off go routine to handle data processing without affecting timing
				goutils.PanicCapturingGo(func() {
					defer dataWorker.Done()
					switch slamSvc.slamLib.AlgoType {
					case dense:
						if _, err := slamSvc.getAndSaveDataDense(); err != nil {
							slamSvc.logger.Warn(err)
						}
					case sparse:
						if _, err := slamSvc.getAndSaveDataSparse(); err != nil {
							slamSvc.logger.Warn(err)
						}
					default:
						slamSvc.logger.Warn("warning invalid algrothim specified")
					}
				})
			}
		}
	})
}

// TODO 05/03/2022: Implement SLAM starting and stopping processes (see JIRA ticket:
// https://viam.atlassian.net/jira/software/c/projects/DATA/boards/30?modal=detail&selectedIssue=DATA-104)
// startSLAMProcess starts up the SLAM library process by calling the executable binary.
func (slamSvc *slamService) startSLAMProcess(ctx context.Context) error {
	return nil
}

// TODO 05/03/2022: Implement closeout of slam service and subprocesses.
// Close out of all slam related processes.
func (slamSvc *slamService) Close() {
	slamSvc.cancelFunc()
	slamSvc.activeBackgroundWorkers.Wait()
}

// getAndSaveDataSparse implements the data extraction for sparse algos and saving to the directory path (data subfolder) specified in
// the config. It returns the full filepath for each file saved along with any error associated with the data creation or saving.
func (slamSvc *slamService) getAndSaveDataSparse() (string, error) {
	// Get Image
	img, _, err := slamSvc.camera.Next(slamSvc.cancelCtx)
	if err != nil {
		if err.Error() == "bad scan: OpTimeout" {
			slamSvc.logger.Warnw("Skipping this scan due to error", "error", err)
			return "", nil
		}
		return "", err
	}

	// Create file
	var fileType string
	switch slamSvc.slamMode {
	case mono:
		fileType = ".jpeg"
	case rgbd:
		// TODO 05/12/2022: Soon wil be deprecated into pointcloud files or rgb and monochromatic depth file. We will want picture pair.
		fileType = ".both"
	case twod:
		return "", errors.Errorf("bad slamMode %v specified for this algorithm", slamSvc.slamMode)
	}

	filename := createTimestampFilename(slamSvc.cameraName, slamSvc.dataDirectory, fileType)
	f, err := os.Create(filename)
	if err != nil {
		return filename, err
	}

	// Write image file based on mode
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
	case twod:
		return "", errors.Errorf("bad slamMode %v specified for this algorithm", slamSvc.slamMode)
	}
	if err = w.Flush(); err != nil {
		return filename, err
	}
	return filename, f.Close()
}

// getAndSaveDataDense implements the data extraction for dense algos and saving to the directory path (data subfolder) specified in
// the config. It returns the full filepath for each file saved along with any error associated with the data creation or saving.
func (slamSvc *slamService) getAndSaveDataDense() (string, error) {
	// Get NextPointCloud
	pointcloud, err := slamSvc.camera.NextPointCloud(slamSvc.cancelCtx)
	if err != nil {
		if err.Error() == "bad scan: OpTimeout" {
			slamSvc.logger.Warnw("Skipping this scan due to error", "error", err)
			return "", nil
		}
		return "", err
	}

	// Create file
	var fileType string
	switch slamSvc.slamMode {
	case twod:
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

	// Write PCD file based on mode
	if err = pc.ToPCD(pointcloud, w, 1); err != nil {
		return filename, err
	}
	if err = w.Flush(); err != nil {
		return filename, err
	}
	return filename, f.Close()
}

// Creates a file in the proper format with timestamp and sensor information.
func createTimestampFilename(cameraName, dataDirectory, fileType string) string {
	timestamp := time.Now()
	filename := filepath.Join(dataDirectory, "data", cameraName+"_data_"+timestamp.UTC().Format("2006-01-02T15_04_05.0000")+fileType)

	return filename
}
