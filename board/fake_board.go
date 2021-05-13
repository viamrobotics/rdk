package board

import (
	"context"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/utils"
)

// init registers a fake board.
func init() {
	RegisterBoard("fake", func(ctx context.Context, cfg Config, logger golog.Logger) (Board, error) {
		return NewFakeBoard(ctx, cfg, logger)
	})
}

// A fakeServo allows setting and reading a single angle.
type fakeServo struct {
	angle uint8
}

func (s *fakeServo) Move(ctx context.Context, angle uint8) error {
	s.angle = angle
	return nil
}

func (s *fakeServo) Current(ctx context.Context) (uint8, error) {
	return s.angle, nil
}

// A fakeAnalog reads back the same set value.
type fakeAnalog struct {
	Value int
}

func (a *fakeAnalog) Read(context.Context) (int, error) {
	return a.Value, nil
}

// A FakeBoard provides dummy data from fake parts in order to implement a Board.
type FakeBoard struct {
	Name     string
	motors   map[string]*FakeMotor
	servos   map[string]*fakeServo
	analogs  map[string]*fakeAnalog
	digitals map[string]DigitalInterrupt

	cfg        Config
	CloseCount int
}

// Motor returns the motor by the given name if it exists.
func (b *FakeBoard) Motor(name string) Motor {
	m, ok := b.motors[name]
	if ok {
		return m
	}
	return nil
}

// Servo returns the servo by the given name if it exists.
func (b *FakeBoard) Servo(name string) Servo {
	return b.servos[name]
}

// AnalogReader returns the analog reader by the given name if it exists.
func (b *FakeBoard) AnalogReader(name string) AnalogReader {
	return b.analogs[name]
}

// DigitalInterrupt returns the interrupt by the given name if it exists.
func (b *FakeBoard) DigitalInterrupt(name string) DigitalInterrupt {
	return b.digitals[name]
}

// Config returns the config used to construct the board.
func (b *FakeBoard) Config(ctx context.Context) (Config, error) {
	return b.cfg, nil
}

// Status returns the current status of the board.
func (b *FakeBoard) Status(ctx context.Context) (*pb.BoardStatus, error) {
	return CreateStatus(ctx, b)
}

// Close attempts to cleanly close each part of the board.
func (b *FakeBoard) Close() error {
	b.CloseCount++
	var err error
	for _, motor := range b.motors {
		err = multierr.Combine(err, utils.TryClose(motor))
	}

	for _, servo := range b.servos {
		err = multierr.Combine(err, utils.TryClose(servo))
	}

	for _, analog := range b.analogs {
		err = multierr.Combine(err, utils.TryClose(analog))
	}

	for _, digital := range b.digitals {
		err = multierr.Combine(err, utils.TryClose(digital))
	}
	return err
}

// NewFakeBoard constructs a new board with fake parts based on the given Config.
func NewFakeBoard(ctx context.Context, cfg Config, logger golog.Logger) (*FakeBoard, error) {
	var err error

	b := &FakeBoard{
		Name:     cfg.Name,
		cfg:      cfg,
		motors:   map[string]*FakeMotor{},
		servos:   map[string]*fakeServo{},
		analogs:  map[string]*fakeAnalog{},
		digitals: map[string]DigitalInterrupt{},
	}

	for _, c := range cfg.Motors {
		b.motors[c.Name] = &FakeMotor{}
	}

	for _, c := range cfg.Servos {
		b.servos[c.Name] = &fakeServo{}
	}

	for _, c := range cfg.Analogs {
		b.analogs[c.Name] = &fakeAnalog{}
	}

	for _, c := range cfg.DigitalInterrupts {
		b.digitals[c.Name], err = CreateDigitalInterrupt(c)
		if err != nil {
			return nil, err
		}
	}

	return b, nil
}
