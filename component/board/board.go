// Package board defines the interfaces that typically live on a single-board computer
// such as a Raspberry Pi.
//
// Besides the board itself, some other interfaces it defines are analog readers and digital interrupts.
package board

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-errors/errors"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/rlog"
	"go.viam.com/utils"
)

// SubtypeName is a constant that identifies the component resource subtype string "board"
const SubtypeName = resource.SubtypeName("board")

// Subtype is a constant that identifies the component resource subtype
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceCore,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named board's typed resource name
func Named(name string) resource.Name {
	return resource.NewFromSubtype(Subtype, name)
}

// A Board represents a physical general purpose board that contains various
// components such as analog readers, and digital interrupts.
type Board interface {
	// SPIByName returns an SPI bus by name.
	SPIByName(name string) (SPI, bool)

	// I2CByName returns an I2C bus by name.
	I2CByName(name string) (I2C, bool)

	// AnalogReaderByName returns an analog reader by name.
	AnalogReaderByName(name string) (AnalogReader, bool)

	// DigitalInterruptByName returns a digital interrupt by name.
	DigitalInterruptByName(name string) (DigitalInterrupt, bool)

	// SPINames returns the name of all known SPI busses.
	SPINames() []string

	// I2CNames returns the name of all known I2C busses.
	I2CNames() []string

	// AnalogReaderNames returns the name of all known analog readers.
	AnalogReaderNames() []string

	// DigitalInterruptNames returns the name of all known digital interrupts.
	DigitalInterruptNames() []string

	// GPIOSet sets the given pin to either low or high.
	GPIOSet(ctx context.Context, pin string, high bool) error

	// GPIOGet gets the high/low state of the given pin.
	GPIOGet(ctx context.Context, pin string) (bool, error)

	// PWMSet sets the given pin to the given duty cycle.
	PWMSet(ctx context.Context, pin string, dutyCycle byte) error

	// PWMSetFreq sets the given pin to the given PWM frequency. 0 will use the board's default PWM frequency.
	PWMSetFreq(ctx context.Context, pin string, freq uint) error

	// Status returns the current status of the board. Usually you
	// should use the CreateStatus helper instead of directly calling
	// this.
	Status(ctx context.Context) (*pb.BoardStatus, error)

	// ModelAttributes returns attributes related to the model of this board.
	ModelAttributes() ModelAttributes

	// Close shuts the board down, no methods should be called on the board after this
	Close() error
}

// ModelAttributes provide info related to a board model.
type ModelAttributes struct {
	// Remote signifies this board is accessed over a remote connection.
	// e.g. gRPC
	Remote bool
}

// SPI represents a shareable SPI bus on the board.
type SPI interface {
	// OpenHandle locks the shared bus and returns a handle interface that MUST be closed when done.
	OpenHandle() (SPIHandle, error)
}

// SPIHandle is similar to an io handle. It MUST be closed to release the bus.
type SPIHandle interface {
	// Xfer performs a single SPI transfer, that is, the complete transaction from chipselect enable to chipselect disable.
	// SPI transfers are synchronous, number of bytes received will be equal to the number of bytes sent.
	// Write-only transfers can usually just discard the returned bytes.
	// Read-only transfers usually transmit a request/address and continue with some number of null bytes to equal the expected size of the returning data.
	// Large transmissions are usually broken up into multiple transfers.
	// There are many different paradigms for most of the above, and implementation details are chip/device specific.
	Xfer(ctx context.Context, baud uint, chipSelect string, mode uint, tx []byte) ([]byte, error)
	// Close closes the handle and releases the lock on the bus.
	Close() error
}

// I2C represents a shareable I2C bus on the board.
type I2C interface {
	// OpenHandle locks returns a handle interface that MUST be closed when done.
	// you cannot have 2 open for the same addr
	OpenHandle(addr byte) (I2CHandle, error)
}

// I2CHandle is similar to an io handle. It MUST be closed to release the bus.
type I2CHandle interface {
	Write(ctx context.Context, tx []byte) error
	Read(ctx context.Context, count int) ([]byte, error)

	ReadByteData(ctx context.Context, register byte) (byte, error)
	WriteByteData(ctx context.Context, register byte, data byte) error

	ReadWordData(ctx context.Context, register byte) (uint16, error)
	WriteWordData(ctx context.Context, register byte, data uint16) error

	// Close closes the handle and releases the lock on the bus.
	Close() error
}

// An AnalogReader represents an analog pin reader that resides on a board.
type AnalogReader interface {
	// Read reads off the current value.
	Read(ctx context.Context) (int, error)
}

// A PostProcessor takes a raw input and transforms it into a new value.
// Multiple post processors can be stacked on each other. This is currently
// only used in DigitalInterrupt readings.
type PostProcessor func(raw int64) int64

var (
	_ = Board(&reconfigurableBoard{})
	_ = resource.Reconfigurable(&reconfigurableBoard{})
)

// IMPORTED FROM robot/impl/proxy.go

type reconfigurableBoard struct {
	mu       sync.RWMutex
	actual   Board
	spis     map[string]*reconfigurableBoardSPI
	i2cs     map[string]*reconfigurableBoardI2C
	analogs  map[string]*reconfigurableBoardAnalogReader
	digitals map[string]*reconfigurableBoardDigitalInterrupt
}

func (r *reconfigurableBoard) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableBoard) SPIByName(name string) (SPI, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.spis[name]
	return s, ok
}

func (r *reconfigurableBoard) I2CByName(name string) (I2C, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.i2cs[name]
	return s, ok
}

func (r *reconfigurableBoard) AnalogReaderByName(name string) (AnalogReader, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.analogs[name]
	return a, ok
}

func (r *reconfigurableBoard) DigitalInterruptByName(name string) (DigitalInterrupt, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.digitals[name]
	return d, ok
}

func (r *reconfigurableBoard) GPIOSet(ctx context.Context, pin string, high bool) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GPIOSet(ctx, pin, high)
}

func (r *reconfigurableBoard) GPIOGet(ctx context.Context, pin string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GPIOGet(ctx, pin)
}

func (r *reconfigurableBoard) PWMSet(ctx context.Context, pin string, dutyCycle byte) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.PWMSet(ctx, pin, dutyCycle)
}

func (r *reconfigurableBoard) PWMSetFreq(ctx context.Context, pin string, freq uint) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.PWMSetFreq(ctx, pin, freq)
}

func (r *reconfigurableBoard) SPINames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := []string{}
	for k := range r.spis {
		names = append(names, k)
	}
	return names
}

func (r *reconfigurableBoard) I2CNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := []string{}
	for k := range r.i2cs {
		names = append(names, k)
	}
	return names
}

func (r *reconfigurableBoard) AnalogReaderNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := []string{}
	for k := range r.analogs {
		names = append(names, k)
	}
	return names
}

func (r *reconfigurableBoard) DigitalInterruptNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := []string{}
	for k := range r.digitals {
		names = append(names, k)
	}
	return names
}

func (r *reconfigurableBoard) Status(ctx context.Context) (*pb.BoardStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.actual.ModelAttributes().Remote {
		return r.actual.Status(ctx)
	}
	return CreateStatus(ctx, r)
}

func (r *reconfigurableBoard) Reconfigure(newBoard resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	actual, ok := newBoard.(*reconfigurableBoard)
	if !ok {
		return errors.Errorf("expected new board to be %T but got %T", r, newBoard)
	}
	if err := utils.TryClose(r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}

	var oldSPINames map[string]struct{}
	var oldI2CNames map[string]struct{}
	var oldAnalogReaderNames map[string]struct{}
	var oldDigitalInterruptNames map[string]struct{}

	if len(r.spis) != 0 {
		oldSPINames = make(map[string]struct{}, len(r.spis))
		for name := range r.spis {
			oldSPINames[name] = struct{}{}
		}
	}
	if len(r.i2cs) != 0 {
		oldI2CNames = make(map[string]struct{}, len(r.i2cs))
		for name := range r.i2cs {
			oldI2CNames[name] = struct{}{}
		}
	}
	if len(r.analogs) != 0 {
		oldAnalogReaderNames = make(map[string]struct{}, len(r.analogs))
		for name := range r.analogs {
			oldAnalogReaderNames[name] = struct{}{}
		}
	}
	if len(r.digitals) != 0 {
		oldDigitalInterruptNames = make(map[string]struct{}, len(r.digitals))
		for name := range r.digitals {
			oldDigitalInterruptNames[name] = struct{}{}
		}
	}

	for name, newPart := range actual.spis {
		oldPart, ok := r.spis[name]
		delete(oldSPINames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		r.spis[name] = newPart
	}
	for name, newPart := range actual.i2cs {
		oldPart, ok := r.i2cs[name]
		delete(oldI2CNames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		r.i2cs[name] = newPart
	}
	for name, newPart := range actual.analogs {
		oldPart, ok := r.analogs[name]
		delete(oldAnalogReaderNames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		r.analogs[name] = newPart
	}
	for name, newPart := range actual.digitals {
		oldPart, ok := r.digitals[name]
		delete(oldDigitalInterruptNames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		r.digitals[name] = newPart
	}

	for name := range oldSPINames {
		delete(r.spis, name)
	}
	for name := range oldI2CNames {
		delete(r.i2cs, name)
	}
	for name := range oldAnalogReaderNames {
		delete(r.analogs, name)
	}
	for name := range oldDigitalInterruptNames {
		delete(r.digitals, name)
	}

	r.actual = actual.actual
	return nil
}

func (r *reconfigurableBoard) ModelAttributes() ModelAttributes {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ModelAttributes()
}

// Close attempts to cleanly close each part of the board.
func (r *reconfigurableBoard) Close() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return utils.TryClose(r.actual)
}

// WrapWithReconfigurable converts a regular Board implementation to a reconfigurableBoard.
// If board is already a reconfigurableBoard, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	arm, ok := r.(Board)
	if !ok {
		return nil, errors.Errorf("expected resource to be Board but got %T", r)
	}
	if reconfigurable, ok := arm.(*reconfigurableBoard); ok {
		return reconfigurable, nil
	}
	return &reconfigurableBoard{actual: arm}, nil
}

type reconfigurableBoardSPI struct {
	mu     sync.RWMutex
	actual SPI
}

// TODO(maximpertsov): replace "replace" with Reconfigure?
func (r *reconfigurableBoardSPI) replace(newSPI SPI) {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newSPI.(*reconfigurableBoardSPI)
	if !ok {
		panic(fmt.Errorf("expected new SPI to be %T but got %T", actual, newSPI))
	}
	if err := utils.TryClose(r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
}

func (r *reconfigurableBoardSPI) OpenHandle() (SPIHandle, error) {
	return r.actual.OpenHandle()
}

type reconfigurableBoardI2C struct {
	mu     sync.RWMutex
	actual I2C
}

func (r *reconfigurableBoardI2C) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

// TODO(maximpertsov): replace "replace" with Reconfigure?
func (r *reconfigurableBoardI2C) replace(newI2C I2C) {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newI2C.(*reconfigurableBoardI2C)
	if !ok {
		panic(fmt.Errorf("expected new I2C to be %T but got %T", actual, newI2C))
	}
	if err := utils.TryClose(r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
}

func (r *reconfigurableBoardI2C) OpenHandle(addr byte) (I2CHandle, error) {
	return r.actual.OpenHandle(addr)
}

type reconfigurableBoardAnalogReader struct {
	mu     sync.RWMutex
	actual AnalogReader
}

// TODO(maximpertsov): replace "replace" with Reconfigure?
func (r *reconfigurableBoardAnalogReader) replace(newAnalogReader AnalogReader) {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newAnalogReader.(*reconfigurableBoardAnalogReader)
	if !ok {
		panic(fmt.Errorf("expected new analog reader to be %T but got %T", actual, newAnalogReader))
	}
	if err := utils.TryClose(r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
}

func (r *reconfigurableBoardAnalogReader) Read(ctx context.Context) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Read(ctx)
}

func (r *reconfigurableBoardAnalogReader) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableBoardAnalogReader) Close() error {
	return utils.TryClose(r.actual)
}

type reconfigurableBoardDigitalInterrupt struct {
	mu     sync.RWMutex
	actual DigitalInterrupt
}

func (r *reconfigurableBoardDigitalInterrupt) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

// TODO(maximpertsov): replace "replace" with Reconfigure?
func (r *reconfigurableBoardDigitalInterrupt) replace(newDigitalInterrupt DigitalInterrupt) {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newDigitalInterrupt.(*reconfigurableBoardDigitalInterrupt)
	if !ok {
		panic(fmt.Errorf("expected new digital interrupt to be %T but got %T", actual, newDigitalInterrupt))
	}
	if err := utils.TryClose(r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
}

func (r *reconfigurableBoardDigitalInterrupt) Config(ctx context.Context) (DigitalInterruptConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Config(ctx)
}

func (r *reconfigurableBoardDigitalInterrupt) Value(ctx context.Context) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Value(ctx)
}

func (r *reconfigurableBoardDigitalInterrupt) Tick(ctx context.Context, high bool, nanos uint64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Tick(ctx, high, nanos)
}

func (r *reconfigurableBoardDigitalInterrupt) AddCallback(c chan bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	r.actual.AddCallback(c)
}

func (r *reconfigurableBoardDigitalInterrupt) AddPostProcessor(pp PostProcessor) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	r.actual.AddPostProcessor(pp)
}

func (r *reconfigurableBoardDigitalInterrupt) Close() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return utils.TryClose(r.actual)
}
