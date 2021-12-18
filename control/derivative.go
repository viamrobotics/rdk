package control

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
)

type finiteDifferenceType string

const (
	backward1st1 finiteDifferenceType = "Backward1st1"
	backward1st2 finiteDifferenceType = "Backward1st2"
	backward1st3 finiteDifferenceType = "Backward1st3"
	backward2nd1 finiteDifferenceType = "Backward2nd1"
	backward2nd2 finiteDifferenceType = "Backward2nd2"
)

type derivativeStencil struct {
	Type   string
	Order  int
	Coeffs []float64
}

var backward1st1Stencil = derivativeStencil{
	Type:   "Backward",
	Order:  1,
	Coeffs: []float64{-1, 1},
}

var backward1st2Stencil = derivativeStencil{
	Type:   "Backward",
	Order:  1,
	Coeffs: []float64{0.5, -2, 1.5},
}

var backward1st3Stencil = derivativeStencil{
	Type:   "Backward",
	Order:  1,
	Coeffs: []float64{-0.3333333333, 1.5, -3, 1.83333333333},
}
var backward2nd1Stencil = derivativeStencil{
	Type:   "Backward",
	Order:  2,
	Coeffs: []float64{1, -2, 1},
}
var backward2nd2Stencil = derivativeStencil{
	Type:   "Backward",
	Order:  2,
	Coeffs: []float64{-1, 4, -5, 2},
}

type derivative struct {
	mu      sync.Mutex
	cfg     ControlBlockConfig
	stencil derivativeStencil
	x       []Signal
	y       []Signal
}

func newDerivative(config ControlBlockConfig, logger golog.Logger) (ControlBlock, error) {
	d := &derivative{cfg: config}
	err := d.reset()
	if err != nil {
		return nil, err
	}
	return d, nil
}

func derive(x []float64, dt time.Duration, stencil *derivativeStencil) (float64, error) {
	if len(x) != len(stencil.Coeffs) {
		return 0.0, errors.Errorf("expected %d inputs got %d", len(stencil.Coeffs), len(x))
	}
	y := 0.0
	for i, coeff := range stencil.Coeffs {
		y += coeff * x[i]
	}
	y /= math.Pow(dt.Seconds(), float64(stencil.Order))
	return y, nil
}

func (d *derivative) Next(ctx context.Context, x []Signal, dt time.Duration) ([]Signal, bool) {
	var err error
	if d.stencil.Type == "Backward" {
		for idx, s := range x {
			d.x[idx].signal = append(d.x[idx].signal[1:], s.signal[0])
			d.y[idx].signal[0], err = derive(d.x[idx].signal, dt, &d.stencil)
			if err != nil {
				return d.y, false
			}
		}
		return d.y, true
	}
	return d.y, false
}

func (d *derivative) reset() error {
	if !d.cfg.Attribute.Has("DeriveType") {
		return errors.Errorf("derive block %s doesn't have a DerivType field", d.cfg.Name)
	}
	if len(d.cfg.DependsOn) != 1 {
		return errors.Errorf("derive block %s only supports one input got %d", d.cfg.Name, len(d.cfg.DependsOn))
	}
	switch finiteDifferenceType(d.cfg.Attribute.String("DeriveType")) {
	case backward1st1:
		d.stencil = backward1st1Stencil
	case backward1st2:
		d.stencil = backward1st2Stencil
	case backward1st3:
		d.stencil = backward1st3Stencil
	case backward2nd1:
		d.stencil = backward2nd1Stencil
	case backward2nd2:
		d.stencil = backward2nd2Stencil
	default:
		return errors.Errorf("unsupported DeriveType %s for block %s", d.cfg.Attribute.String("DeriveType"), d.cfg.Name)
	}
	d.x = make([]Signal, len(d.cfg.DependsOn))
	d.y = make([]Signal, len(d.cfg.DependsOn))
	for i := range d.x {
		d.x[i] = makeSignal(d.cfg.Name, len(d.stencil.Coeffs))
		d.y[i] = makeSignal(d.cfg.Name, 1)
	}
	return nil
}

func (d *derivative) Configure(ctx context.Context, config ControlBlockConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cfg = config
	return d.reset()
}
func (d *derivative) UpdateConfig(ctx context.Context, config ControlBlockConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cfg = config
	return d.reset()
}

func (d *derivative) Reset(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.reset()
}

func (d *derivative) Output(ctx context.Context) []Signal {
	return d.y
}
func (d *derivative) Config(ctx context.Context) ControlBlockConfig {
	return d.cfg
}
