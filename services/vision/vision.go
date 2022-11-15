// Package vision is the service that allows you to access various computer vision algorithms
// (like detection, segmentation, tracking, etc) that usually only require a camera or image input.
package vision

import (
	"context"
	"image"
	"sync"

	"github.com/edaniels/golog"
	"github.com/invopop/jsonschema"
	servicepb "go.viam.com/api/service/vision/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	objdet "go.viam.com/rdk/vision/objectdetection"
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
		MaxInstance:    resource.DefaultMaxInstance,
	})
}

// A Service that implements various computer vision algorithms like detection and segmentation.
type Service interface {
	// model parameters
	GetModelParameterSchema(ctx context.Context, modelType VisModelType, extra map[string]interface{}) (*jsonschema.Schema, error)
	// detector methods
	DetectorNames(ctx context.Context, extra map[string]interface{}) ([]string, error)
	AddDetector(ctx context.Context, cfg VisModelConfig, extra map[string]interface{}) error
	RemoveDetector(ctx context.Context, detectorName string, extra map[string]interface{}) error
	DetectionsFromCamera(ctx context.Context, cameraName, detectorName string, extra map[string]interface{}) ([]objdet.Detection, error)
	Detections(ctx context.Context, img image.Image, detectorName string, extra map[string]interface{}) ([]objdet.Detection, error)
	// classifier methods
	ClassifierNames(ctx context.Context, extra map[string]interface{}) ([]string, error)
	AddClassifier(ctx context.Context, cfg VisModelConfig, extra map[string]interface{}) error
	RemoveClassifier(ctx context.Context, classifierName string, extra map[string]interface{}) error
	ClassificationsFromCamera(
		ctx context.Context,
		cameraName, classifierName string,
		n int,
		extra map[string]interface{},
	) (classification.Classifications, error)
	Classifications(
		ctx context.Context,
		img image.Image,
		classifierName string,
		n int,
		extra map[string]interface{},
	) (classification.Classifications, error)
	// segmenter methods
	SegmenterNames(ctx context.Context, extra map[string]interface{}) ([]string, error)
	AddSegmenter(ctx context.Context, cfg VisModelConfig, extra map[string]interface{}) error
	RemoveSegmenter(ctx context.Context, segmenterName string, extra map[string]interface{}) error
	GetObjectPointClouds(ctx context.Context, cameraName, segmenterName string, extra map[string]interface{}) ([]*viz.Object, error)
}

var (
	_ = Service(&reconfigurableVision{})
	_ = resource.Reconfigurable(&reconfigurableVision{})
	_ = goutils.ContextCloser(&reconfigurableVision{})
)

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((Service)(nil), actual)
}

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

// FromRobot is a helper for getting the named vision service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	resource, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, utils.NewResourceNotFoundError(Named(name))
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(resource)
	}
	return svc, nil
}

// FindFirstName returns name of first vision service found.
func FindFirstName(r robot.Robot) string {
	for _, val := range robot.NamesBySubtype(r, Subtype) {
		return val
	}
	return ""
}

// FirstFromRobot returns the first vision service in this robot.
func FirstFromRobot(r robot.Robot) (Service, error) {
	name := FindFirstName(r)
	return FromRobot(r, name)
}

// VisModelType defines what vision models are known by the vision service.
type VisModelType string

// VisModelConfig specifies the name of the detector, the type of detector,
// and the necessary parameters needed to build the detector.
type VisModelConfig struct {
	Name       string              `json:"name"`
	Type       string              `json:"type"`
	Parameters config.AttributeMap `json:"parameters"`
}

// Attributes contains a list of the user-provided details necessary to register a new vision service.
type Attributes struct {
	ModelRegistry []VisModelConfig `json:"register_models"`
}

type reconfigurableVision struct {
	mu     sync.RWMutex
	actual Service
}

func (svc *reconfigurableVision) GetModelParameterSchema(
	ctx context.Context,
	modelType VisModelType,
	extra map[string]interface{},
) (*jsonschema.Schema, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetModelParameterSchema(ctx, modelType, extra)
}

func (svc *reconfigurableVision) DetectorNames(ctx context.Context, extra map[string]interface{}) ([]string, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.DetectorNames(ctx, extra)
}

func (svc *reconfigurableVision) AddDetector(ctx context.Context, cfg VisModelConfig, extra map[string]interface{}) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.AddDetector(ctx, cfg, extra)
}

func (svc *reconfigurableVision) RemoveDetector(ctx context.Context, detectorName string, extra map[string]interface{}) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.RemoveDetector(ctx, detectorName, extra)
}

func (svc *reconfigurableVision) DetectionsFromCamera(
	ctx context.Context,
	cameraName, detectorName string,
	extra map[string]interface{},
) ([]objdet.Detection, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.DetectionsFromCamera(ctx, cameraName, detectorName, extra)
}

func (svc *reconfigurableVision) Detections(
	ctx context.Context,
	img image.Image,
	detectorName string,
	extra map[string]interface{},
) ([]objdet.Detection, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.Detections(ctx, img, detectorName, extra)
}

func (svc *reconfigurableVision) ClassifierNames(ctx context.Context, extra map[string]interface{}) ([]string, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.ClassifierNames(ctx, extra)
}

func (svc *reconfigurableVision) AddClassifier(ctx context.Context, cfg VisModelConfig, extra map[string]interface{}) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.AddClassifier(ctx, cfg, extra)
}

func (svc *reconfigurableVision) RemoveClassifier(ctx context.Context, classifierName string, extra map[string]interface{}) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.RemoveDetector(ctx, classifierName, extra)
}

func (svc *reconfigurableVision) ClassificationsFromCamera(ctx context.Context, cameraName,
	classifierName string, n int, extra map[string]interface{},
) (classification.Classifications, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.ClassificationsFromCamera(ctx, cameraName, classifierName, n, extra)
}

func (svc *reconfigurableVision) Classifications(ctx context.Context, img image.Image,
	classifierName string, n int, extra map[string]interface{},
) (classification.Classifications, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.Classifications(ctx, img, classifierName, n, extra)
}

func (svc *reconfigurableVision) SegmenterNames(ctx context.Context, extra map[string]interface{}) ([]string, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.SegmenterNames(ctx, extra)
}

func (svc *reconfigurableVision) AddSegmenter(ctx context.Context, cfg VisModelConfig, extra map[string]interface{}) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.AddSegmenter(ctx, cfg, extra)
}

func (svc *reconfigurableVision) RemoveSegmenter(ctx context.Context, segmenterName string, extra map[string]interface{}) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.RemoveSegmenter(ctx, segmenterName, extra)
}

func (svc *reconfigurableVision) GetObjectPointClouds(ctx context.Context,
	cameraName,
	segmenterName string,
	extra map[string]interface{},
) ([]*viz.Object, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetObjectPointClouds(ctx, cameraName, segmenterName, extra)
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
		golog.Global().Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	/*
		theOldServ := svc.actual.(*visionService)
		theNewSerc := rSvc.actual.(*visionService)
		*theOldServ = *theNewSerc
	*/
	return nil
}

// WrapWithReconfigurable wraps a vision service as a Reconfigurable.
func WrapWithReconfigurable(s interface{}) (resource.Reconfigurable, error) {
	svc, ok := s.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(s)
	}

	if reconfigurable, ok := s.(*reconfigurableVision); ok {
		return reconfigurable, nil
	}

	return &reconfigurableVision{actual: svc}, nil
}
