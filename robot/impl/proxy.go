package robotimpl

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/golang/geo/r2"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/utils"

	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/input"
	"go.viam.com/core/lidar"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/rlog"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/compass"
	"go.viam.com/core/sensor/forcematrix"
	"go.viam.com/core/sensor/gps"
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

func (p *proxyBase) Close() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return utils.TryClose(p.actual)
}

func (p *proxyBase) replace(newBase base.Base) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newBase.(*proxyBase)
	if !ok {
		panic(fmt.Errorf("expected new base to be %T but got %T", actual, newBase))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

type proxyLidar struct {
	mu     sync.RWMutex
	actual lidar.Lidar
}

func (p *proxyLidar) ProxyFor() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual
}

func (p *proxyLidar) Info(ctx context.Context) (map[string]interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Info(ctx)
}

func (p *proxyLidar) Start(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Start(ctx)
}

func (p *proxyLidar) Stop(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Stop(ctx)
}

func (p *proxyLidar) Scan(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Scan(ctx, options)
}

func (p *proxyLidar) Range(ctx context.Context) (float64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Range(ctx)
}

func (p *proxyLidar) Bounds(ctx context.Context) (r2.Point, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Bounds(ctx)
}

func (p *proxyLidar) AngularResolution(ctx context.Context) (float64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.AngularResolution(ctx)
}

func (p *proxyLidar) Close() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return utils.TryClose(p.actual)
}

func (p *proxyLidar) replace(newLidar lidar.Lidar) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newLidar.(*proxyLidar)
	if !ok {
		panic(fmt.Errorf("expected new lidar to be %T but got %T", actual, newLidar))
	}
	if err := utils.TryClose(p.actual); err != nil {
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

func (p *proxySensor) Close() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return utils.TryClose(p.actual)
}

func (p *proxySensor) replace(newSensor sensor.Sensor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newSensor.(*proxySensor)
	if !ok {
		panic(fmt.Errorf("expected new sensor to be %T but got %T", actual, newSensor))
	}
	if err := utils.TryClose(p.actual); err != nil {
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

func (p *proxyCompass) replace(newSensor sensor.Sensor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newSensor.(*proxyCompass)
	if !ok {
		panic(fmt.Errorf("expected new compass to be %T but got %T", actual, newSensor))
	}
	if err := utils.TryClose(p.actual); err != nil {
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

func (p *proxyRelativeCompass) replace(newSensor sensor.Sensor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newSensor.(*proxyRelativeCompass)
	if !ok {
		panic(fmt.Errorf("expected new compass to be %T but got %T", actual, newSensor))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
	p.proxyCompass.actual = actual.actual
	p.proxySensor.actual = actual.actual
}

type proxyGPS struct {
	*proxySensor
	mu     sync.RWMutex
	actual gps.GPS
}

func newProxyGPS(actual gps.GPS) *proxyGPS {
	return &proxyGPS{proxySensor: &proxySensor{actual: actual}, actual: actual}
}

func (p *proxyGPS) Location(ctx context.Context) (*geo.Point, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Location(ctx)
}

func (p *proxyGPS) Altitude(ctx context.Context) (float64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Altitude(ctx)
}

func (p *proxyGPS) Speed(ctx context.Context) (float64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Speed(ctx)
}

func (p *proxyGPS) Satellites(ctx context.Context) (int, int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Satellites(ctx)
}

func (p *proxyGPS) Accuracy(ctx context.Context) (float64, float64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Accuracy(ctx)
}

func (p *proxyGPS) Valid(ctx context.Context) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Valid(ctx)
}

func (p *proxyGPS) replace(newSensor sensor.Sensor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newSensor.(*proxyGPS)
	if !ok {
		panic(fmt.Errorf("expected new gps to be %T but got %T", actual, newSensor))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
	p.proxySensor.actual = actual.actual
}

func (p *proxyGPS) ProxyFor() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual
}

type proxyForceMatrix struct {
	*proxySensor
	mu     sync.RWMutex
	actual forcematrix.ForceMatrix
}

func (p *proxyForceMatrix) ProxyFor() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual
}

func newProxyForceMatrix(actual forcematrix.ForceMatrix) *proxyForceMatrix {
	return &proxyForceMatrix{proxySensor: &proxySensor{actual: actual}, actual: actual}
}

func (p *proxyForceMatrix) Matrix(ctx context.Context) ([][]int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Matrix(ctx)
}

func (p *proxyForceMatrix) IsSlipping(ctx context.Context) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.IsSlipping(ctx)
}

func (p *proxyForceMatrix) replace(newSensor sensor.Sensor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newSensor.(*proxyForceMatrix)
	if !ok {
		panic(fmt.Errorf("expected new forcematrix to be %T but got %T", actual, newSensor))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

type proxyInputController struct {
	mu     sync.RWMutex
	actual input.Controller
}

func (p *proxyInputController) replace(newController input.Controller) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newController.(*proxyInputController)
	if !ok {
		panic(fmt.Errorf("expected new input controller to be %T but got %T", actual, newController))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

func (p *proxyInputController) Controls(ctx context.Context) ([]input.Control, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Controls(ctx)
}

func (p *proxyInputController) LastEvents(ctx context.Context) (map[input.Control]input.Event, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.LastEvents(ctx)
}

// InjectEvent allows directly sending an Event (such as a button press) from external code
func (p *proxyInputController) InjectEvent(ctx context.Context, event input.Event) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	iActual, ok := p.actual.(input.Injectable)
	if !ok {
		return errors.New("controller is not input.Injectable")
	}
	return iActual.InjectEvent(ctx, event)
}

func (p *proxyInputController) RegisterControlCallback(ctx context.Context, control input.Control, triggers []input.EventType, ctrlFunc input.ControlFunction) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.RegisterControlCallback(ctx, control, triggers, ctrlFunc)
}

func (p *proxyInputController) Close() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return utils.TryClose(p.actual)
}
