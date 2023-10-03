// Package fake implements a fake board.
package fake

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/board/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

// A Config describes the configuration of a fake board and all of its connected parts.
type Config struct {
	I2Cs              []board.I2CConfig              `json:"i2cs,omitempty"`
	SPIs              []board.SPIConfig              `json:"spis,omitempty"`
	Analogs           []board.AnalogConfig           `json:"analogs,omitempty"`
	DigitalInterrupts []board.DigitalInterruptConfig `json:"digital_interrupts,omitempty"`
	Attributes        rdkutils.AttributeMap          `json:"attributes,omitempty"`
	FailNew           bool                           `json:"fail_new"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	for idx, conf := range conf.SPIs {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "spis", idx)); err != nil {
			return nil, err
		}
	}
	for idx, conf := range conf.I2Cs {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "i2cs", idx)); err != nil {
			return nil, err
		}
	}
	for idx, conf := range conf.Analogs {
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
				logger golog.Logger,
			) (board.Board, error) {
				return NewBoard(ctx, cfg, logger)
			},
		})
}

// NewBoard returns a new fake board.
func NewBoard(ctx context.Context, conf resource.Config, logger golog.Logger) (*Board, error) {
	b := &Board{
		Named:    conf.ResourceName().AsNamed(),
		I2Cs:     map[string]*I2C{},
		SPIs:     map[string]*SPI{},
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
	for _, c := range newConf.I2Cs {
		stillExists[c.Name] = struct{}{}
		if curr, ok := b.I2Cs[c.Name]; ok {
			if curr.bus != c.Bus {
				curr.reset(c.Bus)
			}
			continue
		}
		b.I2Cs[c.Name] = newI2C(c.Bus)
	}
	for name := range b.I2Cs {
		if _, ok := stillExists[name]; ok {
			continue
		}
		delete(b.I2Cs, name)
	}
	stillExists = map[string]struct{}{}

	for _, c := range newConf.SPIs {
		stillExists[c.Name] = struct{}{}
		if curr, ok := b.SPIs[c.Name]; ok {
			if curr.busSelect != c.BusSelect {
				curr.reset(c.BusSelect)
			}
			continue
		}
		b.SPIs[c.Name] = newSPI(c.BusSelect)
	}
	for name := range b.SPIs {
		if _, ok := stillExists[name]; ok {
			continue
		}
		delete(b.SPIs, name)
	}
	stillExists = map[string]struct{}{}

	for _, c := range newConf.Analogs {
		stillExists[c.Name] = struct{}{}
		if curr, ok := b.Analogs[c.Name]; ok {
			if curr.chipSelect != c.ChipSelect {
				curr.reset(c.ChipSelect)
			}
			continue
		}
		b.Analogs[c.Name] = newAnalog(c.ChipSelect)
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

// Reconfigure atomically reconfigures this board© in place based on the new config.
func (b *Board) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	return b.processConfig(conf)
}

// A Board provides dummy data from fake parts in order to implement a Board.
type Board struct {
	resource.Named

	mu         sync.RWMutex
	SPIs       map[string]*SPI
	I2Cs       map[string]*I2C
	Analogs    map[string]*Analog
	Digitals   map[string]*DigitalInterruptWrapper
	GPIOPins   map[string]*GPIOPin
	logger     golog.Logger
	CloseCount int
}

// SPIByName returns the SPI by the given name if it exists.
func (b *Board) SPIByName(name string) (board.SPI, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	s, ok := b.SPIs[name]
	return s, ok
}

// I2CByName returns the i2c by the given name if it exists.
func (b *Board) I2CByName(name string) (board.I2C, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	s, ok := b.I2Cs[name]
	return s, ok
}

// AnalogReaderByName returns the analog reader by the given name if it exists.
func (b *Board) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	a, ok := b.Analogs[name]
	return a, ok
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

// SPINames returns the names of all known SPIs.
func (b *Board) SPINames() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	names := []string{}
	for k := range b.SPIs {
		names = append(names, k)
	}
	return names
}

// I2CNames returns the names of all known I2Cs.
func (b *Board) I2CNames() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	names := []string{}
	for k := range b.I2Cs {
		names = append(names, k)
	}
	return names
}

// AnalogReaderNames returns the names of all known analog readers.
func (b *Board) AnalogReaderNames() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	names := []string{}
	for k := range b.Analogs {
		names = append(names, k)
	}
	return names
}

// AnalogWriterNames returns the names of all known analog writers.
// Unimplemented.
func (b *Board) AnalogWriterNames() []string {
	return nil
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

// GPIOPinNames returns the names of all known GPIO pins.
func (b *Board) GPIOPinNames() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	names := []string{}
	for k := range b.GPIOPins {
		names = append(names, k)
	}
	return names
}

// Status returns the current status of the board.
func (b *Board) Status(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
	return board.CreateStatus(ctx, b, extra)
}

// ModelAttributes returns attributes related to the model of this board.
func (b *Board) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{}
}

// SetPowerMode sets the board to the given power mode. If provided,
// the board will exit the given power mode after the specified
// duration.
func (b *Board) SetPowerMode(ctx context.Context, mode pb.PowerMode, duration *time.Duration) error {
	return grpc.UnimplementedError
}

// WriteAnalog writes the value to the given pin.
func (b *Board) WriteAnalog(ctx context.Context, pin string, value int32, extra map[string]interface{}) error {
	return grpc.UnimplementedError
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

// A SPI allows opening an SPIHandle.
type SPI struct {
	FIFO chan []byte

	mu        sync.Mutex
	busSelect string
}

func newSPI(busSelect string) *SPI {
	return &SPI{busSelect: busSelect}
}

func (s *SPI) reset(busSelect string) {
	s.mu.Lock()
	s.busSelect = busSelect
	s.mu.Unlock()
}

// OpenHandle opens a handle to perform SPI transfers that must be later closed to release access to the bus.
func (s *SPI) OpenHandle() (board.SPIHandle, error) {
	s.mu.Lock()
	return &SPIHandle{s}, nil
}

// Close does nothing.
func (s *SPI) Close(ctx context.Context) error {
	return nil
}

// A SPIHandle allows Xfer and Close.
type SPIHandle struct {
	bus *SPI
}

// Xfer transfers the given data.
func (h *SPIHandle) Xfer(ctx context.Context, baud uint, chipSelect string, mode uint, tx []byte) ([]byte, error) {
	h.bus.FIFO <- tx
	ret := <-h.bus.FIFO
	return ret[:len(tx)], nil
}

// Close releases access to the bus.
func (h *SPIHandle) Close() error {
	h.bus.mu.Unlock()
	return nil
}

// A I2C allows opening an I2CHandle.
type I2C struct {
	fifo chan []byte

	mu  sync.Mutex
	bus string
}

func newI2C(bus string) *I2C {
	return &I2C{bus: bus}
}

func (s *I2C) reset(bus string) {
	s.mu.Lock()
	s.bus = bus
	s.mu.Unlock()
}

// OpenHandle opens a handle to perform I2C transfers that must be later closed to release access to the bus.
func (s *I2C) OpenHandle(addr byte) (board.I2CHandle, error) {
	s.mu.Lock()
	return &I2CHandle{s, addr}, nil
}

// A I2CHandle allows read/write and Close.
type I2CHandle struct {
	bus  *I2C
	addr byte
}

func (h *I2CHandle) Write(ctx context.Context, tx []byte) error {
	h.bus.fifo <- tx
	return nil
}

func (h *I2CHandle) Read(ctx context.Context, count int) ([]byte, error) {
	ret := <-h.bus.fifo
	return ret[:count], nil
}

// ReadByteData reads a byte from the i2c channel.
func (h *I2CHandle) ReadByteData(ctx context.Context, register byte) (byte, error) {
	return 0, errors.New("finish me")
}

// WriteByteData writes a byte to the i2c channel.
func (h *I2CHandle) WriteByteData(ctx context.Context, register, data byte) error {
	return errors.New("finish me")
}

// ReadBlockData reads the given number of bytes from the i2c channel.
func (h *I2CHandle) ReadBlockData(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
	return nil, errors.New("finish me")
}

// WriteBlockData writes the given bytes to the i2c channel.
func (h *I2CHandle) WriteBlockData(ctx context.Context, register byte, data []byte) error {
	return errors.New("finish me")
}

// Close releases access to the bus.
func (h *I2CHandle) Close() error {
	h.bus.mu.Unlock()
	return nil
}

// A Analog reads back the same set value.
type Analog struct {
	Value      int
	CloseCount int
	Mu         sync.RWMutex
	chipSelect string
}

func newAnalog(chipSelect string) *Analog {
	return &Analog{chipSelect: chipSelect}
}

func (a *Analog) reset(chipSelect string) {
	a.Mu.Lock()
	a.chipSelect = chipSelect
	a.Value = 0
	a.Mu.Unlock()
}

func (a *Analog) Read(ctx context.Context, extra map[string]interface{}) (int, error) {
	a.Mu.RLock()
	defer a.Mu.RUnlock()
	return a.Value, nil
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
	pps       []board.PostProcessor
}

// NewDigitalInterruptWrapper returns a new digital interrupt to be used for testing.
func NewDigitalInterruptWrapper(conf board.DigitalInterruptConfig) (*DigitalInterruptWrapper, error) {
	di, err := board.CreateDigitalInterrupt(conf)
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
	reconf, isReconf := s.di.(board.ReconfigurableDigitalInterrupt)
	if conf.Name != s.conf.Name || !isReconf {
		// rebuild
		di, err := board.CreateDigitalInterrupt(conf)
		if err != nil {
			return err
		}
		s.conf = conf
		s.di = di
		for c := range s.callbacks {
			s.di.AddCallback(c)
		}
		for _, pp := range s.pps {
			s.di.AddPostProcessor(pp)
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

// AddPostProcessor adds a post processor that should be used to modify
// what is returned by Value.
func (s *DigitalInterruptWrapper) AddPostProcessor(pp board.PostProcessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pps = append(s.pps, pp)
	s.di.AddPostProcessor(pp)
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
