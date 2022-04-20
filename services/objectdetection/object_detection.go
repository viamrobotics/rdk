// Package objectdetection is the service that allows you to access registered detectors and cameras
// and return bounding boxes and streams of detections. Also allows you to register new
// object detectors.
package objectdetection

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	servicepb "go.viam.com/rdk/proto/api/service/objectdetection/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&servicepb.ObjectDetectionService_ServiceDesc,
				NewServer(subtypeSvc),
				servicepb.RegisterObjectDetectionServiceHandlerFromEndpoint,
			)
		},
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
	config.RegisterServiceAttributeMapConverter(cType, func(attributeMap config.AttributeMap) (interface{}, error) {
		var attrs attributes
		return config.TransformAttributeMapToStruct(&attrs, attributeMap)
	},
		&attributes{},
	)
}

// A Service that returns  list of 2D bounding boxes and labels around objects in a 2D image.
type Service interface {
	DetectorNames(ctx context.Context) ([]string, error)
	AddDetector(ctx context.Context, cfg Config) (bool, error)
	Detect(ctx context.Context, cameraName, detectorName string) ([]objdet.Detection, error)
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("object_detection")

// Subtype is a constant that identifies the object detection service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Name is the ObjectSegmentationService's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

// FromRobot retrieves the object detection service of a robot.
func FromRobot(r robot.Robot) (Service, error) {
	resource, err := r.ResourceByName(Name)
	if err != nil {
		return nil, utils.NewResourceNotFoundError(Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("objectdetection.Service", resource)
	}
	return svc, nil
}

// DetectorType defines what detector types are known.
type DetectorType string

// The set of allowed detector types.
const (
	TFLiteType     = DetectorType("tflite")
	TensorFlowType = DetectorType("tensorflow")
	ColorType      = DetectorType("color")
)

// Config specifies the name of the detector, the type of detector,
// and the necessary parameters needed to build the detector.
type Config struct {
	Name       string              `json:"name"`
	Type       string              `json:"type"`
	Parameters config.AttributeMap `json:"parameters"`
}

// attributes contains a list of the user-provided details necessary to register a new detector.
type attributes struct {
	Registry []Config `json:"register_detectors"`
}

// newDetectorTypeNotImplemented is used when the detector type is not implemented.
func newDetectorTypeNotImplemented(name string) error {
	return errors.Errorf("detector type %q is not implemented", name)
}

// registerNewDetectors take an attributes struct and parses each element by type to create an RDK Detector
// and register it to the detector map.
func registerNewDetectors(ctx context.Context, dm detectorMap, attrs *attributes, logger golog.Logger) error {
	_, span := trace.StartSpan(ctx, "service::objectdetection::registerNewDetectors")
	defer span.End()
	for _, attr := range attrs.Registry {
		logger.Debugf("adding detector %q of type %s", attr.Name, attr.Type)
		switch DetectorType(attr.Type) {
		case TFLiteType:
			return newDetectorTypeNotImplemented(attr.Type)
		case TensorFlowType:
			return newDetectorTypeNotImplemented(attr.Type)
		case ColorType:
			err := registerColorDetector(dm, &attr)
			if err != nil {
				return err
			}
		default:
			return newDetectorTypeNotImplemented(attr.Type)
		}
	}
	return nil
}

// New registers new detectors from the config and returns a new object detection service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	attrs, ok := config.ConvertedAttributes.(*attributes)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
	}
	detMap := make(detectorMap)
	err := registerNewDetectors(ctx, detMap, attrs, logger)
	if err != nil {
		return nil, err
	}
	return &objDetService{
		r:      r,
		reg:    detMap,
		logger: logger,
	}, nil
}

type objDetService struct {
	r      robot.Robot
	reg    detectorMap
	logger golog.Logger
}

// DetectorNames returns a list of the all the names of the detectors in the detector map.
func (srv *objDetService) DetectorNames(ctx context.Context) ([]string, error) {
	_, span := trace.StartSpan(ctx, "service::objectdetection::DetectorNames")
	defer span.End()
	return srv.reg.detectorNames(), nil
}

// AddDetector adds a new detector from an Attribute config struct.
func (srv *objDetService) AddDetector(ctx context.Context, cfg Config) (bool, error) {
	ctx, span := trace.StartSpan(ctx, "service::objectdetection::AddDetector")
	defer span.End()
	attrs := &attributes{Registry: []Config{cfg}}
	err := registerNewDetectors(ctx, srv.reg, attrs, srv.logger)
	if err != nil {
		return false, err
	}
	return true, nil
}

// Detect returns the detections of the next image from the given camera and the given detector.
func (srv *objDetService) Detect(ctx context.Context, cameraName, detectorName string) ([]objdet.Detection, error) {
	cam, err := camera.FromRobot(srv.r, cameraName)
	if err != nil {
		return nil, err
	}
	detector, err := srv.reg.detectorLookup(detectorName)
	if err != nil {
		return nil, err
	}
	img, release, err := cam.Next(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	return detector(img)
}
