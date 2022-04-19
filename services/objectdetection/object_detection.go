// Package objectdetection is the service that allows you to access registered detectors and cameras
// and return bounding boxes and streams of detections. Also allows you to register new
// object detectors.
package objectdetection

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	servicepb "go.viam.com/rdk/proto/api/service/objectdetection/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
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
	config.RegisterServiceAttributeMapConverter(cType, func(attributes config.AttributeMap) (interface{}, error) {
		var attrs Attributes
		return config.TransformAttributeMapToStruct(&attrs, attributes)
	},
		&Attributes{},
	)
}

// A Service that returns  list of 2D bounding boxes and labels around objects in a 2D image.
type Service interface {
	DetectorNames(ctx context.Context) ([]string, error)
	AddDetector(ctx context.Context, cfg RegistryConfig) (bool, error)
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

// NewDetectorTypeNotImplemented is used when the detector type is not implemented.
func NewDetectorTypeNotImplemented(name string) error {
	return errors.Errorf("detector type %q is not implemented", name)
}

// Attributes contains a list of the user-provided details necessary to register a new detector.
type Attributes struct {
	Registry []RegistryConfig `json:"detector_registry"`
}

// RegistryConfig specifies the name of the detector, the type of detector,
// and the necessary parameters needed to build the detector.
type RegistryConfig struct {
	Name       string              `json:"name"`
	Type       string              `json:"type"`
	Parameters config.AttributeMap `json:"parameters"`
}

// RegisterNewDetectors take an Attributes struct and parses each element by type to create an RDK Detector
// and register it to the detector registry.
func RegisterNewDetectors(ctx context.Context, r detRegistry, attrs *Attributes, logger golog.Logger) error {
	for _, attr := range attrs.Registry {
		logger.Debugf("adding detector %q of type %s", attr.Name, attr.Type)
		switch DetectorType(attr.Type) {
		case TFLiteType:
			return NewDetectorTypeNotImplemented(attr.Type)
		case TensorFlowType:
			return NewDetectorTypeNotImplemented(attr.Type)
		case ColorType:
			err := registerColorDetector(ctx, r, &attr)
			if err != nil {
				return err
			}
		default:
			return NewDetectorTypeNotImplemented(attr.Type)
		}
	}
	return nil
}

// New registers new detectors from the config and returns a new object detection service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	attrs, ok := config.ConvertedAttributes.(*Attributes)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
	}
	detectorRegistry := make(detRegistry)
	err := RegisterNewDetectors(ctx, detectorRegistry, attrs, logger)
	if err != nil {
		return nil, err
	}
	return &objDetService{
		r:      r,
		reg:    detectorRegistry,
		logger: logger,
	}, nil
}

type objDetService struct {
	r      robot.Robot
	reg    detRegistry
	logger golog.Logger
}

// DetectorNames returns a list of the all the names of the detectors in the registry.
func (srv *objDetService) DetectorNames(ctx context.Context) ([]string, error) {
	return srv.reg.DetectorNames(), nil
}

// AddDetector adds a new detector from an Attribute config struct.
func (srv *objDetService) AddDetector(ctx context.Context, cfg RegistryConfig) (bool, error) {
	attrs := &Attributes{Registry: []RegistryConfig{cfg}}
	err := RegisterNewDetectors(ctx, srv.reg, attrs, srv.logger)
	if err != nil {
		return false, err
	}
	return true, nil
}
