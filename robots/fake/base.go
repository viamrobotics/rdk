package fake

import "context"

// tracks in CM
type Base struct {
}

func (b *Base) MoveStraight(ctx context.Context, distanceMM int, mmPerSec float64, block bool) error {
	return nil
}

func (b *Base) Spin(ctx context.Context, angleDeg float64, speed int, block bool) error {
	return nil
}

func (b *Base) Width(ctx context.Context) (float64, error) {
	return 0.6, nil
}

func (b *Base) Stop(ctx context.Context) error {
	return nil
}

func (b *Base) Close(ctx context.Context) error {
	return nil
}
