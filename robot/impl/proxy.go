package robotimpl

import (
	"context"
	"fmt"
	"image"
	"sync"

	"github.com/golang/geo/r2"

	"go.viam.com/utils"

	"go.viam.com/core/arm"
	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/gripper"
	"go.viam.com/core/lidar"
	"go.viam.com/core/pointcloud"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/rlog"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/compass"
)

type proxyProvider struct {
	mu     sync.RWMutex
	actual robot.Provider
}

func (p *proxyProvider) Close() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return utils.TryClose(p.actual)
}

func (p *proxyProvider) Ready(r robot.Robot) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Ready(r)
}

func (p *proxyProvider) replace(newProvider robot.Provider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newProvider.(*proxyProvider)
	if !ok {
		panic(fmt.Errorf("expected new provider to be %T but got %T", actual, newProvider))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

type proxyBase struct {
	mu     sync.RWMutex
	actual base.Base
}

func (p *proxyBase) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.MoveStraight(ctx, distanceMillis, millisPerSec, block)
}

func (p *proxyBase) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
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

type proxyArm struct {
	mu     sync.RWMutex
	actual arm.Arm
}

func (p *proxyArm) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.CurrentPosition(ctx)
}

func (p *proxyArm) MoveToPosition(ctx context.Context, c *pb.ArmPosition) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.MoveToPosition(ctx, c)
}

func (p *proxyArm) MoveToJointPositions(ctx context.Context, pos *pb.JointPositions) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.MoveToJointPositions(ctx, pos)
}

func (p *proxyArm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.CurrentJointPositions(ctx)
}

func (p *proxyArm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.JointMoveDelta(ctx, joint, amountDegs)
}

func (p *proxyArm) Close() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return utils.TryClose(p.actual)
}

func (p *proxyArm) replace(newArm arm.Arm) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newArm.(*proxyArm)
	if !ok {
		panic(fmt.Errorf("expected new arm to be %T but got %T", actual, newArm))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

type proxyGripper struct {
	mu     sync.RWMutex
	actual gripper.Gripper
}

func (p *proxyGripper) Open(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Open(ctx)
}

func (p *proxyGripper) Grab(ctx context.Context) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Grab(ctx)
}

func (p *proxyGripper) Close() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return utils.TryClose(p.actual)
}

func (p *proxyGripper) replace(newGripper gripper.Gripper) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newGripper.(*proxyGripper)
	if !ok {
		panic(fmt.Errorf("expected new gripper to be %T but got %T", actual, newGripper))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

type proxyCamera struct {
	mu     sync.RWMutex
	actual camera.Camera
}

func (p *proxyCamera) Next(ctx context.Context) (image.Image, func(), error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Next(ctx)
}

func (p *proxyCamera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.NextPointCloud(ctx)
}

func (p *proxyCamera) Close() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return utils.TryClose(p.actual)
}

func (p *proxyCamera) replace(newCamera camera.Camera) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newCamera.(*proxyCamera)
	if !ok {
		panic(fmt.Errorf("expected new camera to be %T but got %T", actual, newCamera))
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

type proxyBoard struct {
	mu       sync.RWMutex
	actual   board.Board
	motors   map[string]*proxyBoardMotor
	servos   map[string]*proxyBoardServo
	spis     map[string]*proxyBoardSPI
	analogs  map[string]*proxyBoardAnalogReader
	digitals map[string]*proxyBoardDigitalInterrupt
}

func newProxyBoard(actual board.Board) *proxyBoard {
	p := &proxyBoard{
		actual:   actual,
		motors:   map[string]*proxyBoardMotor{},
		servos:   map[string]*proxyBoardServo{},
		spis:     map[string]*proxyBoardSPI{},
		analogs:  map[string]*proxyBoardAnalogReader{},
		digitals: map[string]*proxyBoardDigitalInterrupt{},
	}

	for _, name := range actual.MotorNames() {
		actualPart, ok := actual.MotorByName(name)
		if !ok {
			continue
		}
		p.motors[name] = &proxyBoardMotor{actual: actualPart}
	}
	for _, name := range actual.ServoNames() {
		actualPart, ok := actual.ServoByName(name)
		if !ok {
			continue
		}
		p.servos[name] = &proxyBoardServo{actual: actualPart}
	}
	for _, name := range actual.SPINames() {
		actualPart, ok := actual.SPIByName(name)
		if !ok {
			continue
		}
		p.spis[name] = &proxyBoardSPI{actual: actualPart}
	}
	for _, name := range actual.AnalogReaderNames() {
		actualPart, ok := actual.AnalogReaderByName(name)
		if !ok {
			continue
		}
		p.analogs[name] = &proxyBoardAnalogReader{actual: actualPart}
	}
	for _, name := range actual.DigitalInterruptNames() {
		actualPart, ok := actual.DigitalInterruptByName(name)
		if !ok {
			continue
		}
		p.digitals[name] = &proxyBoardDigitalInterrupt{actual: actualPart}
	}

	return p
}

func (p *proxyBoard) MotorByName(name string) (board.Motor, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	m, ok := p.motors[name]
	return m, ok
}

func (p *proxyBoard) ServoByName(name string) (board.Servo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	s, ok := p.servos[name]
	return s, ok
}

func (p *proxyBoard) SPIByName(name string) (board.SPI, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	s, ok := p.spis[name]
	return s, ok
}

func (p *proxyBoard) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	a, ok := p.analogs[name]
	return a, ok
}

func (p *proxyBoard) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	d, ok := p.digitals[name]
	return d, ok
}

func (p *proxyBoard) GPIOSet(ctx context.Context, pin string, high bool) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.GPIOSet(ctx, pin, high)
}

func (p *proxyBoard) GPIOGet(ctx context.Context, pin string) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.GPIOGet(ctx, pin)
}

func (p *proxyBoard) PWMSet(ctx context.Context, pin string, dutyCycle byte) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.PWMSet(ctx, pin, dutyCycle)
}

func (p *proxyBoard) PWMSetFreq(ctx context.Context, pin string, freq uint) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.PWMSetFreq(ctx, pin, freq)
}

func (p *proxyBoard) MotorNames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := []string{}
	for k := range p.motors {
		names = append(names, k)
	}
	return names
}

func (p *proxyBoard) ServoNames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := []string{}
	for k := range p.servos {
		names = append(names, k)
	}
	return names
}

func (p *proxyBoard) SPINames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := []string{}
	for k := range p.spis {
		names = append(names, k)
	}
	return names
}

func (p *proxyBoard) AnalogReaderNames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := []string{}
	for k := range p.analogs {
		names = append(names, k)
	}
	return names
}

func (p *proxyBoard) DigitalInterruptNames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := []string{}
	for k := range p.digitals {
		names = append(names, k)
	}
	return names
}

func (p *proxyBoard) Status(ctx context.Context) (*pb.BoardStatus, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.actual.ModelAttributes().Remote {
		return p.actual.Status(ctx)
	}
	return board.CreateStatus(ctx, p)
}

func (p *proxyBoard) merge(newBoard board.Board, diff board.ConfigDiff) {
	p.mu.Lock()
	defer p.mu.Unlock()

	actual, ok := newBoard.(*proxyBoard)
	if !ok {
		panic(fmt.Errorf("expected new board to be %T but got %T", actual, newBoard))
	}

	if p.motors == nil && len(diff.Added.Motors) != 0 {
		p.motors = make(map[string]*proxyBoardMotor, len(diff.Added.Motors))
	}
	if p.servos == nil && len(diff.Added.Servos) != 0 {
		p.servos = make(map[string]*proxyBoardServo, len(diff.Added.Servos))
	}
	if p.spis == nil && len(diff.Added.SPIs) != 0 {
		p.spis = make(map[string]*proxyBoardSPI, len(diff.Added.SPIs))
	}
	if p.analogs == nil && len(diff.Added.Analogs) != 0 {
		p.analogs = make(map[string]*proxyBoardAnalogReader, len(diff.Added.Analogs))
	}
	if p.digitals == nil && len(diff.Added.DigitalInterrupts) != 0 {
		p.digitals = make(map[string]*proxyBoardDigitalInterrupt, len(diff.Added.DigitalInterrupts))
	}

	for _, c := range diff.Added.Motors {
		p.motors[c.Name] = actual.motors[c.Name]
	}
	for _, c := range diff.Added.Servos {
		p.servos[c.Name] = actual.servos[c.Name]
	}
	for _, c := range diff.Added.SPIs {
		p.spis[c.Name] = actual.spis[c.Name]
	}
	for _, c := range diff.Added.Analogs {
		p.analogs[c.Name] = actual.analogs[c.Name]
	}
	for _, c := range diff.Added.DigitalInterrupts {
		p.digitals[c.Name] = actual.digitals[c.Name]
	}

	for _, c := range diff.Modified.Motors {
		p.motors[c.Name].replace(actual.motors[c.Name])
	}
	for _, c := range diff.Modified.Servos {
		p.servos[c.Name].replace(actual.servos[c.Name])
	}
	for _, c := range diff.Modified.SPIs {
		p.spis[c.Name].replace(actual.spis[c.Name])
	}
	for _, c := range diff.Modified.Analogs {
		p.analogs[c.Name].replace(actual.analogs[c.Name])
	}
	for _, c := range diff.Modified.DigitalInterrupts {
		p.digitals[c.Name].replace(actual.digitals[c.Name])
	}

	for _, c := range diff.Removed.Motors {
		toRemove, ok := p.motors[c.Name]
		if !ok {
			continue // should not happen
		}
		if err := utils.TryClose(toRemove); err != nil {
			rlog.Logger.Errorw("error closing motor but still reconfiguring", "error", err)
		}
		delete(p.motors, c.Name)
	}
	for _, c := range diff.Removed.Servos {
		toRemove, ok := p.servos[c.Name]
		if !ok {
			continue // should not happen
		}
		if err := utils.TryClose(toRemove); err != nil {
			rlog.Logger.Errorw("error closing servo but still reconfiguring", "error", err)
		}
		delete(p.servos, c.Name)
	}
	for _, c := range diff.Removed.SPIs {
		toRemove, ok := p.spis[c.Name]
		if !ok {
			continue // should not happen
		}
		if err := utils.TryClose(toRemove); err != nil {
			rlog.Logger.Errorw("error closing SPI but still reconfiguring", "error", err)
		}
		delete(p.spis, c.Name)
	}
	for _, c := range diff.Removed.Analogs {
		toRemove, ok := p.analogs[c.Name]
		if !ok {
			continue // should not happen
		}
		if err := utils.TryClose(toRemove); err != nil {
			rlog.Logger.Errorw("error closing analog but still reconfiguring", "error", err)
		}
		delete(p.analogs, c.Name)
	}
	for _, c := range diff.Removed.DigitalInterrupts {
		toRemove, ok := p.digitals[c.Name]
		if !ok {
			continue // should not happen
		}
		if err := utils.TryClose(toRemove); err != nil {
			rlog.Logger.Errorw("error closing digital interrupt but still reconfiguring", "error", err)
		}
		delete(p.digitals, c.Name)
	}

	if diff.Left.Model != diff.Right.Model {
		if err := utils.TryClose(p.actual); err != nil {
			rlog.Logger.Errorw("error closing old", "error", err)
		}
	}

	p.actual = actual.actual
}

func (p *proxyBoard) replace(newBoard board.Board) {
	p.mu.Lock()
	defer p.mu.Unlock()

	actual, ok := newBoard.(*proxyBoard)
	if !ok {
		panic(fmt.Errorf("expected new board to be %T but got %T", actual, newBoard))
	}

	var oldMotorNames map[string]struct{}
	var oldServoNames map[string]struct{}
	var oldSPINames map[string]struct{}
	var oldAnalogReaderNames map[string]struct{}
	var oldDigitalInterruptNames map[string]struct{}

	if len(p.motors) != 0 {
		oldMotorNames = make(map[string]struct{}, len(p.motors))
		for name := range p.motors {
			oldMotorNames[name] = struct{}{}
		}
	}
	if len(p.servos) != 0 {
		oldServoNames = make(map[string]struct{}, len(p.servos))
		for name := range p.servos {
			oldServoNames[name] = struct{}{}
		}
	}
	if len(p.spis) != 0 {
		oldSPINames = make(map[string]struct{}, len(p.spis))
		for name := range p.spis {
			oldSPINames[name] = struct{}{}
		}
	}
	if len(p.analogs) != 0 {
		oldAnalogReaderNames = make(map[string]struct{}, len(p.analogs))
		for name := range p.analogs {
			oldAnalogReaderNames[name] = struct{}{}
		}
	}
	if len(p.digitals) != 0 {
		oldDigitalInterruptNames = make(map[string]struct{}, len(p.digitals))
		for name := range p.digitals {
			oldDigitalInterruptNames[name] = struct{}{}
		}
	}

	for name, newPart := range actual.motors {
		oldPart, ok := p.motors[name]
		delete(oldMotorNames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		p.motors[name] = newPart
	}
	for name, newPart := range actual.servos {
		oldPart, ok := p.servos[name]
		delete(oldServoNames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		p.servos[name] = newPart
	}
	for name, newPart := range actual.spis {
		oldPart, ok := p.spis[name]
		delete(oldSPINames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		p.spis[name] = newPart
	}
	for name, newPart := range actual.analogs {
		oldPart, ok := p.analogs[name]
		delete(oldAnalogReaderNames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		p.analogs[name] = newPart
	}
	for name, newPart := range actual.digitals {
		oldPart, ok := p.digitals[name]
		delete(oldDigitalInterruptNames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		p.digitals[name] = newPart
	}

	for name := range oldMotorNames {
		delete(p.motors, name)
	}
	for name := range oldServoNames {
		delete(p.servos, name)
	}
	for name := range oldSPINames {
		delete(p.spis, name)
	}
	for name := range oldAnalogReaderNames {
		delete(p.analogs, name)
	}
	for name := range oldDigitalInterruptNames {
		delete(p.digitals, name)
	}

	p.actual = actual
}

func (p *proxyBoard) ModelAttributes() board.ModelAttributes {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.ModelAttributes()
}

// Close attempts to cleanly close each part of the board.
func (p *proxyBoard) Close() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return utils.TryClose(p.actual)
}

type proxyBoardMotor struct {
	mu     sync.RWMutex
	actual board.Motor
}

func (p *proxyBoardMotor) replace(newMotor board.Motor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newMotor.(*proxyBoardMotor)
	if !ok {
		panic(fmt.Errorf("expected new motor to be %T but got %T", actual, newMotor))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

func (p *proxyBoardMotor) Power(ctx context.Context, powerPct float32) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Power(ctx, powerPct)
}

func (p *proxyBoardMotor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Go(ctx, d, powerPct)
}

func (p *proxyBoardMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.GoFor(ctx, d, rpm, revolutions)
}

func (p *proxyBoardMotor) Position(ctx context.Context) (float64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Position(ctx)
}

func (p *proxyBoardMotor) PositionSupported(ctx context.Context) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.PositionSupported(ctx)
}

func (p *proxyBoardMotor) Off(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Off(ctx)
}

func (p *proxyBoardMotor) IsOn(ctx context.Context) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.IsOn(ctx)
}

func (p *proxyBoardMotor) Close() error {
	return utils.TryClose(p.actual)
}

func (p *proxyBoardMotor) GoTo(ctx context.Context, rpm float64, position float64) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.GoTo(ctx, rpm, position)
}

func (p *proxyBoardMotor) GoTillStop(ctx context.Context, d pb.DirectionRelative, rpm float64) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.GoTillStop(ctx, d, rpm)
}

func (p *proxyBoardMotor) Zero(ctx context.Context, offset float64) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Zero(ctx, offset)
}

type proxyBoardServo struct {
	mu     sync.RWMutex
	actual board.Servo
}

func (p *proxyBoardServo) replace(newServo board.Servo) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newServo.(*proxyBoardServo)
	if !ok {
		panic(fmt.Errorf("expected new servo to be %T but got %T", actual, newServo))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

func (p *proxyBoardServo) Move(ctx context.Context, angleDegs uint8) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Move(ctx, angleDegs)
}

func (p *proxyBoardServo) Current(ctx context.Context) (uint8, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Current(ctx)
}

func (p *proxyBoardServo) Close() error {
	return utils.TryClose(p.actual)
}

type proxyBoardSPI struct {
	mu     sync.RWMutex
	actual board.SPI
}

func (p *proxyBoardSPI) replace(newSPI board.SPI) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newSPI.(*proxyBoardSPI)
	if !ok {
		panic(fmt.Errorf("expected new SPI to be %T but got %T", actual, newSPI))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

func (p *proxyBoardSPI) OpenHandle() (board.SPIHandle, error) {
	return p.actual.OpenHandle()
}

type proxyBoardAnalogReader struct {
	mu     sync.RWMutex
	actual board.AnalogReader
}

func (p *proxyBoardAnalogReader) replace(newAnalogReader board.AnalogReader) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newAnalogReader.(*proxyBoardAnalogReader)
	if !ok {
		panic(fmt.Errorf("expected new analog reader to be %T but got %T", actual, newAnalogReader))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

func (p *proxyBoardAnalogReader) Read(ctx context.Context) (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Read(ctx)
}

func (p *proxyBoardAnalogReader) Close() error {
	return utils.TryClose(p.actual)
}

type proxyBoardDigitalInterrupt struct {
	mu     sync.RWMutex
	actual board.DigitalInterrupt
}

func (p *proxyBoardDigitalInterrupt) replace(newDigitalInterrupt board.DigitalInterrupt) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newDigitalInterrupt.(*proxyBoardDigitalInterrupt)
	if !ok {
		panic(fmt.Errorf("expected new digital interrupt to be %T but got %T", actual, newDigitalInterrupt))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

func (p *proxyBoardDigitalInterrupt) Config(ctx context.Context) (board.DigitalInterruptConfig, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Config(ctx)
}

func (p *proxyBoardDigitalInterrupt) Value(ctx context.Context) (int64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Value(ctx)
}

func (p *proxyBoardDigitalInterrupt) Tick(ctx context.Context, high bool, nanos uint64) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.Tick(ctx, high, nanos)
}

func (p *proxyBoardDigitalInterrupt) AddCallback(c chan bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	p.actual.AddCallback(c)
}

func (p *proxyBoardDigitalInterrupt) AddPostProcessor(pp board.PostProcessor) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	p.actual.AddPostProcessor(pp)
}

func (p *proxyBoardDigitalInterrupt) Close() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return utils.TryClose(p.actual)
}
