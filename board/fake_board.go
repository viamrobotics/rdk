package board

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"

	"go.viam.com/utils"

	pb "go.viam.com/core/proto/api/v1"
)

// init registers a fake board.
func init() {
	RegisterBoard("fake", func(ctx context.Context, cfg Config, logger golog.Logger) (Board, error) {
		return NewFakeBoard(ctx, cfg, logger)
	})
}

// A fakeServo allows setting and reading a single angle.
type fakeServo struct {
	angle uint8
}

func (s *fakeServo) Move(ctx context.Context, angle uint8) error {
	s.angle = angle
	return nil
}

func (s *fakeServo) Current(ctx context.Context) (uint8, error) {
	return s.angle, nil
}

// A fakeSPI allows opening an SPIHandle.
type fakeSPI struct {
	mu   sync.Mutex
	fifo chan []byte
}

func (s *fakeSPI) OpenHandle() (SPIHandle, error) {
	s.mu.Lock()
	return &fakeSPIHandle{s}, nil
}

// A fakeSPIHandle allows Xfer and Close.
type fakeSPIHandle struct {
	bus *fakeSPI
}

func (h *fakeSPIHandle) Xfer(baud uint, chipSelect string, mode uint, tx []byte) ([]byte, error) {
	h.bus.fifo <- tx
	ret := <-h.bus.fifo
	return ret[:len(tx)], nil
}

func (h *fakeSPIHandle) Close() error {
	h.bus.mu.Unlock()
	return nil
}

// A FakeAnalog reads back the same set value.
type FakeAnalog struct {
	Value      int
	CloseCount int
}

func (a *FakeAnalog) Read(context.Context) (int, error) {
	return a.Value, nil
}

// Close does nothing.
func (a *FakeAnalog) Close() error {
	a.CloseCount++
	return nil
}

// A FakeBoard provides dummy data from fake parts in order to implement a Board.
type FakeBoard struct {
	Name     string
	motors   map[string]*FakeMotor
	servos   map[string]*fakeServo
	spis     map[string]*fakeSPI
	analogs  map[string]*FakeAnalog
	digitals map[string]DigitalInterrupt

	gpio    map[string]bool
	pwm     map[string]byte
	pwmFreq map[string]uint

	CloseCount int
}

// MotorByName returns the motor by the given name if it exists.
func (b *FakeBoard) MotorByName(name string) (Motor, bool) {
	m, ok := b.motors[name]
	return m, ok
}

// ServoByName returns the servo by the given name if it exists.
func (b *FakeBoard) ServoByName(name string) (Servo, bool) {
	s, ok := b.servos[name]
	return s, ok
}

// SPIByName returns the servo by the given name if it exists.
func (b *FakeBoard) SPIByName(name string) (SPI, bool) {
	s, ok := b.spis[name]
	return s, ok
}

// AnalogReaderByName returns the analog reader by the given name if it exists.
func (b *FakeBoard) AnalogReaderByName(name string) (AnalogReader, bool) {
	a, ok := b.analogs[name]
	return a, ok
}

// DigitalInterruptByName returns the interrupt by the given name if it exists.
func (b *FakeBoard) DigitalInterruptByName(name string) (DigitalInterrupt, bool) {
	d, ok := b.digitals[name]
	return d, ok
}

// GPIOSet sets the given pin to either low or high.
func (b *FakeBoard) GPIOSet(ctx context.Context, pin string, high bool) error {
	if b.gpio == nil {
		b.gpio = map[string]bool{}
	}
	b.gpio[pin] = high
	return nil
}

// GPIOGet returns whether the given pin is either low or high.
func (b *FakeBoard) GPIOGet(ctx context.Context, pin string) (bool, error) {
	if b.gpio == nil {
		b.gpio = map[string]bool{}
	}
	return b.gpio[pin], nil
}

// PWMSet sets the given pin to the given duty cycle.
func (b *FakeBoard) PWMSet(ctx context.Context, pin string, dutyCycle byte) error {
	if b.pwm == nil {
		b.pwm = map[string]byte{}
	}
	b.pwm[pin] = dutyCycle
	return nil
}

// PWMSetFreq sets the given pin to the given PWM frequency. 0 will use the board's default PWM frequency.
func (b *FakeBoard) PWMSetFreq(ctx context.Context, pin string, freq uint) error {
	if b.pwmFreq == nil {
		b.pwmFreq = map[string]uint{}
	}
	b.pwmFreq[pin] = freq
	return nil
}

// MotorNames returns the name of all known motors.
func (b *FakeBoard) MotorNames() []string {
	names := []string{}
	for k := range b.motors {
		names = append(names, k)
	}
	return names
}

// ServoNames returns the name of all known servos.
func (b *FakeBoard) ServoNames() []string {
	names := []string{}
	for k := range b.servos {
		names = append(names, k)
	}
	return names
}

// SPINames returns the name of all known SPIs.
func (b *FakeBoard) SPINames() []string {
	names := []string{}
	for k := range b.spis {
		names = append(names, k)
	}
	return names
}

// AnalogReaderNames returns the name of all known analog readers.
func (b *FakeBoard) AnalogReaderNames() []string {
	names := []string{}
	for k := range b.analogs {
		names = append(names, k)
	}
	return names
}

// DigitalInterruptNames returns the name of all known digital interrupts.
func (b *FakeBoard) DigitalInterruptNames() []string {
	names := []string{}
	for k := range b.digitals {
		names = append(names, k)
	}
	return names
}

// Status returns the current status of the board.
func (b *FakeBoard) Status(ctx context.Context) (*pb.BoardStatus, error) {
	return CreateStatus(ctx, b)
}

// ModelAttributes returns attributes related to the model of this board.
func (b *FakeBoard) ModelAttributes() ModelAttributes {
	return ModelAttributes{}
}

// Close attempts to cleanly close each part of the board.
func (b *FakeBoard) Close() error {
	b.CloseCount++
	var err error
	for _, motor := range b.motors {
		err = multierr.Combine(err, utils.TryClose(motor))
	}

	for _, servo := range b.servos {
		err = multierr.Combine(err, utils.TryClose(servo))
	}

	for _, analog := range b.analogs {
		err = multierr.Combine(err, utils.TryClose(analog))
	}

	for _, digital := range b.digitals {
		err = multierr.Combine(err, utils.TryClose(digital))
	}
	return err
}

// NewFakeBoard constructs a new board with fake parts based on the given Config.
func NewFakeBoard(ctx context.Context, cfg Config, logger golog.Logger) (*FakeBoard, error) {
	var err error

	b := &FakeBoard{
		Name:     cfg.Name,
		motors:   map[string]*FakeMotor{},
		servos:   map[string]*fakeServo{},
		spis:     map[string]*fakeSPI{},
		analogs:  map[string]*FakeAnalog{},
		digitals: map[string]DigitalInterrupt{},
	}

	for _, c := range cfg.Motors {
		b.motors[c.Name] = &FakeMotor{mu: &sync.Mutex{}}
	}

	for _, c := range cfg.Servos {
		b.servos[c.Name] = &fakeServo{}
	}

	for _, c := range cfg.SPIs {
		b.spis[c.Name] = &fakeSPI{}
	}

	for _, c := range cfg.Analogs {
		b.analogs[c.Name] = &FakeAnalog{}
	}

	for _, c := range cfg.DigitalInterrupts {
		b.digitals[c.Name], err = CreateDigitalInterrupt(c)
		if err != nil {
			return nil, err
		}
	}

	return b, nil
}
