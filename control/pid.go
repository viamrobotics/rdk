package control

import (
	"context"
	"sync"
	"time"

	"github.com/go-errors/errors"
)

// BasicPID is the standard implementation of a PID controller
type basicPID struct {
	mu    sync.Mutex
	cfg   ControlBlockConfig
	error float64
	Ki    float64
	Kd    float64
	Kp    float64
	int   float64
	sat   int
	y     []Signal
}

// Output returns the discrete step of the PID controller, dt is the delta time between two subsequent call, setPoint is the desired value, measured is the measured value. Returns false when the output is invalid (the integral is saturating) in this case continue to use the last valid value
func (p *basicPID) Next(ctx context.Context, x []Signal, dt time.Duration) ([]Signal, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	dtS := dt.Seconds()
	error := x[0].signal[0]
	if (p.sat > 0 && error > 0) || (p.sat < 0 && error < 0) {
		return p.y, false
	}
	p.int += p.Ki * p.error * dtS
	if p.int > 100 {
		p.int = 100
		p.sat = 1
	} else if p.int < 0 {
		p.int = 0
		p.sat = -1
	} else {
		p.sat = 0
	}
	deriv := (error - p.error) / dtS
	output := p.Kp*error + p.int + p.Kd*deriv
	p.error = error
	if output > 100 {
		output = 100
	} else if output < 0 {
		output = 0
	}
	p.y[0].signal[0] = output
	return p.y, true

}

func (p *basicPID) reset(ctx context.Context) error {
	p.int = 0
	p.error = 0
	p.sat = 0

	if !p.cfg.Attribute.Has("Ki") &&
		!p.cfg.Attribute.Has("Kd") &&
		!p.cfg.Attribute.Has("Kp") {
		return errors.Errorf("pid block %s should have at least one Ki, Kp or Kd field", p.cfg.Name)
	}
	if len(p.cfg.DependsOn) != 1 {
		return errors.Errorf("pid block %s should have 1 input got %d", p.cfg.Name, len(p.cfg.DependsOn))
	}
	p.Ki = p.cfg.Attribute.Float64("Ki", 0.0)
	p.Kd = p.cfg.Attribute.Float64("Kd", 0.0)
	p.Kp = p.cfg.Attribute.Float64("Kp", 0.0)
	p.y = make([]Signal, 1)
	p.y[0] = makeSignal(p.cfg.Name, 1)
	return nil
}

func (p *basicPID) Reset(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.reset(ctx)
}

func (p *basicPID) Configure(ctx context.Context, config ControlBlockConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg = config
	return p.reset(ctx)
}
func (p *basicPID) UpdateConfig(ctx context.Context, config ControlBlockConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg = config
	return p.reset(ctx)
}

func (p *basicPID) Output(ctx context.Context) []Signal {
	return p.y
}

func (p *basicPID) Config(ctx context.Context) ControlBlockConfig {
	return p.cfg
}
