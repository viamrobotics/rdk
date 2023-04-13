// Package slam implements simultaneous localization and mapping.
// This is an Experimental package.
package slam

import (
	"context"
	"io"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	pb "go.viam.com/api/service/slam/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
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
	return utils.NewUnimplementedInterfaceError((*Service)(nil), actual)
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

// FromRobot is a helper for getting the named SLAM service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

var (
	_ = Service(&reconfigurableSlam{})
	_ = resource.Reconfigurable(&reconfigurableSlam{})
	_ = goutils.ContextCloser(&reconfigurableSlam{})
)

// Service describes the functions that are available to the service.
type Service interface {
	GetPosition(context.Context) (spatialmath.Pose, string, error)
	GetPointCloudMap(ctx context.Context) (func() ([]byte, error), error)
	GetInternalState(ctx context.Context) (func() ([]byte, error), error)
	resource.Generic
}

// Helper function that concatenates the chunks from a streamed grpc endpoint.
func helperConcatenateChunksToFull(f func() ([]byte, error)) ([]byte, error) {
	var fullBytes []byte
	for {
		chunk, err := f()
		if errors.Is(err, io.EOF) {
			return fullBytes, nil
		}
		if err != nil {
			return nil, err
		}

		fullBytes = append(fullBytes, chunk...)
	}
}

// GetPointCloudMapFull concatenates the streaming responses from GetPointCloudMap into a full point cloud.
func GetPointCloudMapFull(ctx context.Context, slamSvc Service) ([]byte, error) {
	ctx, span := trace.StartSpan(ctx, "slam::GetPointCloudMapFull")
	defer span.End()
	callback, err := slamSvc.GetPointCloudMap(ctx)
	if err != nil {
		return nil, err
	}
	return helperConcatenateChunksToFull(callback)
}

// GetInternalStateFull concatenates the streaming responses from GetInternalState into
// the internal serialized state of the slam algorithm.
func GetInternalStateFull(ctx context.Context, slamSvc Service) ([]byte, error) {
	ctx, span := trace.StartSpan(ctx, "slam::GetInternalStateFull")
	defer span.End()
	callback, err := slamSvc.GetInternalState(ctx)
	if err != nil {
		return nil, err
	}
	return helperConcatenateChunksToFull(callback)
}

type reconfigurableSlam struct {
	mu     sync.RWMutex
	name   resource.Name
	actual Service
}

func (svc *reconfigurableSlam) Name() resource.Name {
	return svc.name
}

func (svc *reconfigurableSlam) GetPosition(ctx context.Context) (spatialmath.Pose, string, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetPosition(ctx)
}

func (svc *reconfigurableSlam) GetPointCloudMap(ctx context.Context) (func() ([]byte, error), error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetPointCloudMap(ctx)
}

func (svc *reconfigurableSlam) GetInternalState(ctx context.Context) (func() ([]byte, error), error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetInternalState(ctx)
}

func (svc *reconfigurableSlam) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.DoCommand(ctx, cmd)
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
