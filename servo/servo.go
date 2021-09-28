package servo

import "context"

// A Servo represents a physical servo connected to a board.
type Servo interface {

	// Move moves the servo to the given angle (0-180 degrees)
	Move(ctx context.Context, angleDegs uint8) error

	// Current returns the current set angle (degrees) of the servo.
	Current(ctx context.Context) (uint8, error)
}
