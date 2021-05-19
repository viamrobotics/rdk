package board

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/rlog"
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

func (s *fakeServo) Reconfigure(newServo Servo) {
	actual, ok := newServo.(*fakeServo)
	if !ok {
		panic(fmt.Errorf("expected new servo to be %T but got %T", actual, newServo))
	}
	*s = *actual
}

// A FakeAnalog reads back the same set value.
type FakeAnalog struct {
	Value            int
	CloseCount       int
	ReconfigureCount int
}

func (a *FakeAnalog) Read(context.Context) (int, error) {
	return a.Value, nil
}

// Close does nothing.
func (a *FakeAnalog) Close() error {
	a.CloseCount++
	return nil
}

// Reconfigure replaces this analog reader with the given analog reader.
func (a *FakeAnalog) Reconfigure(newAnalog AnalogReader) {
	actual, ok := newAnalog.(*FakeAnalog)
	if !ok {
		panic(fmt.Errorf("expected new analog to be %T but got %T", actual, newAnalog))
	}
	if err := a.Close(); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	oldCloseCount := a.CloseCount
	oldReconfigureCount := a.ReconfigureCount + 1
	*a = *actual
	a.CloseCount += oldCloseCount
	a.ReconfigureCount += oldReconfigureCount
}

// A FakeBoard provides dummy data from fake parts in order to implement a Board.
type FakeBoard struct {
	Name     string
	motors   map[string]*FakeMotor
	servos   map[string]*fakeServo
	analogs  map[string]*FakeAnalog
	digitals map[string]DigitalInterrupt

	cfg              Config
	CloseCount       int
	ReconfigureCount int
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

// Reconfigure replaces this board with the given board.
func (b *FakeBoard) Reconfigure(newBoard Board, diff ConfigDiff) {
	actual, ok := newBoard.(*FakeBoard)
	if !ok {
		panic(fmt.Errorf("expected new base to be %T but got %T", actual, newBoard))
	}

	if b.motors == nil && len(diff.Added.Motors) != 0 {
		b.motors = make(map[string]*FakeMotor, len(diff.Added.Motors))
	}
	if b.servos == nil && len(diff.Added.Servos) != 0 {
		b.servos = make(map[string]*fakeServo, len(diff.Added.Servos))
	}
	if b.analogs == nil && len(diff.Added.Analogs) != 0 {
		b.analogs = make(map[string]*FakeAnalog, len(diff.Added.Analogs))
	}
	if b.digitals == nil && len(diff.Added.DigitalInterrupts) != 0 {
		b.digitals = make(map[string]DigitalInterrupt, len(diff.Added.DigitalInterrupts))
	}

	for _, c := range diff.Added.Motors {
		b.motors[c.Name] = actual.motors[c.Name]
	}
	for _, c := range diff.Added.Servos {
		b.servos[c.Name] = actual.servos[c.Name]
	}
	for _, c := range diff.Added.Analogs {
		b.analogs[c.Name] = actual.analogs[c.Name]
	}
	for _, c := range diff.Added.DigitalInterrupts {
		b.digitals[c.Name] = actual.digitals[c.Name]
	}

	for _, c := range diff.Modified.Motors {
		b.motors[c.Name].Reconfigure(actual.motors[c.Name])
	}
	for _, c := range diff.Modified.Servos {
		b.servos[c.Name].Reconfigure(actual.servos[c.Name])
	}
	for _, c := range diff.Modified.Analogs {
		b.analogs[c.Name].Reconfigure(actual.analogs[c.Name])
	}
	for _, c := range diff.Modified.DigitalInterrupts {
		b.digitals[c.Name].Reconfigure(actual.digitals[c.Name])
	}

	for _, c := range diff.Removed.Motors {
		toRemove, ok := b.motors[c.Name]
		if !ok {
			continue // should not happen
		}
		if err := utils.TryClose(toRemove); err != nil {
			rlog.Logger.Errorw("error closing motor but still reconfiguring", "error", err)
		}
		delete(b.motors, c.Name)
	}
	for _, c := range diff.Removed.Servos {
		toRemove, ok := b.servos[c.Name]
		if !ok {
			continue // should not happen
		}
		if err := utils.TryClose(toRemove); err != nil {
			rlog.Logger.Errorw("error closing servo but still reconfiguring", "error", err)
		}
		delete(b.servos, c.Name)
	}
	for _, c := range diff.Removed.Analogs {
		toRemove, ok := b.analogs[c.Name]
		if !ok {
			continue // should not happen
		}
		if err := utils.TryClose(toRemove); err != nil {
			rlog.Logger.Errorw("error closing analog but still reconfiguring", "error", err)
		}
		delete(b.analogs, c.Name)
	}
	for _, c := range diff.Removed.DigitalInterrupts {
		toRemove, ok := b.digitals[c.Name]
		if !ok {
			continue // should not happen
		}
		if err := utils.TryClose(toRemove); err != nil {
			rlog.Logger.Errorw("error closing digital interrupt but still reconfiguring", "error", err)
		}
		delete(b.digitals, c.Name)
	}

	b.cfg = actual.cfg
	b.CloseCount += actual.CloseCount
	b.ReconfigureCount += actual.ReconfigureCount + 1
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
		analogs:  map[string]*FakeAnalog{},
		digitals: map[string]DigitalInterrupt{},
	}

	for _, c := range cfg.Motors {
		b.motors[c.Name] = &FakeMotor{mu: &sync.Mutex{}}
	}

	for _, c := range cfg.Servos {
		b.servos[c.Name] = &fakeServo{}
	}

	for _, c := range cfg.Analogs {
		b.analogs[c.Name] = &FakeAnalog{}
	}

	for _, c := range cfg.DigitalInterrupts {
		b.digitals[c.Name], err = CreateDigitalInterrupt(c)
		if err != nil {
			return nil, err
		}
	}

	return b, nil
}
