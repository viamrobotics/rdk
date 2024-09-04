package board

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
)

// A GPIOPin represents an individual GPIO pin on a board.
//
// Set example:
//
//	myBoard, err := board.FromRobot(robot, "my_board")
//
//	// Get the GPIOPin with pin number 15.
//	pin, err := myBoard.GPIOPinByName("15")
//
//	// Set the pin to high.
//	err := pin.Set(context.Background(), "true", nil)
//
// Get example:
//
//	myBoard, err := board.FromRobot(robot, "my_board")
//
//	// Get the GPIOPin with pin number 15.
//	pin, err := myBoard.GPIOPinByName("15")
//
//	// Get if it is true or false that the state of the pin is high.
//	high := pin.Get(context.Background(), nil)
//
// PWM example:
//
//	myBoard, err := board.FromRobot(robot, "my_board")
//
//	// Get the GPIOPin with pin number 15.
//	pin, err := myBoard.GPIOPinByName("15")
//
//	// Returns the duty cycle.
//	duty_cycle := pin.PWM(context.Background(), nil)
//
// SetPWM example:
//
//	myBoard, err := board.FromRobot(robot, "my_board")
//
//	// Get the GPIOPin with pin number 15.
//	pin, err := myBoard.GPIOPinByName("15")
//
//	// Set the duty cycle to .6, meaning that this pin will be in the high state for 60% of the duration of the PWM interval period.
//	err := pin.SetPWM(context.Background(), .6, nil)
//
// PWMFreq example:
//
//	myBoard, err := board.FromRobot(robot, "my_board")
//
//	// Get the GPIOPin with pin number 15.
//	pin, err := myBoard.GPIOPinByName("15")
//
//	// Get the PWM frequency of this pin.
//	freqHz, err := pin.PWMFreq(context.Background(), nil)
//
// SetPWMFreq example:
//
//	myBoard, err := board.FromRobot(robot, "my_board")
//
//	// Get the GPIOPin with pin number 15.
//	pin, err := myBoard.GPIOPinByName("15")
//
//	// Set the PWM frequency of this pin to 1600 Hz.
//	high := pin.SetPWMFreq(context.Background(), 1600, nil)
type GPIOPin interface {
	// Set sets the pin to either low or high.
	Set(ctx context.Context, high bool, extra map[string]interface{}) error

	// Get gets the high/low state of the pin.
	Get(ctx context.Context, extra map[string]interface{}) (bool, error)

	// PWM gets the pin's given duty cycle.
	PWM(ctx context.Context, extra map[string]interface{}) (float64, error)

	// SetPWM sets the pin to the given duty cycle.
	SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error

	// PWMFreq gets the PWM frequency of the pin.
	PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error)

	// SetPWMFreq sets the given pin to the given PWM frequency. For Raspberry Pis,
	// 0 will use a default PWM frequency of 800.
	SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error
}

// ValidatePWMDutyCycle makes sure the value passed in is a believable duty cycle value. It returns
// the preferred duty cycle value if it is, and an error if it is not.
func ValidatePWMDutyCycle(dutyCyclePct float64) (float64, error) {
	if dutyCyclePct < 0.0 {
		return 0.0, errors.New("cannot set negative duty cycle")
	}
	if dutyCyclePct > 1.0 {
		if dutyCyclePct < 1.01 {
			// Someone was probably setting it to 1 and had a roundoff error.
			return 1.0, nil
		}
		return 0.0, fmt.Errorf("cannot set duty cycle to %f: range is 0.0 to 1.0", dutyCyclePct)
	}
	return dutyCyclePct, nil
}
