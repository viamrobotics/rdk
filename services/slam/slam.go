// Package slam implements simultaneous localization and mapping
// This is an Experimental package
package slam

import (
	"context"
	"image"
	"sync"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/service/slam/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
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
		Reconfigurable: WrapWithReconfigurable,
	})
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((Service)(nil), actual)
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("slam")

// Subtype is a constant that identifies the slam resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named service's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

var (
	_ = Service(&reconfigurableSlam{})
	_ = resource.Reconfigurable(&reconfigurableSlam{})
	_ = goutils.ContextCloser(&reconfigurableSlam{})
)

// Service describes the functions that are available to the service.
type Service interface {
	Position(context.Context, string, map[string]interface{}) (*referenceframe.PoseInFrame, error)
	GetMap(
		context.Context,
		string,
		string,
		*referenceframe.PoseInFrame,
		bool,
		map[string]interface{},
	) (string, image.Image, *vision.Object, error)
}

type reconfigurableSlam struct {
	mu     sync.RWMutex
	name   resource.Name
	actual Service
}

func (svc *reconfigurableSlam) Name() resource.Name {
	return svc.name
}

func (svc *reconfigurableSlam) Position(
	ctx context.Context,
	val string,
	extra map[string]interface{},
) (*referenceframe.PoseInFrame, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.Position(ctx, val, extra)
}

func (svc *reconfigurableSlam) GetMap(ctx context.Context,
	name string,
	mimeType string,
	cp *referenceframe.PoseInFrame,
	include bool,
	extra map[string]interface{},
) (string, image.Image, *vision.Object, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetMap(ctx, name, mimeType, cp, include, extra)
}

func (svc *reconfigurableSlam) Close(ctx context.Context) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return goutils.TryClose(ctx, svc.actual)
}

// Reconfigure replaces the old slam service with a new slam.
func (svc *reconfigurableSlam) Reconfigure(ctx context.Context, newSvc resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newSvc.(*reconfigurableSlam)
	if !ok {
		return utils.NewUnexpectedTypeError(svc, newSvc)
	}
	if err := goutils.TryClose(ctx, svc.actual); err != nil {
		golog.Global().Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// WrapWithReconfigurable wraps a slam service as a Reconfigurable.
func WrapWithReconfigurable(s interface{}, name resource.Name) (resource.Reconfigurable, error) {
	svc, ok := s.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(s)
	}

	if reconfigurable, ok := s.(*reconfigurableSlam); ok {
		return reconfigurable, nil
	}

	return &reconfigurableSlam{name: name, actual: svc}, nil
}
