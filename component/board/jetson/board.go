// Package jetson implements a jetson based board.
package jetson

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"

	"go.viam.com/rdk/component/board"
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
			return &jetsonBoard{
				spis:    spis,
				analogs: analogs,
				pwms:    map[string]setPWMting{},
			}, nil
		}})
	board.RegisterConfigAttributeConverter(modelName)
}

type jetsonBoard struct {
	mu      sync.Mutex
	spis    map[string]*spiBus
	analogs map[string]board.AnalogReader
	pwms    map[string]setPWMting
}

type setPWMting struct {
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

func (b *jetsonBoard) getGPIOLine(hwPin string) (gpio.PinIO, error) {
	pinParsed, err := strconv.ParseInt(hwPin, 10, 32)
	if err != nil {
		return nil, err
	}
	mapping, ok := gpioMappings[int(pinParsed)]
	if !ok {
		return nil, errors.Errorf("invalid pin %q", hwPin)
	}

	pin := gpioreg.ByName(fmt.Sprintf("%d", mapping.gpioGlobal))
	if pin == nil {
		return nil, errors.Errorf("no global pin found for '%d'", mapping.gpioGlobal)
	}
	return pin, nil
}

func (b *jetsonBoard) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, errors.New("Do() unimplemented")
}

type gpioPin struct {
	b       *jetsonBoard
	pin     gpio.PinIO
	pinName string
}

func (b *jetsonBoard) GPIOPinByName(pinName string) (board.GPIOPin, error) {
	pin, err := b.getGPIOLine(pinName)
	if err != nil {
		return nil, err
	}

	return gpioPin{b, pin, pinName}, nil
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

func (gp gpioPin) SetPWM(ctx context.Context, dutyCyclePct float64) error {
	gp.b.mu.Lock()
	defer gp.b.mu.Unlock()

	last := gp.b.pwms[gp.pinName]
	var freqHz physic.Frequency
	if last.frequency != 0 {
		freqHz = last.frequency
	}
	duty := gpio.Duty(dutyCyclePct * float64(gpio.DutyMax))
	last.dutyCycle = duty
	gp.b.pwms[gp.pinName] = last

	return gp.pin.PWM(duty, freqHz)
}

func (gp gpioPin) PWMFreq(ctx context.Context) (uint, error) {
	gp.b.mu.Lock()
	defer gp.b.mu.Unlock()

	return uint(gp.b.pwms[gp.pinName].frequency / physic.Hertz), nil
}

func (gp gpioPin) SetPWMFreq(ctx context.Context, freqHz uint) error {
	gp.b.mu.Lock()
	defer gp.b.mu.Unlock()

	last := gp.b.pwms[gp.pinName]
	var duty gpio.Duty
	if last.dutyCycle != 0 {
		duty = last.dutyCycle
	}
	frequency := physic.Hertz * physic.Frequency(freqHz)
	last.frequency = frequency
	gp.b.pwms[gp.pinName] = last

	return gp.pin.PWM(duty, frequency)
}

func (b *jetsonBoard) Status(ctx context.Context) (*commonpb.BoardStatus, error) {
	return &commonpb.BoardStatus{}, nil
}

func (b *jetsonBoard) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{}
}
