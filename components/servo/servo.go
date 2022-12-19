package servo

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/servo/v1"
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
		RPCServiceDesc: &pb.ServoService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    Subtype,
		MethodName: position.String(),
	}, newPositionCollector)
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
	Move(ctx context.Context, angleDeg uint32, extra map[string]interface{}) error

	// Position returns the current set angle (degrees) of the servo.
	Position(ctx context.Context, extra map[string]interface{}) (uint32, error)

	// Stop stops the servo. It is assumed the servo stops immediately.
	Stop(ctx context.Context, extra map[string]interface{}) error

	generic.Generic
	resource.MovingCheckable
}

// A LocalServo represents a Servo that can report whether it is moving or not.
type LocalServo interface {
	Servo
}

// Named is a helper for getting the named Servo's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

var (
	_ = Servo(&reconfigurableServo{})
	_ = LocalServo(&reconfigurableLocalServo{})
	_ = resource.Reconfigurable(&reconfigurableServo{})
	_ = resource.Reconfigurable(&reconfigurableLocalServo{})
)

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((*Servo)(nil), actual)
}

// NewUnimplementedLocalInterfaceError is used when there is a failed interface check.
func NewUnimplementedLocalInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((*LocalServo)(nil), actual)
}

// FromRobot is a helper for getting the named servo from the given Robot.
func FromRobot(r robot.Robot, name string) (Servo, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(Servo)
	if !ok {
		return nil, NewUnimplementedInterfaceError(res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all servo names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// CreateStatus creates a status from the servo.
func CreateStatus(ctx context.Context, resource interface{}) (*pb.Status, error) {
	servo, ok := resource.(Servo)
	if !ok {
		return nil, NewUnimplementedLocalInterfaceError(resource)
	}
	position, err := servo.Position(ctx, nil)
	if err != nil {
		return nil, err
	}
	isMoving, err := servo.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.Status{PositionDeg: position, IsMoving: isMoving}, nil
}

type reconfigurableServo struct {
	mu     sync.RWMutex
	name   resource.Name
	actual Servo
}

func (r *reconfigurableServo) Name() resource.Name {
	return r.name
}

func (r *reconfigurableServo) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

// DoCommand passes generic commands/data.
func (r *reconfigurableServo) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.DoCommand(ctx, cmd)
}

func (r *reconfigurableServo) Move(ctx context.Context, angleDeg uint32, extra map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Move(ctx, angleDeg, extra)
}

func (r *reconfigurableServo) Position(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Position(ctx, extra)
}

func (r *reconfigurableServo) Stop(ctx context.Context, extra map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Stop(ctx, extra)
}

func (r *reconfigurableServo) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableServo) IsMoving(ctx context.Context) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.IsMoving(ctx)
}

func (r *reconfigurableServo) Reconfigure(ctx context.Context, newServo resource.Reconfigurable) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.reconfigure(ctx, newServo)
}

func (r *reconfigurableServo) reconfigure(ctx context.Context, newServo resource.Reconfigurable) error {
	actual, ok := newServo.(*reconfigurableServo)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newServo)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		golog.Global().Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

type reconfigurableLocalServo struct {
	*reconfigurableServo
	actual LocalServo
}

func (r *reconfigurableLocalServo) IsMoving(ctx context.Context) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.IsMoving(ctx)
}

func (r *reconfigurableLocalServo) Reconfigure(ctx context.Context, newServo resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	Servo, ok := newServo.(*reconfigurableLocalServo)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newServo)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		golog.Global().Errorw("error closing old", "error", err)
	}

	r.actual = Servo.actual
	return r.reconfigurableServo.reconfigure(ctx, Servo.reconfigurableServo)
}

// WrapWithReconfigurable converts a regular Servo implementation to a reconfigurableServo.
// If servo is already a reconfigurableServo, then nothing is done.
func WrapWithReconfigurable(r interface{}, name resource.Name) (resource.Reconfigurable, error) {
	servo, ok := r.(Servo)
	if !ok {
		return nil, NewUnimplementedInterfaceError(r)
	}
	if reconfigurable, ok := servo.(*reconfigurableServo); ok {
		return reconfigurable, nil
	}
	rServo := &reconfigurableServo{name: name, actual: servo}
	gLocal, ok := r.(LocalServo)
	if !ok {
		return rServo, nil
	}
	if reconfigurable, ok := servo.(*reconfigurableLocalServo); ok {
		return reconfigurable, nil
	}

	return &reconfigurableLocalServo{actual: gLocal, reconfigurableServo: rServo}, nil
}
