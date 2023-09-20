package gpio

import "context"

func (m *EncodedMotor) SetState(ctx context.Context, state float64) error {
	return m.SetPower(ctx, state, nil)
}

func (m *EncodedMotor) State(ctx context.Context) (float64, error) {
	return m.position(ctx, nil)
}
