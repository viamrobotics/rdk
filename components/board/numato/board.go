// Package numato is for numato IO boards.
package numato

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	goserial "github.com/jacobsa/go-serial/serial"
	"go.uber.org/multierr"
	pb "go.viam.com/api/component/board/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/serial"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/pinwrappers"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var model = resource.DefaultModelFamily.WithModel("numato")

var errNoBoard = errors.New("no numato boards found")

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	Analogs    []AnalogConfig `json:"analogs,omitempty"`
	Pins       int            `json:"pins"`
	SerialPath string         `json:"serial_path,omitempty"`
}

// AnalogConfig describes the configuration of an analog reader on a numato board.
type AnalogConfig struct {
	Name              string `json:"name"`
	Pin               string `json:"pin"`
	AverageOverMillis int    `json:"average_over_ms,omitempty"`
	SamplesPerSecond  int    `json:"samples_per_sec,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (config *AnalogConfig) Validate(path string) error {
	if config.Name == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "name")
	}
	return nil
}

func init() {
	resource.RegisterComponent(
		board.API,
		model,
		resource.Registration[board.Board, *Config]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (board.Board, error) {
				newConf, err := resource.NativeConfig[*Config](conf)
				if err != nil {
					return nil, err
				}

				return connect(ctx, conf.ResourceName(), newConf, logger)
			},
		})
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	if conf.Pins <= 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "pins")
	}

	for idx, conf := range conf.Analogs {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "analogs", idx)); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

type mask []byte

// numato uses a weird bit mask for setting some variables on the firmware.
func newMask(bits int) mask {
	m := mask{}
	for bits >= 8 {
		m = append(m, byte(0))
		bits -= 8
	}
	if bits != 0 {
		panic(fmt.Errorf("bad number of bits %d", bits))
	}
	return m
}

func (m mask) hex() string {
	return hex.EncodeToString(m)
}

func (m *mask) set(bit int) {
	idx := len(*m) - (bit / 8) - 1
	bitToSet := bit % 8

	(*m)[idx] |= 1 << bitToSet
}

type numatoBoard struct {
	resource.Named
	resource.AlwaysRebuild
	pins    int
	analogs map[string]*pinwrappers.AnalogSmoother

	port   io.ReadWriteCloser
	closed int32
	logger logging.Logger

	lines chan string
	mu    sync.Mutex

	sent    map[string]bool
	sentMu  sync.Mutex
	workers *utils.StoppableWorkers

	maxAnalogVoltage float32
	stepSize         float32
}

func (b *numatoBoard) addToSent(msg string) {
	b.sentMu.Lock()
	defer b.sentMu.Unlock()

	if b.sent == nil {
		b.sent = make(map[string]bool)
	}
	b.sent[msg] = true
}

func (b *numatoBoard) wasSent(msg string) bool {
	b.sentMu.Lock()
	defer b.sentMu.Unlock()

	return b.sent[msg]
}

func fixPin(bit int, pin string) string {
	l := 1
	if bit >= 100 {
		l = 3
	} else if bit >= 10 {
		l = 2
	}

	for len(pin) < l {
		pin = "0" + pin
	}

	return pin
}

func (b *numatoBoard) fixPin(pin string) string {
	return fixPin(b.pins, pin)
}

func (b *numatoBoard) doSendLocked(ctx context.Context, msg string) error {
	_, err := b.port.Write(([]byte)(msg + "\n"))

	utils.SelectContextOrWait(ctx, 100*time.Microsecond)
	return err
}

func (b *numatoBoard) doSend(ctx context.Context, msg string) error {
	b.addToSent(msg)

	b.mu.Lock()
	defer b.mu.Unlock()

	return b.doSendLocked(ctx, msg)
}

func (b *numatoBoard) doSendReceive(ctx context.Context, msg string) (string, error) {
	b.addToSent(msg)

	b.mu.Lock()
	defer b.mu.Unlock()

	err := b.doSendLocked(ctx, msg)
	if err != nil {
		return "", err
	}

	select {
	case <-ctx.Done():
		return "", errors.New("context ended")
	case res := <-b.lines:
		return res, nil
	case <-time.After(1 * time.Second):
		return "", multierr.Combine(errors.New("numato read timeout"), b.port.Close())
	}
}

func (b *numatoBoard) readThread(_ context.Context) {
	debug := true

	in := bufio.NewReader(b.port)
	for {
		if atomic.LoadInt32(&b.closed) == 1 {
			close(b.lines)
			return
		}
		line, err := in.ReadString('\n')
		if err != nil {
			if atomic.LoadInt32(&b.closed) == 1 {
				close(b.lines)
				return
			}
			b.logger.Warnw("error reading", "err", err)
			break // TODO: restart connection
		}
		line = strings.TrimSpace(line)

		if debug {
			b.logger.Debugf("got line %s", line)
		}

		if len(line) == 0 || line[0] == '>' {
			continue
		}

		if b.wasSent(line) {
			continue
		}

		if debug {
			b.logger.Debugf("    sending line %s", line)
		}
		b.lines <- line
	}
}

// StreamTicks streams digital interrupt ticks.
// The numato board does not have the systems hardware to implement a Tick counter.
func (b *numatoBoard) StreamTicks(ctx context.Context, interrupts []board.DigitalInterrupt, ch chan board.Tick,
	extra map[string]interface{},
) error {
	return grpc.UnimplementedError
}

// AnalogByName returns an analog pin by name.
func (b *numatoBoard) AnalogByName(name string) (board.Analog, error) {
	ar, ok := b.analogs[name]
	if !ok {
		return nil, fmt.Errorf("can't find AnalogReader (%s)", name)
	}
	return ar, nil
}

// DigitalInterruptByName returns a digital interrupt by name.
func (b *numatoBoard) DigitalInterruptByName(name string) (board.DigitalInterrupt, error) {
	return nil, grpc.UnimplementedError
}

// AnalogNames returns the names of all known analog pins.
func (b *numatoBoard) AnalogNames() []string {
	names := []string{}
	for n := range b.analogs {
		names = append(names, n)
	}
	return names
}

// DigitalInterruptNames returns the names of all known digital interrupts.
func (b *numatoBoard) DigitalInterruptNames() []string {
	return nil
}

// GPIOPinByName returns the GPIO pin by the given name.
func (b *numatoBoard) GPIOPinByName(pin string) (board.GPIOPin, error) {
	return &gpioPin{b, pin}, nil
}

type gpioPin struct {
	b   *numatoBoard
	pin string
}

func (gp *gpioPin) Set(ctx context.Context, high bool, extra map[string]interface{}) error {
	fixedPin := gp.b.fixPin(gp.pin)
	if high {
		return gp.b.doSend(ctx, fmt.Sprintf("gpio set %s", fixedPin))
	}
	return gp.b.doSend(ctx, fmt.Sprintf("gpio clear %s", fixedPin))
}

func (gp *gpioPin) Get(ctx context.Context, extra map[string]interface{}) (bool, error) {
	fixedPin := gp.b.fixPin(gp.pin)
	res, err := gp.b.doSendReceive(ctx, fmt.Sprintf("gpio read %s", fixedPin))
	if err != nil {
		return false, err
	}
	return res[len(res)-1] == '1', nil
}

func (gp *gpioPin) PWM(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return math.NaN(), errors.New("numato doesn't support PWM")
}

func (gp *gpioPin) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
	if dutyCyclePct == 1.0 {
		return gp.Set(ctx, true, extra)
	}
	if dutyCyclePct == 0.0 {
		return gp.Set(ctx, false, extra)
	}
	return errors.New("numato doesn't support pwm")
}

func (gp *gpioPin) PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error) {
	return 0, errors.New("numato doesn't support PWMFreq")
}

func (gp *gpioPin) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
	if freqHz == 0 {
		return nil
	}
	return errors.New("numato doesn't support pwm")
}

func (b *numatoBoard) SetPowerMode(ctx context.Context, mode pb.PowerMode, duration *time.Duration) error {
	return grpc.UnimplementedError
}

func (b *numatoBoard) Close(ctx context.Context) error {
	for _, analog := range b.analogs {
		if err := analog.Close(ctx); err != nil {
			return err
		}
	}

	atomic.AddInt32(&b.closed, 1)

	// Without this line, the coroutine gets stuck in the call to in.ReadString.
	// Closing the device port will complete the call on some OSes, but on Mac attempting to close
	// a serial device currently being read from will result in a deadlock.
	// Send the board a command so we get a response back to read and complete the call so the coroutine can wake up
	// and see it should exit.
	_, err := b.doSendReceive(ctx, "ver")
	if err != nil {
		return err
	}
	if err := b.port.Close(); err != nil {
		return err
	}

	b.workers.Stop()
	return nil
}

type analog struct {
	b   *numatoBoard
	pin string
}

// Read returns the analog value with the range and step size in V/bit.
func (a *analog) Read(ctx context.Context, extra map[string]interface{}) (board.AnalogValue, error) {
	res, err := a.b.doSendReceive(ctx, fmt.Sprintf("adc read %s", a.pin))
	if err != nil {
		return board.AnalogValue{}, err
	}
	reading, err := strconv.Atoi(res)
	if err != nil {
		return board.AnalogValue{}, err
	}

	return board.AnalogValue{Value: reading, Min: 0, Max: a.b.maxAnalogVoltage, StepSize: a.b.stepSize}, nil
}

func (a *analog) Write(ctx context.Context, value int, extra map[string]interface{}) error {
	return grpc.UnimplementedError
}

func connect(ctx context.Context, name resource.Name, conf *Config, logger logging.Logger) (board.Board, error) {
	pins := conf.Pins
	var path string
	if conf.SerialPath != "" {
		path = conf.SerialPath
	} else {
		filter := serial.SearchFilter{Type: serial.TypeNumatoGPIO}
		devs := serial.Search(filter)
		if len(devs) == 0 {
			return nil, errNoBoard
		}
		if len(devs) > 1 {
			return nil, fmt.Errorf("found more than 1 numato board: %d", len(devs))
		}

		path = devs[0].Path
	}

	// Find the numato board's productid
	products := getSerialDevices()

	var productID int
	for _, product := range products {
		if product.ID.Vendor != 0x2a19 {
			continue
		}
		// we can safely get the first numato productID we find because
		// we only support one board being used at a time
		productID = product.ID.Product
		break
	}

	// Find the max analog voltage and stepSize based on the productID.
	var max float32
	var stepSize float32
	switch productID {
	case 0x800:
		// 8 and 16 pin usb versions have the same product ID but different voltage ranges
		// both have 10 bit resolution
		if conf.Pins == 8 {
			max = 5.0
		} else if conf.Pins == 16 {
			max = 3.3
		}
		stepSize = max / 1024
	case 0x802:
		// 32 channel usb numato has 10 bit resolution
		max = 3.3
		stepSize = max / 1024
	case 0x805:
		// 128 channel usb numato has 12 bit resolution
		max = 3.3
		stepSize = max / 4096
	case 0xC05:
		// 1 channel usb relay module numato - 10 bit resolution
		max = 5.0
		stepSize = max / 1024
	default:
		logger.Warnf("analog range and step size are not supported for numato with product id %d", productID)
	}

	options := goserial.OpenOptions{
		PortName:        path,
		BaudRate:        19200,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	device, err := goserial.Open(options)
	if err != nil {
		return nil, err
	}
	b := &numatoBoard{
		Named:            name.AsNamed(),
		pins:             pins,
		port:             device,
		logger:           logger,
		maxAnalogVoltage: max,
		stepSize:         stepSize,
	}

	b.analogs = map[string]*pinwrappers.AnalogSmoother{}
	for _, c := range conf.Analogs {
		r := &analog{b, c.Pin}
		b.analogs[c.Name] = numatoAnalogToSmoothAnalog(r, c, logger)
	}

	b.lines = make(chan string)

	b.workers = utils.NewBackgroundStoppableWorkers(b.readThread)

	ver, err := b.doSendReceive(ctx, "ver")
	if err != nil {
		return nil, multierr.Combine(b.Close(ctx), err)
	}
	b.logger.CDebugw(ctx, "numato startup", "version", ver)
	return b, nil
}

func numatoAnalogToSmoothAnalog(r board.Analog, c AnalogConfig, logger logging.Logger) *pinwrappers.AnalogSmoother {
	genericCfg := board.AnalogReaderConfig{
		Name:              c.Name,
		Channel:           c.Pin,
		SamplesPerSecond:  c.SamplesPerSecond,
		AverageOverMillis: c.AverageOverMillis,
	}

	return pinwrappers.SmoothAnalogReader(r, genericCfg, logger)
}
