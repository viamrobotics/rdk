// Package vision is the service that allows you to access various computer vision algorithms
// (like detection, segmentation, tracking, etc) that usually only require a camera or image input.
package vision

import (
	"context"
	"image"
	"sync"

	"github.com/edaniels/golog"
	"go.opencensus.io/trace"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	servicepb "go.viam.com/rdk/proto/api/service/vision/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
	viz "go.viam.com/rdk/vision"
	objdet "go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/segmentation"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&servicepb.VisionService_ServiceDesc,
				NewServer(subtypeSvc),
				servicepb.RegisterVisionServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &servicepb.VisionService_ServiceDesc,
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
	config.RegisterServiceAttributeMapConverter(cType, func(attributeMap config.AttributeMap) (interface{}, error) {
		var attrs Attributes
		return config.TransformAttributeMapToStruct(&attrs, attributeMap)
	},
		&Attributes{},
	)

	resource.AddDefaultService(Named(resource.DefaultServiceName))
}

// A Service that implements various computer vision algorithms like detection and segmentation.
type Service interface {
	// detector methods
	GetDetectorNames(ctx context.Context) ([]string, error)
	AddDetector(ctx context.Context, cfg DetectorConfig) error
	GetDetectionsFromCamera(ctx context.Context, cameraName, detectorName string) ([]objdet.Detection, error)
	GetDetections(ctx context.Context, img image.Image, detectorName string) ([]objdet.Detection, error)
	// segmenter methods
	GetSegmenterNames(ctx context.Context) ([]string, error)
	GetSegmenterParameters(ctx context.Context, segmenterName string) ([]utils.TypedName, error)
	GetObjectPointClouds(ctx context.Context, cameraName, segmenterName string, params config.AttributeMap) ([]*viz.Object, error)
}

var (
	_ = Service(&reconfigurableVision{})
	_ = resource.Reconfigurable(&reconfigurableVision{})
	_ = goutils.ContextCloser(&reconfigurableVision{})
)

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("vision")

// Subtype is a constant that identifies the vision service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named vision's typed resource name.
// RSDK-347 Implements vision's Named.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// FromRobot retrieves the vision service of a robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	resource, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, utils.NewResourceNotFoundError(Named(name))
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("vision.Service", resource)
	}
	return svc, nil
}

// FindVisionName Returns name of first vision service found.
func FindVisionName(r robot.Robot) string {
	for _, val := range robot.NamesBySubtype(r, Subtype) {
		return val
	}
	return resource.DefaultServiceName
}

// Attributes contains a list of the user-provided details necessary to register a new vision service.
type Attributes struct {
	DetectorRegistry []DetectorConfig `json:"register_detectors"`
}

// New registers new detectors from the config and returns a new object detection service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	detMap := make(detectorMap)
	segMap := make(segmenterMap)
	// register default segmenters
	err := segMap.registerSegmenter(RadiusClusteringSegmenter, SegmenterRegistration{
		segmentation.Segmenter(segmentation.RadiusClustering),
		utils.JSONTags(segmentation.RadiusClusteringConfig{}),
	}, logger)
	if err != nil {
		return nil, err
	}
	// register detectors and user defined things if config is defined
	if config.ConvertedAttributes != nil {
		attrs, ok := config.ConvertedAttributes.(*Attributes)
		if !ok {
			return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
		}
		err = registerNewDetectors(ctx, detMap, attrs, logger)
		if err != nil {
			return nil, err
		}
	}
	service := &visionService{
		r:      r,
		detReg: detMap,
		segReg: segMap,
		logger: logger,
	}
	// turn detectors into segmenters
	for _, detName := range service.detReg.detectorNames() {
		err := service.registerSegmenterFromDetector(detName, logger)
		if err != nil {
			return nil, err
		}
	}
	return service, nil
}

type visionService struct {
	r      robot.Robot
	detReg detectorMap
	segReg segmenterMap
	logger golog.Logger
}

// Detection Methods
// GetDetectorNames returns a list of the all the names of the detectors in the detector map.
func (vs *visionService) GetDetectorNames(ctx context.Context) ([]string, error) {
	_, span := trace.StartSpan(ctx, "service::vision::GetDetectorNames")
	defer span.End()
	return vs.detReg.detectorNames(), nil
}

// AddDetector adds a new detector from an Attribute config struct.
func (vs *visionService) AddDetector(ctx context.Context, cfg DetectorConfig) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::AddDetector")
	defer span.End()
	attrs := &Attributes{DetectorRegistry: []DetectorConfig{cfg}}
	err := registerNewDetectors(ctx, vs.detReg, attrs, vs.logger)
	if err != nil {
		return err
	}
	// also create a new segmenter from the detector
	return vs.registerSegmenterFromDetector(cfg.Name, vs.logger)
}

// GetDetections returns the detections of the next image from the given camera and the given detector.
func (vs *visionService) GetDetectionsFromCamera(ctx context.Context, cameraName, detectorName string) ([]objdet.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::GetDetectionsFromCamera")
	defer span.End()
	cam, err := camera.FromRobot(vs.r, cameraName)
	if err != nil {
		return nil, err
	}
	detector, err := vs.detReg.detectorLookup(detectorName)
	if err != nil {
		return nil, err
	}
	img, release, err := cam.Next(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	return detector(ctx, img)
}

// GetDetections returns the detections of the next image from the given camera and the given detector.
func (vs *visionService) GetDetections(ctx context.Context, img image.Image, detectorName string,
) ([]objdet.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::GetDetections")
	defer span.End()

	detector, err := vs.detReg.detectorLookup(detectorName)
	if err != nil {
		return nil, err
	}

	return detector(ctx, img)
}

// Segmentation Methods
// GetSegmenterNames returns a list of all the names of the segmenters in the segmenter map.
func (vs *visionService) GetSegmenterNames(ctx context.Context) ([]string, error) {
	_, span := trace.StartSpan(ctx, "service::vision::GetSegmenterNames")
	defer span.End()
	return vs.segReg.segmenterNames(), nil
}

// GetSegmenterParameters returns a list of parameter name and type for the necessary parameters of the chosen segmenter.
func (vs *visionService) GetSegmenterParameters(ctx context.Context, segmenterName string) ([]utils.TypedName, error) {
	_, span := trace.StartSpan(ctx, "service::vision::GetSegmenterParameters")
	defer span.End()
	segmenter, err := vs.segReg.segmenterLookup(segmenterName)
	if err != nil {
		return nil, err
	}
	return segmenter.Parameters, nil
}

// GetObjectPointClouds returns all the found objects in a 3D image according to the chosen segmenter.
func (vs *visionService) GetObjectPointClouds(
	ctx context.Context,
	cameraName, segmenterName string,
	params config.AttributeMap,
) ([]*viz.Object, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::GetObjectPointClouds")
	defer span.End()
	cam, err := camera.FromRobot(vs.r, cameraName)
	if err != nil {
		return nil, err
	}
	segmenter, err := vs.segReg.segmenterLookup(segmenterName)
	if err != nil {
		return nil, err
	}
	return segmenter.Segmenter(ctx, cam, params)
}

// Helpers
// registerSegmenterFromDetector creates and registers a segmenter from an already registered detector.
func (vs *visionService) registerSegmenterFromDetector(detName string, logger golog.Logger) error {
	det, err := vs.detReg.detectorLookup(detName)
	if err != nil {
		return err
	}
	detSegmenter, params, err := segmentation.DetectionSegmenter(det)
	if err != nil {
		return err
	}
	return vs.segReg.registerSegmenter(detName, SegmenterRegistration{detSegmenter, params}, logger)
}

// Close removes all existing detectors from the vision service.
func (vs *visionService) Close() error {
	detectors := vs.detReg.detectorNames()
	for _, detectorName := range detectors {
		err := vs.detReg.removeDetector(detectorName, vs.logger)
		if err != nil {
			return err
		}
	}
	return nil
}

type reconfigurableVision struct {
	mu     sync.RWMutex
	actual Service
}

func (svc *reconfigurableVision) GetDetectorNames(ctx context.Context) ([]string, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetDetectorNames(ctx)
}

func (svc *reconfigurableVision) AddDetector(ctx context.Context, cfg DetectorConfig) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.AddDetector(ctx, cfg)
}

func (svc *reconfigurableVision) GetDetectionsFromCamera(ctx context.Context, cameraName, detectorName string) ([]objdet.Detection, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetDetectionsFromCamera(ctx, cameraName, detectorName)
}

func (svc *reconfigurableVision) GetDetections(ctx context.Context, img image.Image, detectorName string,
) ([]objdet.Detection, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetDetections(ctx, img, detectorName)
}

func (svc *reconfigurableVision) GetSegmenterNames(ctx context.Context) ([]string, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetSegmenterNames(ctx)
}

func (svc *reconfigurableVision) GetSegmenterParameters(ctx context.Context, segmenterName string) ([]utils.TypedName, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetSegmenterParameters(ctx, segmenterName)
}

func (svc *reconfigurableVision) GetObjectPointClouds(ctx context.Context,
	cameraName,
	segmenterName string,
	params config.AttributeMap,
) ([]*viz.Object, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetObjectPointClouds(ctx, cameraName, segmenterName, params)
}

func (svc *reconfigurableVision) Close(ctx context.Context) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return goutils.TryClose(ctx, svc.actual)
}

// Reconfigure replaces the old vision service with a new vision.
func (svc *reconfigurableVision) Reconfigure(ctx context.Context, newSvc resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newSvc.(*reconfigurableVision)
	if !ok {
		return utils.NewUnexpectedTypeError(svc, newSvc)
	}
	if err := goutils.TryClose(ctx, svc.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// WrapWithReconfigurable wraps a vision service as a Reconfigurable.
func WrapWithReconfigurable(s interface{}) (resource.Reconfigurable, error) {
	svc, ok := s.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("vision.Service", s)
	}

	if reconfigurable, ok := s.(*reconfigurableVision); ok {
		return reconfigurable, nil
	}

	return &reconfigurableVision{actual: svc}, nil
}
