// Package ultrasonic implements an ultrasonic sensor based of the yahboom ultrasonic sensor
package ultrasonic

import (
	"context"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	rdkutils "go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

const (
	modelname = "ultrasonic"
)

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	TriggerPin    string `json:"trigger_pin"`
	EchoInterrupt string `json:"echo_interrupt_pin"`
	Board         string `json:"board"`
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) ([]string, error) {
	var deps []string
	if len(config.Board) == 0 {
		return nil, rdkutils.NewConfigValidationFieldRequiredError(path, "board")
	}
	deps = append(deps, config.Board)
	if len(config.TriggerPin) == 0 {
		return nil, rdkutils.NewConfigValidationFieldRequiredError(path, "trigger pin")
	}
	if len(config.EchoInterrupt) == 0 {
		return nil, rdkutils.NewConfigValidationFieldRequiredError(path, "echo interrupt pin")
	}
	return deps, nil
}

func init() {
	registry.RegisterComponent(
		sensor.Subtype,
		modelname,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newSensor(ctx, deps, config.Name, config.ConvertedAttributes.(*AttrConfig))
		}})

	config.RegisterComponentAttributeMapConverter(sensor.SubtypeName, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

func newSensor(ctx context.Context, deps registry.Dependencies, name string, config *AttrConfig) (sensor.Sensor, error) {
	golog.Global().Debug("building ultrasonic sensor")
	s := &Sensor{Name: name, config: config}

	res, ok := deps[board.Named(config.Board)]
	if !ok {
		return nil, errors.Errorf("ultrasonic: board %q missing from dependencies", config.Board)
	}

	b, ok := res.(board.Board)
	if !ok {
		return nil, errors.Errorf("ultrasonic: cannot find board %q", config.Board)
	}
	i, ok := b.DigitalInterruptByName(config.EchoInterrupt)
	if !ok {
		return nil, errors.Errorf("ultrasonic: cannot grab digital interrupt %q", config.EchoInterrupt)
	}
	g, err := b.GPIOPinByName(config.TriggerPin)
	if err != nil {
		return nil, errors.Wrapf(err, "ultrasonic: cannot grab gpio %q", config.TriggerPin)
	}
	s.echoInterrupt = i
	s.triggerPin = g
	if err := s.triggerPin.Set(ctx, false, nil); err != nil {
		return nil, errors.Wrap(err, "ultrasonic: cannot set trigger pin to low")
	}
	s.intChan = make(chan bool)
	s.echoInterrupt.AddCallback(s.intChan)
	return s, nil
}

// Sensor ultrasonic sensor.
type Sensor struct {
	Name          string
	config        *AttrConfig
	echoInterrupt board.DigitalInterrupt
	triggerPin    board.GPIOPin
	intChan       chan bool
	generic.Unimplemented
}

// Readings returns the calculated distance.
func (s *Sensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	if err := s.triggerPin.Set(ctx, true, nil); err != nil {
		return nil, errors.Wrap(err, "ultrasonic cannot set trigger pin to high")
	}

	rdkutils.SelectContextOrWait(ctx, time.Microsecond*20)
	if err := s.triggerPin.Set(ctx, false, nil); err != nil {
		return nil, errors.Wrap(err, "ultrasonic cannot set trigger pin to low")
	}
	var timeA, timeB time.Time
	select {
	case <-s.intChan:
		timeB = time.Now()
	case <-ctx.Done():
		return nil, errors.New("ultrasonic: context canceled")
	case <-time.After(time.Second * 1):
		return nil, errors.New("ultrasonic timeout")
	}
	select {
	case <-s.intChan:
		timeA = time.Now()
	case <-ctx.Done():
		return nil, errors.New("ultrasonic: context canceled")
	case <-time.After(time.Second * 1):
		return nil, errors.New("ultrasonic timeout")
	}
	dist := timeA.Sub(timeB).Seconds() * 340 / 2
	return map[string]interface{}{"distance": dist}, nil
}

// Close remove interrupt callback of ultrasonic sensor.
func (s *Sensor) Close() error {
	s.echoInterrupt.RemoveCallback(s.intChan)
	return nil
}
