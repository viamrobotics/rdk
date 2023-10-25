package gpio

import (
	"context"

	"go.viam.com/rdk/control"
)

// SetState sets the state of the motor for the built-in control loop.
func (m *EncodedMotor) SetState(ctx context.Context, state []*control.Signal) error {
	power := state[0].GetSignalValueAt(0)
	return m.SetPower(ctx, power, nil)
}

// State gets the state of the motor for the built-in control loop.
func (m *EncodedMotor) State(ctx context.Context) ([]float64, error) {
	pos, err := m.position(ctx, nil)
	return []float64{pos}, err
}
