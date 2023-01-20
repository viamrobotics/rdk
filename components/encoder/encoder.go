// Package encoder implements the encoder component
package encoder

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
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

// A Encoder turns a position into a signal.
type Encoder interface {
	// TicksCount returns number of ticks since last zeroing
	TicksCount(ctx context.Context, extra map[string]interface{}) (float64, error)

	// Reset sets the current position of the motor (adjusted by a given offset)
	// to be its new zero position.
	Reset(ctx context.Context, offset float64, extra map[string]interface{}) error

	generic.Generic
}

// Named is a helper for getting the named Encoder's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

var (
	_ = Encoder(&reconfigurableEncoder{})
	_ = resource.Reconfigurable(&reconfigurableEncoder{})
	_ = resource.Reconfigurable(&reconfigurableEncoder{})
)

// FromDependencies is a helper for getting the named encoder from a collection of
// dependencies.
func FromDependencies(deps registry.Dependencies, name string) (Encoder, error) {
	res, ok := deps[Named(name)]
	if !ok {
		return nil, utils.DependencyNotFoundError(name)
	}
	part, ok := res.(Encoder)
	if !ok {
		return nil, DependencyTypeError(name, res)
	}
	return part, nil
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((*Encoder)(nil), actual)
}

// DependencyTypeError is used when a resource doesn't implement the expected interface.
func DependencyTypeError(name string, actual interface{}) error {
	return utils.DependencyTypeError(name, (*Encoder)(nil), actual)
}

// FromRobot is a helper for getting the named encoder from the given Robot.
func FromRobot(r robot.Robot, name string) (Encoder, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(Encoder)
	if !ok {
		return nil, NewUnimplementedInterfaceError(res)
	}
	return part, nil
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

func (r *reconfigurableEncoder) TicksCount(ctx context.Context, extra map[string]interface{}) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.TicksCount(ctx, extra)
}

func (r *reconfigurableEncoder) Reset(ctx context.Context, offset float64, extra map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Reset(ctx, offset, extra)
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

// ValidateIntegerOffset returns an error if a non-integral value for offset
// is passed to Reset for an incremental encoder (these encoders count based on
// square-wave pulses and so cannot be supplied an offset that is not an integer).
func ValidateIntegerOffset(offset float64) error {
	if offset != float64(int64(offset)) {
		return errors.Errorf(
			"incremental encoders can only reset with integer value offsets, value passed was %f",
			offset,
		)
	}
	return nil
}
