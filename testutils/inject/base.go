package inject

import "go.viam.com/robotcore/base"

type Base struct {
	base.Device
	MoveStraightFunc func(distanceMM int, speed float64, block bool) error
	SpinFunc         func(angleDeg float64, speed int, block bool) error
	StopFunc         func() error
	CloseFunc        func()
}

func (b *Base) MoveStraight(distanceMM int, speed float64, block bool) error {
	if b.MoveStraightFunc == nil {
		return b.Device.MoveStraight(distanceMM, speed, block)
	}
	return b.MoveStraightFunc(distanceMM, speed, block)
}

func (b *Base) Spin(angleDeg float64, speed int, block bool) error {
	if b.SpinFunc == nil {
		return b.Device.Spin(angleDeg, speed, block)
	}
	return b.SpinFunc(angleDeg, speed, block)
}

func (b *Base) Stop() error {
	if b.StopFunc == nil {
		return b.Device.Stop()
	}
	return b.StopFunc()
}

func (b *Base) Close() {
	if b.CloseFunc == nil {
		b.Device.Close()
		return
	}
	b.CloseFunc()
}
