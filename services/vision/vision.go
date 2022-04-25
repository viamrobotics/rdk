// Package vision is the service that allows you to access registered detectors and cameras
// and return bounding boxes and streams of detections. Also allows you to register new
// object detectors.
package vision

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	servicepb "go.viam.com/rdk/proto/api/service/vision/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
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
	// detector methods
	DetectorNames(ctx context.Context) ([]string, error)
	AddDetector(ctx context.Context, cfg DetectorConfig) (bool, error)
	Detect(ctx context.Context, cameraName, detectorName string) ([]objdet.Detection, error)
	// segmenter methods
	GetSegmenters(ctx context.Context) ([]string, error)
	GetSegmenterParameters(ctx context.Context, segmenterName string) ([]utils.TypedName, error)
	GetObjectPointClouds(ctx context.Context, cameraName, segmenterName string, params config.AttributeMap) ([]*viz.Object, error)
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("vision")

// Subtype is a constant that identifies the object detection service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Name is the VisionService's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

// FromRobot retrieves the object detection service of a robot.
func FromRobot(r robot.Robot) (Service, error) {
	resource, err := r.ResourceByName(Name)
	if err != nil {
		return nil, utils.NewResourceNotFoundError(Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("vision.Service", resource)
	}
	return svc, nil
}

// attributes contains a list of the user-provided details necessary to register a new vision service.
type attributes struct {
	DetectorRegistry []DetectorConfig `json:"register_detectors"`
}

// New registers new detectors from the config and returns a new object detection service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	attrs, ok := config.ConvertedAttributes.(*attributes)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
	}
	detMap := make(detectorMap)
	segMap := make(segmenterMap)
	// register segmenters
	err := segMap.registerSegmenter(RadiusClusteringSegmenter, SegmenterRegistration{
		segmentation.Segmenter(segmentation.RadiusClustering),
		utils.JSONTags(segmentation.RadiusClusteringConfig{}),
	})
	if err != nil {
		return nil, err
	}
	// register detectors
	err = registerNewDetectors(ctx, detMap, attrs, logger)
	if err != nil {
		return nil, err
	}
	return &visionService{
		r:      r,
		detReg: detMap,
		segReg: segMap,
		logger: logger,
	}, nil
}

type visionService struct {
	r      robot.Robot
	detReg detectorMap
	segReg segmenterMap
	logger golog.Logger
}
