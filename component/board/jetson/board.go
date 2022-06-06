// Package jetson implements a jetson based board.
package jetson

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	goutils "go.viam.com/utils"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

var (
	_ = board.LocalBoard(&jetsonBoard{})

	gpioMappings map[int]gpioBoardMapping
)

const modelName = "jetson"

func init() {
	if _, err := host.Init(); err != nil {
		rlog.Logger.Debugw("error initializing host", "error", err)
	}

	var err error
	gpioMappings, err = getGPIOBoardMappings()
	if err != nil && !errors.Is(err, errNoJetson) {
		rlog.Logger.Debugw("error getting jetson GPIO board mapping", "error", err)
	}

	registry.RegisterComponent(
		board.Subtype,
		modelName,
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			conf, ok := config.ConvertedAttributes.(*board.Config)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(conf, config.ConvertedAttributes)
			}
			if len(conf.DigitalInterrupts) != 0 {
				return nil, errors.New("digital interrupts unsupported")
			}
			if len(conf.I2Cs) != 0 {
				return nil, errors.New("i2c unsupported")
			}
			var spis map[string]*spiBus
			if len(conf.SPIs) != 0 {
				spis = make(map[string]*spiBus, len(conf.SPIs))
				for _, spiConf := range conf.SPIs {
					spis[spiConf.Name] = &spiBus{bus: spiConf.BusSelect}
				}
			}
			var analogs map[string]board.AnalogReader
			if len(conf.Analogs) != 0 {
				analogs = make(map[string]board.AnalogReader, len(conf.Analogs))
				for _, analogConf := range conf.Analogs {
					channel, err := strconv.Atoi(analogConf.Pin)
					if err != nil {
						return nil, errors.Errorf("bad analog pin (%s)", analogConf.Pin)
					}

					bus, ok := spis[analogConf.SPIBus]
					if !ok {
						return nil, errors.Errorf("can't find SPI bus (%s) requested by AnalogReader", analogConf.SPIBus)
					}

					ar := &board.MCP3008AnalogReader{channel, bus, analogConf.ChipSelect}
					analogs[analogConf.Name] = board.SmoothAnalogReader(ar, analogConf, logger)
				}
			}

			cancelCtx, cancelFunc := context.WithCancel(context.Background())
			jb := &jetsonBoard{
				spis:       spis,
				analogs:    analogs,
				pwms:       map[string]pwmSetting{},
				logger:     logger,
				cancelCtx:  cancelCtx,
				cancelFunc: cancelFunc,
			}

			return jb, nil
		}})
	board.RegisterConfigAttributeConverter(modelName)
}

type jetsonBoard struct {
	generic.Unimplemented
	mu      sync.RWMutex
	spis    map[string]*spiBus
	analogs map[string]board.AnalogReader
	pwms    map[string]pwmSetting
	logger  golog.Logger

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

type pwmSetting struct {
	dutyCycle gpio.Duty
	frequency physic.Frequency
}

func (b *jetsonBoard) SPIByName(name string) (board.SPI, bool) {
	s, ok := b.spis[name]
	if !ok {
		return nil, false
	}
	return s, true
}

type spiBus struct {
	mu         sync.Mutex
	openHandle *spiHandle
	bus        string
}

type spiHandle struct {
	bus      *spiBus
	isClosed bool
}

func (sb *spiBus) OpenHandle() (board.SPIHandle, error) {
	sb.mu.Lock()
	sb.openHandle = &spiHandle{bus: sb, isClosed: false}
	return sb.openHandle, nil
}

func (sh *spiHandle) Xfer(ctx context.Context, baud uint, chipSelect string, mode uint, tx []byte) (rx []byte, err error) {
	if sh.isClosed {
		return nil, errors.New("can't use Xfer() on an already closed SPIHandle")
	}

	port, err := spireg.Open(fmt.Sprintf("SPI%s.%s", sh.bus.bus, chipSelect))
	if err != nil {
		return nil, err
	}
	defer func() {
		err = multierr.Combine(err, port.Close())
	}()
	conn, err := port.Connect(physic.Hertz*physic.Frequency(baud), spi.Mode(mode), 8)
	if err != nil {
		return nil, err
	}
	rx = make([]byte, len(tx))
	return rx, conn.Tx(tx, rx)
}

func (sh *spiHandle) Close() error {
	sh.isClosed = true
	sh.bus.mu.Unlock()
	return nil
}

func (b *jetsonBoard) I2CByName(name string) (board.I2C, bool) {
	return nil, false
}

func (b *jetsonBoard) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	a, ok := b.analogs[name]
	return a, ok
}

func (b *jetsonBoard) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
	return nil, false
}

func (b *jetsonBoard) SPINames() []string {
	if len(b.spis) == 0 {
		return nil
	}
	names := make([]string, 0, len(b.spis))
	for k := range b.spis {
		names = append(names, k)
	}
	return names
}

func (b *jetsonBoard) I2CNames() []string {
	return nil
}

func (b *jetsonBoard) AnalogReaderNames() []string {
	names := []string{}
	for k := range b.analogs {
		names = append(names, k)
	}
	return names
}

func (b *jetsonBoard) DigitalInterruptNames() []string {
	return nil
}

func (b *jetsonBoard) GPIOPinNames() []string {
	names := []string{}
	for k := range gpioMappings {
		names = append(names, fmt.Sprintf("%d", k))
	}
	return names
}

func (b *jetsonBoard) getGPIOLine(hwPin string) (gpio.PinIO, bool, error) {
	pinParsed, err := strconv.ParseInt(hwPin, 10, 32)
	if err != nil {
		return nil, false, err
	}
	mapping, ok := gpioMappings[int(pinParsed)]
	if !ok {
		return nil, false, errors.Errorf("invalid pin %q", hwPin)
	}

	pin := gpioreg.ByName(fmt.Sprintf("%d", mapping.gpioGlobal))
	if pin == nil {
		return nil, false, errors.Errorf("no global pin found for '%d'", mapping.gpioGlobal)
	}
	return pin, mapping.hwPWMSupported, nil
}

type gpioPin struct {
	b              *jetsonBoard
	pin            gpio.PinIO
	pinName        string
	hwPWMSupported bool
}

func (b *jetsonBoard) GPIOPinByName(pinName string) (board.GPIOPin, error) {
	pin, hwPWMSupported, err := b.getGPIOLine(pinName)
	if err != nil {
		return nil, err
	}

	return gpioPin{b, pin, pinName, hwPWMSupported}, nil
}

func (gp gpioPin) Set(ctx context.Context, high bool) error {
	l := gpio.Low
	if high {
		l = gpio.High
	}
	return gp.pin.Out(l)
}

func (gp gpioPin) Get(ctx context.Context) (bool, error) {
	return gp.pin.Read() == gpio.High, nil
}

func (gp gpioPin) PWM(ctx context.Context) (float64, error) {
	gp.b.mu.Lock()
	defer gp.b.mu.Unlock()

	return float64(gp.b.pwms[gp.pinName].dutyCycle), nil
}

// expects to already have lock acquired.
func (b *jetsonBoard) startSoftwarePWMLoop(gp gpioPin) {
	b.activeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		b.softwarePWMLoop(b.cancelCtx, gp)
	}, b.activeBackgroundWorkers.Done)
}

func (b *jetsonBoard) softwarePWMLoop(ctx context.Context, gp gpioPin) {
	for {
		b.mu.RLock()
		pwmSetting := b.pwms[gp.pin.Name()]
		b.mu.RUnlock()

		if err := gp.Set(ctx, true); err != nil {
			b.logger.Errorw("error setting pin", "pin_name", gp.pinName, "error", err)
			continue
		}
		onPeriod := time.Duration(int64((float64(pwmSetting.dutyCycle) / float64(gpio.DutyMax)) * float64(pwmSetting.frequency.Period())))
		b.logger.Debugw("pwm on", "pin_name", gp.pinName, "period", onPeriod)
		if !goutils.SelectContextOrWait(ctx, onPeriod) {
			return
		}
		if err := gp.Set(ctx, false); err != nil {
			b.logger.Errorw("error setting pin", "pin_name", gp.pinName, "error", err)
			continue
		}
		offPeriod := pwmSetting.frequency.Period() - onPeriod
		b.logger.Debugw("pwm off", "pin_name", gp.pinName, "period", offPeriod)

		if !goutils.SelectContextOrWait(ctx, offPeriod) {
			return
		}
	}
}

func (gp gpioPin) SetPWM(ctx context.Context, dutyCyclePct float64) error {
	gp.b.mu.Lock()
	defer gp.b.mu.Unlock()

	last, alreadySet := gp.b.pwms[gp.pinName]
	var freqHz physic.Frequency
	if last.frequency != 0 {
		freqHz = last.frequency
	}
	duty := gpio.Duty(dutyCyclePct * float64(gpio.DutyMax))
	last.dutyCycle = duty
	gp.b.pwms[gp.pinName] = last

	if gp.hwPWMSupported {
		return gp.pin.PWM(duty, freqHz)
	}

	if !alreadySet {
		gp.b.startSoftwarePWMLoop(gp)
	}

	return nil
}

func (gp gpioPin) PWMFreq(ctx context.Context) (uint, error) {
	gp.b.mu.Lock()
	defer gp.b.mu.Unlock()

	return uint(gp.b.pwms[gp.pinName].frequency / physic.Hertz), nil
}

func (gp gpioPin) SetPWMFreq(ctx context.Context, freqHz uint) error {
	gp.b.mu.Lock()
	defer gp.b.mu.Unlock()

	last, alreadySet := gp.b.pwms[gp.pinName]
	var duty gpio.Duty
	if last.dutyCycle != 0 {
		duty = last.dutyCycle
	}
	frequency := physic.Hertz * physic.Frequency(freqHz)
	last.frequency = frequency
	gp.b.pwms[gp.pinName] = last

	if gp.hwPWMSupported {
		return gp.pin.PWM(duty, frequency)
	}

	if !alreadySet {
		gp.b.startSoftwarePWMLoop(gp)
	}

	return nil
}

func (b *jetsonBoard) Status(ctx context.Context) (*commonpb.BoardStatus, error) {
	return &commonpb.BoardStatus{}, nil
}

func (b *jetsonBoard) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{}
}

func (b *jetsonBoard) Close() {
	b.cancelFunc()
	b.activeBackgroundWorkers.Wait()
}
