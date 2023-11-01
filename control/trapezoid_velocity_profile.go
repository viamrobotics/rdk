package control

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
)

const (
	rest = iota
	active
)

type trapezoidVelocityGenerator struct {
	mu           sync.Mutex
	cfg          BlockConfig
	maxAcc       float64
	maxVel       float64
	lastVelCmd   float64
	trapDistance float64
	kPP          float64 //nolint: revive
	kPP0         float64 //nolint: revive
	vDec         float64
	targetPos    float64
	y            []*Signal
	currentPhase int
	lastsetPoint float64
	dir          int
	logger       golog.Logger
	posWindow    float64
	kppGain      float64
}

func newTrapezoidVelocityProfile(config BlockConfig, logger golog.Logger) (Block, error) {
	t := &trapezoidVelocityGenerator{cfg: config, logger: logger}
	if err := t.reset(); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *trapezoidVelocityGenerator) Next(ctx context.Context, x []*Signal, dt time.Duration) ([]*Signal, bool) {
	var pos float64
	var setPoint float64
	if len(x) == 2 {
		for _, sig := range x {
			switch sig.name {
			case "set_point":
				setPoint = sig.GetSignalValueAt(0)
			case "endpoint":
				pos = sig.GetSignalValueAt(0)
			default:
				return s.y, false
			}
		}
		if setPoint != s.lastsetPoint && s.currentPhase == rest && pos != setPoint || setPoint == 0 {
			s.lastsetPoint = setPoint
			if setPoint < pos {
				s.dir = -1
			} else {
				s.dir = 1
			}
			s.trapDistance = math.Abs(setPoint - pos)
			s.lastVelCmd = 0
			s.vDec = math.Min(math.Sqrt(s.trapDistance*s.maxAcc), s.maxVel)
			s.kPP0 = 2.0 * s.maxAcc / s.vDec
			s.kPP = s.kppGain * s.kPP0
			s.targetPos = s.trapDistance*float64(s.dir) + pos
		}
	}
	var posErr float64
	if s.dir == 1 {
		posErr = s.targetPos - pos
	} else {
		posErr = pos - s.targetPos
	}
	if math.Abs(posErr) > s.posWindow {
		s.currentPhase = active
		velMfd := math.Pow(s.lastVelCmd, 2.0)*s.kPP0/(2.0*s.maxAcc) - s.lastVelCmd/s.vDec
		vel := s.kPP*posErr - velMfd
		velUp := math.Min(s.lastVelCmd+s.maxAcc*dt.Seconds(), s.maxVel)
		velDown := math.Max(s.lastVelCmd-s.maxAcc*dt.Seconds(), -s.maxVel)
		if vel > velUp {
			vel = velUp
		} else if vel < velDown {
			vel = velDown
		}
		s.lastVelCmd = vel
		s.y[0].SetSignalValueAt(0, vel*float64(s.dir))
	} else {
		s.y[0].SetSignalValueAt(0, 0.0)
		s.currentPhase = rest
	}
	return s.y, true
}

func (s *trapezoidVelocityGenerator) reset() error {
	if !s.cfg.Attribute.Has("max_acc") {
		return errors.Errorf("trapezoidale velocity profile block %s needs max_acc field", s.cfg.Name)
	}
	if !s.cfg.Attribute.Has("max_vel") {
		return errors.Errorf("trapezoidale velocity profile block %s needs max_vel field", s.cfg.Name)
	}
	s.maxAcc = s.cfg.Attribute["max_acc"].(float64) // default 0.0
	s.maxVel = s.cfg.Attribute["max_vel"].(float64) // default 0.0

	s.posWindow = 0
	if s.cfg.Attribute.Has("pos_window") {
		s.posWindow = s.cfg.Attribute["pos_window"].(float64)
	}

	s.kppGain = 0
	if s.cfg.Attribute.Has("kpp_gain") {
		s.kppGain = s.cfg.Attribute["kpp_gain"].(float64)
	}
	if s.kppGain == 0 {
		s.kppGain = 0.45
	}

	s.currentPhase = rest
	s.y = make([]*Signal, 1)
	s.y[0] = makeSignal(s.cfg.Name)
	return nil
}

func (s *trapezoidVelocityGenerator) Reset(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reset()
}

func (s *trapezoidVelocityGenerator) UpdateConfig(ctx context.Context, config BlockConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = config
	return s.reset()
}

func (s *trapezoidVelocityGenerator) Output(ctx context.Context) []*Signal {
	return s.y
}

func (s *trapezoidVelocityGenerator) Config(ctx context.Context) BlockConfig {
	return s.cfg
}
