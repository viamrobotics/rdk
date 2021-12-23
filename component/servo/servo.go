package servo

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
)

// SubtypeName is a constant that identifies the component resource subtype string "servo"
const SubtypeName = resource.SubtypeName("servo")

// Subtype is a constant that identifies the component resource subtype
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceCore,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// A Servo represents a physical servo connected to a board.
type Servo interface {

	// Move moves the servo to the given angle (0-180 degrees)
	Move(ctx context.Context, angleDegs uint8) error

	// AngularOffset returns the current set angle (degrees) of the servo.
	AngularOffset(ctx context.Context) (uint8, error)
}

// Named is a helper for getting the named Servo's typed resource name
func Named(name string) resource.Name {
	return resource.NewFromSubtype(Subtype, name)
}

var (
	_ = Servo(&reconfigurableServo{})
	_ = resource.Reconfigurable(&reconfigurableServo{})
)

type reconfigurableServo struct {
	mu     sync.RWMutex
	actual Servo
}

func (r *reconfigurableServo) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableServo) Move(ctx context.Context, angleDegs uint8) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Move(ctx, angleDegs)
}

func (r *reconfigurableServo) AngularOffset(ctx context.Context) (uint8, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.AngularOffset(ctx)
}

func (r *reconfigurableServo) Close() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(r.actual)
}

func (r *reconfigurableServo) Reconfigure(newServo resource.Reconfigurable) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	actual, ok := newServo.(*reconfigurableServo)
	if !ok {
		return errors.Errorf("expected new arm to be %T but got %T", r, newServo)
	}
	if err := viamutils.TryClose(r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular Servo implementation to a reconfigurableServo.
// If servo is already a reconfigurableServo, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	servo, ok := r.(Servo)
	if !ok {
		return nil, errors.Errorf("expected resource to be Servo but got %T", r)
	}
	if reconfigurable, ok := servo.(*reconfigurableServo); ok {
		return reconfigurable, nil
	}
	return &reconfigurableServo{actual: servo}, nil
}
