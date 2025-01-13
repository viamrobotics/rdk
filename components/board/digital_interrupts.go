package board

import (
	"context"
)

// Tick represents a signal received by an interrupt pin. This signal is communicated
// via registered channel to the various drivers. Depending on board implementation there may be a
// wraparound in timestamp values past 4294967295000 nanoseconds (~72 minutes) if the value
// was originally in microseconds as a 32-bit integer. The timestamp in nanoseconds of the
// tick SHOULD ONLY BE USED FOR CALCULATING THE TIME ELAPSED BETWEEN CONSECUTIVE TICKS AND NOT
// AS AN ABSOLUTE TIMESTAMP.
type Tick struct {
	Name             string
	High             bool
	TimestampNanosec uint64
}

// A DigitalInterrupt represents a configured interrupt on the board that
// when interrupted, calls the added callbacks.
//
// Value example:
//
//	myBoard, err := board.FromRobot(robot, "my_board")
//
//	// Get the DigitalInterrupt "my_example_digital_interrupt".
//	interrupt, err := myBoard.DigitalInterruptByName("my_example_digital_interrupt")
//
//	// Get the amount of times this DigitalInterrupt has ticked.
//	count, err := interrupt.Value(context.Background(), nil)
//
// For more information, see the [Value method docs].
//
// [Value method docs]: https://docs.viam.com/dev/reference/apis/components/board/#getdigitalinterruptvalue
type DigitalInterrupt interface {
	// Name returns the name of the interrupt.
	Name() string

	// Value returns the current value of the interrupt which is
	// based on the type of interrupt.
	Value(ctx context.Context, extra map[string]interface{}) (int64, error)
}
