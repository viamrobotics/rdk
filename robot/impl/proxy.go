package robotimpl

import (
	"context"
	"fmt"
	"sync"

	"go.viam.com/utils"

	"go.viam.com/rdk/base"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/sensor"
	"go.viam.com/rdk/sensor/compass"
)

type proxyBase struct {
	mu     sync.RWMutex
	actual base.Base
}

func (p *proxyBase) ProxyFor() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual
}

func (p *proxyBase) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.MoveStraight(ctx, distanceMillis, millisPerSec, block)
}

func (p *proxyBase) MoveArc(ctx context.Context, distanceMillis int, millisPerSec float64, degsPerSec float64, block bool) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.MoveArc(ctx, distanceMillis, millisPerSec, degsPerSec, block)
}

func (p *proxyBase) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Spin(ctx, angleDeg, degsPerSec, block)
}

func (p *proxyBase) Stop(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Stop(ctx)
}

func (p *proxyBase) WidthMillis(ctx context.Context) (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.WidthMillis(ctx)
}

func (p *proxyBase) Close(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return utils.TryClose(ctx, p.actual)
}

func (p *proxyBase) replace(ctx context.Context, newBase base.Base) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newBase.(*proxyBase)
	if !ok {
		panic(fmt.Errorf("expected new base to be %T but got %T", actual, newBase))
	}
	if err := utils.TryClose(ctx, p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

type proxySensor struct {
	mu     sync.RWMutex
	actual sensor.Sensor
}

func (p *proxySensor) ProxyFor() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual
}

func (p *proxySensor) Readings(ctx context.Context) ([]interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Readings(ctx)
}

func (p *proxySensor) Desc() sensor.Description {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Desc()
}

func (p *proxySensor) Close(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return utils.TryClose(ctx, p.actual)
}

func (p *proxySensor) replace(ctx context.Context, newSensor sensor.Sensor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newSensor.(*proxySensor)
	if !ok {
		panic(fmt.Errorf("expected new sensor to be %T but got %T", actual, newSensor))
	}
	if err := utils.TryClose(ctx, p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

type proxyCompass struct {
	*proxySensor
	mu     sync.RWMutex
	actual compass.Compass
}

func (p *proxyCompass) ProxyFor() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual
}

func newProxyCompass(actual compass.Compass) *proxyCompass {
	return &proxyCompass{proxySensor: &proxySensor{actual: actual}, actual: actual}
}

func (p *proxyCompass) Heading(ctx context.Context) (float64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Heading(ctx)
}

func (p *proxyCompass) StartCalibration(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.StartCalibration(ctx)
}

func (p *proxyCompass) StopCalibration(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.StopCalibration(ctx)
}

func (p *proxyCompass) replace(ctx context.Context, newSensor sensor.Sensor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newSensor.(*proxyCompass)
	if !ok {
		panic(fmt.Errorf("expected new compass to be %T but got %T", actual, newSensor))
	}
	if err := utils.TryClose(ctx, p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
	p.proxySensor.actual = actual.actual
}

type proxyRelativeCompass struct {
	*proxyCompass
	mu     sync.RWMutex
	actual compass.RelativeCompass
}

func (p *proxyRelativeCompass) ProxyFor() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual
}

func newProxyRelativeCompass(actual compass.RelativeCompass) *proxyRelativeCompass {
	return &proxyRelativeCompass{proxyCompass: newProxyCompass(actual), actual: actual}
}

func (p *proxyRelativeCompass) Mark(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Mark(ctx)
}

func (p *proxyRelativeCompass) replace(ctx context.Context, newSensor sensor.Sensor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newSensor.(*proxyRelativeCompass)
	if !ok {
		panic(fmt.Errorf("expected new compass to be %T but got %T", actual, newSensor))
	}
	if err := utils.TryClose(ctx, p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
	p.proxyCompass.actual = actual.actual
	p.proxySensor.actual = actual.actual
}
