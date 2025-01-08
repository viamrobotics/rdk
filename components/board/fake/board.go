// Package fake implements a fake board.
package fake

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/component/board/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// In order to maintain test functionality, testPin will always return the analog value it is set
// to (defaults to 0 before being set). To see changing fake analog values on a fake board, add an
// analog reader to any other pin.
var analogTestPin = "1"

// In order to maintain test functionality, digital interrtups on any pin except nonZeroInterruptPin
// will always return a digital interrupt value of 0. To see non-zero fake interrupt values on a fake board,
// add an digital interrupt to pin 0.
var nonZeroInterruptPin = "0"

// A Config describes the configuration of a fake board and all of its connected parts.
type Config struct {
	AnalogReaders     []board.AnalogReaderConfig     `json:"analogs,omitempty"`
	DigitalInterrupts []board.DigitalInterruptConfig `json:"digital_interrupts,omitempty"`
	FailNew           bool                           `json:"fail_new"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	for idx, conf := range conf.AnalogReaders {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "analogs", idx)); err != nil {
			return nil, err
		}
	}
	for idx, conf := range conf.DigitalInterrupts {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "digital_interrupts", idx)); err != nil {
			return nil, err
		}
	}

	if conf.FailNew {
		return nil, errors.New("whoops")
	}

	return nil, nil
}

var model = resource.DefaultModelFamily.WithModel("fake")

func init() {
	resource.RegisterComponent(
		board.API,
		model,
		resource.Registration[board.Board, *Config]{
			Constructor: func(
				ctx context.Context,
				_ resource.Dependencies,
				cfg resource.Config,
				logger logging.Logger,
			) (board.Board, error) {
				return NewBoard(ctx, cfg, logger)
			},
		})
}

// NewBoard returns a new fake board.
func NewBoard(ctx context.Context, conf resource.Config, logger logging.Logger) (*Board, error) {
	b := &Board{
		Named:    conf.ResourceName().AsNamed(),
		Analogs:  map[string]*Analog{},
		Digitals: map[string]*DigitalInterrupt{},
		GPIOPins: map[string]*GPIOPin{},
		workers:  utils.NewBackgroundStoppableWorkers(),
		logger:   logger,
	}

	if err := b.processConfig(conf); err != nil {
		return nil, err
	}

	return b, nil
}

func (b *Board) processConfig(conf resource.Config) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	// TODO(RSDK-2684): we dont configure pins so we just unset them here. not really great behavior.
	b.GPIOPins = map[string]*GPIOPin{}

	stillExists := map[string]struct{}{}

	for _, c := range newConf.AnalogReaders {
		stillExists[c.Name] = struct{}{}
		if curr, ok := b.Analogs[c.Name]; ok {
			if curr.pin != c.Pin {
				curr.reset(c.Pin)
			}
			continue
		}
		b.Analogs[c.Name] = newAnalogReader(c.Pin)
	}
	for name := range b.Analogs {
		if _, ok := stillExists[name]; ok {
			continue
		}
		delete(b.Analogs, name)
	}
	stillExists = map[string]struct{}{}

	var errs error
	for _, c := range newConf.DigitalInterrupts {
		stillExists[c.Name] = struct{}{}
		if curr, ok := b.Digitals[c.Name]; ok {
			if !reflect.DeepEqual(curr.conf, c) {
				curr.reset(c)
			}
			continue
		}
		var err error
		b.Digitals[c.Name], err = NewDigitalInterrupt(c)
		if err != nil {
			errs = multierr.Combine(errs, err)
		}
	}
	for name := range b.Digitals {
		if _, ok := stillExists[name]; ok {
			continue
		}
		delete(b.Digitals, name)
	}

	return nil
}

// Reconfigure atomically reconfigures this board in place based on the new config.
func (b *Board) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	return b.processConfig(conf)
}

// A Board provides dummy data from fake parts in order to implement a Board.
type Board struct {
	resource.Named

	mu         sync.RWMutex
	Analogs    map[string]*Analog
	Digitals   map[string]*DigitalInterrupt
	GPIOPins   map[string]*GPIOPin
	logger     logging.Logger
	CloseCount int

	workers *utils.StoppableWorkers
}

// AnalogByName returns the analog pin by the given name if it exists.
func (b *Board) AnalogByName(name string) (board.Analog, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	a, ok := b.Analogs[name]
	if !ok {
		return nil, errors.Errorf("can't find AnalogReader (%s)", name)
	}
	return a, nil
}

// DigitalInterruptByName returns the interrupt by the given name if it exists.
func (b *Board) DigitalInterruptByName(name string) (board.DigitalInterrupt, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	d, ok := b.Digitals[name]
	if !ok {
		return nil, fmt.Errorf("cant find DigitalInterrupt (%s)", name)
	}
	return d, nil
}

// GPIOPinByName returns the GPIO pin by the given name if it exists.
func (b *Board) GPIOPinByName(name string) (board.GPIOPin, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	p, ok := b.GPIOPins[name]
	if !ok {
		pin := &GPIOPin{}
		b.GPIOPins[name] = pin
		return pin, nil
	}
	return p, nil
}

// SetPowerMode sets the board to the given power mode. If provided,
// the board will exit the given power mode after the specified
// duration.
func (b *Board) SetPowerMode(ctx context.Context, mode pb.PowerMode, duration *time.Duration) error {
	return grpc.UnimplementedError
}

// StreamTicks starts a stream of digital interrupt ticks.
func (b *Board) StreamTicks(ctx context.Context, interrupts []board.DigitalInterrupt, ch chan board.Tick,
	extra map[string]interface{},
) error {
	for _, di := range interrupts {
		_, ok := b.Digitals[di.Name()]
		if !ok {
			return fmt.Errorf("could not find digital interrupt: %s", di.Name())
		}
	}

	for _, di := range interrupts {
		// Don't need to check if interrupt exists, just did that above
		b.workers.Add(func(workersContext context.Context) {
			for {
				// sleep to avoid a busy loop
				if !utils.SelectContextOrWait(workersContext, 700*time.Millisecond) {
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-workersContext.Done():
					return
				default:
					// Keep going
				}
				// Get a random bool for the high tick value.
				// linter complains about security but we don't care if someone
				// can predict if the fake interrupts will be high or low.
				//nolint:gosec
				randBool := rand.Int()%2 == 0
				select {
				case ch <- board.Tick{Name: di.Name(), High: randBool, TimestampNanosec: uint64(time.Now().Unix())}:
				default:
					// if nothing is listening to the channel just do nothing.
				}
			}
		})
	}
	return nil
}

// Close attempts to cleanly close each part of the board.
func (b *Board) Close(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.CloseCount++

	b.workers.Stop()
	return nil
}

// An Analog reads back the same set value.
type Analog struct {
	pin        string
	Value      int
	CloseCount int
	Mu         sync.RWMutex
	fakeValue  int
}

func newAnalogReader(pin string) *Analog {
	return &Analog{pin: pin}
}

func (a *Analog) reset(pin string) {
	a.Mu.Lock()
	a.pin = pin
	a.Value = 0
	a.Mu.Unlock()
}

func (a *Analog) Read(ctx context.Context, extra map[string]interface{}) (board.AnalogValue, error) {
	a.Mu.RLock()
	defer a.Mu.RUnlock()
	if a.pin != analogTestPin {
		a.fakeValue++
		a.fakeValue %= 1001
		a.Value = a.fakeValue
	}
	return board.AnalogValue{Value: a.Value, Min: 0, Max: 1000, StepSize: 1}, nil
}

func (a *Analog) Write(ctx context.Context, value int, extra map[string]interface{}) error {
	a.Set(value)
	return nil
}

// Set is used to set the value of an Analog.
func (a *Analog) Set(value int) {
	a.Mu.Lock()
	defer a.Mu.Unlock()
	a.Value = value
}

// A GPIOPin reads back the same set values.
type GPIOPin struct {
	high    bool
	pwm     float64
	pwmFreq uint

	mu sync.Mutex
}

// Set sets the pin to either low or high.
func (gp *GPIOPin) Set(ctx context.Context, high bool, extra map[string]interface{}) error {
	gp.mu.Lock()
	defer gp.mu.Unlock()

	gp.high = high
	gp.pwm = 0
	gp.pwmFreq = 0
	return nil
}

// Get gets the high/low state of the pin.
func (gp *GPIOPin) Get(ctx context.Context, extra map[string]interface{}) (bool, error) {
	gp.mu.Lock()
	defer gp.mu.Unlock()

	return gp.high, nil
}

// PWM gets the pin's given duty cycle.
func (gp *GPIOPin) PWM(ctx context.Context, extra map[string]interface{}) (float64, error) {
	gp.mu.Lock()
	defer gp.mu.Unlock()

	return gp.pwm, nil
}

// SetPWM sets the pin to the given duty cycle.
func (gp *GPIOPin) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
	gp.mu.Lock()
	defer gp.mu.Unlock()

	gp.pwm = dutyCyclePct
	return nil
}

// PWMFreq gets the PWM frequency of the pin.
func (gp *GPIOPin) PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error) {
	gp.mu.Lock()
	defer gp.mu.Unlock()

	return gp.pwmFreq, nil
}

// SetPWMFreq sets the given pin to the given PWM frequency.
func (gp *GPIOPin) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
	gp.mu.Lock()
	defer gp.mu.Unlock()

	gp.pwmFreq = freqHz
	return nil
}

// DigitalInterrupt is a fake digital interrupt.
type DigitalInterrupt struct {
	mu    sync.Mutex
	conf  board.DigitalInterruptConfig
	value int64
}

// NewDigitalInterrupt returns a new fake digital interrupt.
func NewDigitalInterrupt(conf board.DigitalInterruptConfig) (*DigitalInterrupt, error) {
	return &DigitalInterrupt{
		conf: conf,
	}, nil
}

func (s *DigitalInterrupt) reset(conf board.DigitalInterruptConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conf = conf
}

// Value returns the current value of the interrupt which is
// based on the type of interrupt.
func (s *DigitalInterrupt) Value(ctx context.Context, extra map[string]interface{}) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conf.Pin == nonZeroInterruptPin {
		s.value++
		return s.value, nil
	}
	return 0, nil
}

// Name returns the name of the digital interrupt.
func (s *DigitalInterrupt) Name() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conf.Name
}
