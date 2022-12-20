package motor

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/motor/v1"
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
				&pb.MotorService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterMotorServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.MotorService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    Subtype,
		MethodName: position.String(),
	}, newPositionCollector)
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    Subtype,
		MethodName: isPowered.String(),
	}, newIsPoweredCollector)
}

// SubtypeName is a constant that identifies the component resource subtype string "motor".
const SubtypeName = resource.SubtypeName("motor")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// A Motor represents a physical motor connected to a board.
type Motor interface {
	// SetPower sets the percentage of power the motor should employ between -1 and 1.
	// Negative power implies a backward directional rotational
	SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error

	// GoFor instructs the motor to go in a specific direction for a specific amount of
	// revolutions at a given speed in revolutions per minute. Both the RPM and the revolutions
	// can be assigned negative values to move in a backwards direction. Note: if both are
	// negative the motor will spin in the forward direction.
	// If revolutions is 0, this will run the motor at rpm indefinitely
	// If revolutions != 0, this will block until the number of revolutions has been completed or another operation comes in.
	GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error

	// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
	// at a specific speed. Regardless of the directionality of the RPM this function will move the motor
	// towards the specified target/position
	// This will block until the position has been reached
	GoTo(ctx context.Context, rpm, positionRevolutions float64, extra map[string]interface{}) error

	// Set the current position (+/- offset) to be the new zero (home) position.
	ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error

	// Position reports the position of the motor based on its encoder. If it's not supported, the returned
	// data is undefined. The unit returned is the number of revolutions which is intended to be fed
	// back into calls of GoFor.
	Position(ctx context.Context, extra map[string]interface{}) (float64, error)

	// Properties returns whether or not the motor supports certain optional features.
	Properties(ctx context.Context, extra map[string]interface{}) (map[Feature]bool, error)

	// Stop turns the power to the motor off immediately, without any gradual step down.
	Stop(ctx context.Context, extra map[string]interface{}) error

	// IsPowered returns whether or not the motor is currently on, and the percent power (between 0
	// and 1, if the motor is off then the percent power will be 0).
	IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error)

	generic.Generic
	resource.MovingCheckable
}

// A LocalMotor is a motor that supports additional features provided by RDK
// (e.g. GoTillStop).
type LocalMotor interface {
	Motor
	// GoTillStop moves a motor until stopped. The "stop" mechanism is up to the underlying motor implementation.
	// Ex: EncodedMotor goes until physically stopped/stalled (detected by change in position being very small over a fixed time.)
	// Ex: TMCStepperMotor has "StallGuard" which detects the current increase when obstructed and stops when that reaches a threshold.
	// Ex: Other motors may use an endstop switch (such as via a DigitalInterrupt) or be configured with other sensors.
	GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error
}

// Named is a helper for getting the named Motor's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

var (
	_ = Motor(&reconfigurableMotor{})
	_ = LocalMotor(&reconfigurableLocalMotor{})
	_ = resource.Reconfigurable(&reconfigurableMotor{})
	_ = resource.Reconfigurable(&reconfigurableLocalMotor{})
	_ = viamutils.ContextCloser(&reconfigurableLocalMotor{})
)

// FromDependencies is a helper for getting the named motor from a collection of
// dependencies.
func FromDependencies(deps registry.Dependencies, name string) (Motor, error) {
	res, ok := deps[Named(name)]
	if !ok {
		return nil, utils.DependencyNotFoundError(name)
	}
	part, ok := res.(Motor)
	if !ok {
		return nil, DependencyTypeError(name, res)
	}
	return part, nil
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((*Motor)(nil), actual)
}

// NewUnimplementedLocalInterfaceError is used when there is a failed interface check.
func NewUnimplementedLocalInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((*LocalMotor)(nil), actual)
}

// DependencyTypeError is used when a resource doesn't implement the expected interface.
func DependencyTypeError(name string, actual interface{}) error {
	return utils.DependencyTypeError(name, (*Motor)(nil), actual)
}

// FromRobot is a helper for getting the named motor from the given Robot.
func FromRobot(r robot.Robot, name string) (Motor, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(Motor)
	if !ok {
		return nil, NewUnimplementedInterfaceError(res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all motor names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// CreateStatus creates a status from the motor.
func CreateStatus(ctx context.Context, resource interface{}) (*pb.Status, error) {
	motor, ok := resource.(Motor)
	if !ok {
		return nil, NewUnimplementedLocalInterfaceError(resource)
	}
	isPowered, _, err := motor.IsPowered(ctx, nil)
	if err != nil {
		return nil, err
	}
	features, err := motor.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}
	var position float64
	if features[PositionReporting] {
		position, err = motor.Position(ctx, nil)
		if err != nil {
			return nil, err
		}
	}
	isMoving, err := motor.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.Status{
		IsPowered: isPowered,
		Position:  position,
		IsMoving:  isMoving,
	}, nil
}

type reconfigurableMotor struct {
	mu     sync.RWMutex
	name   resource.Name
	actual Motor
}

func (r *reconfigurableMotor) Name() resource.Name {
	return r.name
}

func (r *reconfigurableMotor) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableMotor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.DoCommand(ctx, cmd)
}

func (r *reconfigurableMotor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.SetPower(ctx, powerPct, extra)
}

func (r *reconfigurableMotor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GoFor(ctx, rpm, revolutions, extra)
}

func (r *reconfigurableMotor) GoTo(ctx context.Context, rpm, positionRevolutions float64, extra map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GoTo(ctx, rpm, positionRevolutions, extra)
}

func (r *reconfigurableMotor) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ResetZeroPosition(ctx, offset, extra)
}

func (r *reconfigurableMotor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Position(ctx, extra)
}

func (r *reconfigurableMotor) Properties(ctx context.Context, extra map[string]interface{}) (map[Feature]bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Properties(ctx, extra)
}

func (r *reconfigurableMotor) Stop(ctx context.Context, extra map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Stop(ctx, extra)
}

func (r *reconfigurableMotor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.IsPowered(ctx, extra)
}

func (r *reconfigurableMotor) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableMotor) Reconfigure(ctx context.Context, newMotor resource.Reconfigurable) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.reconfigure(ctx, newMotor)
}

func (r *reconfigurableMotor) reconfigure(ctx context.Context, newMotor resource.Reconfigurable) error {
	actual, ok := newMotor.(*reconfigurableMotor)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newMotor)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		golog.Global().Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

func (r *reconfigurableMotor) IsMoving(ctx context.Context) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.IsMoving(ctx)
}

type reconfigurableLocalMotor struct {
	*reconfigurableMotor
	actual LocalMotor
}

func (r *reconfigurableLocalMotor) Reconfigure(ctx context.Context, newMotor resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	motor, ok := newMotor.(*reconfigurableLocalMotor)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newMotor)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		golog.Global().Errorw("error closing old", "error", err)
	}

	r.actual = motor.actual
	return r.reconfigurableMotor.reconfigure(ctx, motor.reconfigurableMotor)
}

func (r *reconfigurableLocalMotor) GoTillStop(
	ctx context.Context, rpm float64,
	stopFunc func(ctx context.Context) bool,
) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GoTillStop(ctx, rpm, stopFunc)
}

func (r *reconfigurableLocalMotor) IsMoving(ctx context.Context) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.IsMoving(ctx)
}

// WrapWithReconfigurable converts a regular Motor implementation to a reconfigurableMotor.
// If motor is already a reconfigurableMotor, then nothing is done.
func WrapWithReconfigurable(r interface{}, name resource.Name) (resource.Reconfigurable, error) {
	m, ok := r.(Motor)
	if !ok {
		return nil, NewUnimplementedInterfaceError(r)
	}
	if reconfigurable, ok := m.(*reconfigurableMotor); ok {
		return reconfigurable, nil
	}
	rMotor := &reconfigurableMotor{name: name, actual: m}
	mLocal, ok := r.(LocalMotor)
	if !ok {
		return rMotor, nil
	}
	if reconfigurable, ok := m.(*reconfigurableLocalMotor); ok {
		return reconfigurable, nil
	}

	return &reconfigurableLocalMotor{actual: mLocal, reconfigurableMotor: rMotor}, nil
}
