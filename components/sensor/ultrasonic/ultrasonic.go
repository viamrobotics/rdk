// Package ultrasonic implements an ultrasonic sensor based of the yahboom ultrasonic sensor
package ultrasonic

import (
	"context"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	rdkutils "go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var modelname = resource.NewDefaultModel("ultrasonic")

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	TriggerPin    string `json:"trigger_pin"`
	EchoInterrupt string `json:"echo_interrupt_pin"`
	Board         string `json:"board"`
	TimeoutMs     uint   `json:"timeout_ms,omitempty"`
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

	config.RegisterComponentAttributeMapConverter(sensor.Subtype, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

func newSensor(ctx context.Context, deps registry.Dependencies, name string, config *AttrConfig) (sensor.Sensor, error) {
	golog.Global().Debug("building ultrasonic sensor")
	s := &Sensor{Name: name, config: config}
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	s.cancelCtx = cancelCtx
	s.cancelFunc = cancelFunc

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

	if config.TimeoutMs > 0 {
		s.timeoutMs = config.TimeoutMs
	} else {
		// default to 1 sec
		s.timeoutMs = 1000
	}

	s.intChan = make(chan bool)
	s.readingChan = make(chan bool)
	s.errChan = make(chan error)
	s.distanceChan = make(chan float64)
	if err := s.triggerPin.Set(ctx, false, nil); err != nil {
		return nil, errors.Wrap(err, "ultrasonic: cannot set trigger pin to low")
	}
	s.startUpdateLoop(ctx)
	return s, nil
}

// Sensor ultrasonic sensor.
type Sensor struct {
	mu                      sync.Mutex
	Name                    string
	config                  *AttrConfig
	echoInterrupt           board.DigitalInterrupt
	triggerPin              board.GPIOPin
	intChan                 chan bool
	timeoutMs               uint
	readingChan             chan bool
	distanceChan            chan float64
	errChan                 chan error
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
	generic.Unimplemented
}

func (s *Sensor) namedError(err error) error {
	return errors.Wrapf(
		err, "Error in ultrasonic sensor with name %s: ", s.Name,
	)
}

func (s *Sensor) startUpdateLoop(ctx context.Context) {
	s.activeBackgroundWorkers.Add(1)
	rdkutils.ManagedGo(
		func() {
			s.echoInterrupt.AddCallback(s.intChan)
			defer s.echoInterrupt.RemoveCallback(s.intChan)
			for {
				select {
				case <-s.cancelCtx.Done():
					return
				case <-s.readingChan:
					// a call to Readings has occurred so we request a distance
					// reading from the sensor and send it to the distance channel
					if err := s.measureDistance(ctx); err != nil {
						s.errChan <- err
					}
				// we must consume any signals from the interrupt that occur
				// outside of a call to readings, otherwise we will potentially
				// block callback code for *all* interrupts (see the implementation
				// of pigpioInterruptCallback in components/board/pi/impl/board.go)
				case <-s.intChan:
				}
			}
		},
		s.activeBackgroundWorkers.Done,
	)
}

func (s *Sensor) measureDistance(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// we send a high and a low to the trigger pin 10 microseconds
	// apart to signal the sensor to begin sending the sonic pulse
	if err := s.triggerPin.Set(ctx, true, nil); err != nil {
		return s.namedError(errors.Wrap(err, "ultrasonic cannot set trigger pin to high"))
	}
	rdkutils.SelectContextOrWait(ctx, time.Microsecond*10)
	if err := s.triggerPin.Set(ctx, false, nil); err != nil {
		return s.namedError(errors.Wrap(err, "ultrasonic cannot set trigger pin to low"))
	}
	// the first signal from the interrupt indicates that the sonic
	// pulse has been sent
	var timeA, timeB time.Time
	select {
	case <-s.intChan:
		timeB = time.Now()
	case <-s.cancelCtx.Done():
		return s.namedError(errors.New("ultrasonic: context canceled"))
	case <-time.After(time.Millisecond * time.Duration(s.timeoutMs)):
		return s.namedError(errors.New("timed out waiting for signal that sound pulse was emitted"))
	}
	// the second signal from the interrupt indicates that the echo has
	// been received
	select {
	case <-s.intChan:
		timeA = time.Now()
	case <-s.cancelCtx.Done():
		return s.namedError(errors.New("ultrasonic: context canceled"))
	case <-time.After(time.Millisecond * time.Duration(s.timeoutMs)):
		return s.namedError(errors.New("timed out waiting for signal that echo was received"))
	}
	// we calculate the distance to the nearest object based
	// on the time interval between the sound and its echo
	// and the speed of sound (343 m/s)
	distMeters := timeA.Sub(timeB).Seconds() * 343.0 / 2.0
	s.distanceChan <- distMeters
	return nil
}

// Readings returns the calculated distance.
func (s *Sensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	s.readingChan <- true
	select {
	case <-ctx.Done():
		return nil, s.namedError(errors.New("ultrasonic: context canceled"))
	case dist := <-s.distanceChan:
		return map[string]interface{}{"distance": dist}, nil
	case err := <-s.errChan:
		return nil, err
	case <-time.After(time.Millisecond * time.Duration(s.timeoutMs)):
		return nil, s.namedError(errors.New("timeout waiting for measureDistance"))
	}
}

// Close remove interrupt callback of ultrasonic sensor.
func (s *Sensor) Close() error {
	s.cancelFunc()
	s.activeBackgroundWorkers.Wait()
	return nil
}
