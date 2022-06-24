// Package board defines the interfaces that typically live on a single-board computer
// such as a Raspberry Pi.
//
// Besides the board itself, some other interfaces it defines are analog readers and digital interrupts.
package board

import (
	"context"
	"strings"
	"sync"

	"github.com/edaniels/golog"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/board/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
		Status: func(ctx context.Context, resource interface{}) (interface{}, error) {
			board, ok := resource.(Board)
			if !ok {
				return nil, utils.NewUnimplementedInterfaceError("Board", resource)
			}
			return board.Status(ctx)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.BoardService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterBoardServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.BoardService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

// SubtypeName is a constant that identifies the component resource subtype string "board".
const SubtypeName = resource.SubtypeName("board")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named board's typed resource name.
func Named(name string) resource.Name {
	remotes := strings.Split(name, ":")
	if len(remotes) > 1 {
		rName := resource.NameFromSubtype(Subtype, remotes[len(remotes)-1])
		return rName.PrependRemote(resource.RemoteName(strings.Join(remotes[:len(remotes)-1], ":")))
	}
	return resource.NameFromSubtype(Subtype, name)
}

// A Board represents a physical general purpose board that contains various
// components such as analog readers, and digital interrupts.
type Board interface {
	// AnalogReaderByName returns an analog reader by name.
	AnalogReaderByName(name string) (AnalogReader, bool)

	// DigitalInterruptByName returns a digital interrupt by name.
	DigitalInterruptByName(name string) (DigitalInterrupt, bool)

	// GPIOPinByName returns a GPIOPin by name.
	GPIOPinByName(name string) (GPIOPin, error)

	// SPINames returns the names of all known SPI buses.
	SPINames() []string

	// I2CNames returns the names of all known I2C buses.
	I2CNames() []string

	// AnalogReaderNames returns the name of all known analog readers.
	AnalogReaderNames() []string

	// DigitalInterruptNames returns the name of all known digital interrupts.
	DigitalInterruptNames() []string

	// GPIOPinNames returns the names of all known GPIO pins.
	GPIOPinNames() []string

	// Status returns the current status of the board. Usually you
	// should use the CreateStatus helper instead of directly calling
	// this.
	Status(ctx context.Context) (*commonpb.BoardStatus, error)

	// ModelAttributes returns attributes related to the model of this board.
	ModelAttributes() ModelAttributes

	generic.Generic
}

// A LocalBoard represents a Board where you can request SPIs and I2Cs by name.
type LocalBoard interface {
	Board

	// SPIByName returns an SPI bus by name.
	SPIByName(name string) (SPI, bool)

	// I2CByName returns an I2C bus by name.
	I2CByName(name string) (I2C, bool)
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
	// Read-only transfers usually transmit a request/address and continue with some number of null bytes to equal the expected size of the
	// returning data.
	// Large transmissions are usually broken up into multiple transfers.
	// There are many different paradigms for most of the above, and implementation details are chip/device specific.
	Xfer(
		ctx context.Context,
		baud uint,
		chipSelect string,
		mode uint,
		tx []byte,
	) ([]byte, error)
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
	_ = LocalBoard(&reconfigurableBoard{})
	_ = resource.Reconfigurable(&reconfigurableBoard{})
)

// FromRobot is a helper for getting the named board from the given Robot.
func FromRobot(r robot.Robot, name string) (Board, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(Board)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Board", res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all board names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

type reconfigurableBoard struct {
	mu       sync.RWMutex
	actual   LocalBoard
	spis     map[string]*reconfigurableSPI
	i2cs     map[string]*reconfigurableI2C
	analogs  map[string]*reconfigurableAnalogReader
	digitals map[string]*reconfigurableDigitalInterrupt
}

func (r *reconfigurableBoard) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableBoard) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Do(ctx, cmd)
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

func (r *reconfigurableBoard) GPIOPinByName(name string) (GPIOPin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GPIOPinByName(name)
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

func (r *reconfigurableBoard) GPIOPinNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GPIOPinNames()
}

func (r *reconfigurableBoard) Status(ctx context.Context) (*commonpb.BoardStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.actual.ModelAttributes().Remote {
		return r.actual.Status(ctx)
	}
	return CreateStatus(ctx, r)
}

func (r *reconfigurableBoard) Reconfigure(ctx context.Context, newBoard resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	actual, ok := newBoard.(*reconfigurableBoard)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newBoard)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
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
			oldPart.reconfigure(ctx, newPart)
			continue
		}
		r.spis[name] = newPart
	}
	for name, newPart := range actual.i2cs {
		oldPart, ok := r.i2cs[name]
		delete(oldI2CNames, name)
		if ok {
			oldPart.reconfigure(ctx, newPart)
			continue
		}
		r.i2cs[name] = newPart
	}
	for name, newPart := range actual.analogs {
		oldPart, ok := r.analogs[name]
		delete(oldAnalogReaderNames, name)
		if ok {
			oldPart.reconfigure(ctx, newPart)
			continue
		}
		r.analogs[name] = newPart
	}
	for name, newPart := range actual.digitals {
		oldPart, ok := r.digitals[name]
		delete(oldDigitalInterruptNames, name)
		if ok {
			oldPart.reconfigure(ctx, newPart)
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
func (r *reconfigurableBoard) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

// UpdateAction helps hinting the reconfiguration process on what strategy to use given a modified config.
// See config.UpdateActionType for more information.
func (r *reconfigurableBoard) UpdateAction(c *config.Component) config.UpdateActionType {
	obj, canUpdate := r.actual.(interface {
		UpdateAction(config *config.Component) config.UpdateActionType
	})
	if canUpdate {
		return obj.UpdateAction(c)
	}
	return config.Reconfigure
}

// WrapWithReconfigurable converts a regular Board implementation to a reconfigurableBoard.
// If board is already a reconfigurableBoard, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	board, ok := r.(LocalBoard)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("LocalBoard", r)
	}
	if reconfigurable, ok := board.(*reconfigurableBoard); ok {
		return reconfigurable, nil
	}
	rb := reconfigurableBoard{
		actual:   board,
		spis:     map[string]*reconfigurableSPI{},
		i2cs:     map[string]*reconfigurableI2C{},
		analogs:  map[string]*reconfigurableAnalogReader{},
		digitals: map[string]*reconfigurableDigitalInterrupt{},
	}

	for _, name := range rb.actual.SPINames() {
		actualPart, ok := rb.actual.SPIByName(name)
		if !ok {
			continue
		}
		rb.spis[name] = &reconfigurableSPI{actual: actualPart}
	}
	for _, name := range rb.actual.I2CNames() {
		actualPart, ok := rb.actual.I2CByName(name)
		if !ok {
			continue
		}
		rb.i2cs[name] = &reconfigurableI2C{actual: actualPart}
	}
	for _, name := range rb.actual.AnalogReaderNames() {
		actualPart, ok := rb.actual.AnalogReaderByName(name)
		if !ok {
			continue
		}
		rb.analogs[name] = &reconfigurableAnalogReader{actual: actualPart}
	}
	for _, name := range rb.actual.DigitalInterruptNames() {
		actualPart, ok := rb.actual.DigitalInterruptByName(name)
		if !ok {
			continue
		}
		rb.digitals[name] = &reconfigurableDigitalInterrupt{actual: actualPart}
	}

	return &rb, nil
}

type reconfigurableSPI struct {
	mu     sync.RWMutex
	actual SPI
}

func (r *reconfigurableSPI) reconfigure(ctx context.Context, newSPI SPI) {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newSPI.(*reconfigurableSPI)
	if !ok {
		panic(utils.NewUnexpectedTypeError(r, newSPI))
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
}

func (r *reconfigurableSPI) OpenHandle() (SPIHandle, error) {
	return r.actual.OpenHandle()
}

type reconfigurableI2C struct {
	mu     sync.RWMutex
	actual I2C
}

func (r *reconfigurableI2C) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableI2C) reconfigure(ctx context.Context, newI2C I2C) {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newI2C.(*reconfigurableI2C)
	if !ok {
		panic(utils.NewUnexpectedTypeError(r, newI2C))
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
}

func (r *reconfigurableI2C) OpenHandle(addr byte) (I2CHandle, error) {
	return r.actual.OpenHandle(addr)
}

type reconfigurableAnalogReader struct {
	mu     sync.RWMutex
	actual AnalogReader
}

func (r *reconfigurableAnalogReader) reconfigure(ctx context.Context, newAnalogReader AnalogReader) {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newAnalogReader.(*reconfigurableAnalogReader)
	if !ok {
		panic(utils.NewUnexpectedTypeError(r, newAnalogReader))
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
}

func (r *reconfigurableAnalogReader) Read(ctx context.Context) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Read(ctx)
}

func (r *reconfigurableAnalogReader) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableAnalogReader) Close(ctx context.Context) error {
	return viamutils.TryClose(ctx, r.actual)
}

type reconfigurableDigitalInterrupt struct {
	mu     sync.RWMutex
	actual DigitalInterrupt
}

func (r *reconfigurableDigitalInterrupt) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableDigitalInterrupt) reconfigure(ctx context.Context, newDigitalInterrupt DigitalInterrupt) {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newDigitalInterrupt.(*reconfigurableDigitalInterrupt)
	if !ok {
		panic(utils.NewUnexpectedTypeError(r, newDigitalInterrupt))
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
}

func (r *reconfigurableDigitalInterrupt) Value(ctx context.Context) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Value(ctx)
}

func (r *reconfigurableDigitalInterrupt) Tick(ctx context.Context, high bool, nanos uint64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Tick(ctx, high, nanos)
}

func (r *reconfigurableDigitalInterrupt) AddCallback(c chan bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	r.actual.AddCallback(c)
}

func (r *reconfigurableDigitalInterrupt) AddPostProcessor(pp PostProcessor) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	r.actual.AddPostProcessor(pp)
}

func (r *reconfigurableDigitalInterrupt) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}
