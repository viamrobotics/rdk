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
	accelPhase
	steadyStatePhase
	deccelPhase
)

type trapezoidVelocityGenerator struct {
	mu           sync.Mutex
	cfg          ControlBlockConfig
	maxAcc       float64
	maxVel       float64
	trapDistance float64
	t            []float64
	y            []Signal
	currentPhase int
	lastsetPoint float64
	logger       golog.Logger
}

func newTrapezoidVelocityProfile(config ControlBlockConfig, logger golog.Logger) (ControlBlock, error) {
	t := &trapezoidVelocityGenerator{cfg: config, logger: logger}
	if err := t.reset(); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *trapezoidVelocityGenerator) Next(ctx context.Context, x []Signal, dt time.Duration) ([]Signal, bool) {
	if len(x) == 2 {
		var pos float64
		var setPoint float64
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
		if setPoint != s.lastsetPoint {
			s.lastsetPoint = setPoint
			// Right now we support forward direction only
			s.trapDistance = math.Abs(setPoint-pos) * 0.94
			aT := s.maxVel / s.maxAcc
			if 0.5*math.Pow(aT, 2.0)*s.maxAcc > s.trapDistance {
				aT = math.Sqrt(s.trapDistance / s.maxAcc)
			}
			s.t[0] = aT
			s.t[1] = s.t[0] + (s.trapDistance-math.Pow(aT, 2.0)*s.maxAcc)/s.maxVel
			s.t[2] = s.t[1] + s.t[0]
			s.currentPhase = accelPhase
		}
	}
	for i := range s.t {
		if s.t[i] > 0 {
			s.t[i] -= dt.Seconds()
			if s.t[i] < 0 {
				s.currentPhase++
			}
		}
	}
	if s.currentPhase > deccelPhase {
		s.currentPhase = rest
	}
	switch s.currentPhase {
	case accelPhase:
		// s.y[0].signal[0] = s.maxAcc
		s.y[0].SetSignalValueAt(0, s.y[0].GetSignalValueAt(0)+dt.Seconds()*s.maxAcc)
	//	s.y[2].signal[0] += s.y[1].signal[0]*dt.Seconds() + 0.5*math.Pow(dt.Seconds(), 2.0)*s.maxAcc
	case steadyStatePhase:
		// s.y[0].signal[0] = 0
		s.y[0].SetSignalValueAt(0, s.maxVel)
	//	s.y[2].signal[0] += s.y[1].signal[0] * dt.Seconds()
	case deccelPhase:
		// s.y[0].signal[0] = -s.maxAcc
		//	s.y[2].signal[0] += s.y[1].signal[0]*dt.Seconds() - 0.5*math.Pow(dt.Seconds(), 2.0)*s.maxAcc
		s.y[0].SetSignalValueAt(0, s.y[0].GetSignalValueAt(0)-dt.Seconds()*s.maxAcc)
	default:
		s.y[0].SetSignalValueAt(0, 0.0)
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
	s.maxAcc = s.cfg.Attribute.Float64("max_acc", 0.0)
	s.maxVel = s.cfg.Attribute.Float64("max_vel", 0.0)
	s.t = make([]float64, 3)
	s.currentPhase = rest
	s.y = make([]Signal, 1)
	s.y[0] = makeSignal(s.cfg.Name, 1)
	return nil
}

func (s *trapezoidVelocityGenerator) Reset(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reset()
}

func (s *trapezoidVelocityGenerator) UpdateConfig(ctx context.Context, config ControlBlockConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = config
	return s.reset()
}

func (s *trapezoidVelocityGenerator) Output(ctx context.Context) []Signal {
	return s.y
}

func (s *trapezoidVelocityGenerator) Config(ctx context.Context) ControlBlockConfig {
	return s.cfg
}
