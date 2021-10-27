package jetson

import (
	"context"
	"fmt"
	"strconv"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"go.uber.org/multierr"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/rlog"
	"go.viam.com/core/robot"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
)

var gpioMappings map[int]gpioBoardMapping

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

	registry.RegisterBoard(modelName, registry.Board{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (board.Board, error) {
		conf := config.ConvertedAttributes.(*board.Config)
		if len(conf.DigitalInterrupts) != 0 {
			return nil, errors.New("digital interrupts unsupported")
		}
		if len(conf.I2Cs) != 0 {
			return nil, errors.New("i2c unsupported")
		}
		var spis map[string]spiWrapper
		if len(conf.SPIs) != 0 {
			spis = make(map[string]spiWrapper, len(conf.SPIs))
			for _, spiConf := range conf.SPIs {
				spis[spiConf.Name] = spiWrapper{spiConf.BusSelect}
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
		}, nil
	}})
	board.RegisterConfigAttributeConverter(modelName)
}

type jetsonBoard struct {
	spis    map[string]spiWrapper
	analogs map[string]board.AnalogReader
}

func (b *jetsonBoard) SPIByName(name string) (board.SPI, bool) {
	s, ok := b.spis[name]
	if !ok {
		return nil, false
	}
	return s, true
}

type spiWrapper struct {
	bus string
}

type spiHandleWrapper struct {
	bus string
}

func (sw spiWrapper) OpenHandle() (board.SPIHandle, error) {
	return &spiHandleWrapper{sw.bus}, nil
}

func (sw *spiHandleWrapper) Xfer(ctx context.Context, baud uint, chipSelect string, mode uint, tx []byte) (rx []byte, err error) {
	port, err := spireg.Open(fmt.Sprintf("SPI%s.%s", sw.bus, chipSelect))
	if err != nil {
		return nil, err
	}
	defer func() {
		err = multierr.Combine(err, port.Close())
	}()
	conn, err := port.Connect(physic.Frequency(baud*1e6), spi.Mode(mode), 8)
	if err != nil {
		return nil, err
	}
	rx = make([]byte, len(tx))
	return rx, conn.Tx(tx, rx)
}

func (sw *spiHandleWrapper) Close() error {
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

func (b *jetsonBoard) getGPIOLine(hwPin int) (gpio.PinIO, error) {
	mapping, ok := gpioMappings[hwPin]
	if !ok {
		return nil, errors.Errorf("invalid pin '%d'", hwPin)
	}

	pin := gpioreg.ByName(fmt.Sprintf("%d", mapping.gpioGlobal))
	if pin == nil {
		return nil, errors.Errorf("no global pin found for '%d'", mapping.gpioGlobal)
	}
	return pin, nil
}

func (b *jetsonBoard) GPIOSet(ctx context.Context, pinName string, high bool) error {
	pinParsed, err := strconv.ParseInt(pinName, 10, 32)
	if err != nil {
		return err
	}
	pin, err := b.getGPIOLine(int(pinParsed))
	if err != nil {
		return err
	}

	l := gpio.Low
	if high {
		l = gpio.High
	}
	return pin.Out(l)
}

func (b *jetsonBoard) GPIOGet(ctx context.Context, pinName string) (bool, error) {
	pinParsed, err := strconv.ParseInt(pinName, 10, 32)
	if err != nil {
		return false, err
	}
	pin, err := b.getGPIOLine(int(pinParsed))
	if err != nil {
		return false, err
	}

	return pin.Read() == gpio.High, nil
}

func (b *jetsonBoard) PWMSet(ctx context.Context, pin string, dutyCycle byte) error {
	return errors.New("unsupported")
}

func (b *jetsonBoard) PWMSetFreq(ctx context.Context, pin string, freq uint) error {
	return errors.New("unsupported")
}

func (b *jetsonBoard) Status(ctx context.Context) (*pb.BoardStatus, error) {
	return &pb.BoardStatus{}, nil
}

func (b *jetsonBoard) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{}
}

func (b *jetsonBoard) Close() error {
	return nil
}
