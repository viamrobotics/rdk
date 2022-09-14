// Package motion implements an motion service.
package motion

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	servicepb "go.viam.com/rdk/proto/api/service/motion/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&servicepb.MotionService_ServiceDesc,
				NewServer(subtypeSvc),
				servicepb.RegisterMotionServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &servicepb.MotionService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
		Reconfigurable: WrapWithReconfigurable,
	})
}

// A Service controls the flow of moving components.
type Service interface {
	Move(
		ctx context.Context,
		componentName resource.Name,
		destination *referenceframe.PoseInFrame,
		worldState *commonpb.WorldState,
	) (bool, error)
	MoveSingleComponent(
		ctx context.Context,
		componentName resource.Name,
		destination *referenceframe.PoseInFrame,
		worldState *commonpb.WorldState,
	) (bool, error)
	GetPose(
		ctx context.Context,
		componentName resource.Name,
		destinationFrame string,
		supplementalTransforms []*commonpb.Transform,
	) (*referenceframe.PoseInFrame, error)
}

var (
	_ = Service(&reconfigurableMotionService{})
	_ = resource.Reconfigurable(&reconfigurableMotionService{})
	_ = goutils.ContextCloser(&reconfigurableMotionService{})
)

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("motion")

// Subtype is a constant that identifies the motion service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named motion service's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((Service)(nil), actual)
}

// FromRobot is a helper for getting the named motion service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	resource, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(resource)
	}
	return svc, nil
}

type reconfigurableMotionService struct {
	mu     sync.RWMutex
	actual Service
}

func (svc *reconfigurableMotionService) Move(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *commonpb.WorldState,
) (bool, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.Move(ctx, componentName, destination, worldState)
}

func (svc *reconfigurableMotionService) MoveSingleComponent(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *commonpb.WorldState,
) (bool, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.MoveSingleComponent(ctx, componentName, destination, worldState)
}

func (svc *reconfigurableMotionService) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
	supplementalTransforms []*commonpb.Transform,
) (*referenceframe.PoseInFrame, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetPose(ctx, componentName, destinationFrame, supplementalTransforms)
}

func (svc *reconfigurableMotionService) Close(ctx context.Context) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return goutils.TryClose(ctx, svc.actual)
}

// Reconfigure replaces the old Motion Service with a new Motion Service.
func (svc *reconfigurableMotionService) Reconfigure(ctx context.Context, newSvc resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newSvc.(*reconfigurableMotionService)
	if !ok {
		return utils.NewUnexpectedTypeError(svc, newSvc)
	}
	if err := goutils.TryClose(ctx, svc.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// WrapWithReconfigurable wraps a Motion Service as a Reconfigurable.
func WrapWithReconfigurable(s interface{}) (resource.Reconfigurable, error) {
	svc, ok := s.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(s)
	}

	if reconfigurable, ok := s.(*reconfigurableMotionService); ok {
		return reconfigurable, nil
	}

	return &reconfigurableMotionService{actual: svc}, nil
}
