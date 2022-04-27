// Package generic defines an abstract generic device and Do() method
package generic

import (
	"context"
	"errors"
	"sync"

	"github.com/edaniels/golog"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	pb "go.viam.com/rdk/proto/api/component/generic/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.GenericService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterGenericServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

// SubtypeName is a constant that identifies the component resource subtype string "Generic".
const SubtypeName = resource.SubtypeName("generic")
var (
	ErrUnimplemented = errors.New("Do() unimplemented")
	EchoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return cmd, nil
	}
	TestCommand = map[string]interface{}{"command": "test", "data": 500}
)

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named Generic's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// Generic represents a general purpose interface
type Generic interface {
	// Do sends and recieves arbitrary data
	Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// Unimplemented can be embedded in other components to save boilerplate.
type Unimplemented struct {}

// Do covers the unimplemented case for other components
func (u *Unimplemented) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, ErrUnimplemented
}

// Echo can be embedded in other (fake) components to save boilerplate.
type Echo struct {}

// Do covers the echo case for other components
func (e *Echo) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

var (
	_ = Generic(&reconfigurableGeneric{})
	_ = resource.Reconfigurable(&reconfigurableGeneric{})
)

// FromRobot is a helper for getting the named Generic from the given Robot.
func FromRobot(r robot.Robot, name string) (Generic, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(Generic)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Generic", res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all generic names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

type reconfigurableGeneric struct {
	mu     sync.RWMutex
	actual Generic
}

func (r *reconfigurableGeneric) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableGeneric) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableGeneric) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Do(ctx, cmd)
}

func (r *reconfigurableGeneric) Reconfigure(ctx context.Context, newGeneric resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newGeneric.(*reconfigurableGeneric)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newGeneric)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular Generic implementation to a reconfigurableGeneric.
// If Generic is already a reconfigurableGeneric, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	Generic, ok := r.(Generic)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Generic", r)
	}
	if reconfigurable, ok := Generic.(*reconfigurableGeneric); ok {
		return reconfigurable, nil
	}
	return &reconfigurableGeneric{actual: Generic}, nil
}

// RegisterService is a helper for testing in other components
func RegisterService(server rpc.Server, service subtype.Service) {
	resourceSubtype := registry.ResourceSubtypeLookup(Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), server, service)
}
