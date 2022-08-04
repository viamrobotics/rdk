// package encoder implements the encoder component
package encoder

import (
	"context"
	"sync"

	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
	})
}

// SubtypeName is a constant that identifies the component resource subtype string "motor".
const SubtypeName = resource.SubtypeName("encoder")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// A Encoder represents a physical motor connected to a board.
type Encoder interface {
	// GetTicksCount returns number of ticks since last zeroing
	GetTicksCount(ctx context.Context) (int64, error)

	// ResetZeroPosition resets the counted ticks to 0
	ResetZeroPosition(ctx context.Context, offset int64) error

	// TicksPerRotation returns the number of ticks needed for a full rotation
	TicksPerRotation(ctx context.Context) (int64, error)

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
		return nil, utils.DependencyTypeError(name, "Encoder", res)
	}
	return part, nil
}

// FromRobot is a helper for getting the named encoder from the given Robot.
func FromRobot(r robot.Robot, name string) (Encoder, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(Encoder)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Encoder", res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all encoder names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

type reconfigurableEncoder struct {
	mu     sync.RWMutex
	actual Encoder
}

func (r *reconfigurableEncoder) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableEncoder) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Do(ctx, cmd)
}

func (r *reconfigurableEncoder) GetTicksCount(ctx context.Context) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetTicksCount(ctx)
}

func (r *reconfigurableEncoder) ResetZeroPosition(ctx context.Context, offset int64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ResetZeroPosition(ctx, offset)
}

func (r *reconfigurableEncoder) TicksPerRotation(ctx context.Context) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.TicksPerRotation(ctx)
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
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular Encoder implementation to a reconfigurableEncoder.
// If encoder is already a reconfigurableEncoder, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	m, ok := r.(Encoder)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Encoder", r)
	}
	if reconfigurable, ok := m.(*reconfigurableEncoder); ok {
		return reconfigurable, nil
	}
	return &reconfigurableEncoder{actual: m}, nil
}

// Config describes the configuration of an encoder.
type Config struct {
	Pins      interface{} `json:"pins"`
	BoardName string      `json:"board"`

	TicksPerRotation int `json:"ticks_per_rotation,omitempty"`
}
