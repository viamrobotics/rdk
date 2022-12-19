// Package builtin implements simultaneous localization and mapping
// This is an Experimental package
package builtin

import (
	"bufio"
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/protoutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	v1 "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/slam/v1"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
	"go.viam.com/utils"
)

var (
	cameraValidationMaxTimeoutSec = 30 // reconfigurable for testing
	dialMaxTimeoutSec             = 30 // reconfigurable for testing
)

const (
	defaultDataRateMs           = 200
	minDataRateMs               = 200
	defaultMapRateSec           = 60
	cameraValidationIntervalSec = 1.
	parsePortMaxTimeoutSec      = 60
	// time format for the slam service.
	slamTimeFormat        = "2006-01-02T15:04:05.0000Z"
	opTimeoutErrorMessage = "bad scan: OpTimeout"
	localhost0            = "localhost:0"
)

// SetCameraValidationMaxTimeoutSecForTesting sets cameraValidationMaxTimeoutSec for testing.
func SetCameraValidationMaxTimeoutSecForTesting(val int) {
	cameraValidationMaxTimeoutSec = val
}

// SetDialMaxTimeoutSecForTesting sets dialMaxTimeoutSec for testing.
func SetDialMaxTimeoutSecForTesting(val int) {
	dialMaxTimeoutSec = val
}

// TBD 05/04/2022: Needs more work once GRPC is included (future PR).
func init() {
	for _, slamLibrary := range slam.SLAMLibraries {
		registry.RegisterService(slam.Subtype, slamLibrary.AlgoName, registry.Service{
			Constructor: func(ctx context.Context, deps registry.Dependencies, c config.Service, logger golog.Logger) (interface{}, error) {
				return NewBuiltIn(ctx, deps, c, logger, false)
			},
		})
		cType := config.ServiceType(slam.SubtypeName)
		config.RegisterServiceAttributeMapConverter(cType, slamLibrary.AlgoName, func(attributes config.AttributeMap) (interface{}, error) {
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
}

// RuntimeConfigValidation ensures that required config parameters are valid at runtime. If any of the required config parameters are
// not valid, this function will throw a warning, but not close out/shut down the server. The required parameters that are checked here
// are: 'algorithm', 'data_dir', and 'config_param' (required due to the 'mode' parameter internal to it).
// Returns the slam mode.
func RuntimeConfigValidation(svcConfig *AttrConfig, model string, logger golog.Logger) (slam.Mode, error) {
	slamLib, ok := slam.SLAMLibraries[model]
	if !ok {
		return "", errors.Errorf("%v algorithm specified not in implemented list", model)
	}

	slamMode, ok := slamLib.SlamMode[svcConfig.ConfigParams["mode"]]
	if !ok {
		return "", errors.Errorf("getting data with specified algorithm %v, and desired mode %v",
			model, svcConfig.ConfigParams["mode"])
	}

	for _, directoryName := range [4]string{"", "data", "map", "config"} {
		directoryPath := filepath.Join(svcConfig.DataDirectory, directoryName)
		if _, err := os.Stat(directoryPath); os.IsNotExist(err) {
			logger.Warnf("%v directory does not exist", directoryPath)
			if err := os.Mkdir(directoryPath, os.ModePerm); err != nil {
				return "", errors.Errorf("issue creating directory at %v: %v", directoryPath, err)
			}
		}
	}

	if slamMode == slam.Rgbd || slamMode == slam.Mono {
		var directoryNames []string
		if slamMode == slam.Rgbd {
			directoryNames = []string{"rgb", "depth"}
		} else if slamMode == slam.Mono {
			directoryNames = []string{"rgb"}
		}
		for _, directoryName := range directoryNames {
			directoryPath := filepath.Join(svcConfig.DataDirectory, "data", directoryName)
			if _, err := os.Stat(directoryPath); os.IsNotExist(err) {
				logger.Warnf("%v directory does not exist", directoryPath)
				if err := os.Mkdir(directoryPath, os.ModePerm); err != nil {
					return "", errors.Errorf("issue creating directory at %v: %v", directoryPath, err)
				}
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
			return "", errors.Errorf("input_file_pattern (%v) does not match the regex pattern %v", svcConfig.InputFilePattern, pattern)
		}

		re = regexp.MustCompile(`(\d+)`)
		res2 := re.FindAllString(svcConfig.InputFilePattern, 3)
		startFileIndex, err := strconv.Atoi(res2[0])
		if err != nil {
			return "", err
		}
		endFileIndex, err := strconv.Atoi(res2[1])
		if err != nil {
			return "", err
		}

		interval, err := strconv.Atoi(res2[2])
		if err != nil {
			return "", err
		}

		if interval == 0 {
			return "", errors.New("the file input pattern's interval must be greater than zero")
		}

		if startFileIndex > endFileIndex {
			return "", errors.Errorf("second value in input file pattern must be larger than the first [%v]", svcConfig.InputFilePattern)
		}
	}

	if svcConfig.DataRateMs != 0 && svcConfig.DataRateMs < minDataRateMs {
		return "", errors.Errorf("cannot specify data_rate_msec less than %v", minDataRateMs)
	}

	if svcConfig.MapRateSec != nil && *svcConfig.MapRateSec < 0 {
		return "", errors.New("cannot specify map_rate_sec less than zero")
	}

	return slamMode, nil
}

// runtimeServiceValidation ensures the service's data processing and saving is valid for the mode and
// cameras given.

func runtimeServiceValidation(
	ctx context.Context,
	cams []camera.Camera,
	camStreams []gostream.VideoStream,
	slamSvc *builtIn,
) error {
	if len(cams) == 0 {
		return nil
	}

	var err error
	paths := make([]string, 0, 1)
	startTime := time.Now()

	// TODO 05/05/2022: This will be removed once GRPC data transfer is available as the responsibility for
	// calling the right algorithms (Next vs NextPointCloud) will be held by the slam libraries themselves
	// Note: if GRPC data transfer is delayed to after other algorithms (or user custom algos) are being
	// added this point will be revisited
	for {
		switch slamSvc.slamLib.AlgoType {
		case slam.Sparse:
			var currPaths []string
			currPaths, err = slamSvc.getAndSaveDataSparse(ctx, cams, camStreams)
			paths = append(paths, currPaths...)
		case slam.Dense:
			var path string
			path, err = slamSvc.getAndSaveDataDense(ctx, cams)
			paths = append(paths, path)
		default:
			return errors.Errorf("invalid slam algorithm %q", slamSvc.slamLib.AlgoName)
		}

		if err == nil {
			break
		}

		// This takes about 5 seconds, so the timeout should be sufficient.
		if time.Since(startTime) >= time.Duration(cameraValidationMaxTimeoutSec)*time.Second {
			return errors.Wrap(err, "error getting data in desired mode")
		}
		if !goutils.SelectContextOrWait(ctx, cameraValidationIntervalSec*time.Second) {
			return ctx.Err()
		}
	}

	// For ORBSLAM, generate a new yaml file based off the camera configuration and presence of maps
	if strings.Contains(slamSvc.slamLib.AlgoName, "orbslamv3") {
		if err = slamSvc.orbGenYAML(ctx, cams[0]); err != nil {
			return errors.Wrap(err, "error generating .yaml config")
		}
	}

	for _, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			return errors.Wrap(err, "error removing generated file during validation")
		}
	}

	return nil
}

// AttrConfig describes how to configure the service.
type AttrConfig struct {
	Sensors          []string          `json:"sensors"`
	ConfigParams     map[string]string `json:"config_params"`
	DataRateMs       int               `json:"data_rate_msec"`
	MapRateSec       *int              `json:"map_rate_sec"`
	DataDirectory    string            `json:"data_dir"`
	InputFilePattern string            `json:"input_file_pattern"`
	Port             string            `json:"port"`
}

// Validate creates the list of implicit dependencies.
func (config *AttrConfig) Validate(path string) ([]string, error) {

	if config.ConfigParams["mode"] == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "config_params[mode]")
	}

	if config.DataDirectory == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "data_dir")
	}

	deps := config.Sensors

	return deps, nil
}

// builtIn is the structure of the slam service.
type builtIn struct {
	cameraName      string
	slamLib         slam.LibraryMetadata
	slamMode        slam.Mode
	slamProcess     pexec.ProcessManager
	clientAlgo      pb.SLAMServiceClient
	clientAlgoClose func() error

	configParams     map[string]string
	dataDirectory    string
	inputFilePattern string

	port       string
	dataRateMs int
	mapRateSec int

	camStreams []gostream.VideoStream

	cancelFunc              func()
	logger                  golog.Logger
	activeBackgroundWorkers sync.WaitGroup

	bufferSLAMProcessLogs        bool
	slamProcessLogReader         io.ReadCloser
	slamProcessLogWriter         io.WriteCloser
	slamProcessBufferedLogReader bufio.Reader
}

// configureCameras will check the config to see if any cameras are desired and if so, grab the cameras from
// the robot. We assume there are at most two cameras and that we only require intrinsics from the first one.
// Returns the name of the first camera.
func configureCameras(ctx context.Context, svcConfig *AttrConfig, deps registry.Dependencies, logger golog.Logger) (string, []camera.Camera, error) {
	if len(svcConfig.Sensors) > 0 {
		logger.Debug("Running in live mode")
		cams := make([]camera.Camera, 0, len(svcConfig.Sensors))
		// The first camera is expected to be RGB or LIDAR.
		cameraName := svcConfig.Sensors[0]
		cam, err := camera.FromDependencies(deps, cameraName)
		if err != nil {
			return "", nil, errors.Wrapf(err, "error getting camera %v for slam service", cameraName)
		}
		proj, err := cam.Projector(ctx)
		if err != nil {
			if len(svcConfig.Sensors) == 1 {
				// LiDAR do not have intrinsic parameters and only send point clouds,
				// so no error should occur here, just inform the user
				logger.Debug("No camera features found, user possibly using LiDAR")
			} else {
				return "", nil, errors.Wrap(err,
					"Unable to get camera features for first camera, make sure the color camera is listed first")
			}
		} else {
			intrinsics, ok := proj.(*transform.PinholeCameraIntrinsics)
			if !ok {
				return "", nil, transform.NewNoIntrinsicsError("Intrinsics do not exist")
			}
			err = intrinsics.CheckValid()
			if err != nil {
				return "", nil, err
			}
		}
		cams = append(cams, cam)

		// If there is a second camera, it is expected to be depth.
		if len(svcConfig.Sensors) > 1 {
			depthCameraName := svcConfig.Sensors[1]
			logger.Debugf("Two cameras found for slam service, assuming %v is for color and %v is for depth",
				cameraName, depthCameraName)
			depthCam, err := camera.FromDependencies(deps, depthCameraName)
			if err != nil {
				return "", nil, errors.Wrapf(err, "error getting camera %v for slam service", depthCameraName)
			}
			cams = append(cams, depthCam)
		}

		return cameraName, cams, nil
	}
	return "", nil, nil
}

// setupGRPCConnection uses the defined port to create a GRPC client for communicating with the SLAM algorithms.
func setupGRPCConnection(ctx context.Context, port string, logger golog.Logger) (pb.SLAMServiceClient, func() error, error) {
	ctx, span := trace.StartSpan(ctx, "slam::builtIn::setupGRPCConnection")
	defer span.End()

	// This takes about 1 second, so the timeout should be sufficient.
	ctx, timeoutCancel := context.WithTimeout(ctx, time.Duration(dialMaxTimeoutSec)*time.Second)
	defer timeoutCancel()
	// The 'port' provided in the config is already expected to include "localhost:", if needed, so that it doesn't need to be
	// added anywhere in the code. This will allow cloud-based SLAM processing to exist in the future.
	// TODO: add credentials when running SLAM processing in the cloud.

	// Increasing the gRPC max message size from the default value of 4MB to 32MB, to match the limit that is set in RDK. This is
	// necessary for transmitting large pointclouds.
	maxMsgSizeOption := grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(32 * 1024 * 1024))
	connLib, err := grpc.DialContext(ctx, port, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock(), maxMsgSizeOption)
	if err != nil {
		logger.Errorw("error connecting to slam process", "error", err)
		return nil, nil, err
	}
	return pb.NewSLAMServiceClient(connLib), connLib.Close, err
}

// Position forwards the request for positional data to the slam library's gRPC service. Once a response is received,
// it is unpacked into a PoseInFrame.
func (slamSvc *builtIn) Position(ctx context.Context, name string, extra map[string]interface{}) (*referenceframe.PoseInFrame, error) {
	ctx, span := trace.StartSpan(ctx, "slam::builtIn::Position")
	defer span.End()

	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	req := &pb.GetPositionRequest{Name: name, Extra: ext}

	resp, err := slamSvc.clientAlgo.GetPosition(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "error getting SLAM position")
	}

	pInFrame := referenceframe.ProtobufToPoseInFrame(resp.Pose)

	// TODO DATA-531: https://viam.atlassian.net/jira/software/c/projects/DATA/boards/30?modal=detail&selectedIssue=DATA-531
	// Remove extraction and conversion of quaternion from the extra field in the response once the Rust
	// spatial math library is available and the desired math can be implemented on the orbSLAM side
	returnedExt := resp.Extra.AsMap()

	if val, ok := returnedExt["quat"]; ok {
		q := val.(map[string]interface{})

		valReal, ok1 := q["real"].(float64)
		valIMag, ok2 := q["imag"].(float64)
		valJMag, ok3 := q["jmag"].(float64)
		valKMag, ok4 := q["kmag"].(float64)

		if !ok1 || !ok2 || !ok3 || !ok4 {
			slamSvc.logger.Debugf("quaternion given, but invalid format detected, %v, skipping quaternion transform", q)
			return pInFrame, nil
		}
		newPose := spatialmath.NewPoseFromOrientation(pInFrame.Pose().Point(),
			&spatialmath.Quaternion{Real: valReal, Imag: valIMag, Jmag: valJMag, Kmag: valKMag})
		pInFrame = referenceframe.NewPoseInFrame(pInFrame.Parent(), newPose)
	}

	return pInFrame, nil
}

// GetMap forwards the request for map data to the slam library's gRPC service. Once a response is received it is unpacked
// into a mimeType and either a vision.Object or image.Image.
func (slamSvc *builtIn) GetMap(
	ctx context.Context,
	name, mimeType string,
	cp *referenceframe.PoseInFrame,
	include bool,
	extra map[string]interface{},
) (
	string, image.Image, *vision.Object, error,
) {
	ctx, span := trace.StartSpan(ctx, "slam::builtIn::GetMap")
	defer span.End()

	var cameraPosition *v1.Pose
	if cp != nil {
		cameraPosition = referenceframe.PoseInFrameToProtobuf(cp).Pose
	}

	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return "", nil, nil, err
	}
	req := &pb.GetMapRequest{
		Name:               name,
		MimeType:           mimeType,
		CameraPosition:     cameraPosition,
		IncludeRobotMarker: include,
		Extra:              ext,
	}

	var imData image.Image
	var vObj *vision.Object

	resp, err := slamSvc.clientAlgo.GetMap(ctx, req)
	if err != nil {
		return "", imData, vObj, errors.Errorf("error getting SLAM map (%v) : %v", mimeType, err)
	}

	switch mimeType {
	case rdkutils.MimeTypeJPEG:
		imData, err = jpeg.Decode(bytes.NewReader(resp.GetImage()))
		if err != nil {
			return "", nil, nil, errors.Wrap(err, "get map decode image failed")
		}
	case rdkutils.MimeTypePCD:
		pointcloudData := resp.GetPointCloud()
		if pointcloudData == nil {
			return "", nil, nil, errors.New("get map read pointcloud unavailable")
		}
		pc, err := pc.ReadPCD(bytes.NewReader(pointcloudData.PointCloud))
		if err != nil {
			return "", nil, nil, errors.Wrap(err, "get map read pointcloud failed")
		}

		vObj, err = vision.NewObject(pc)
		if err != nil {
			return "", nil, nil, errors.Wrap(err, "get map creating vision object failed")
		}
	}

	return resp.MimeType, imData, vObj, nil
}

// NewBuiltIn returns a new slam service for the given robot.
func NewBuiltIn(ctx context.Context, deps registry.Dependencies, config config.Service, logger golog.Logger, bufferSLAMProcessLogs bool) (slam.Service, error) {
	ctx, span := trace.StartSpan(ctx, "slam::slamService::New")
	defer span.End()

	svcConfig, ok := config.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(svcConfig, config.ConvertedAttributes)
	}

	cameraName, cams, err := configureCameras(ctx, svcConfig, deps, logger)
	if err != nil {
		return nil, errors.Wrap(err, "configuring camera error")
	}

	slamMode, err := RuntimeConfigValidation(svcConfig, config.Model, logger)
	if err != nil {
		return nil, errors.Wrap(err, "runtime slam config error")
	}

	var port string
	if svcConfig.Port == "" {
		port = localhost0
	} else {
		port = svcConfig.Port
	}

	var dataRate int
	if svcConfig.DataRateMs == 0 {
		dataRate = defaultDataRateMs
		logger.Debugf("no data_rate_msec given, setting to default value of %d", defaultDataRateMs)
	} else {
		dataRate = svcConfig.DataRateMs
	}

	var mapRate int
	if svcConfig.MapRateSec == nil {
		logger.Debugf("no map_rate_secs given, setting to default value of %d", defaultMapRateSec)
		mapRate = defaultMapRateSec
	} else if *svcConfig.MapRateSec == 0 {
		logger.Info("setting slam system to localization mode")
		mapRate = 0
	} else {
		mapRate = *svcConfig.MapRateSec
	}

	camStreams := make([]gostream.VideoStream, 0, len(cams))
	for _, cam := range cams {
		camStreams = append(camStreams, gostream.NewEmbeddedVideoStream(cam))
	}

	cancelCtx, cancelFunc := context.WithCancel(ctx)

	// SLAM Service Object
	slamSvc := &builtIn{
		cameraName:            cameraName,
		slamLib:               slam.SLAMLibraries[config.Model],
		slamMode:              slamMode,
		slamProcess:           pexec.NewProcessManager(logger),
		configParams:          svcConfig.ConfigParams,
		dataDirectory:         svcConfig.DataDirectory,
		inputFilePattern:      svcConfig.InputFilePattern,
		port:                  port,
		dataRateMs:            dataRate,
		mapRateSec:            mapRate,
		camStreams:            camStreams,
		cancelFunc:            cancelFunc,
		logger:                logger,
		bufferSLAMProcessLogs: bufferSLAMProcessLogs,
	}

	var success bool
	defer func() {
		if !success {
			if err := slamSvc.Close(); err != nil {
				logger.Errorw("error closing out after error", "error", err)
			}
		}
	}()

	if err := runtimeServiceValidation(cancelCtx, cams, camStreams, slamSvc); err != nil {
		return nil, errors.Wrap(err, "runtime slam service error")
	}

	slamSvc.StartDataProcess(cancelCtx, cams, camStreams, nil)

	if err := slamSvc.StartSLAMProcess(ctx); err != nil {
		return nil, errors.Wrap(err, "error with slam service slam process")
	}

	client, clientClose, err := setupGRPCConnection(ctx, slamSvc.port, logger)
	if err != nil {
		return nil, errors.Wrap(err, "error with initial grpc client to slam algorithm")
	}
	slamSvc.clientAlgo = client
	slamSvc.clientAlgoClose = clientClose

	success = true
	return slamSvc, nil
}

// Close out of all slam related processes.
func (slamSvc *builtIn) Close() error {
	defer func() {
		if slamSvc.clientAlgoClose != nil {
			goutils.UncheckedErrorFunc(slamSvc.clientAlgoClose)
		}
	}()
	slamSvc.cancelFunc()
	if slamSvc.bufferSLAMProcessLogs {
		if slamSvc.slamProcessLogReader != nil {
			slamSvc.slamProcessLogReader.Close()
		}
		if slamSvc.slamProcessLogWriter != nil {
			slamSvc.slamProcessLogWriter.Close()
		}
	}
	if err := slamSvc.StopSLAMProcess(); err != nil {
		return errors.Wrap(err, "error occurred during closeout of process")
	}
	slamSvc.activeBackgroundWorkers.Wait()
	for idx, stream := range slamSvc.camStreams {
		i := idx
		s := stream
		defer func() {
			if err := s.Close(context.Background()); err != nil {
				slamSvc.logger.Errorw("error closing cam", "number", i, "error", err)
			}
		}()
	}
	return nil
}

// TODO 05/10/2022: Remove from SLAM service once GRPC data transfer is available.
// startDataProcess is the background control loop for sending data from camera to the data directory for processing.
func (slamSvc *builtIn) StartDataProcess(
	cancelCtx context.Context,
	cams []camera.Camera,
	camStreams []gostream.VideoStream,
	c chan int,
) {
	if len(cams) == 0 {
		return
	}

	slamSvc.activeBackgroundWorkers.Add(1)
	if err := cancelCtx.Err(); err != nil {
		if !errors.Is(err, context.Canceled) {
			slamSvc.logger.Errorw("unexpected error in SLAM service", "error", err)
		}
		slamSvc.activeBackgroundWorkers.Done()
		return
	}
	goutils.PanicCapturingGo(func() {
		ticker := time.NewTicker(time.Millisecond * time.Duration(slamSvc.dataRateMs))
		defer ticker.Stop()
		defer slamSvc.activeBackgroundWorkers.Done()

		for {
			if err := cancelCtx.Err(); err != nil {
				if !errors.Is(err, context.Canceled) {
					slamSvc.logger.Errorw("unexpected error in SLAM data process", "error", err)
				}
				return
			}

			select {
			case <-cancelCtx.Done():
				return
			case <-ticker.C:
				slamSvc.activeBackgroundWorkers.Add(1)
				if err := cancelCtx.Err(); err != nil {
					if !errors.Is(err, context.Canceled) {
						slamSvc.logger.Errorw("unexpected error in SLAM service", "error", err)
					}
					slamSvc.activeBackgroundWorkers.Done()
					return
				}
				goutils.PanicCapturingGo(func() {
					defer slamSvc.activeBackgroundWorkers.Done()
					switch slamSvc.slamLib.AlgoType {
					case slam.Dense:
						if _, err := slamSvc.getAndSaveDataDense(cancelCtx, cams); err != nil {
							slamSvc.logger.Warn(err)
						}
						if c != nil {
							c <- 1
						}
					case slam.Sparse:
						if _, err := slamSvc.getAndSaveDataSparse(cancelCtx, cams, camStreams); err != nil {
							slamSvc.logger.Warn(err)
						}
						if c != nil {
							c <- 1
						}
					default:
						slamSvc.logger.Warnw("warning invalid algorithm specified", "algorithm", slamSvc.slamLib.AlgoType)
					}
				})
			}
		}
	})
}

// GetSLAMProcessConfig returns the process config for the SLAM process.
func (slamSvc *builtIn) GetSLAMProcessConfig() pexec.ProcessConfig {
	var args []string

	args = append(args, "-sensors="+slamSvc.cameraName)
	args = append(args, "-config_param="+createKeyValuePairs(slamSvc.configParams))
	args = append(args, "-data_rate_ms="+strconv.Itoa(slamSvc.dataRateMs))
	args = append(args, "-map_rate_sec="+strconv.Itoa(slamSvc.mapRateSec))
	args = append(args, "-data_dir="+slamSvc.dataDirectory)
	args = append(args, "-input_file_pattern="+slamSvc.inputFilePattern)
	args = append(args, "-port="+slamSvc.port)
	args = append(args, "--aix-auto-update")

	return pexec.ProcessConfig{
		ID:      "slam_" + slamSvc.slamLib.AlgoName,
		Name:    slam.SLAMLibraries[slamSvc.slamLib.AlgoName].BinaryLocation,
		Args:    args,
		Log:     true,
		OneShot: false,
	}
}

func (slamSvc *builtIn) GetSLAMProcessBufferedLogReader() bufio.Reader {
	return slamSvc.slamProcessBufferedLogReader
}

// startSLAMProcess starts up the SLAM library process by calling the executable binary and giving it the necessary arguments.
func (slamSvc *builtIn) StartSLAMProcess(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "slam::slamService::StartSLAMProcess")
	defer span.End()

	processConfig := slamSvc.GetSLAMProcessConfig()

	var logReader io.ReadCloser
	var logWriter io.WriteCloser
	var bufferedLogReader bufio.Reader
	if slamSvc.port == localhost0 || slamSvc.bufferSLAMProcessLogs {
		logReader, logWriter = io.Pipe()
		bufferedLogReader = *bufio.NewReader(logReader)
		processConfig.LogWriter = logWriter
	}

	_, err := slamSvc.slamProcess.AddProcessFromConfig(ctx, processConfig)
	if err != nil {
		return errors.Wrap(err, "problem adding slam process")
	}

	slamSvc.logger.Debug("starting slam process")

	if err = slamSvc.slamProcess.Start(ctx); err != nil {
		return errors.Wrap(err, "problem starting slam process")
	}

	if slamSvc.port == localhost0 {
		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, parsePortMaxTimeoutSec*time.Second)
		defer timeoutCancel()

		if !slamSvc.bufferSLAMProcessLogs {
			//nolint:errcheck
			defer logReader.Close()
			//nolint:errcheck
			defer logWriter.Close()
		}

		for {
			if err := timeoutCtx.Err(); err != nil {
				return errors.Wrapf(err, "error getting port from slam process")
			}

			line, err := bufferedLogReader.ReadString('\n')
			if err != nil {
				return errors.Wrapf(err, "error getting port from slam process")
			}
			portLogLinePrefix := "Server listening on "
			if strings.Contains(line, portLogLinePrefix) {
				linePieces := strings.Split(line, portLogLinePrefix)
				if len(linePieces) != 2 {
					return errors.Errorf("failed to parse port from slam process log line: %v", line)
				}
				slamSvc.port = "localhost:" + strings.TrimRight(linePieces[1], "\n")
				break
			}
		}
	}

	if slamSvc.bufferSLAMProcessLogs {
		slamSvc.slamProcessLogReader = logReader
		slamSvc.slamProcessLogWriter = logWriter
		slamSvc.slamProcessBufferedLogReader = bufferedLogReader
	}

	return nil
}

// stopSLAMProcess uses the process manager to stop the created slam process from running.
func (slamSvc *builtIn) StopSLAMProcess() error {
	if err := slamSvc.slamProcess.Stop(); err != nil {
		return errors.Wrap(err, "problem stopping slam process")
	}
	return nil
}

func (slamSvc *builtIn) getPNGImage(ctx context.Context, cam camera.Camera) ([]byte, func(), error) {
	// We will hint that we want a PNG.
	// The Camera service server implementation in RDK respects this; others may not.
	readImgCtx := gostream.WithMIMETypeHint(ctx, rdkutils.WithLazyMIMEType(rdkutils.MimeTypePNG))
	img, release, err := camera.ReadImage(readImgCtx, cam)
	if err != nil {
		return nil, nil, err
	}
	if lazyImg, ok := img.(*rimage.LazyEncodedImage); ok {
		if lazyImg.MIMEType() != rdkutils.MimeTypePNG {
			return nil, nil, errors.Errorf("expected mime type %v, got %T", rdkutils.MimeTypePNG, img)
		}
		return lazyImg.RawData(), release, nil
	}

	if ycbcrImg, ok := img.(*image.YCbCr); ok {
		pngImage, err := rimage.EncodeImage(ctx, ycbcrImg, rdkutils.MimeTypePNG)
		if err != nil {
			return nil, nil, err
		}
		return pngImage, release, nil
	}

	return nil, nil, errors.Errorf("expected lazily encoded image or ycbcrImg, got %T", img)
}

// getAndSaveDataSparse implements the data extraction for sparse algos and saving to the directory path (data subfolder) specified in
// the config. It returns the full filepath for each file saved along with any error associated with the data creation or saving.
func (slamSvc *builtIn) getAndSaveDataSparse(
	ctx context.Context,
	cams []camera.Camera,
	camStreams []gostream.VideoStream,
) ([]string, error) {
	ctx, span := trace.StartSpan(ctx, "slam::builtIn::getAndSaveDataSparse")
	defer span.End()

	switch slamSvc.slamMode {
	case slam.Mono:
		if len(camStreams) != 1 {
			return nil, errors.Errorf("expected 1 camera for mono slam, found %v", len(camStreams))
		}

		image, release, err := slamSvc.getPNGImage(ctx, cams[0])
		if release != nil {
			defer release()
		}
		if err != nil {
			if err.Error() == opTimeoutErrorMessage {
				slamSvc.logger.Warnw("Skipping this scan due to error", "error", err)
				return nil, nil
			}
			return nil, err
		}
		filenames, err := createTimestampFilenames(slamSvc.cameraName, slamSvc.dataDirectory, ".png", slamSvc.slamMode)
		if err != nil {
			return nil, err
		}
		filename := filenames[0]
		//nolint:gosec
		f, err := os.Create(filename)
		if err != nil {
			return []string{filename}, err
		}
		w := bufio.NewWriter(f)
		if _, err := w.Write(image); err != nil {
			return []string{filename}, err
		}
		if err := w.Flush(); err != nil {
			return []string{filename}, err
		}
		return []string{filename}, f.Close()
	case slam.Rgbd:
		if len(cams) != 2 {
			return nil, errors.Errorf("expected 2 cameras for Rgbd slam, found %v", len(cams))
		}

		images, releaseFuncs, err := slamSvc.getSimultaneousColorAndDepth(ctx, cams)
		for _, rFunc := range releaseFuncs {
			if rFunc != nil {
				defer rFunc()
			}
		}
		if err != nil {
			if err.Error() == opTimeoutErrorMessage {
				slamSvc.logger.Warnw("Skipping this scan due to error", "error", err)
				return nil, nil
			}
			return nil, err
		}

		filenames, err := createTimestampFilenames(slamSvc.cameraName, slamSvc.dataDirectory, ".png", slamSvc.slamMode)
		if err != nil {
			return nil, err
		}
		for i, filename := range filenames {
			//nolint:gosec
			f, err := os.Create(filename)
			if err != nil {
				return filenames, err
			}
			w := bufio.NewWriter(f)
			if _, err := w.Write(images[i]); err != nil {
				return filenames, err
			}
			if err := w.Flush(); err != nil {
				return filenames, err
			}
			if err := f.Close(); err != nil {
				return filenames, err
			}
		}
		return filenames, nil
	case slam.Dim2d, slam.Dim3d:
		return nil, errors.Errorf("bad slamMode %v specified for this algorithm", slamSvc.slamMode)
	default:
		return nil, errors.Errorf("invalid slamMode %v specified", slamSvc.slamMode)
	}
}

// Gets the color image and depth image from the cameras as close to simultaneously as possible.
func (slamSvc *builtIn) getSimultaneousColorAndDepth(
	ctx context.Context,
	cams []camera.Camera,
) ([2][]byte, [2]func(), error) {
	var wg sync.WaitGroup
	var images [2][]byte
	var releaseFuncs [2]func()
	var errs [2]error

	for i := 0; i < 2; i++ {
		slamSvc.activeBackgroundWorkers.Add(1)
		wg.Add(1)
		if err := ctx.Err(); err != nil {
			if !errors.Is(err, context.Canceled) {
				slamSvc.logger.Errorw("unexpected error in SLAM service", "error", err)
			}
			slamSvc.activeBackgroundWorkers.Done()
			return images, releaseFuncs, err
		}
		iLoop := i
		goutils.PanicCapturingGo(func() {
			defer slamSvc.activeBackgroundWorkers.Done()
			defer wg.Done()
			images[iLoop], releaseFuncs[iLoop], errs[iLoop] = slamSvc.getPNGImage(ctx, cams[iLoop])
		})
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return images, releaseFuncs, err
		}
	}

	return images, releaseFuncs, nil
}

// getAndSaveDataDense implements the data extraction for dense algos and saving to the directory path (data subfolder) specified in
// the config. It returns the full filepath for each file saved along with any error associated with the data creation or saving.
func (slamSvc *builtIn) getAndSaveDataDense(ctx context.Context, cams []camera.Camera) (string, error) {
	ctx, span := trace.StartSpan(ctx, "slam::builtIn::getAndSaveDataDense")
	defer span.End()

	if len(cams) != 1 {
		return "", errors.Errorf("expected 1 camera for this slam algorithm, found %v", len(cams))
	}

	pointcloud, err := cams[0].NextPointCloud(ctx)
	if err != nil {
		if err.Error() == opTimeoutErrorMessage {
			slamSvc.logger.Warnw("Skipping this scan due to error", "error", err)
			return "", nil
		}
		return "", err
	}

	var fileType string
	switch slamSvc.slamMode {
	case slam.Dim2d, slam.Dim3d:
		fileType = ".pcd"
	case slam.Rgbd, slam.Mono:
		return "", errors.Errorf("bad slamMode %v specified for this algorithm", slamSvc.slamMode)
	}
	filenames, err := createTimestampFilenames(slamSvc.cameraName, slamSvc.dataDirectory, fileType, slamSvc.slamMode)
	if err != nil {
		return "", err
	}
	filename := filenames[0]
	//nolint:gosec
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

// Creates a file for camera data with the specified sensor name and timestamp written into the filename.
// For RGBD cameras, two filenames are created with the same timestamp in different directories.
func createTimestampFilenames(cameraName, dataDirectory, fileType string, slamMode slam.Mode) ([]string, error) {
	timeStamp := time.Now()

	switch slamMode {
	case slam.Rgbd:
		colorFilename := filepath.Join(dataDirectory, "data", "rgb", cameraName+"_data_"+timeStamp.UTC().Format(slamTimeFormat)+fileType)
		depthFilename := filepath.Join(dataDirectory, "data", "depth", cameraName+"_data_"+timeStamp.UTC().Format(slamTimeFormat)+fileType)
		return []string{colorFilename, depthFilename}, nil
	case slam.Mono:
		colorFilename := filepath.Join(dataDirectory, "data", "rgb", cameraName+"_data_"+timeStamp.UTC().Format(slamTimeFormat)+fileType)
		return []string{colorFilename}, nil
	case slam.Dim2d, slam.Dim3d:
		filename := filepath.Join(dataDirectory, "data", cameraName+"_data_"+timeStamp.UTC().Format(slamTimeFormat)+fileType)
		return []string{filename}, nil
	default:
		return nil, errors.Errorf("Invalid slam mode: %v", slamMode)
	}
}

// Converts a dictionary to a string for so that it can be loaded into an arg for the slam process.
func createKeyValuePairs(m map[string]string) string {
	stringMapList := make([]string, len(m))
	i := 0
	for k, val := range m {
		stringMapList[i] = k + "=" + val
		i++
	}

	stringMap := strings.Join(stringMapList, ",")

	return "{" + stringMap + "}"
}
