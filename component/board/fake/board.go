package fake

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/core/component/board"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
)

const modelName = "fake"

func init() {
	registry.RegisterComponent(
		board.Subtype,
		modelName,
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			if config.Attributes.Bool("fail_new", false) {
				return nil, errors.New("whoops")
			}
			return NewBoard(ctx, config, logger)
		}})
	board.RegisterConfigAttributeConverter(modelName)
}

// NewBoard returns a new fake board.
func NewBoard(ctx context.Context, config config.Component, logger golog.Logger) (*Board, error) {
	boardConfig := config.ConvertedAttributes.(*board.Config)

	b := &Board{
		Name:     config.Name,
		I2Cs:     map[string]*I2C{},
		SPIs:     map[string]*SPI{},
		Analogs:  map[string]*Analog{},
		Digitals: map[string]board.DigitalInterrupt{},
	}

	for _, c := range boardConfig.I2Cs {
		b.I2Cs[c.Name] = &I2C{}
	}

	for _, c := range boardConfig.SPIs {
		b.SPIs[c.Name] = &SPI{}
	}

	for _, c := range boardConfig.Analogs {
		b.Analogs[c.Name] = &Analog{}
	}

	for _, c := range boardConfig.DigitalInterrupts {
		var err error
		b.Digitals[c.Name], err = board.CreateDigitalInterrupt(c)
		if err != nil {
			return nil, err
		}
	}

	return b, nil
}

// A Board provides dummy data from fake parts in order to implement a Board.
type Board struct {
	Name     string
	SPIs     map[string]*SPI
	I2Cs     map[string]*I2C
	Analogs  map[string]*Analog
	Digitals map[string]board.DigitalInterrupt

	GPIO    map[string]bool
	PWM     map[string]byte
	PWMFreq map[string]uint

	CloseCount int
}

// SPIByName returns the SPI by the given name if it exists.
func (b *Board) SPIByName(name string) (board.SPI, bool) {
	s, ok := b.SPIs[name]
	return s, ok
}

// I2CByName returns the i2c by the given name if it exists.
func (b *Board) I2CByName(name string) (board.I2C, bool) {
	s, ok := b.I2Cs[name]
	return s, ok
}

// AnalogReaderByName returns the analog reader by the given name if it exists.
func (b *Board) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	a, ok := b.Analogs[name]
	return a, ok
}

// DigitalInterruptByName returns the interrupt by the given name if it exists.
func (b *Board) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
	d, ok := b.Digitals[name]
	return d, ok
}

// GPIOSet sets the given pin to either low or high.
func (b *Board) GPIOSet(ctx context.Context, pin string, high bool) error {
	if b.GPIO == nil {
		b.GPIO = map[string]bool{}
	}
	b.GPIO[pin] = high
	if high {
		return b.PWMSet(ctx, pin, 255)
	}
	return b.PWMSet(ctx, pin, 0)
}

// GPIOGet returns whether the given pin is either low or high.
func (b *Board) GPIOGet(ctx context.Context, pin string) (bool, error) {
	if b.GPIO == nil {
		b.GPIO = map[string]bool{}
	}
	return b.GPIO[pin], nil
}

// PWMSet sets the given pin to the given duty cycle.
func (b *Board) PWMSet(ctx context.Context, pin string, dutyCycle byte) error {
	if b.PWM == nil {
		b.PWM = map[string]byte{}
	}
	if b.PWM[pin] != dutyCycle {
		b.PWM[pin] = dutyCycle
		if dutyCycle == 255 {
			return b.GPIOSet(ctx, pin, true)
		} else if dutyCycle == 0 {
			return b.GPIOSet(ctx, pin, false)
		}
	}
	return nil
}

// PWMSetFreq sets the given pin to the given PWM frequency. 0 will use the board's default PWM frequency.
func (b *Board) PWMSetFreq(ctx context.Context, pin string, freq uint) error {
	if b.PWMFreq == nil {
		b.PWMFreq = map[string]uint{}
	}
	b.PWMFreq[pin] = freq
	return nil
}

// SPINames returns the name of all known SPIs.
func (b *Board) SPINames() []string {
	names := []string{}
	for k := range b.SPIs {
		names = append(names, k)
	}
	return names
}

// I2CNames returns the name of all known I2Cs.
func (b *Board) I2CNames() []string {
	names := []string{}
	for k := range b.I2Cs {
		names = append(names, k)
	}
	return names
}

// AnalogReaderNames returns the name of all known analog readers.
func (b *Board) AnalogReaderNames() []string {
	names := []string{}
	for k := range b.Analogs {
		names = append(names, k)
	}
	return names
}

// DigitalInterruptNames returns the name of all known digital interrupts.
func (b *Board) DigitalInterruptNames() []string {
	names := []string{}
	for k := range b.Digitals {
		names = append(names, k)
	}
	return names
}

// Status returns the current status of the board.
func (b *Board) Status(ctx context.Context) (*pb.BoardStatus, error) {
	return board.CreateStatus(ctx, b)
}

// ModelAttributes returns attributes related to the model of this board.
func (b *Board) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{}
}

// Close attempts to cleanly close each part of the board.
func (b *Board) Close() error {
	b.CloseCount++
	var err error

	for _, analog := range b.Analogs {
		err = multierr.Combine(err, utils.TryClose(analog))
	}

	for _, digital := range b.Digitals {
		err = multierr.Combine(err, utils.TryClose(digital))
	}
	return err
}

// A SPI allows opening an SPIHandle.
type SPI struct {
	mu   sync.Mutex
	FIFO chan []byte
}

// OpenHandle opens a handle to perform SPI transfers that must be later closed to release access to the bus.
func (s *SPI) OpenHandle() (board.SPIHandle, error) {
	s.mu.Lock()
	return &SPIHandle{s}, nil
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
	mu   sync.Mutex
	fifo chan []byte
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

// ReadByteData reads a byte from the i2c channelC
func (h *I2CHandle) ReadByteData(ctx context.Context, register byte) (byte, error) {
	return 0, errors.New("finish me")
}

// WriteByteData writes a byte to the i2c channelC
func (h *I2CHandle) WriteByteData(ctx context.Context, register byte, data byte) error {
	return errors.New("finish me")
}

// ReadWordData reads a word from the i2c channelC
func (h *I2CHandle) ReadWordData(ctx context.Context, register byte) (uint16, error) {
	return 0, errors.New("finish me")
}

// WriteWordData writes a word to the i2c channelC
func (h *I2CHandle) WriteWordData(ctx context.Context, register byte, data uint16) error {
	return errors.New("finish me")
}

// Close releases access to the bus
func (h *I2CHandle) Close() error {
	h.bus.mu.Unlock()
	return nil
}

// A Analog reads back the same set value.
type Analog struct {
	Value      int
	CloseCount int
}

func (a *Analog) Read(context.Context) (int, error) {
	return a.Value, nil
}

// Close does nothing.
func (a *Analog) Close() error {
	a.CloseCount++
	return nil
}
