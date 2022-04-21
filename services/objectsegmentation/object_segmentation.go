// Package objectsegmentation implements an object segmentation service for getting 3D objects.
package objectsegmentation

import (
	"context"
	"errors"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	servicepb "go.viam.com/rdk/proto/api/service/objectsegmentation/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
	objdet "go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/segmentation"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&servicepb.ObjectSegmentationService_ServiceDesc,
				NewServer(subtypeSvc),
				servicepb.RegisterObjectSegmentationServiceHandlerFromEndpoint,
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
}

// A Service that defines how to segment 2D and/or 3D images from a given camera into objects.
type Service interface {
	GetSegmenters(ctx context.Context) ([]string, error)
	GetSegmenterParameters(ctx context.Context, segmenterName string) ([]utils.TypedName, error)
	GetObjectPointClouds(ctx context.Context, cameraName, segmenterName string, params config.AttributeMap) ([]*vision.Object, error)
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("object_segmentation")

// Subtype is a constant that identifies the object segmentation service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Name is the ObjectSegmentationService's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

// FromRobot retrieves the object segmentation service of a robot.
func FromRobot(r robot.Robot) (Service, error) {
	resource, err := r.ResourceByName(Name)
	if err != nil {
		return nil, utils.NewResourceNotFoundError(Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("objectsegmentation.Service", resource)
	}
	return svc, nil
}

func DetectorToSegmenter(seg Service, detectorName string, detector objdet.Detector) error {
	srv, ok := seg.(*objectSegService)
	if !ok {
		return errors.New("adding segmenters is only supported on segmentation service hosted on local robots")
	}
	detSegmenter, params, err := segmentation.DetectionSegmenter(detector)
	if err != nil {
		return err
	}
	return srv.reg.registerSegmenter(detectorName, SegmenterRegistration{detSegmenter, params})
}

// radiusClusteringSegmenter is  the name of a segmenter that finds well separated objects on a flat plane.
const RadiusClusteringSegmenter = "radius_clustering"

// New returns a new object segmentation service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	segMap := make(segmenterMap)
	// RadiusClustering is a default segmentation algorithm
	err := segMap.registerSegmenter(RadiusClusteringSegmenter, SegmenterRegistration{
		segmentation.Segmenter(segmentation.RadiusClustering),
		utils.JSONTags(segmentation.RadiusClusteringConfig{}),
	})
	if err != nil {
		return nil, err
	}
	return &objectSegService{
		r:      r,
		logger: logger,
		reg:    segMap,
	}, nil
}

type objectSegService struct {
	r      robot.Robot
	logger golog.Logger
	reg    segmenterMap
}

func (seg *objectSegService) GetObjectPointClouds(
	ctx context.Context,
	cameraName, segmenterName string,
	params config.AttributeMap,
) ([]*vision.Object, error) {
	cam, err := camera.FromRobot(seg.r, cameraName)
	if err != nil {
		return nil, err
	}
	segmenter, err := seg.reg.segmenterLookup(segmenterName)
	if err != nil {
		return nil, err
	}
	return segmenter.Segmenter(ctx, cam, params)
}

func (seg *objectSegService) GetSegmenters(ctx context.Context) ([]string, error) {
	return seg.reg.segmenterNames(), nil
}

func (seg *objectSegService) GetSegmenterParameters(ctx context.Context, segmenterName string) ([]utils.TypedName, error) {
	segmenter, err := seg.reg.segmenterLookup(segmenterName)
	if err != nil {
		return nil, err
	}
	return segmenter.Parameters, nil
}
