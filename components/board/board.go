// Package board defines the interfaces that typically live on a single-board computer
// such as a Raspberry Pi.
//
// Besides the board itself, some other interfaces it defines are analog readers and digital interrupts.
package board

import (
	"context"
	"time"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/board/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Board]{
		Status: func(ctx context.Context, b Board) (interface{}, error) {
			return b.Status(ctx, nil)
		},
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterBoardServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.BoardService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: analogs.String(),
	}, NewAnalogCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: gpios.String(),
	}, NewGPIOCollector)
}

// SubtypeName is a constant that identifies the component resource API string "board".
const SubtypeName = "board"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named board's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// A Board represents a physical general purpose board that contains various
// components such as analog readers, and digital interrupts.
type Board interface {
	resource.Resource

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

	// AnalogReaderNames returns the names of all known analog readers.
	AnalogReaderNames() []string

	// DigitalInterruptNames returns the names of all known digital interrupts.
	DigitalInterruptNames() []string

	// GPIOPinNames returns the names of all known GPIO pins.
	GPIOPinNames() []string

	// Status returns the current status of the board. Usually you
	// should use the CreateStatus helper instead of directly calling
	// this.
	Status(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error)

	// ModelAttributes returns attributes related to the model of this board.
	ModelAttributes() ModelAttributes

	// SetPowerMode sets the board to the given power mode. If
	// provided, the board will exit the given power mode after
	// the specified duration.
	SetPowerMode(ctx context.Context, mode pb.PowerMode, duration *time.Duration) error

	// WriteAnalog writes an analog value to a pin on the board.
	WriteAnalog(ctx context.Context, pin string, value int32, extra map[string]interface{}) error
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
	Close(ctx context.Context) error
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
	Read(ctx context.Context, extra map[string]interface{}) (int, error)
	Close(ctx context.Context) error
}

// A PostProcessor takes a raw input and transforms it into a new value.
// Multiple post processors can be stacked on each other. This is currently
// only used in DigitalInterrupt readings.
type PostProcessor func(raw int64) int64

// FromDependencies is a helper for getting the named board from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Board, error) {
	return resource.FromDependencies[Board](deps, Named(name))
}

// FromRobot is a helper for getting the named board from the given Robot.
func FromRobot(r robot.Robot, name string) (Board, error) {
	return robot.ResourceFromRobot[Board](r, Named(name))
}

// NamesFromRobot is a helper for getting all board names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}
