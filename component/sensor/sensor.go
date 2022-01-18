// Package sensor defines an abstract sensing device that can provide measurement readings.
package sensor

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
)

// SubtypeName is a constant that identifies the component resource subtype string "Sensor".
const SubtypeName = resource.SubtypeName("sensor")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named Sensor's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// A Sensor represents a general purpose sensors that can give arbitrary readings
// of some thing that it is sensing.
type Sensor interface {
	// Readings return data specific to the type of sensor and can be of any type.
	Readings(ctx context.Context) ([]interface{}, error)

	// Desc returns a description of this sensor.
	Desc() Description
}

// Type specifies the type of sensor.
type Type string

// Description describes information about the device.
type Description struct {
	Type Type

	// Path is some universal descriptor of how to find the device.
	Path string
}

var (
	_ = Sensor(&reconfigurableSensor{})
	_ = resource.Reconfigurable(&reconfigurableSensor{})
)

type reconfigurableSensor struct {
	mu     sync.RWMutex
	actual Sensor
}

func (r *reconfigurableSensor) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return utils.TryClose(ctx, r.actual)
}

func (r *reconfigurableSensor) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableSensor) Readings(ctx context.Context) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Readings(ctx)
}

func (r *reconfigurableSensor) Desc() Description {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Desc()
}

func (r *reconfigurableSensor) Reconfigure(ctx context.Context, newSensor resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newSensor.(*reconfigurableSensor)
	if !ok {
		return errors.Errorf("expected new Sensor to be %T but got %T", r, newSensor)
	}
	if err := utils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular Sensor implementation to a reconfigurableSensor.
// If Sensor is already a reconfigurableSensor, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	Sensor, ok := r.(Sensor)
	if !ok {
		return nil, errors.Errorf("expected resource to be Sensor but got %T", r)
	}
	if reconfigurable, ok := Sensor.(*reconfigurableSensor); ok {
		return reconfigurable, nil
	}
	return &reconfigurableSensor{actual: Sensor}, nil
}
