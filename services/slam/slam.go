// Package slam implements simultaneous localization and mapping
package slam

import (
	"bufio"
	"bytes"
	"context"
	"image"
	"image/jpeg"
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
func runtimeConfigValidation(svcConfig *AttrConfig, logger golog.Logger) error {
	slamLib, ok := SLAMLibraries[svcConfig.Algorithm]
	if !ok {
		return errors.Errorf("%v algorithm specified not in implemented list", svcConfig.Algorithm)
	}

	if _, ok := slamLib.SlamMode[svcConfig.ConfigParams["mode"]]; !ok {
		return errors.Errorf("getting data with specified algorithm %v, and desired mode %v",
			svcConfig.Algorithm, svcConfig.ConfigParams["mode"])
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
		startFileIndex, err := strconv.Atoi(res2[0])
		if err != nil {
			return err
		}
		endFileIndex, err := strconv.Atoi(res2[1])
		if err != nil {
			return err
		}

		interval, err := strconv.Atoi(res2[2])
		if err != nil {
			return err
		}

		if interval == 0 {
			return errors.New("the file input pattern's interval must be greater than zero")
		}

		if startFileIndex > endFileIndex {
			return errors.Errorf("second value in input file pattern must be larger than the first [%v]", svcConfig.InputFilePattern)
		}
	}

	return nil
}

// runtimeServiceValidation ensures the service's data processing and saving is valid for the mode and cam given.
func runtimeServiceValidation(ctx context.Context, cam camera.Camera, slamSvc *slamService) error {
	if cam == nil {
		return nil
	}

	var err error
	var path string
	startTime := time.Now()

	// TODO 05/05/2022: This will be removed once GRPC data transfer is available as the responsibility for
	// calling the right algorithms (Next vs NextPointCloud) will be held by the slam libraries themselves
	// Note: if GRPC data transfer is delayed to after other algorithms (or user custom algos) are being
	// added this point will be revisited
	for {
		switch slamSvc.slamLib.AlgoType {
		case sparse:
			path, err = slamSvc.getAndSaveDataSparse(ctx, cam)
		case dense:
			path, err = slamSvc.getAndSaveDataDense(ctx, cam)
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

	if err := os.RemoveAll(path); err != nil {
		return errors.Wrap(err, "error removing generated file during validation")
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

// configureCamera will check the config to see if a camera is desired and if so, grab the camera from
// the robot as well as get the intrinsic associated with it.
func configureCamera(svcConfig *AttrConfig, r robot.Robot, logger golog.Logger) (string, camera.Camera, error) {
	var cam camera.Camera
	var cameraName string
	var err error
	if len(svcConfig.Sensors) > 0 {
		logger.Debug("Running in live mode")
		cameraName = svcConfig.Sensors[0]
		cam, err = camera.FromRobot(r, cameraName)
		if err != nil {
			return "", nil, errors.Wrap(err, "error getting camera for slam service")
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
	ctx, span := trace.StartSpan(ctx, "slam::GetMap")
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

	cameraName, cam, err := configureCamera(svcConfig, r, logger)
	if err != nil {
		return nil, errors.Wrap(err, "configuring camera error")
	}

	if err := runtimeConfigValidation(svcConfig, logger); err != nil {
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

	slamLib := SLAMLibraries[svcConfig.Algorithm]
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

	if err := runtimeServiceValidation(cancelCtx, cam, slamSvc); err != nil {
		return nil, errors.Wrap(err, "runtime slam service error")
	}

	slamSvc.StartDataProcess(cancelCtx, cam)

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
func (slamSvc *slamService) StartDataProcess(cancelCtx context.Context, cam camera.Camera) {
	if cam == nil {
		return
	}

	slamSvc.activeBackgroundWorkers.Add(1)
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
					if _, err := slamSvc.getAndSaveDataDense(cancelCtx, cam); err != nil {
						slamSvc.logger.Warn(err)
					}
				case sparse:
					if _, err := slamSvc.getAndSaveDataSparse(cancelCtx, cam); err != nil {
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
func (slamSvc *slamService) getAndSaveDataSparse(ctx context.Context, cam camera.Camera) (string, error) {
	ctx, span := trace.StartSpan(ctx, "slam::slamService::getAndSaveDataSparse")
	defer span.End()

	img, _, err := cam.Next(ctx)
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
	case dim2d, dim3d:
		return "", errors.Errorf("bad slamMode %v specified for this algorithm", slamSvc.slamMode)
	}

	filename := createTimestampFilename(slamSvc.cameraName, slamSvc.dataDirectory, fileType)
	//nolint:gosec
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
	case dim2d, dim3d:
		return "", errors.Errorf("bad slamMode %v specified for this algorithm", slamSvc.slamMode)
	}
	if err = w.Flush(); err != nil {
		return filename, err
	}
	return filename, f.Close()
}

// getAndSaveDataDense implements the data extraction for dense algos and saving to the directory path (data subfolder) specified in
// the config. It returns the full filepath for each file saved along with any error associated with the data creation or saving.
func (slamSvc *slamService) getAndSaveDataDense(ctx context.Context, cam camera.Camera) (string, error) {
	ctx, span := trace.StartSpan(ctx, "slam::slamService::getAndSaveDataDense")
	defer span.End()

	pointcloud, err := cam.NextPointCloud(ctx)
	if err != nil {
		if err.Error() == "bad scan: OpTimeout" {
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
	filename := createTimestampFilename(slamSvc.cameraName, slamSvc.dataDirectory, fileType)
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
func createTimestampFilename(cameraName, dataDirectory, fileType string) string {
	timeStamp := time.Now()
	filename := filepath.Join(dataDirectory, "data", cameraName+"_data_"+timeStamp.UTC().Format("2006-01-02T15_04_05.0000")+fileType)

	return filename
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
