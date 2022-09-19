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

	"github.com/edaniels/golog"
	goserial "github.com/jacobsa/go-serial/serial"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/serial"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/registry"
	rdkutils "go.viam.com/rdk/utils"
)

const modelName = "numato"

var errNoBoard = errors.New("no numato boards found")

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	Analogs    []board.AnalogConfig `json:"analogs,omitempty"`
	Attributes config.AttributeMap  `json:"attributes,omitempty"`
}

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

			return connect(ctx, conf, logger)
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

// Validate ensures all parts of the config are valid.
func (config *Config) Validate(path string) error {
	for idx, conf := range config.Analogs {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "analogs", idx)); err != nil {
			return err
		}
	}
	return nil
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
	generic.Unimplemented
	pins    int
	analogs map[string]board.AnalogReader

	port   io.ReadWriteCloser
	closed int32
	logger golog.Logger

	lines chan string
	mu    sync.Mutex

	sent   map[string]bool
	sentMu sync.Mutex
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

func (b *numatoBoard) readThread() {
	debug := true

	in := bufio.NewReader(b.port)
	for {
		line, err := in.ReadString('\n')
		if err != nil {
			if atomic.LoadInt32(&b.closed) == 1 {
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

// SPIByName returns an SPI bus by name.
func (b *numatoBoard) SPIByName(name string) (board.SPI, bool) {
	return nil, false
}

// I2CByName returns an I2C bus by name.
func (b *numatoBoard) I2CByName(name string) (board.I2C, bool) {
	return nil, false
}

// AnalogReaderByName returns an analog reader by name.
func (b *numatoBoard) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	ar, ok := b.analogs[name]
	return ar, ok
}

// DigitalInterruptByName returns a digital interrupt by name.
func (b *numatoBoard) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
	return nil, false
}

// SPINames returns the name of all known SPI busses.
func (b *numatoBoard) SPINames() []string {
	return nil
}

// I2CNames returns the name of all known I2C busses.
func (b *numatoBoard) I2CNames() []string {
	return nil
}

// AnalogReaderNames returns the name of all known analog readers.
func (b *numatoBoard) AnalogReaderNames() []string {
	names := []string{}
	for n := range b.analogs {
		names = append(names, n)
	}
	return names
}

// DigitalInterruptNames returns the name of all known digital interrupts.
func (b *numatoBoard) DigitalInterruptNames() []string {
	return nil
}

// GPIOPinNames returns the name of all known GPIO pins.
func (b *numatoBoard) GPIOPinNames() []string {
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

// Status returns the current status of the board. Usually you
// should use the CreateStatus helper instead of directly calling
// this.
func (b *numatoBoard) Status(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
	return board.CreateStatus(ctx, b, extra)
}

// ModelAttributes returns attributes related to the model of this board.
func (b *numatoBoard) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{}
}

func (b *numatoBoard) Close() error {
	atomic.AddInt32(&b.closed, 1)
	if err := b.port.Close(); err != nil {
		return err
	}
	return nil
}

type analogReader struct {
	b   *numatoBoard
	pin string
}

func (ar *analogReader) Read(ctx context.Context, extra map[string]interface{}) (int, error) {
	res, err := ar.b.doSendReceive(ctx, fmt.Sprintf("adc read %s", ar.pin))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(res)
}

func connect(ctx context.Context, conf *Config, logger golog.Logger) (board.LocalBoard, error) {
	pins := conf.Attributes.Int("pins", 0)
	if pins <= 0 {
		return nil, errors.New("numato board needs pins set in attributes")
	}

	filter := serial.SearchFilter{Type: serial.TypeNumatoGPIO}
	devs := serial.Search(filter)
	if len(devs) == 0 {
		return nil, errNoBoard
	}
	if len(devs) > 1 {
		return nil, fmt.Errorf("found more than 1 numato board: %d", len(devs))
	}

	options := goserial.OpenOptions{
		PortName:        devs[0].Path,
		BaudRate:        19200,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	device, err := goserial.Open(options)
	if err != nil {
		return nil, err
	}

	b := &numatoBoard{pins: pins, port: device, logger: logger}

	b.analogs = map[string]board.AnalogReader{}
	for _, c := range conf.Analogs {
		r := &analogReader{b, c.Pin}
		b.analogs[c.Name] = board.SmoothAnalogReader(r, c, logger)
	}

	b.lines = make(chan string)
	go b.readThread()

	ver, err := b.doSendReceive(ctx, "ver")
	if err != nil {
		return nil, multierr.Combine(b.Close(), err)
	}
	b.logger.Debugw("numato startup", "version", ver)

	return b, nil
}
