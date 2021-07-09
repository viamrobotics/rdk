package arduino

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"

	slib "github.com/jacobsa/go-serial/serial"
	"go.uber.org/multierr"

	"go.viam.com/core/board"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/serial"
)

// init registers a pi board based on pigpio.
func init() {
	board.RegisterBoard("arduino", newArduino)
}

func getSerialConfig(cfg board.Config) (slib.OpenOptions, error) {

	options := slib.OpenOptions{
		PortName:        cfg.Attributes["port"],
		BaudRate:        9600,
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

func newArduino(ctx context.Context, cfg board.Config, logger golog.Logger) (board.Board, error) {
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

	time.Sleep(1000 * time.Millisecond) // wait for startup?

	err = b.configure(cfg)
	if err != nil {
		return nil, multierr.Combine(err, b.Close())
	}
	return b, nil
}

type arduinoBoard struct {
	cfg        board.Config
	port       io.ReadWriteCloser
	portReader *bufio.Reader
	logger     golog.Logger
	cmdLock    sync.Mutex

	motors map[string]board.Motor
}

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

		line = strings.TrimSpace(line)

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

func (b *arduinoBoard) configureMotor(cfg board.MotorConfig) error {
	if cfg.Pins["pwm"] == "" || cfg.Pins["a"] == "" || cfg.Pins["b"] == "" {
		return errors.New("arduino needs a, b, and pwm pins")
	}

	if cfg.Encoder == "" || cfg.EncoderB == "" {
		return errors.New("arduino needs a and b hall encoders")
	}

	if cfg.TicksPerRotation <= 0 {
		return errors.New("arduino motors TicksPerRotation to be set")
	}

	cmd := fmt.Sprintf("config-motor-dc %s %s %s %s e %s %s",
		cfg.Name,
		cfg.Pins["pwm"],
		cfg.Pins["a"],
		cfg.Pins["b"],
		cfg.Encoder,
		cfg.EncoderB,
	)

	res, err := b.runCommand(cmd)
	if err != nil {
		return err
	}

	if res != "ok" {
		return fmt.Errorf("got unknown response when configureMotor %s", res)
	}

	m, err := board.NewEncodedMotor(cfg, &motor{b, cfg}, &encoder{b, cfg}, b.logger)
	if err != nil {
		return err
	}
	b.motors[cfg.Name] = m
	return nil
}

func (b *arduinoBoard) configure(cfg board.Config) error {

	check, err := b.runCommand("!")
	if err != nil {
		return err
	}
	if check != "!" {
		return fmt.Errorf("! didn't get expected result, got [%s]", check)
	}

	check, err = b.runCommand("echo abc")
	if err != nil {
		return err
	}
	if check != "abc" {
		return fmt.Errorf("echo didn't get expected result, got [%s]", check)
	}

	b.motors = map[string]board.Motor{}
	for _, c := range cfg.Motors {
		err = b.configureMotor(c)
		if err != nil {
			return err
		}
	}

	for _, c := range cfg.Servos {
		return fmt.Errorf("arduino doesn't support servos yet %v", c)
	}

	for _, c := range cfg.Analogs {
		return fmt.Errorf("arduino doesn't support analogs yet %v", c)
	}

	for _, c := range cfg.DigitalInterrupts {
		return fmt.Errorf("arduino doesn't support DigitalInterrupts yet %v", c)
	}

	return nil
}

func (b *arduinoBoard) Motor(name string) board.Motor {
	return b.motors[name]
}

// Servo returns a servo by name. If it does not exist
// nil is returned.
func (b *arduinoBoard) Servo(name string) board.Servo {
	return nil
}

// AnalogReader returns an analog reader by name. If it does not exist
// nil is returned.
func (b *arduinoBoard) AnalogReader(name string) board.AnalogReader {
	return nil
}

// DigitalInterrupt returns a digital interrupt by name. If it does not exist
// nil is returned.
func (b *arduinoBoard) DigitalInterrupt(name string) board.DigitalInterrupt {
	return nil
}

// GPIOSet sets the given pin to either low or high.
func (b *arduinoBoard) GPIOSet(pin string, high bool) error {
	return errors.New("GPIO not supported on arduino yet")
}

// GPIOGet returns whether the given pin is either low or high.
func (b *arduinoBoard) GPIOGet(pin string) (bool, error) {
	return false, errors.New("GPIO not supported on arduino yet")
}

// PWMSet sets the given pin to the given duty cycle.
func (b *arduinoBoard) PWMSet(pin string, dutyCycle byte) error {
	return errors.New("GPIO not supported on arduino yet")
}

// PWMSetFreq sets the given pin to the given PWM frequency. 0 will use the board's default PWM frequency.
func (b *arduinoBoard) PWMSetFreq(pin string, freq uint) error {
	return errors.New("GPIO not supported on arduino yet")
}

// MotorNames returns the name of all known motors.
func (b *arduinoBoard) MotorNames() []string {
	names := []string{}
	for n := range b.motors {
		names = append(names, n)
	}
	return names
}

// ServoNames returns the name of all known servos.
func (b *arduinoBoard) ServoNames() []string {
	return nil
}

// AnalogReaderNames returns the name of all known analog readers.
func (b *arduinoBoard) AnalogReaderNames() []string {
	return nil
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
	for _, m := range b.motors {
		err := m.Off(context.Background())
		if err != nil {
			return err
		}
	}

	// TODO(erh): actually clean up on arduino side using reset pin

	return b.port.Close()
}

type encoder struct {
	b   *arduinoBoard
	cfg board.MotorConfig
}

// Position returns the current position in terms of ticks
func (e *encoder) Position(ctx context.Context) (int64, error) {
	res, err := e.b.runCommand("motor-position " + e.cfg.Name)
	if err != nil {
		return 0, err
	}

	ticks, err := strconv.ParseInt(res, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("couldn't parse # ticks (%s) : %w", res, err)
	}

	return ticks, nil
}

// Start starts a background thread to run the encoder, if there is none needed this is a no-op
func (e *encoder) Start(cancelCtx context.Context, activeBackgroundWorkers *sync.WaitGroup, onStart func()) {
	// no-op for arduino
	onStart()
}

type motor struct {
	b   *arduinoBoard
	cfg board.MotorConfig
}

// Power sets the percentage of power the motor should employ between 0-1.
func (m *motor) Power(ctx context.Context, powerPct float32) error {
	if powerPct <= .001 {
		return m.Off(ctx)
	}

	_, err := m.b.runCommand(fmt.Sprintf("motor-power %s %d", m.cfg.Name, int(255.0*powerPct)))
	return err
}

// Go instructs the motor to go in a specific direction at a percentage
// of power between 0-1.
func (m *motor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	if powerPct <= 0 {
		return m.Off(ctx)
	}

	var dir string
	switch d {
	case pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD:
		dir = "f"
	case pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD:
		dir = "n"
	default:
		return m.Off(ctx)
	}

	_, err := m.b.runCommand(fmt.Sprintf("motor-go %s %s %d", m.cfg.Name, dir, int(255.0*powerPct)))
	return err
}

// GoFor instructs the motor to go in a specific direction for a specific amount of
// revolutions at a given speed in revolutions per minute.
func (m *motor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
	ticks := int(revolutions * float64(m.cfg.TicksPerRotation))
	ticksPerSecond := int(rpm * float64(m.cfg.TicksPerRotation) / 60.0)
	if d == pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD {
		// no-op
	} else if d == pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD {
		ticks *= -1
	} else {
		return errors.New("unknown direction")
	}

	_, err := m.b.runCommand(fmt.Sprintf("motor-gofor %s %d %d", m.cfg.Name, ticks, ticksPerSecond))
	return err
}

// Position reports the position of the motor based on its encoder. If it's not supported, the returned
// data is undefined. The unit returned is the number of revolutions which is intended to be fed
// back into calls of GoFor.
func (m *motor) Position(ctx context.Context) (float64, error) {
	res, err := m.b.runCommand("motor-position " + m.cfg.Name)
	if err != nil {
		return 0, err
	}

	ticks, err := strconv.Atoi(res)
	if err != nil {
		return 0, fmt.Errorf("couldn't parse # ticks (%s) : %w", res, err)
	}

	return float64(ticks) / float64(m.cfg.TicksPerRotation), nil
}

// PositionSupported returns whether or not the motor supports reporting of its position which
// is reliant on having an encoder.
func (m *motor) PositionSupported(ctx context.Context) (bool, error) {
	return true, nil
}

// Off turns the motor off.
func (m *motor) Off(ctx context.Context) error {
	_, err := m.b.runCommand("motor-off " + m.cfg.Name)
	return err
}

// IsOn returns whether or not the motor is currently on.
func (m *motor) IsOn(ctx context.Context) (bool, error) {
	res, err := m.b.runCommand("motor-ison " + m.cfg.Name)
	if err != nil {
		return false, err
	}
	return res[0] == 't', nil
}
