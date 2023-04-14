// Package encoder implements the encoder component
package encoder

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/encoder/v1"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
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
				&pb.EncoderService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterEncoderServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.EncoderService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    Subtype,
		MethodName: ticksCount.String(),
	}, newTicksCountCollector)
}

// SubtypeName is a constant that identifies the component resource subtype string "encoder".
const SubtypeName = resource.SubtypeName("encoder")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// PositionType is an enum representing the encoders position type.
type PositionType int32

const (
	// PositionTypeUNSPECIFIED is the return type for
	// when the user has not specified a PositionType.
	PositionTypeUNSPECIFIED PositionType = 0
	// PositionTypeTICKS is the return type for relative encoders
	// that report how far they've gone from a start position.
	PositionTypeTICKS PositionType = 1
	// PositionTypeDEGREES is the return type for absolute encoders
	// that report their position in degrees along the radial axis.
	PositionTypeDEGREES PositionType = 2
)

// Enum reveals the value of PositionType.
func (x PositionType) Enum() *PositionType {
	p := new(PositionType)
	*p = x
	return p
}

// A Encoder turns a position into a signal.
type Encoder interface {
	// GetPosition returns the current position in terms of ticks or degrees, and whether it is a relative or absolute position.
	GetPosition(ctx context.Context, positionType *PositionType, extra map[string]interface{}) (float64, PositionType, error)

	// ResetPosition sets the current position of the motor to be its new zero position.
	ResetPosition(ctx context.Context, extra map[string]interface{}) error

	// GetProperties returns a list of all the position types that are supported by a given encoder
	GetProperties(ctx context.Context, extra map[string]interface{}) (map[Feature]bool, error)

	generic.Generic
}

// Named is a helper for getting the named Encoder's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

var (
	_ = Encoder(&reconfigurableEncoder{})
	_ = resource.Reconfigurable(&reconfigurableEncoder{})
	_ = viamutils.ContextCloser(&reconfigurableEncoder{})
)

// FromDependencies is a helper for getting the named encoder from a collection of
// dependencies.
func FromDependencies(deps registry.Dependencies, name string) (Encoder, error) {
	return registry.ResourceFromDependencies[Encoder](deps, Named(name))
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((*Encoder)(nil), actual)
}

// FromRobot is a helper for getting the named encoder from the given Robot.
func FromRobot(r robot.Robot, name string) (Encoder, error) {
	return robot.ResourceFromRobot[Encoder](r, Named(name))
}

// NamesFromRobot is a helper for getting all encoder names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

type reconfigurableEncoder struct {
	mu     sync.RWMutex
	name   resource.Name
	actual Encoder
}

func (r *reconfigurableEncoder) Name() resource.Name {
	return r.name
}

func (r *reconfigurableEncoder) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableEncoder) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.DoCommand(ctx, cmd)
}

func (r *reconfigurableEncoder) GetPosition(
	ctx context.Context,
	positionType *PositionType,
	extra map[string]interface{},
) (float64, PositionType, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetPosition(ctx, positionType, extra)
}

func (r *reconfigurableEncoder) ResetPosition(ctx context.Context, extra map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ResetPosition(ctx, extra)
}

func (r *reconfigurableEncoder) GetProperties(ctx context.Context, extra map[string]interface{}) (map[Feature]bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetProperties(ctx, extra)
}

func (r *reconfigurableEncoder) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableEncoder) Reconfigure(ctx context.Context, newEncoder resource.Reconfigurable) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.reconfigure(ctx, newEncoder)
}

func (r *reconfigurableEncoder) reconfigure(ctx context.Context, newEncoder resource.Reconfigurable) error {
	actual, ok := newEncoder.(*reconfigurableEncoder)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newEncoder)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		golog.Global().Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular Encoder implementation to a reconfigurableEncoder.
// If encoder is already a reconfigurableEncoder, then nothing is done.
func WrapWithReconfigurable(r interface{}, name resource.Name) (resource.Reconfigurable, error) {
	m, ok := r.(Encoder)
	if !ok {
		return nil, NewUnimplementedInterfaceError(r)
	}
	if reconfigurable, ok := m.(*reconfigurableEncoder); ok {
		return reconfigurable, nil
	}
	return &reconfigurableEncoder{name: name, actual: m}, nil
}

// ToEncoderPositionType takes a GetPositionResponse and returns
// an equivalent PositionType-to-int map.
func ToEncoderPositionType(positionType *pb.PositionType) PositionType {
	if positionType == nil {
		return PositionTypeUNSPECIFIED
	}
	if *positionType == pb.PositionType_POSITION_TYPE_ANGLE_DEGREES {
		return PositionTypeDEGREES
	}
	if *positionType == pb.PositionType_POSITION_TYPE_TICKS_COUNT {
		return PositionTypeTICKS
	}
	return PositionTypeUNSPECIFIED
}

// ToProtoPositionType takes a map of PositionType-to-int (indicating
// the PositionType) and converts it to a GetPositionResponse.
func ToProtoPositionType(positionType *PositionType) pb.PositionType {
	if positionType == nil {
		return pb.PositionType_POSITION_TYPE_UNSPECIFIED
	}
	if *positionType == PositionTypeDEGREES {
		return pb.PositionType_POSITION_TYPE_ANGLE_DEGREES
	}
	if *positionType == PositionTypeTICKS {
		return pb.PositionType_POSITION_TYPE_TICKS_COUNT
	}
	return pb.PositionType_POSITION_TYPE_UNSPECIFIED
}
