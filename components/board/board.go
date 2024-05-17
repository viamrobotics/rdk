// Package board defines the interfaces that typically live on a single-board computer
// such as a Raspberry Pi.
//
// Besides the board itself, some other interfaces it defines are analog pins and digital interrupts.
package board

import (
	"context"
	"time"

	pb "go.viam.com/api/component/board/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Board]{
		Status:                      resource.StatusFunc(CreateStatus),
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterBoardServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.BoardService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: analogs.String(),
	}, newAnalogCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: gpios.String(),
	}, newGPIOCollector)
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
// components such as analogs, and digital interrupts.
type Board interface {
	resource.Resource

	// AnalogByName returns an analog pin by name.
	AnalogByName(name string) (Analog, error)

	// DigitalInterruptByName returns a digital interrupt by name.
	DigitalInterruptByName(name string) (DigitalInterrupt, error)

	// GPIOPinByName returns a GPIOPin by name.
	GPIOPinByName(name string) (GPIOPin, error)

	// AnalogNames returns the names of all known analog pins.
	AnalogNames() []string

	// DigitalInterruptNames returns the names of all known digital interrupts.
	DigitalInterruptNames() []string

	// SetPowerMode sets the board to the given power mode. If
	// provided, the board will exit the given power mode after
	// the specified duration.
	SetPowerMode(ctx context.Context, mode pb.PowerMode, duration *time.Duration) error

	// StreamTicks starts a stream of digital interrupt ticks.
	StreamTicks(ctx context.Context, interrupts []DigitalInterrupt, ch chan Tick,
		extra map[string]interface{}) error
}

// An Analog represents an analog pin that resides on a board.
type Analog interface {
	// Read reads off the current value.
	Read(ctx context.Context, extra map[string]interface{}) (AnalogValue, error)

	// Write writes a value to the analog pin.
	Write(ctx context.Context, value int, extra map[string]interface{}) error
}

// AnalogValue contains all info about the analog reading.
// Value represents the reading in bits.
// Min and Max represent the range of raw analog values.
// StepSize is the precision per bit of the reading.
type AnalogValue struct {
	Value    int
	Min      float32
	Max      float32
	StepSize float32
}

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
