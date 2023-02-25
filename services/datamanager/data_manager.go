// Package datamanager contains a service type that can be used to capture data from a robot's components.
package datamanager

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	servicepb "go.viam.com/api/service/datamanager/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&servicepb.DataManagerService_ServiceDesc,
				NewServer(subtypeSvc),
				servicepb.RegisterDataManagerServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &servicepb.DataManagerService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
		Reconfigurable: WrapWithReconfigurable,
		MaxInstance:    resource.DefaultMaxInstance,
	})
}

// Service defines what a Data Manager Service should expose to the users.
type Service interface {
	Sync(ctx context.Context, extra map[string]interface{}) error
	resource.Generic
}

var (
	_ = Service(&reconfigurableDataManager{})
	_ = resource.Reconfigurable(&reconfigurableDataManager{})
	_ = goutils.ContextCloser(&reconfigurableDataManager{})
)

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((*Service)(nil), actual)
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("data_manager")

// Subtype is a constant that identifies the data manager service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named datamanager's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

type reconfigurableDataManager struct {
	mu     sync.RWMutex
	name   resource.Name
	actual Service
}

func (svc *reconfigurableDataManager) Name() resource.Name {
	return svc.name
}

func (svc *reconfigurableDataManager) Sync(ctx context.Context, extra map[string]interface{}) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.Sync(ctx, extra)
}

func (svc *reconfigurableDataManager) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.DoCommand(ctx, cmd)
}

func (svc *reconfigurableDataManager) Close(ctx context.Context) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return goutils.TryClose(ctx, svc.actual)
}

func (svc *reconfigurableDataManager) Update(ctx context.Context, resources *config.Config) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	updateableSvc, ok := svc.actual.(config.Updateable)
	if !ok {
		return errors.New("reconfigurable datamanager is not ConfigUpdateable")
	}
	return updateableSvc.Update(ctx, resources)
}

// Reconfigure replaces the old data manager service with a new data manager.
func (svc *reconfigurableDataManager) Reconfigure(ctx context.Context, newSvc resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newSvc.(*reconfigurableDataManager)
	if !ok {
		return utils.NewUnexpectedTypeError(svc, newSvc)
	}
	if err := goutils.TryClose(ctx, svc.actual); err != nil {
		golog.Global().Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// WrapWithReconfigurable wraps a data_manager as a Reconfigurable.
func WrapWithReconfigurable(s interface{}, name resource.Name) (resource.Reconfigurable, error) {
	svc, ok := s.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(s)
	}

	if reconfigurable, ok := s.(*reconfigurableDataManager); ok {
		return reconfigurable, nil
	}

	return &reconfigurableDataManager{name: name, actual: svc}, nil
}
