package gpio

import "context"

// SetState sets the state of the motor for the built-in control loop.
func (m *EncodedMotor) SetState(ctx context.Context, state float64) error {
	return m.SetPower(ctx, state, nil)
}

// State gets the state of the motor for the built-in control loop.
func (m *EncodedMotor) State(ctx context.Context) (float64, error) {
	return m.position(ctx, nil)
}
