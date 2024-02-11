package board

import "context"

// A GPIOPin represents an individual GPIO pin on a board.
type GPIOPin interface {
	// Set sets the pin to either low or high.
	Set(ctx context.Context, high bool, extra map[string]any) error

	// Get gets the high/low state of the pin.
	Get(ctx context.Context, extra map[string]any) (bool, error)

	// PWM gets the pin's given duty cycle.
	PWM(ctx context.Context, extra map[string]any) (float64, error)

	// SetPWM sets the pin to the given duty cycle.
	SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]any) error

	// PWMFreq gets the PWM frequency of the pin.
	PWMFreq(ctx context.Context, extra map[string]any) (uint, error)

	// SetPWMFreq sets the given pin to the given PWM frequency. For Raspberry Pis,
	// 0 will use a default PWM frequency of 800.
	SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]any) error
}
