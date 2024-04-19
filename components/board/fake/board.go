// Package fake implements a fake board.
package fake

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/board/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/pinwrappers"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

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
		Digitals: map[string]*DigitalInterruptWrapper{},
		GPIOPins: map[string]*GPIOPin{},
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
				utils.UncheckedError(curr.reset(c))
			}
			continue
		}
		var err error
		b.Digitals[c.Name], err = NewDigitalInterruptWrapper(c)
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
	Digitals   map[string]*DigitalInterruptWrapper
	GPIOPins   map[string]*GPIOPin
	logger     logging.Logger
	CloseCount int
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
func (b *Board) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	d, ok := b.Digitals[name]
	return d, ok
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

// AnalogNames returns the names of all known analog pins.
func (b *Board) AnalogNames() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	names := []string{}
	for k := range b.Analogs {
		names = append(names, k)
	}
	return names
}

// DigitalInterruptNames returns the names of all known digital interrupts.
func (b *Board) DigitalInterruptNames() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	names := []string{}
	for k := range b.Digitals {
		names = append(names, k)
	}
	return names
}

// Status returns the current status of the board.
func (b *Board) Status(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
	return board.CreateStatus(ctx, b, extra)
}

// SetPowerMode sets the board to the given power mode. If provided,
// the board will exit the given power mode after the specified
// duration.
func (b *Board) SetPowerMode(ctx context.Context, mode pb.PowerMode, duration *time.Duration) error {
	return grpc.UnimplementedError
}

// WriteAnalog writes the value to the given pin, which can be read back by adding it to AnalogReaders.
func (b *Board) WriteAnalog(ctx context.Context, pin string, value int32, extra map[string]interface{}) error {
	alg := &Analog{pin: pin, Value: int(value)}
	b.Analogs[pin] = alg
	return nil
}

// StreamTicks starts a stream of digital interrupt ticks.
func (b *Board) StreamTicks(ctx context.Context, interruptNames []string, ch chan board.Tick, extra map[string]interface{}) error {
	var interrupts []board.DigitalInterrupt
	for _, name := range interruptNames {
		interrupt, ok := b.DigitalInterruptByName(name)
		if !ok {
			return errors.Errorf("unknown digital interrupt: %s", name)
		}
		interrupts = append(interrupts, interrupt)
	}

	for _, i := range interrupts {
		i.AddCallback(ch)
	}
	return nil
}

// Close attempts to cleanly close each part of the board.
func (b *Board) Close(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.CloseCount++
	var err error

	for _, analog := range b.Analogs {
		err = multierr.Combine(err, analog.Close(ctx))
	}
	for _, digital := range b.Digitals {
		err = multierr.Combine(err, digital.Close(ctx))
	}
	return err
}

// An Analog reads back the same set value.
type Analog struct {
	pin        string
	Value      int
	CloseCount int
	Mu         sync.RWMutex
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

func (a *Analog) Read(ctx context.Context, extra map[string]interface{}) (int, error) {
	a.Mu.RLock()
	defer a.Mu.RUnlock()
	return a.Value, nil
}

func (a *Analog) Write(ctx context.Context, value int, extra map[string]interface{}) error {
	a.Set(value)
	return nil
}

// Set is used during testing.
func (a *Analog) Set(value int) {
	a.Mu.Lock()
	defer a.Mu.Unlock()
	a.Value = value
}

// Close does nothing.
func (a *Analog) Close(ctx context.Context) error {
	a.CloseCount++
	return nil
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

// DigitalInterruptWrapper is a wrapper around a digital interrupt for testing fake boards.
type DigitalInterruptWrapper struct {
	mu        sync.Mutex
	di        board.DigitalInterrupt
	conf      board.DigitalInterruptConfig
	callbacks map[chan board.Tick]struct{}
}

// NewDigitalInterruptWrapper returns a new digital interrupt to be used for testing.
func NewDigitalInterruptWrapper(conf board.DigitalInterruptConfig) (*DigitalInterruptWrapper, error) {
	di, err := pinwrappers.CreateDigitalInterrupt(conf)
	if err != nil {
		return nil, err
	}
	return &DigitalInterruptWrapper{
		di:        di,
		callbacks: map[chan board.Tick]struct{}{},
		conf:      conf,
	}, nil
}

func (s *DigitalInterruptWrapper) reset(conf board.DigitalInterruptConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	reconf, isReconf := s.di.(pinwrappers.ReconfigurableDigitalInterrupt)
	if conf.Name != s.conf.Name || !isReconf {
		// rebuild
		di, err := pinwrappers.CreateDigitalInterrupt(conf)
		if err != nil {
			return err
		}
		s.conf = conf
		s.di = di
		for c := range s.callbacks {
			s.di.AddCallback(c)
		}
		return nil
	}
	// reconf
	if err := reconf.Reconfigure(conf); err != nil {
		return err
	}
	s.conf = conf
	return nil
}

// Value returns the current value of the interrupt which is
// based on the type of interrupt.
func (s *DigitalInterruptWrapper) Value(ctx context.Context, extra map[string]interface{}) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.di.Value(ctx, extra)
}

// Tick is to be called either manually if the interrupt is a proxy to some real
// hardware interrupt or for tests.
// nanoseconds is from an arbitrary point in time, but always increasing and always needs
// to be accurate.
func (s *DigitalInterruptWrapper) Tick(ctx context.Context, high bool, nanoseconds uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.di.Tick(ctx, high, nanoseconds)
}

// AddCallback adds a callback to be sent a low/high value to when a tick
// happens.
func (s *DigitalInterruptWrapper) AddCallback(c chan board.Tick) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callbacks[c] = struct{}{}
	s.di.AddCallback(c)
}

// RemoveCallback removes a listener for interrupts.
func (s *DigitalInterruptWrapper) RemoveCallback(c chan board.Tick) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.callbacks, c)
	s.di.RemoveCallback(c)
}

// Close does nothing.
func (s *DigitalInterruptWrapper) Close(ctx context.Context) error {
	return nil
}
