package board

import "context"

// A GPIOPin represents an individual GPIO pin on a board.
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

	// SetPWMFreq sets the given pin to the given PWM frequency. 0 will use the board's default PWM frequency.
	SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error
}
