package board

func init() {
	RegisterBoard("fake", NewBoard)
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

type FakeBoard struct {
	motors   map[string]*FakeMotor
	servos   map[string]*fakeServo
	analogs  map[string]*fakeAnalog
	digitals map[string]DigitalInterrupt

	cfg Config
}

func (b *FakeBoard) Motor(name string) Motor {
	return b.motors[name]
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

func (b *FakeBoard) Close() error {
	return nil
}

func (b *FakeBoard) GetConfig() Config {
	return b.cfg
}

func (b *FakeBoard) Status() (Status, error) {
	return CreateStatus(b)
}

func NewFakeBoard(cfg Config) (Board, error) {
	var err error

	b := &FakeBoard{
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
