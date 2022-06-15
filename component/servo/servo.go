package servo

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	pb "go.viam.com/rdk/proto/api/component/servo/v1"
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
		Status: func(ctx context.Context, resource interface{}) (interface{}, error) {
			return CreateStatus(ctx, resource)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.ServoService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterServoServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

// SubtypeName is a constant that identifies the component resource subtype string "servo".
const SubtypeName = resource.SubtypeName("servo")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// A Servo represents a physical servo connected to a board.
type Servo interface {
	// Move moves the servo to the given angle (0-180 degrees)
	// This will block until done or a new operation cancels this one
	Move(ctx context.Context, angleDeg uint8) error

	// GetPosition returns the current set angle (degrees) of the servo.
	GetPosition(ctx context.Context) (uint8, error)

	// Stop stops the servo. It is assumed the servo stops immediately.
	Stop(ctx context.Context) error

	generic.Generic
}

// A LocalServo represents a Servo that can report whether it is moving or not.
type LocalServo interface {
	Servo

	resource.MovingCheckable
}

// Named is a helper for getting the named Servo's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

var (
	_ = LocalServo(&reconfigurableServo{})
	_ = resource.Reconfigurable(&reconfigurableServo{})
)

// FromRobot is a helper for getting the named servo from the given Robot.
func FromRobot(r robot.Robot, name string) (Servo, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(Servo)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Servo", res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all servo names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// CreateStatus creates a status from the servo.
func CreateStatus(ctx context.Context, resource interface{}) (*pb.Status, error) {
	servo, ok := resource.(LocalServo)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("LocalServo", resource)
	}
	position, err := servo.GetPosition(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.Status{PositionDeg: uint32(position), IsMoving: servo.IsMoving()}, nil
}

type reconfigurableServo struct {
	mu     sync.RWMutex
	actual LocalServo
}

func (r *reconfigurableServo) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

// Do passes generic commands/data.
func (r *reconfigurableServo) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Do(ctx, cmd)
}

func (r *reconfigurableServo) Move(ctx context.Context, angleDeg uint8) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Move(ctx, angleDeg)
}

func (r *reconfigurableServo) GetPosition(ctx context.Context) (uint8, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetPosition(ctx)
}

func (r *reconfigurableServo) Stop(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Stop(ctx)
}

func (r *reconfigurableServo) IsMoving() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.IsMoving()
}

func (r *reconfigurableServo) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableServo) Reconfigure(ctx context.Context, newServo resource.Reconfigurable) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	actual, ok := newServo.(*reconfigurableServo)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newServo)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular Servo implementation to a reconfigurableServo.
// If servo is already a reconfigurableServo, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	servo, ok := r.(LocalServo)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("LocalServo", r)
	}
	if reconfigurable, ok := servo.(*reconfigurableServo); ok {
		return reconfigurable, nil
	}
	return &reconfigurableServo{actual: servo}, nil
}
