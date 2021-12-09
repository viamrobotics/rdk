// Package board defines the interfaces that typically live on a single-board computer
// such as a Raspberry Pi.
//
// Besides the board itself, some other interfaces it defines are analog readers and digital interrupts.
package board

import (
	"context"
	"fmt"
	"sync"

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

// IMPORTED FROM robot/impl/proxy.go

type proxyBoard struct {
	mu       sync.RWMutex
	actual   Board
	spis     map[string]*proxyBoardSPI
	i2cs     map[string]*proxyBoardI2C
	analogs  map[string]*proxyBoardAnalogReader
	digitals map[string]*proxyBoardDigitalInterrupt
}

func newProxyBoard(actual Board) *proxyBoard {
	p := &proxyBoard{
		actual:   actual,
		spis:     map[string]*proxyBoardSPI{},
		i2cs:     map[string]*proxyBoardI2C{},
		analogs:  map[string]*proxyBoardAnalogReader{},
		digitals: map[string]*proxyBoardDigitalInterrupt{},
	}

	for _, name := range actual.SPINames() {
		actualPart, ok := actual.SPIByName(name)
		if !ok {
			continue
		}
		p.spis[name] = &proxyBoardSPI{actual: actualPart}
	}
	for _, name := range actual.I2CNames() {
		actualPart, ok := actual.I2CByName(name)
		if !ok {
			continue
		}
		p.i2cs[name] = &proxyBoardI2C{actual: actualPart}
	}
	for _, name := range actual.AnalogReaderNames() {
		actualPart, ok := actual.AnalogReaderByName(name)
		if !ok {
			continue
		}
		p.analogs[name] = &proxyBoardAnalogReader{actual: actualPart}
	}
	for _, name := range actual.DigitalInterruptNames() {
		actualPart, ok := actual.DigitalInterruptByName(name)
		if !ok {
			continue
		}
		p.digitals[name] = &proxyBoardDigitalInterrupt{actual: actualPart}
	}

	return p
}

func (p *proxyBoard) ProxyFor() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual
}

func (p *proxyBoard) SPIByName(name string) (SPI, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	s, ok := p.spis[name]
	return s, ok
}

func (p *proxyBoard) I2CByName(name string) (I2C, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	s, ok := p.i2cs[name]
	return s, ok
}

func (p *proxyBoard) AnalogReaderByName(name string) (AnalogReader, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	a, ok := p.analogs[name]
	return a, ok
}

func (p *proxyBoard) DigitalInterruptByName(name string) (DigitalInterrupt, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	d, ok := p.digitals[name]
	return d, ok
}

func (p *proxyBoard) GPIOSet(ctx context.Context, pin string, high bool) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.GPIOSet(ctx, pin, high)
}

func (p *proxyBoard) GPIOGet(ctx context.Context, pin string) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.GPIOGet(ctx, pin)
}

func (p *proxyBoard) PWMSet(ctx context.Context, pin string, dutyCycle byte) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.PWMSet(ctx, pin, dutyCycle)
}

func (p *proxyBoard) PWMSetFreq(ctx context.Context, pin string, freq uint) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.PWMSetFreq(ctx, pin, freq)
}

func (p *proxyBoard) SPINames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := []string{}
	for k := range p.spis {
		names = append(names, k)
	}
	return names
}

func (p *proxyBoard) I2CNames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := []string{}
	for k := range p.i2cs {
		names = append(names, k)
	}
	return names
}

func (p *proxyBoard) AnalogReaderNames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := []string{}
	for k := range p.analogs {
		names = append(names, k)
	}
	return names
}

func (p *proxyBoard) DigitalInterruptNames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := []string{}
	for k := range p.digitals {
		names = append(names, k)
	}
	return names
}

func (p *proxyBoard) Status(ctx context.Context) (*pb.BoardStatus, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.actual.ModelAttributes().Remote {
		return p.actual.Status(ctx)
	}
	return CreateStatus(ctx, p)
}

func (p *proxyBoard) replace(newBoard Board) {
	p.mu.Lock()
	defer p.mu.Unlock()

	actual, ok := newBoard.(*proxyBoard)
	if !ok {
		panic(fmt.Errorf("expected new board to be %T but got %T", actual, newBoard))
	}

	var oldSPINames map[string]struct{}
	var oldI2CNames map[string]struct{}
	var oldAnalogReaderNames map[string]struct{}
	var oldDigitalInterruptNames map[string]struct{}

	if len(p.spis) != 0 {
		oldSPINames = make(map[string]struct{}, len(p.spis))
		for name := range p.spis {
			oldSPINames[name] = struct{}{}
		}
	}
	if len(p.i2cs) != 0 {
		oldI2CNames = make(map[string]struct{}, len(p.i2cs))
		for name := range p.i2cs {
			oldI2CNames[name] = struct{}{}
		}
	}
	if len(p.analogs) != 0 {
		oldAnalogReaderNames = make(map[string]struct{}, len(p.analogs))
		for name := range p.analogs {
			oldAnalogReaderNames[name] = struct{}{}
		}
	}
	if len(p.digitals) != 0 {
		oldDigitalInterruptNames = make(map[string]struct{}, len(p.digitals))
		for name := range p.digitals {
			oldDigitalInterruptNames[name] = struct{}{}
		}
	}

	for name, newPart := range actual.spis {
		oldPart, ok := p.spis[name]
		delete(oldSPINames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		p.spis[name] = newPart
	}
	for name, newPart := range actual.i2cs {
		oldPart, ok := p.i2cs[name]
		delete(oldI2CNames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		p.i2cs[name] = newPart
	}
	for name, newPart := range actual.analogs {
		oldPart, ok := p.analogs[name]
		delete(oldAnalogReaderNames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		p.analogs[name] = newPart
	}
	for name, newPart := range actual.digitals {
		oldPart, ok := p.digitals[name]
		delete(oldDigitalInterruptNames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		p.digitals[name] = newPart
	}

	for name := range oldSPINames {
		delete(p.spis, name)
	}
	for name := range oldI2CNames {
		delete(p.i2cs, name)
	}
	for name := range oldAnalogReaderNames {
		delete(p.analogs, name)
	}
	for name := range oldDigitalInterruptNames {
		delete(p.digitals, name)
	}

	p.actual = actual.actual
}

func (p *proxyBoard) ModelAttributes() ModelAttributes {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.ModelAttributes()
}

// Close attempts to cleanly close each part of the board.
func (p *proxyBoard) Close() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return utils.TryClose(p.actual)
}

type proxyBoardSPI struct {
	mu     sync.RWMutex
	actual SPI
}

func (p *proxyBoardSPI) replace(newSPI SPI) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newSPI.(*proxyBoardSPI)
	if !ok {
		panic(fmt.Errorf("expected new SPI to be %T but got %T", actual, newSPI))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

func (p *proxyBoardSPI) OpenHandle() (SPIHandle, error) {
	return p.actual.OpenHandle()
}

type proxyBoardI2C struct {
	mu     sync.RWMutex
	actual I2C
}

func (p *proxyBoardI2C) ProxyFor() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual
}

func (p *proxyBoardI2C) replace(newI2C I2C) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newI2C.(*proxyBoardI2C)
	if !ok {
		panic(fmt.Errorf("expected new I2C to be %T but got %T", actual, newI2C))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

func (p *proxyBoardI2C) OpenHandle(addr byte) (I2CHandle, error) {
	return p.actual.OpenHandle(addr)
}

type proxyBoardAnalogReader struct {
	mu     sync.RWMutex
	actual AnalogReader
}

func (p *proxyBoardAnalogReader) replace(newAnalogReader AnalogReader) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newAnalogReader.(*proxyBoardAnalogReader)
	if !ok {
		panic(fmt.Errorf("expected new analog reader to be %T but got %T", actual, newAnalogReader))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

func (p *proxyBoardAnalogReader) Read(ctx context.Context) (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Read(ctx)
}

func (p *proxyBoardAnalogReader) ProxyFor() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual
}

func (p *proxyBoardAnalogReader) Close() error {
	return utils.TryClose(p.actual)
}

type proxyBoardDigitalInterrupt struct {
	mu     sync.RWMutex
	actual DigitalInterrupt
}

func (p *proxyBoardDigitalInterrupt) ProxyFor() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual
}

func (p *proxyBoardDigitalInterrupt) replace(newDigitalInterrupt DigitalInterrupt) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newDigitalInterrupt.(*proxyBoardDigitalInterrupt)
	if !ok {
		panic(fmt.Errorf("expected new digital interrupt to be %T but got %T", actual, newDigitalInterrupt))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

func (p *proxyBoardDigitalInterrupt) Config(ctx context.Context) (DigitalInterruptConfig, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Config(ctx)
}

func (p *proxyBoardDigitalInterrupt) Value(ctx context.Context) (int64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Value(ctx)
}

func (p *proxyBoardDigitalInterrupt) Tick(ctx context.Context, high bool, nanos uint64) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Tick(ctx, high, nanos)
}

func (p *proxyBoardDigitalInterrupt) AddCallback(c chan bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	p.actual.AddCallback(c)
}

func (p *proxyBoardDigitalInterrupt) AddPostProcessor(pp PostProcessor) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	p.actual.AddPostProcessor(pp)
}

func (p *proxyBoardDigitalInterrupt) Close() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return utils.TryClose(p.actual)
}
