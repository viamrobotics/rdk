package fake

import (
	"go.viam.com/robotcore/board"
)

func init() {
	board.RegisterBoard("fake", NewBoard)
}

type fakeServo struct {
	angle uint8
}

func (s *fakeServo) Move(angle uint8) error {
	s.angle = angle
	return nil
}

func (s *fakeServo) Current() uint8 {
	return s.angle
}

type fakeAnalog struct {
	Value int
}

func (a *fakeAnalog) Read() (int, error) {
	return a.Value, nil
}

type Board struct {
	motors   map[string]*board.FakeMotor
	servos   map[string]*fakeServo
	analogs  map[string]*fakeAnalog
	digitals map[string]board.DigitalInterrupt

	cfg board.Config
}

func (b *Board) Motor(name string) board.Motor {
	return b.motors[name]
}

func (b *Board) Servo(name string) board.Servo {
	return b.servos[name]
}

func (b *Board) AnalogReader(name string) board.AnalogReader {
	return b.analogs[name]
}

func (b *Board) DigitalInterrupt(name string) board.DigitalInterrupt {
	return b.digitals[name]
}

func (b *Board) Close() error {
	return nil
}

func (b *Board) GetConfig() board.Config {
	return b.cfg
}

func (b *Board) Status() (board.Status, error) {
	return board.CreateStatus(b)
}

func NewBoard(cfg board.Config) (board.Board, error) {
	var err error

	b := &Board{
		cfg:      cfg,
		motors:   map[string]*board.FakeMotor{},
		servos:   map[string]*fakeServo{},
		analogs:  map[string]*fakeAnalog{},
		digitals: map[string]board.DigitalInterrupt{},
	}

	for _, c := range cfg.Motors {
		b.motors[c.Name] = &board.FakeMotor{}
	}

	for _, c := range cfg.Servos {
		b.servos[c.Name] = &fakeServo{}
	}

	for _, c := range cfg.Analogs {
		b.analogs[c.Name] = &fakeAnalog{}
	}

	for _, c := range cfg.DigitalInterrupts {
		b.digitals[c.Name], err = board.CreateDigitalInterrupt(c)
		if err != nil {
			return nil, err
		}
	}

	return b, nil
}
