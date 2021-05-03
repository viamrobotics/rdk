package board

import (
	"context"

	"github.com/edaniels/golog"

	pb "go.viam.com/robotcore/proto/api/v1"
)

func init() {
	RegisterBoard("fake", func(ctx context.Context, cfg Config, logger golog.Logger) (Board, error) {
		return NewFakeBoard(ctx, cfg, logger)
	})
}

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

type fakeAnalog struct {
	Value int
}

func (a *fakeAnalog) Read(context.Context) (int, error) {
	return a.Value, nil
}

type FakeBoard struct {
	Name     string
	motors   map[string]*FakeMotor
	servos   map[string]*fakeServo
	analogs  map[string]*fakeAnalog
	digitals map[string]DigitalInterrupt

	cfg        Config
	CloseCount int
}

func (b *FakeBoard) Motor(name string) Motor {
	m, ok := b.motors[name]
	if ok {
		return m
	}
	return nil
}

func (b *FakeBoard) Servo(name string) Servo {
	return b.servos[name]
}

func (b *FakeBoard) AnalogReader(name string) AnalogReader {
	return b.analogs[name]
}

func (b *FakeBoard) DigitalInterrupt(name string) DigitalInterrupt {
	return b.digitals[name]
}

func (b *FakeBoard) GetConfig(ctx context.Context) (Config, error) {
	return b.cfg, nil
}

func (b *FakeBoard) Status(ctx context.Context) (*pb.BoardStatus, error) {
	return CreateStatus(ctx, b)
}

func (b *FakeBoard) Close() error {
	b.CloseCount++
	return nil
}

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
