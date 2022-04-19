// Package objectdetection is the service that allows you to access registered detectors and cameras
// and return bounding boxes and streams of detections. Also allows you to register new
// object detectors.
package objectdetection

import (
	"context"

	"github.com/edaniels/golog"
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
	GetDetectors(ctx context.Context) ([]string, error)
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

// New registers new detectors from the config and returns a new object detection service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	attrs, ok := config.ConvertedAttributes.(*Attributes)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
	}
	err := registerNewDetectors(attrs, logger)
	if err != nil {
		return nil, err
	}
	return &objDetService{
		r:      r,
		logger: logger,
	}, nil
}

type objDetService struct {
	r      robot.Robot
	logger golog.Logger
}

func (srv *objDetService) GetDetectors(ctx context.Context) ([]string, error) {
	return DetectorNames(), nil
}
