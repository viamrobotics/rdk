// Package commonsysfs implements a sysfs (https://en.wikipedia.org/wiki/Sysfs) based board. This does not provide
// a board model itself but provides the underlying logic for any sysfs based board.
package commonsysfs

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	commonpb "go.viam.com/api/common/v1"
	goutils "go.viam.com/utils"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

var _ = board.LocalBoard(&sysfsBoard{})

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	I2Cs              []board.I2CConfig              `json:"i2cs,omitempty"`
	SPIs              []board.SPIConfig              `json:"spis,omitempty"`
	Analogs           []board.AnalogConfig           `json:"analogs,omitempty"`
	DigitalInterrupts []board.DigitalInterruptConfig `json:"digital_interrupts,omitempty"`
	Attributes        config.AttributeMap            `json:"attributes,omitempty"`
}

// RegisterBoard registers a sysfs based board of the given model.
func RegisterBoard(modelName string, gpioMappings map[int]GPIOBoardMapping) {
	registry.RegisterComponent(
		board.Subtype,
		resource.NewDefaultModel(resource.ModelName(modelName)),
		registry.Component{Constructor: func(
			ctx context.Context,
			_ registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			conf, ok := config.ConvertedAttributes.(*Config)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(conf, config.ConvertedAttributes)
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
			return &sysfsBoard{
				gpioMappings: gpioMappings,
				spis:         spis,
				analogs:      analogs,
				pwms:         map[string]pwmSetting{},
				logger:       logger,
				cancelCtx:    cancelCtx,
				cancelFunc:   cancelFunc,
			}, nil
		}})
	config.RegisterComponentAttributeMapConverter(
		board.Subtype,
		resource.NewDefaultModel(resource.ModelName(modelName)),
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&Config{})
}

// Validate ensures all parts of the config are valid.
func (config *Config) Validate(path string) error {
	for idx, conf := range config.SPIs {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "spis", idx)); err != nil {
			return err
		}
	}
	for idx, conf := range config.I2Cs {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "i2cs", idx)); err != nil {
			return err
		}
	}
	for idx, conf := range config.Analogs {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "analogs", idx)); err != nil {
			return err
		}
	}
	for idx, conf := range config.DigitalInterrupts {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "digital_interrupts", idx)); err != nil {
			return err
		}
	}
	return nil
}

func init() {
}

type sysfsBoard struct {
	generic.Unimplemented
	mu           sync.RWMutex
	gpioMappings map[int]GPIOBoardMapping
	spis         map[string]*spiBus
	analogs      map[string]board.AnalogReader
	pwms         map[string]pwmSetting
	logger       golog.Logger

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

type pwmSetting struct {
	dutyCycle gpio.Duty
	frequency physic.Frequency
}

func (b *sysfsBoard) SPIByName(name string) (board.SPI, bool) {
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

func (b *sysfsBoard) I2CByName(name string) (board.I2C, bool) {
	return nil, false
}

func (b *sysfsBoard) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	a, ok := b.analogs[name]
	return a, ok
}

func (b *sysfsBoard) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
	return nil, false
}

func (b *sysfsBoard) SPINames() []string {
	if len(b.spis) == 0 {
		return nil
	}
	names := make([]string, 0, len(b.spis))
	for k := range b.spis {
		names = append(names, k)
	}
	return names
}

func (b *sysfsBoard) I2CNames() []string {
	return nil
}

func (b *sysfsBoard) AnalogReaderNames() []string {
	names := []string{}
	for k := range b.analogs {
		names = append(names, k)
	}
	return names
}

func (b *sysfsBoard) DigitalInterruptNames() []string {
	return nil
}

func (b *sysfsBoard) GPIOPinNames() []string {
	if b.gpioMappings == nil {
		return nil
	}
	names := []string{}
	for k := range b.gpioMappings {
		names = append(names, fmt.Sprintf("%d", k))
	}
	return names
}

func (b *sysfsBoard) getGPIOLine(hwPin string) (gpio.PinIO, bool, error) {
	pinName := hwPin
	hwPWMSupported := true
	if b.gpioMappings != nil {
		pinParsed, err := strconv.ParseInt(hwPin, 10, 32)
		if err != nil {
			return nil, false, errors.New("pin cannot be parsed or unset")
		}

		mapping, ok := b.gpioMappings[int(pinParsed)]
		if !ok {
			return nil, false, errors.Errorf("invalid pin \"%d\"", pinParsed)
		}
		pinName = fmt.Sprintf("%d", mapping.GPIOGlobal)
		hwPWMSupported = mapping.HWPWMSupported
	}

	pin := gpioreg.ByName(pinName)
	if pin == nil {
		return nil, false, errors.Errorf("no global pin found for %q", pinName)
	}
	return pin, hwPWMSupported, nil
}

type gpioPin struct {
	b              *sysfsBoard
	pin            gpio.PinIO
	pinName        string
	hwPWMSupported bool
}

func (b *sysfsBoard) GPIOPinByName(pinName string) (board.GPIOPin, error) {
	pin, hwPWMSupported, err := b.getGPIOLine(pinName)
	if err != nil {
		return nil, err
	}

	return gpioPin{b, pin, pinName, hwPWMSupported}, nil
}

func (gp gpioPin) Set(ctx context.Context, high bool, extra map[string]interface{}) error {
	gp.b.mu.Lock()
	defer gp.b.mu.Unlock()

	delete(gp.b.pwms, gp.pinName)

	return gp.set(high)
}

func (gp gpioPin) set(high bool) error {
	l := gpio.Low
	if high {
		l = gpio.High
	}
	return gp.pin.Out(l)
}

func (gp gpioPin) Get(ctx context.Context, extra map[string]interface{}) (bool, error) {
	return gp.pin.Read() == gpio.High, nil
}

func (gp gpioPin) PWM(ctx context.Context, extra map[string]interface{}) (float64, error) {
	gp.b.mu.RLock()
	defer gp.b.mu.RUnlock()

	pwm, ok := gp.b.pwms[gp.pinName]
	if !ok {
		return 0, fmt.Errorf("missing pin %s", gp.pinName)
	}
	return float64(pwm.dutyCycle) / float64(gpio.DutyMax), nil
}

// expects to already have lock acquired.
func (b *sysfsBoard) startSoftwarePWMLoop(gp gpioPin) {
	b.activeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		b.softwarePWMLoop(b.cancelCtx, gp)
	}, b.activeBackgroundWorkers.Done)
}

func (b *sysfsBoard) softwarePWMLoop(ctx context.Context, gp gpioPin) {
	for {
		cont := func() bool {
			b.mu.RLock()
			defer b.mu.RUnlock()
			pwmSetting, ok := b.pwms[gp.pinName]
			if !ok {
				b.logger.Debug("pwm setting deleted; stopping")
				return false
			}

			if err := gp.set(true); err != nil {
				b.logger.Errorw("error setting pin", "pin_name", gp.pinName, "error", err)
				return true
			}
			onPeriod := time.Duration(
				int64((float64(pwmSetting.dutyCycle) / float64(gpio.DutyMax)) * float64(pwmSetting.frequency.Period())),
			)
			if !goutils.SelectContextOrWait(ctx, onPeriod) {
				return false
			}
			if err := gp.set(false); err != nil {
				b.logger.Errorw("error setting pin", "pin_name", gp.pinName, "error", err)
				return true
			}
			offPeriod := pwmSetting.frequency.Period() - onPeriod

			return goutils.SelectContextOrWait(ctx, offPeriod)
		}()
		if !cont {
			return
		}
	}
}

func (gp gpioPin) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
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
		err := gp.pin.PWM(duty, freqHz)
		// TODO: [RSDK-569] (rh) find or implement a PWM sysfs that works with hardware pwm mappings
		// periph.io does not implement PWM
		if err != nil {
			return errors.New("sysfs PWM not currently supported, use another pin for software PWM loops")
		}
	}

	if !alreadySet {
		gp.b.startSoftwarePWMLoop(gp)
	}

	return nil
}

func (gp gpioPin) PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error) {
	gp.b.mu.RLock()
	defer gp.b.mu.RUnlock()

	return uint(gp.b.pwms[gp.pinName].frequency / physic.Hertz), nil
}

func (gp gpioPin) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
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

func (b *sysfsBoard) Status(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
	return &commonpb.BoardStatus{}, nil
}

func (b *sysfsBoard) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{}
}

func (b *sysfsBoard) Close() {
	b.mu.Lock()
	b.cancelFunc()
	b.mu.Unlock()
	b.activeBackgroundWorkers.Wait()
}
