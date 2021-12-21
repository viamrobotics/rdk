package arduino

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/edaniels/golog"

	slib "github.com/jacobsa/go-serial/serial"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/serial"
)

const modelName = "arduino"

// init registers an arduino board.
func init() {
	registry.RegisterBoard(
		modelName,
		registry.Board{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (board.Board, error) {
			boardConfig := config.ConvertedAttributes.(*board.Config)
			return newArduino(ctx, boardConfig, logger)
		}})
	board.RegisterConfigAttributeConverter(modelName)
}

func getSerialConfig(cfg *board.Config) (slib.OpenOptions, error) {

	options := slib.OpenOptions{
		PortName:        cfg.Attributes["port"],
		BaudRate:        230400,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	if options.PortName == "" {
		ds := serial.Search(serial.SearchFilter{serial.TypeArduino})
		if len(ds) != 1 {
			return options, fmt.Errorf("found %d arduinos", len(ds))
		}
		options.PortName = ds[0].Path
	}

	return options, nil
}

func newArduino(ctx context.Context, cfg *board.Config, logger golog.Logger) (*arduinoBoard, error) {
	options, err := getSerialConfig(cfg)
	if err != nil {
		return nil, err
	}

	port, err := slib.Open(options)
	if err != nil {
		return nil, err
	}

	b := &arduinoBoard{
		cfg:        cfg,
		port:       port,
		portReader: bufio.NewReader(port),
		logger:     logger,
	}

	err = b.configure(cfg)
	if err != nil {
		return nil, multierr.Combine(err, b.Close())
	}
	return b, nil
}

type arduinoBoard struct {
	cfg        *board.Config
	port       io.ReadWriteCloser
	portReader *bufio.Reader
	logger     golog.Logger
	cmdLock    sync.Mutex

	analogs map[string]board.AnalogReader
}

// func (b *arduinoBoard) Logger() golog.Logger {
// 	return b.logger
// }

func (b *arduinoBoard) runCommand(cmd string) (string, error) {
	b.cmdLock.Lock()
	defer b.cmdLock.Unlock()

	cmd = strings.TrimSpace(cmd)
	_, err := b.port.Write([]byte(cmd + "\n"))
	if err != nil {
		return "", fmt.Errorf("error sending command to arduino: %w", err)
	}

	for {
		line, err := b.portReader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("error reading from arduino: %w", err)
		}
		line = strings.TrimSpace(strings.Trim(line, "\x00")) // TrimSpace alone doesn't remove NULs

		if len(line) == 0 {
			continue
		}

		if line[0] == '@' {
			return line[1:], nil
		}

		if line[0] == '#' {
			return "", fmt.Errorf("error from arduino: %s", line[1:])
		}

		if line[0] == '!' {
			if cmd == "!" {
				// this is the init message
				return "!", nil
			}
			continue
		}

		b.logger.Infof("got debug message from arduino: %s", line)
	}
}

func (b *arduinoBoard) configureAnalog(cfg board.AnalogConfig) error {
	var reader board.AnalogReader
	reader = &analogReader{b, cfg.Pin}
	reader = board.SmoothAnalogReader(reader, cfg, b.logger)
	b.analogs[cfg.Name] = reader
	return nil
}

func (b *arduinoBoard) resetBoard() error {
	check, err := b.runCommand("!")
	if err != nil {
		return err
	}
	if check != "!" {
		return fmt.Errorf("! (reset) didn't get expected result, got [%s]", check)
	}
	return nil
}

func (b *arduinoBoard) configure(cfg *board.Config) error {
	err := b.resetBoard()
	if err != nil {
		return err
	}

	check, err := b.runCommand("echo abc")
	if err != nil {
		return err
	}
	if check != "abc" {
		return fmt.Errorf("echo didn't get expected result, got [%s]", check)
	}

	b.analogs = map[string]board.AnalogReader{}
	for _, c := range cfg.Analogs {
		err = b.configureAnalog(c)
		if err != nil {
			return err
		}
	}

	for _, c := range cfg.DigitalInterrupts {
		return fmt.Errorf("arduino doesn't support DigitalInterrupts yet %v", c)
	}

	for _, c := range cfg.SPIs {
		return fmt.Errorf("arduino doesn't support SPI yet %v", c)
	}

	for _, c := range cfg.I2Cs {
		return fmt.Errorf("arduino doesn't support I2C yet %v", c)
	}

	return nil
}

// SPIByName returns an SPI by name.
func (b *arduinoBoard) SPIByName(name string) (board.SPI, bool) {
	return nil, false
}

// I2CByName returns an I2C by name.
func (b *arduinoBoard) I2CByName(name string) (board.I2C, bool) {
	return nil, false
}

// AnalogReaderByName returns an analog reader by name.
func (b *arduinoBoard) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	a, ok := b.analogs[name]
	return a, ok
}

// DigitalInterruptByName returns a digital interrupt by name.
func (b *arduinoBoard) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
	return nil, false
}

// GPIOSet sets the given pin to either low or high.
func (b *arduinoBoard) GPIOSet(ctx context.Context, pin string, high bool) error {
	return errors.New("GPIO not supported on arduino yet")
}

// GPIOGet returns whether the given pin is either low or high.
func (b *arduinoBoard) GPIOGet(ctx context.Context, pin string) (bool, error) {
	return false, errors.New("GPIO not supported on arduino yet")
}

// PWMSet sets the given pin to the given duty cycle.
func (b *arduinoBoard) PWMSet(ctx context.Context, pin string, dutyCycle byte) error {
	return b.pwmSetArduino(pin, dutyCycle)
}
func (b *arduinoBoard) pwmSetArduino(pin string, dutyCycle byte) error {
	cmd := fmt.Sprintf("set-pwm-duty %s %d", pin, dutyCycle)
	_, err := b.runCommand(cmd)
	if err != nil {
		return fmt.Errorf("unexpected return from PWMSet got %w", err)
	}
	return nil
}

// PWMSetFreq sets the given pin to the given PWM frequency. 0 will use the board's default PWM frequency.
func (b *arduinoBoard) PWMSetFreq(ctx context.Context, pin string, freq uint) error {
	return b.pwmSetFreqArduino(pin, freq)
}
func (b *arduinoBoard) pwmSetFreqArduino(pin string, freq uint) error {
	cmd := fmt.Sprintf("set-pwm-freq %s %d", pin, freq)
	_, err := b.runCommand(cmd)
	if err != nil {
		return fmt.Errorf("unexpected return from PWMSetFreq got %w", err)
	}
	return nil
}

// SPINames returns the name of all known SPIs.
func (b *arduinoBoard) SPINames() []string {
	return nil
}

// I2CNames returns the name of all known I2Cs.
func (b *arduinoBoard) I2CNames() []string {
	return nil
}

// AnalogReaderNames returns the name of all known analog readers.
func (b *arduinoBoard) AnalogReaderNames() []string {
	names := []string{}
	for n := range b.analogs {
		names = append(names, n)
	}
	return names
}

// DigitalInterruptNames returns the name of all known digital interrupts.
func (b *arduinoBoard) DigitalInterruptNames() []string {
	return nil
}

// Status returns the current status of the board. Usually you
// should use the CreateStatus helper instead of directly calling
// this.
func (b *arduinoBoard) Status(ctx context.Context) (*pb.BoardStatus, error) {
	return nil, errors.New("finish me")
}

// ModelAttributes returns attributes related to the model of this board.
func (b *arduinoBoard) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{}
}

// Close shuts the board down, no methods should be called on the board after this
func (b *arduinoBoard) Close() error {
	err := b.resetBoard()
	if err != nil {
		return err
	}

	return b.port.Close()
}

type analogReader struct {
	b   *arduinoBoard
	pin string
}

// Read reads off the current value.
func (ar *analogReader) Read(ctx context.Context) (int, error) {
	res, err := ar.b.runCommand("analog-read " + ar.pin)
	if err != nil {
		return 0, err
	}

	value, err := strconv.ParseInt(res, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("couldn't parse analog value (%s) : %w", res, err)
	}

	return int(value), nil
}
