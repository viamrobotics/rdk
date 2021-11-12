package control

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/go-errors/errors"
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
}

func (s *trapezoidVelocityGenerator) Next(ctx context.Context, x []Signal, dt time.Duration) ([]Signal, bool) {
	if len(x) == 2 {
		var pos float64
		var setPoint float64
		for _, sig := range x {
			if sig.name == "SetPoint" {
				setPoint = sig.signal[0]
			} else if sig.name == "CurrentPosition" {
				pos = sig.signal[0]
			} else {
				return s.y, false
			}
		}
		//Right now we support forward direction only
		s.trapDistance = math.Abs(setPoint - pos)
		aT := s.maxVel / s.maxAcc
		if 0.5*math.Pow(aT, 2.0)*s.maxAcc > s.trapDistance {
			aT = math.Sqrt(s.trapDistance / s.maxAcc)
		}
		s.t[0] = aT
		s.t[1] = s.t[0] + (s.trapDistance-math.Pow(aT, 2.0)*s.maxAcc)/s.maxVel
		s.t[2] = s.t[1] + s.t[0]
		s.currentPhase = accelPhase
		return s.y, true
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
		s.y[0].signal[0] = s.maxAcc
		s.y[1].signal[0] += dt.Seconds() * s.maxAcc
		s.y[2].signal[0] += s.y[1].signal[0]*dt.Seconds() + 0.5*math.Pow(dt.Seconds(), 2.0)*s.maxAcc
	case steadyStatePhase:
		s.y[0].signal[0] = 0
		s.y[1].signal[0] = s.maxVel
		s.y[2].signal[0] += s.y[1].signal[0] * dt.Seconds()
	case deccelPhase:
		s.y[0].signal[0] = -s.maxAcc
		s.y[2].signal[0] += s.y[1].signal[0]*dt.Seconds() - 0.5*math.Pow(dt.Seconds(), 2.0)*s.maxAcc
		s.y[1].signal[0] -= dt.Seconds() * s.maxAcc
	default:
		return s.y, false
	}

	return s.y, true
}

func (s *trapezoidVelocityGenerator) reset(ctx context.Context) error {
	if !s.cfg.Attribute.Has("MaxAcc") {
		return errors.Errorf("trapezoidale velocity profile block %s needs MaxAcc field", s.cfg.Name)
	}
	if !s.cfg.Attribute.Has("MaxVel") {
		return errors.Errorf("trapezoidale velocity profile block %s needs MaxVel field", s.cfg.Name)
	}
	s.maxAcc = s.cfg.Attribute.Float64("MaxAcc", 0.0)
	s.maxVel = s.cfg.Attribute.Float64("MaxVel", 0.0)
	s.t = make([]float64, 3)
	s.currentPhase = rest
	s.y = make([]Signal, 3)
	s.y[0] = makeSignal("Acc", 1)
	s.y[1] = makeSignal("Vel", 1)
	s.y[2] = makeSignal("Pos", 1)
	return nil
}

func (s *trapezoidVelocityGenerator) Configure(ctx context.Context, config ControlBlockConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = config
	return s.reset(ctx)
}

func (s *trapezoidVelocityGenerator) Reset(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reset(ctx)
}

func (s *trapezoidVelocityGenerator) UpdateConfig(ctx context.Context, config ControlBlockConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = config
	return s.reset(ctx)
}
func (s *trapezoidVelocityGenerator) Output(ctx context.Context) []Signal {
	return s.y
}
func (s *trapezoidVelocityGenerator) Config(ctx context.Context) ControlBlockConfig {
	return s.cfg
}
