// Package object segmentation implements an object segmentation service for getting 3D objects.
package objectsegmentation

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	servicepb "go.viam.com/rdk/proto/api/service/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/segmentation"
	"go.viam.com/utils/rpc"
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

// A Service controls the flow of manipulating other objects with a robot's gripper.
type Service interface {
	GetObjects(ctx context.Context, cameraName string, parameters *vision.Parameters3D) ([]*vision.Object, error)
}

// An ObjectSource3D is anything that generates 3D objects in a scene.
type ObjectSource3D interface {
	NextObjects(ctx context.Context, parameters *vision.Parameters3D) ([]*vision.Object, error)
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

// FromRobot retrieves the object manipulation service of a robot.
func FromRobot(r robot.Robot) (Service, error) {
	resource, ok := r.ResourceByName(Name)
	if !ok {
		return nil, errors.Errorf("resource %q not found", Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("objectsegmentation.Service", resource)
	}
	return svc, nil
}

// New returns a new object segmentation service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	return &objectSegService{
		r:      r,
		logger: logger,
	}, nil
}

type objectSegService struct {
	r      robot.Robot
	logger golog.Logger
}

func (seg *objectSegService) GetObjects(ctx context.Context, cameraName string, pmtrs *vision.Parameters3D) ([]*vision.Object, error) {
	// get camera component
	cam, err := camera.FromRobot(seg.r, cameraName)
	if err != nil {
		return nil, err
	}
	// return next objects if camera has a NextObjects defined
	if c, ok := cam.(ObjectSource3D); ok {
		return c.NextObjects(ctx, pmtrs)
	}
	if c, ok := utils.UnwrapProxy(cam).(ObjectSource3D); ok {
		return c.NextObjects(ctx, pmtrs)
	}
	// do default segmentation otherwise
	cloud, err := cam.NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}
	segments, err := segmentation.NewObjectSegmentation(ctx, cloud, pmtrs) // default object segmentation
	if err != nil {
		return nil, err
	}
	return segments.Objects(), nil
}
