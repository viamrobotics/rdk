// Package pca9685 implements a PCA9685 HAT. It's probably also a generic PCA9685
// but that has not been verified yet.
package pca9685

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/registry"
	rdkutils "go.viam.com/rdk/utils"
)

const modelName = "pca9685"

var (
	_ = board.Board(&PCA9685{})
	_ = board.GPIOPin(&gpioPin{})
)

func init() {
	registry.RegisterComponent(
		board.Subtype,
		modelName,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			conf, ok := config.ConvertedAttributes.(*Config)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(conf, config.ConvertedAttributes)
			}

			b, err := board.FromDependencies(deps, conf.BoardName)
			if err != nil {
				return nil, err
			}
			localBoard, ok := b.(board.LocalBoard)
			if !ok {
				return nil, fmt.Errorf("board %s is not local", conf.BoardName)
			}
			bus, ok := localBoard.I2CByName(conf.I2CName)
			if !ok {
				return nil, errors.Errorf("can't find I2C bus (%s) requested by Motor", conf.I2CName)
			}

			return New(ctx, bus, byte(*conf.I2CAddress))
		}})

	config.RegisterComponentAttributeMapConverter(
		board.SubtypeName,
		modelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&Config{})
}

// Config describes a PCA9685 board attached to some other board via I2C.
type Config struct {
	BoardName  string `json:"board_name"`
	I2CName    string `json:"i2c_name"`
	I2CAddress *int   `json:"i2c_address"`
}

// Validate ensures all parts of the config are valid.
func (config *Config) Validate(path string) error {
	if config.BoardName == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "board_name")
	}
	if config.I2CName == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_name")
	}
	if config.I2CAddress == nil {
		config.I2CAddress = &defaultAddr
	}
	if *config.I2CAddress < 0 || *config.I2CAddress > 255 {
		return utils.NewConfigValidationError(path, errors.New("i2c_address must be an unsigned byte"))
	}
	return nil
}

// PCA9685 is a general purpose 16-channel 12-bit PWM controller.
type PCA9685 struct {
	generic.Unimplemented
	address             byte
	referenceClockSpeed int
	bus                 board.I2C
	gpioPins            [16]gpioPin
}

const (
	defaultReferenceClockSpeed = 25000000

	mode1Reg    = 0x00
	prescaleReg = 0xFE
)

var defaultAddr = 0x60

// New returns a new PCA9685 residing on the given bus and address.
func New(ctx context.Context, bus board.I2C, address byte) (*PCA9685, error) {
	pca := PCA9685{
		address:             address,
		referenceClockSpeed: defaultReferenceClockSpeed,
		bus:                 bus,
	}

	// each PWM combination spans 4 bytes
	startAddr := byte(0x06)

	for chanIdx := 0; chanIdx < len(pca.gpioPins); chanIdx++ {
		pca.gpioPins[chanIdx].pca = &pca
		pca.gpioPins[chanIdx].startAddr = startAddr
		startAddr += 4
	}
	if err := pca.reset(ctx); err != nil {
		return nil, err
	}

	return &pca, nil
}

func (pca *PCA9685) parsePin(pin string) (int, error) {
	pinInt, err := strconv.ParseInt(pin, 10, 32)
	if err != nil {
		return 0, err
	}
	if pinInt < 0 || int(pinInt) >= len(pca.gpioPins) {
		return 0, errors.Errorf("channel number must be between [0, %d)", len(pca.gpioPins))
	}
	return int(pinInt), nil
}

// ModelAttributes returns attributes related to the model of this board.
func (pca *PCA9685) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{}
}

// Status returns the board status which is always empty.
func (pca *PCA9685) Status(ctx context.Context) (*commonpb.BoardStatus, error) {
	return &commonpb.BoardStatus{}, nil
}

// GPIOPinByName returns a GPIOPin by name.
func (pca *PCA9685) GPIOPinByName(pin string) (board.GPIOPin, error) {
	pinInt, err := pca.parsePin(pin)
	if err != nil {
		return nil, err
	}

	if pinInt < 0 || pinInt >= len(pca.gpioPins) {
		return nil, errors.New("pin name must be between [0, 16)")
	}
	return &pca.gpioPins[pinInt], nil
}

// GPIOPinNames returns the names of all known GPIO pins.
func (pca *PCA9685) GPIOPinNames() []string {
	return []string{
		"0", "1", "2", "3",
		"4", "5", "6", "7",
		"8", "9", "10", "11",
		"12", "13", "14", "15",
	}
}

func (pca *PCA9685) openHandle() (board.I2CHandle, error) {
	return pca.bus.OpenHandle(pca.address)
}

func (pca *PCA9685) reset(ctx context.Context) error {
	handle, err := pca.openHandle()
	if err != nil {
		return err
	}
	defer func() {
		utils.UncheckedError(handle.Close())
	}()
	return handle.WriteByteData(ctx, mode1Reg, 0x00)
}

func (pca *PCA9685) frequency(ctx context.Context) (float64, error) {
	handle, err := pca.openHandle()
	if err != nil {
		return 0, err
	}
	defer func() {
		utils.UncheckedError(handle.Close())
	}()

	prescale, err := handle.ReadByteData(ctx, prescaleReg)
	if err != nil {
		return 0, err
	}
	return float64(pca.referenceClockSpeed) / 4096.0 / float64(prescale), nil
}

// SetFrequency sets the global PWM frequency for the pca.
func (pca *PCA9685) SetFrequency(ctx context.Context, frequency float64) error {
	prescale := byte((float64(pca.referenceClockSpeed) / 4096.0 / frequency) + 0.5)
	if prescale < 3 {
		return errors.New("invalid frequency")
	}

	handle, err := pca.openHandle()
	if err != nil {
		return err
	}
	defer func() {
		utils.UncheckedError(handle.Close())
	}()

	oldMode1, err := handle.ReadByteData(ctx, mode1Reg)
	if err != nil {
		return err
	}

	if err := handle.WriteByteData(ctx, mode1Reg, (oldMode1&0x7F)|0x10); err != nil {
		return err
	}
	if err := handle.WriteByteData(ctx, prescaleReg, prescale); err != nil {
		return err
	}
	if err := handle.WriteByteData(ctx, mode1Reg, oldMode1); err != nil {
		return err
	}
	time.Sleep(5 * time.Millisecond)
	if err := handle.WriteByteData(ctx, mode1Reg, oldMode1|0xA0); err != nil {
		return err
	}
	return nil
}

// SPINames returns the name of all known SPIs.
func (pca *PCA9685) SPINames() []string {
	return nil
}

// I2CNames returns the name of all known I2Cs.
func (pca *PCA9685) I2CNames() []string {
	return nil
}

// AnalogReaderNames returns the name of all known analog readers.
func (pca *PCA9685) AnalogReaderNames() []string {
	return nil
}

// DigitalInterruptNames returns the name of all known digital interrupts.
func (pca *PCA9685) DigitalInterruptNames() []string {
	return nil
}

// SPIByName returns the SPI by the given name if it exists.
func (pca *PCA9685) SPIByName(name string) (board.SPI, bool) {
	return nil, false
}

// I2CByName returns the i2c by the given name if it exists.
func (pca *PCA9685) I2CByName(name string) (board.I2C, bool) {
	return nil, false
}

// AnalogReaderByName returns the analog reader by the given name if it exists.
func (pca *PCA9685) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	return nil, false
}

// DigitalInterruptByName returns the interrupt by the given name if it exists.
func (pca *PCA9685) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
	return nil, false
}

// A gpioPin in PCA9685 is the combination of a PWM's T_on and T_off
// represented as two 12-bit (4096 step) values.
type gpioPin struct {
	pca       *PCA9685
	startAddr byte
}

func (gp *gpioPin) Get(ctx context.Context) (bool, error) {
	dutyCycle, err := gp.PWM(ctx)
	if err != nil {
		return false, err
	}
	return dutyCycle != 0, nil
}

func (gp *gpioPin) Set(ctx context.Context, high bool) error {
	var dutyCyclePct float64
	if high {
		dutyCyclePct = 1
	}

	return gp.SetPWM(ctx, dutyCyclePct)
}

func (gp *gpioPin) PWM(ctx context.Context) (float64, error) {
	handle, err := gp.pca.openHandle()
	if err != nil {
		return 0, err
	}
	defer func() {
		utils.UncheckedError(handle.Close())
	}()

	regOnLow := board.I2CRegister{handle, gp.startAddr}
	regOnHigh := board.I2CRegister{handle, gp.startAddr + 1}
	regOffLow := board.I2CRegister{handle, gp.startAddr + 2}
	regOffHigh := board.I2CRegister{handle, gp.startAddr + 3}

	onLow, err := regOnLow.ReadByteData(ctx)
	if err != nil {
		return 0, err
	}
	onHigh, err := regOnHigh.ReadByteData(ctx)
	if err != nil {
		return 0, err
	}
	onVal := uint16(onLow) | (uint16(onHigh) << 8)
	if onVal == 0x1000 {
		return 1, nil
	}

	// Off takes up zero steps
	offLow, err := regOffLow.ReadByteData(ctx)
	if err != nil {
		return 0, err
	}
	offHigh, err := regOffHigh.ReadByteData(ctx)
	if err != nil {
		return 0, err
	}
	offVal := uint16(offLow) | (uint16(offHigh) << 8)
	return float64(offVal<<4) / 0xffff, nil
}

func (gp *gpioPin) SetPWM(ctx context.Context, dutyCyclePct float64) error {
	dutyCycle := uint16(dutyCyclePct * float64(0xffff))

	handle, err := gp.pca.openHandle()
	if err != nil {
		return err
	}
	defer func() {
		utils.UncheckedError(handle.Close())
	}()

	regOnLow := board.I2CRegister{handle, gp.startAddr}
	regOnHigh := board.I2CRegister{handle, gp.startAddr + 1}
	regOffLow := board.I2CRegister{handle, gp.startAddr + 2}
	regOffHigh := board.I2CRegister{handle, gp.startAddr + 3}

	if dutyCycle == 0xffff {
		// On takes up all steps
		if err := regOnLow.WriteByteData(ctx, 0x00); err != nil {
			return err
		}
		if err := regOnHigh.WriteByteData(ctx, 0x10); err != nil {
			return err
		}

		// Off takes up zero steps
		if err := regOffLow.WriteByteData(ctx, 0x00); err != nil {
			return err
		}
		if err := regOffHigh.WriteByteData(ctx, 0x00); err != nil {
			return err
		}
		return nil
	}

	// On takes up zero steps
	if err := regOnLow.WriteByteData(ctx, 0x00); err != nil {
		return err
	}
	if err := regOnHigh.WriteByteData(ctx, 0x00); err != nil {
		return err
	}

	// Off takes up "dutyCycle" steps
	dutyCycle >>= 4

	if err := regOffLow.WriteByteData(ctx, byte(dutyCycle&0xff)); err != nil {
		return err
	}
	if err := regOffHigh.WriteByteData(ctx, byte(dutyCycle>>8)); err != nil {
		return err
	}
	return nil
}

func (gp *gpioPin) PWMFreq(ctx context.Context) (uint, error) {
	freqHz, err := gp.pca.frequency(ctx)
	if err != nil {
		return 0, err
	}
	return uint(freqHz), nil
}

func (gp *gpioPin) SetPWMFreq(ctx context.Context, freqHz uint) error {
	return gp.pca.SetFrequency(ctx, float64(freqHz))
}
