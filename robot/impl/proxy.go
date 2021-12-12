package robotimpl

import (
	"context"
	"errors"
	"fmt"
	"sync"

	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/utils"

	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/input"
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

type proxyBoard struct {
	mu       sync.RWMutex
	actual   board.Board
	spis     map[string]*proxyBoardSPI
	i2cs     map[string]*proxyBoardI2C
	analogs  map[string]*proxyBoardAnalogReader
	digitals map[string]*proxyBoardDigitalInterrupt
}

func newProxyBoard(actual board.Board) *proxyBoard {
	p := &proxyBoard{
		actual:   actual,
		spis:     map[string]*proxyBoardSPI{},
		i2cs:     map[string]*proxyBoardI2C{},
		analogs:  map[string]*proxyBoardAnalogReader{},
		digitals: map[string]*proxyBoardDigitalInterrupt{},
	}

	for _, name := range actual.SPINames() {
		actualPart, ok := actual.SPIByName(name)
		if !ok {
			continue
		}
		p.spis[name] = &proxyBoardSPI{actual: actualPart}
	}
	for _, name := range actual.I2CNames() {
		actualPart, ok := actual.I2CByName(name)
		if !ok {
			continue
		}
		p.i2cs[name] = &proxyBoardI2C{actual: actualPart}
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

func (p *proxyBoard) ProxyFor() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual
}

func (p *proxyBoard) SPIByName(name string) (board.SPI, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	s, ok := p.spis[name]
	return s, ok
}

func (p *proxyBoard) I2CByName(name string) (board.I2C, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	s, ok := p.i2cs[name]
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

func (p *proxyBoard) SPINames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := []string{}
	for k := range p.spis {
		names = append(names, k)
	}
	return names
}

func (p *proxyBoard) I2CNames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := []string{}
	for k := range p.i2cs {
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

func (p *proxyBoard) replace(newBoard board.Board) {
	p.mu.Lock()
	defer p.mu.Unlock()

	actual, ok := newBoard.(*proxyBoard)
	if !ok {
		panic(fmt.Errorf("expected new board to be %T but got %T", actual, newBoard))
	}

	var oldSPINames map[string]struct{}
	var oldI2CNames map[string]struct{}
	var oldAnalogReaderNames map[string]struct{}
	var oldDigitalInterruptNames map[string]struct{}

	if len(p.spis) != 0 {
		oldSPINames = make(map[string]struct{}, len(p.spis))
		for name := range p.spis {
			oldSPINames[name] = struct{}{}
		}
	}
	if len(p.i2cs) != 0 {
		oldI2CNames = make(map[string]struct{}, len(p.i2cs))
		for name := range p.i2cs {
			oldI2CNames[name] = struct{}{}
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

	for name, newPart := range actual.spis {
		oldPart, ok := p.spis[name]
		delete(oldSPINames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		p.spis[name] = newPart
	}
	for name, newPart := range actual.i2cs {
		oldPart, ok := p.i2cs[name]
		delete(oldI2CNames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		p.i2cs[name] = newPart
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

	for name := range oldSPINames {
		delete(p.spis, name)
	}
	for name := range oldI2CNames {
		delete(p.i2cs, name)
	}
	for name := range oldAnalogReaderNames {
		delete(p.analogs, name)
	}
	for name := range oldDigitalInterruptNames {
		delete(p.digitals, name)
	}

	p.actual = actual.actual
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

type proxyBoardI2C struct {
	mu     sync.RWMutex
	actual board.I2C
}

func (p *proxyBoardI2C) ProxyFor() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual
}

func (p *proxyBoardI2C) replace(newI2C board.I2C) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newI2C.(*proxyBoardI2C)
	if !ok {
		panic(fmt.Errorf("expected new I2C to be %T but got %T", actual, newI2C))
	}
	if err := utils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

func (p *proxyBoardI2C) OpenHandle(addr byte) (board.I2CHandle, error) {
	return p.actual.OpenHandle(addr)
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

func (p *proxyBoardAnalogReader) ProxyFor() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual
}

func (p *proxyBoardAnalogReader) Close() error {
	return utils.TryClose(p.actual)
}

type proxyBoardDigitalInterrupt struct {
	mu     sync.RWMutex
	actual board.DigitalInterrupt
}

func (p *proxyBoardDigitalInterrupt) ProxyFor() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual
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
