// Package slam implements simultaneous localization and mapping
package slam

import (
	"bufio"
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	pc "go.viam.com/rdk/pointcloud"
	v1 "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/slam/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

const (
	defaultDataRateMs             = 200
	defaultMapRateSec             = 60
	cameraValidationMaxTimeoutSec = 30
	cameraValidationIntervalSec   = 1
	dialMaxTimeoutSec             = 5
	// TODO change time format to .Format(time.RFC3339Nano) https://viam.atlassian.net/browse/DATA-277
	// time format for the slam service.
	slamTimeFormat        = "2006-01-02T15_04_05.0000"
	opTimeoutErrorMessage = "bad scan: OpTimeout"
)

// TBD 05/04/2022: Needs more work once GRPC is included (future PR).
func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.SLAMService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterSLAMServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.SLAMService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
		Reconfigurable: WrapWithReconfigurable,
	})
	registry.RegisterService(Subtype, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
		},
	})
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

// runtimeConfigValidation ensures that required config parameters are valid at runtime. If any of the required config parameters are
// not valid, this function will throw a warning, but not close out/shut down the server. The required parameters that are checked here
// are: 'algorithm', 'data_dir', and 'config_param' (required due to the 'mode' parameter internal to it).
// Returns the slam mode.
func runtimeConfigValidation(svcConfig *AttrConfig, logger golog.Logger) (mode, error) {
	slamLib, ok := SLAMLibraries[svcConfig.Algorithm]
	if !ok {
		return "", errors.Errorf("%v algorithm specified not in implemented list", svcConfig.Algorithm)
	}

	slamMode, ok := slamLib.SlamMode[svcConfig.ConfigParams["mode"]]
	if !ok {
		return "", errors.Errorf("getting data with specified algorithm %v, and desired mode %v",
			svcConfig.Algorithm, svcConfig.ConfigParams["mode"])
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
	if slamMode == rgbd {
		for _, directoryName := range [2]string{"rgb", "depth"} {
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

	return slamMode, nil
}

// runtimeServiceValidation ensures the service's data processing and saving is valid for the mode and
// cameras given.
func runtimeServiceValidation(ctx context.Context, cams []camera.Camera, slamSvc *slamService) error {
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
		case sparse:
			var currPaths []string
			currPaths, err = slamSvc.getAndSaveDataSparse(ctx, cams)
			paths = append(paths, currPaths...)
		case dense:
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
		if time.Since(startTime) >= cameraValidationMaxTimeoutSec*time.Second {
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
	Algorithm        string            `json:"algorithm"`
	ConfigParams     map[string]string `json:"config_params"`
	DataRateMs       int               `json:"data_rate_ms"`
	MapRateSec       int               `json:"map_rate_sec"`
	DataDirectory    string            `json:"data_dir"`
	InputFilePattern string            `json:"input_file_pattern"`
	Port             string            `json:"port"`
}

var (
	_ = Service(&reconfigurableSlam{})
	_ = resource.Reconfigurable(&reconfigurableSlam{})
	_ = goutils.ContextCloser(&reconfigurableSlam{})
)

// Service describes the functions that are available to the service.
type Service interface {
	GetPosition(context.Context, string) (*referenceframe.PoseInFrame, error)
	GetMap(context.Context, string, string, *referenceframe.PoseInFrame, bool) (string, image.Image, *vision.Object, error)
}

// SlamService is the structure of the slam service.
type slamService struct {
	cameraName      string
	slamLib         LibraryMetadata
	slamMode        mode
	slamProcess     pexec.ProcessManager
	clientAlgo      pb.SLAMServiceClient
	clientAlgoClose func() error

	configParams     map[string]string
	dataDirectory    string
	inputFilePattern string

	port       string
	dataRateMs int
	mapRateSec int

	cancelFunc              func()
	logger                  golog.Logger
	activeBackgroundWorkers sync.WaitGroup
}

// configureCameras will check the config to see if any cameras are desired and if so, grab the cameras from
// the robot. We assume there are at most two cameras and that we only require intrinsics from the first one.
// Returns the name of the first camera.
func configureCameras(ctx context.Context, svcConfig *AttrConfig, r robot.Robot, logger golog.Logger) (string, []camera.Camera, error) {
	if len(svcConfig.Sensors) > 0 {
		logger.Debug("Running in live mode")
		cams := make([]camera.Camera, 0, len(svcConfig.Sensors))

		// The first camera is expected to be RGB or LIDAR.
		cameraName := svcConfig.Sensors[0]
		cam, err := camera.FromRobot(r, cameraName)
		if err != nil {
			return "", nil, errors.Wrapf(err, "error getting camera %v for slam service", cameraName)
		}

		proj, err := cam.GetProperties(ctx)
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
			depthCam, err := camera.FromRobot(r, depthCameraName)
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
	ctx, span := trace.StartSpan(ctx, "slam::slamService::setupGRPCConnection")
	defer span.End()

	// This takes about 1 second, so the timeout should be sufficient.
	ctx, timeoutCancel := context.WithTimeout(ctx, dialMaxTimeoutSec*time.Second)
	defer timeoutCancel()
	// The 'port' provided in the config is already expected to include "localhost:", if needed, so that it doesn't need to be
	// added anywhere in the code. This will allow cloud-based SLAM processing to exist in the future.
	// TODO: add credentials when running SLAM processing in the cloud.
	connLib, err := grpc.DialContext(ctx, port, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		logger.Errorw("error connecting to slam process", "error", err)
		return nil, nil, err
	}
	return pb.NewSLAMServiceClient(connLib), connLib.Close, err
}

// GetPosition forwards the request for positional data to the slam library's gRPC service. Once a response is received,
// it is unpacked into a PoseInFrame.
func (slamSvc *slamService) GetPosition(ctx context.Context, name string) (*referenceframe.PoseInFrame, error) {
	ctx, span := trace.StartSpan(ctx, "slam::slamService::GetPosition")
	defer span.End()

	req := &pb.GetPositionRequest{Name: name}

	resp, err := slamSvc.clientAlgo.GetPosition(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "error getting SLAM position")
	}

	return referenceframe.ProtobufToPoseInFrame(resp.Pose), nil
}

// GetMap forwards the request for map data to the slam library's gRPC service. Once a response is received it is unpacked
// into a mimeType and either a vision.Object or image.Image.
func (slamSvc *slamService) GetMap(ctx context.Context, name, mimeType string, cp *referenceframe.PoseInFrame, include bool) (
	string, image.Image, *vision.Object, error,
) {
	ctx, span := trace.StartSpan(ctx, "slam::slamService::GetMap")
	defer span.End()

	var cameraPosition *v1.Pose
	if cp != nil {
		cameraPosition = referenceframe.PoseInFrameToProtobuf(cp).Pose
	}

	req := &pb.GetMapRequest{
		Name:               name,
		MimeType:           mimeType,
		CameraPosition:     cameraPosition,
		IncludeRobotMarker: include,
	}

	var imData image.Image
	var vObj *vision.Object

	resp, err := slamSvc.clientAlgo.GetMap(ctx, req)
	if err != nil {
		return "", imData, vObj, errors.Errorf("error getting SLAM map (%v) : %v", mimeType, err)
	}

	switch mimeType {
	case utils.MimeTypeJPEG:
		imData, err = jpeg.Decode(bytes.NewReader(resp.GetImage()))
		if err != nil {
			return "", nil, nil, errors.Wrap(err, "get map decode image failed")
		}
	case utils.MimeTypePCD:
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

// New returns a new slam service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	ctx, span := trace.StartSpan(ctx, "slam::slamService::New")
	defer span.End()

	svcConfig, ok := config.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(svcConfig, config.ConvertedAttributes)
	}

	cameraName, cams, err := configureCameras(ctx, svcConfig, r, logger)
	if err != nil {
		return nil, errors.Wrap(err, "configuring camera error")
	}

	slamMode, err := runtimeConfigValidation(svcConfig, logger)
	if err != nil {
		return nil, errors.Wrap(err, "runtime slam config error")
	}

	var port string
	if svcConfig.Port == "" {
		p, err := goutils.TryReserveRandomPort()
		if err != nil {
			return nil, errors.Wrap(err, "error trying to return a random port")
		}
		port = "localhost:" + strconv.Itoa(p)
	} else {
		port = svcConfig.Port
	}

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
		cameraName:       cameraName,
		slamLib:          SLAMLibraries[svcConfig.Algorithm],
		slamMode:         slamMode,
		slamProcess:      pexec.NewProcessManager(logger),
		configParams:     svcConfig.ConfigParams,
		dataDirectory:    svcConfig.DataDirectory,
		inputFilePattern: svcConfig.InputFilePattern,
		port:             port,
		dataRateMs:       dataRate,
		mapRateSec:       mapRate,
		cancelFunc:       cancelFunc,
		logger:           logger,
	}

	var success bool
	defer func() {
		if !success {
			if err := slamSvc.Close(); err != nil {
				logger.Errorw("error closing out after error", "error", err)
			}
		}
	}()

	if err := runtimeServiceValidation(cancelCtx, cams, slamSvc); err != nil {
		return nil, errors.Wrap(err, "runtime slam service error")
	}

	slamSvc.StartDataProcess(cancelCtx, cams)

	if err := slamSvc.StartSLAMProcess(ctx); err != nil {
		return nil, errors.Wrap(err, "error with slam service slam process")
	}

	client, clientClose, err := setupGRPCConnection(ctx, port, logger)
	if err != nil {
		return nil, errors.Wrap(err, "error with initial grpc client to slam algorithm")
	}
	slamSvc.clientAlgo = client
	slamSvc.clientAlgoClose = clientClose

	success = true
	return slamSvc, nil
}

// Close out of all slam related processes.
func (slamSvc *slamService) Close() error {
	defer func() {
		if slamSvc.clientAlgoClose != nil {
			goutils.UncheckedErrorFunc(slamSvc.clientAlgoClose)
		}
	}()
	slamSvc.cancelFunc()
	if err := slamSvc.StopSLAMProcess(); err != nil {
		return errors.Wrap(err, "error occurred during closeout of process")
	}
	slamSvc.activeBackgroundWorkers.Wait()
	return nil
}

// TODO 05/10/2022: Remove from SLAM service once GRPC data transfer is available.
// startDataProcess is the background control loop for sending data from camera to the data directory for processing.
func (slamSvc *slamService) StartDataProcess(cancelCtx context.Context, cams []camera.Camera) {
	if len(cams) == 0 {
		return
	}

	slamSvc.activeBackgroundWorkers.Add(1)
	if err := cancelCtx.Err(); err != nil {
		if !errors.Is(err, context.Canceled) {
			slamSvc.logger.Errorw("unexpected error in SLAM service", "error", err)
		}
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
				switch slamSvc.slamLib.AlgoType {
				case dense:
					if _, err := slamSvc.getAndSaveDataDense(cancelCtx, cams); err != nil {
						slamSvc.logger.Warn(err)
					}
				case sparse:
					if _, err := slamSvc.getAndSaveDataSparse(cancelCtx, cams); err != nil {
						slamSvc.logger.Warn(err)
					}
				default:
					slamSvc.logger.Warnw("warning invalid algorithm specified", "algorithm", slamSvc.slamLib.AlgoType)
				}
			}
		}
	})
}

// GetSLAMProcessConfig returns the process config for the SLAM process.
func (slamSvc *slamService) GetSLAMProcessConfig() pexec.ProcessConfig {
	var args []string

	args = append(args, "-sensors="+slamSvc.cameraName)
	args = append(args, "-config_param="+createKeyValuePairs(slamSvc.configParams))
	args = append(args, "-data_rate_ms="+strconv.Itoa(slamSvc.dataRateMs))
	args = append(args, "-map_rate_sec="+strconv.Itoa(slamSvc.mapRateSec))
	args = append(args, "-data_dir="+slamSvc.dataDirectory)
	args = append(args, "-input_file_pattern="+slamSvc.inputFilePattern)
	args = append(args, "-port="+slamSvc.port)

	return pexec.ProcessConfig{
		ID:      "slam_" + slamSvc.slamLib.AlgoName,
		Name:    SLAMLibraries[slamSvc.slamLib.AlgoName].BinaryLocation,
		Args:    args,
		Log:     true,
		OneShot: false,
	}
}

// startSLAMProcess starts up the SLAM library process by calling the executable binary and giving it the necessary arguments.
func (slamSvc *slamService) StartSLAMProcess(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "slam::slamService::StartSLAMProcess")
	defer span.End()

	_, err := slamSvc.slamProcess.AddProcessFromConfig(ctx, slamSvc.GetSLAMProcessConfig())
	if err != nil {
		return errors.Wrap(err, "problem adding slam process")
	}

	slamSvc.logger.Debug("starting slam process")

	if err = slamSvc.slamProcess.Start(ctx); err != nil {
		return errors.Wrap(err, "problem starting slam process")
	}

	return nil
}

// stopSLAMProcess uses the process manager to stop the created slam process from running.
func (slamSvc *slamService) StopSLAMProcess() error {
	if err := slamSvc.slamProcess.Stop(); err != nil {
		return errors.Wrap(err, "problem stopping slam process")
	}
	return nil
}

// getAndSaveDataSparse implements the data extraction for sparse algos and saving to the directory path (data subfolder) specified in
// the config. It returns the full filepath for each file saved along with any error associated with the data creation or saving.
func (slamSvc *slamService) getAndSaveDataSparse(ctx context.Context, cams []camera.Camera) ([]string, error) {
	ctx, span := trace.StartSpan(ctx, "slam::slamService::getAndSaveDataSparse")
	defer span.End()

	switch slamSvc.slamMode {
	case mono:
		if len(cams) != 1 {
			return nil, errors.Errorf("expected 1 camera for mono slam, found %v", len(cams))
		}
		img, _, err := cams[0].Next(ctx)
		if err != nil {
			if err.Error() == opTimeoutErrorMessage {
				slamSvc.logger.Warnw("Skipping this scan due to error", "error", err)
				return nil, nil
			}
			return nil, err
		}
		filenames := createTimestampFilenames(slamSvc.cameraName, slamSvc.dataDirectory, ".jpeg", false)
		filename := filenames[0]
		//nolint:gosec
		f, err := os.Create(filename)
		if err != nil {
			return []string{filename}, err
		}
		w := bufio.NewWriter(f)
		if err := jpeg.Encode(w, img, nil); err != nil {
			return []string{filename}, err
		}
		if err := w.Flush(); err != nil {
			return []string{filename}, err
		}
		return []string{filename}, f.Close()
	case rgbd:
		if len(cams) != 2 {
			return nil, errors.Errorf("expected 2 cameras for rgbd slam, found %v", len(cams))
		}

		images, err := slamSvc.getSimultaneousColorAndDepth(ctx, cams)
		if err != nil {
			if err.Error() == opTimeoutErrorMessage {
				slamSvc.logger.Warnw("Skipping this scan due to error", "error", err)
				return nil, nil
			}
			return nil, err
		}
		filenames := createTimestampFilenames(slamSvc.cameraName, slamSvc.dataDirectory, ".png", true)
		for i, filename := range filenames {
			//nolint:gosec
			f, err := os.Create(filename)
			if err != nil {
				return filenames, err
			}
			w := bufio.NewWriter(f)
			if err := png.Encode(w, images[i]); err != nil {
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
	case dim2d, dim3d:
		return nil, errors.Errorf("bad slamMode %v specified for this algorithm", slamSvc.slamMode)
	default:
		return nil, errors.Errorf("invalid slamMode %v specified", slamSvc.slamMode)
	}
}

// Gets the color image and depth image from the cameras as close to simultaneously as possible.
// Assume the first camera is for color and the second is for depth.
func (slamSvc *slamService) getSimultaneousColorAndDepth(ctx context.Context, cams []camera.Camera) ([2]image.Image, error) {
	var wg sync.WaitGroup
	var images [2]image.Image
	var errs [2]error

	// Get color image.
	slamSvc.activeBackgroundWorkers.Add(1)
	wg.Add(1)
	if err := ctx.Err(); err != nil {
		if !errors.Is(err, context.Canceled) {
			slamSvc.logger.Errorw("unexpected error in SLAM service", "error", err)
		}
		return images, err
	}
	goutils.PanicCapturingGo(func() {
		defer slamSvc.activeBackgroundWorkers.Done()
		defer wg.Done()
		images[0], _, errs[0] = cams[0].Next(ctx)
	})

	// Get depth image.
	slamSvc.activeBackgroundWorkers.Add(1)
	wg.Add(1)
	if err := ctx.Err(); err != nil {
		if !errors.Is(err, context.Canceled) {
			slamSvc.logger.Errorw("unexpected error in SLAM service", "error", err)
		}
		return images, err
	}
	goutils.PanicCapturingGo(func() {
		defer slamSvc.activeBackgroundWorkers.Done()
		defer wg.Done()
		var depthImage image.Image
		depthImage, _, errs[1] = cams[1].Next(ctx)
		if errs[1] != nil {
			return
		}
		var depthMap *rimage.DepthMap
		depthMap, errs[1] = rimage.ConvertImageToDepthMap(depthImage)
		if errs[1] != nil {
			return
		}
		images[1] = depthMap.ToGray16Picture()
	})
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return images, err
		}
	}

	return images, nil
}

// getAndSaveDataDense implements the data extraction for dense algos and saving to the directory path (data subfolder) specified in
// the config. It returns the full filepath for each file saved along with any error associated with the data creation or saving.
func (slamSvc *slamService) getAndSaveDataDense(ctx context.Context, cams []camera.Camera) (string, error) {
	ctx, span := trace.StartSpan(ctx, "slam::slamService::getAndSaveDataDense")
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
	case dim2d, dim3d:
		fileType = ".pcd"
	case rgbd, mono:
		return "", errors.Errorf("bad slamMode %v specified for this algorithm", slamSvc.slamMode)
	}
	filenames := createTimestampFilenames(slamSvc.cameraName, slamSvc.dataDirectory, fileType, false)
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

type reconfigurableSlam struct {
	mu     sync.RWMutex
	actual Service
}

func (svc *reconfigurableSlam) GetPosition(ctx context.Context, val string) (*referenceframe.PoseInFrame, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetPosition(ctx, val)
}

func (svc *reconfigurableSlam) GetMap(ctx context.Context,
	name string,
	mimeType string,
	cp *referenceframe.PoseInFrame,
	include bool,
) (string, image.Image, *vision.Object, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetMap(ctx, name, mimeType, cp, include)
}

func (svc *reconfigurableSlam) Close(ctx context.Context) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return goutils.TryClose(ctx, svc.actual)
}

// Reconfigure replaces the old slam service with a new slam.
func (svc *reconfigurableSlam) Reconfigure(ctx context.Context, newSvc resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newSvc.(*reconfigurableSlam)
	if !ok {
		return utils.NewUnexpectedTypeError(svc, newSvc)
	}
	if err := goutils.TryClose(ctx, svc.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// WrapWithReconfigurable wraps a slam service as a Reconfigurable.
func WrapWithReconfigurable(s interface{}) (resource.Reconfigurable, error) {
	svc, ok := s.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("slam.Service", s)
	}

	if reconfigurable, ok := s.(*reconfigurableSlam); ok {
		return reconfigurable, nil
	}

	return &reconfigurableSlam{actual: svc}, nil
}

// Creates a file for camera data with the specified sensor name and timestamp written into the filename.
// For RGBD cameras, two filenames are created with the same timestamp in different directories.
func createTimestampFilenames(cameraName, dataDirectory, fileType string, rgbd bool) []string {
	// TODO change time format to .Format(time.RFC3339Nano) https://viam.atlassian.net/browse/DATA-277
	timeStamp := time.Now()

	if rgbd {
		colorFilename := filepath.Join(dataDirectory, "data", "rgb", cameraName+"_data_"+timeStamp.UTC().Format(slamTimeFormat)+fileType)
		depthFilename := filepath.Join(dataDirectory, "data", "depth", cameraName+"_data_"+timeStamp.UTC().Format(slamTimeFormat)+fileType)
		return []string{colorFilename, depthFilename}
	}
	filename := filepath.Join(dataDirectory, "data", cameraName+"_data_"+timeStamp.UTC().Format(slamTimeFormat)+fileType)
	return []string{filename}
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
