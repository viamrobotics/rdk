package control

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

type finiteDifferenceType string

const (
	backward1st1 finiteDifferenceType = "backward1st1"
	backward1st2 finiteDifferenceType = "backward1st2"
	backward1st3 finiteDifferenceType = "backward1st3"
	backward2nd1 finiteDifferenceType = "backward2nd1"
	backward2nd2 finiteDifferenceType = "backward2nd2"
)

type derivativeStencil struct {
	Type   string
	Order  int
	Coeffs []float64
}

var backward1st1Stencil = derivativeStencil{
	Type:   "backward",
	Order:  1,
	Coeffs: []float64{-1, 1},
}

var backward1st2Stencil = derivativeStencil{
	Type:   "backward",
	Order:  1,
	Coeffs: []float64{0.5, -2, 1.5},
}

var backward1st3Stencil = derivativeStencil{
	Type:   "backward",
	Order:  1,
	Coeffs: []float64{-0.3333333333, 1.5, -3, 1.83333333333},
}

var backward2nd1Stencil = derivativeStencil{
	Type:   "backward",
	Order:  2,
	Coeffs: []float64{1, -2, 1},
}

var backward2nd2Stencil = derivativeStencil{
	Type:   "backward",
	Order:  2,
	Coeffs: []float64{-1, 4, -5, 2},
}

type derivative struct {
	mu      sync.Mutex
	cfg     BlockConfig
	stencil derivativeStencil
	px      [][]float64
	y       []*Signal
	logger  logging.Logger
}

func newDerivative(config BlockConfig, logger logging.Logger) (Block, error) {
	d := &derivative{cfg: config, logger: logger}
	if err := d.reset(); err != nil {
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

func (d *derivative) Next(ctx context.Context, x []*Signal, dt time.Duration) ([]*Signal, bool) {
	if d.stencil.Type == "backward" {
		for idx, s := range x {
			d.px[idx] = append(d.px[idx][1:], s.GetSignalValueAt(0))
			y, err := derive(d.px[idx], dt, &d.stencil)
			d.y[idx].SetSignalValueAt(0, y)
			if err != nil {
				return d.y, false
			}
		}
		return d.y, true
	}
	return d.y, false
}

func (d *derivative) reset() error {
	if !d.cfg.Attribute.Has("derive_type") {
		return errors.Errorf("derive block %s doesn't have a derive_type field", d.cfg.Name)
	}
	if len(d.cfg.DependsOn) != 1 {
		return errors.Errorf("derive block %s only supports one input got %d", d.cfg.Name, len(d.cfg.DependsOn))
	}
	switch finiteDifferenceType(d.cfg.Attribute["derive_type"].(string)) {
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
		return errors.Errorf("unsupported derive_type %s for block %s", d.cfg.Attribute["derive_type"].(string), d.cfg.Name)
	}
	d.px = make([][]float64, len(d.cfg.DependsOn))
	d.y = make([]*Signal, len(d.cfg.DependsOn))
	for i := range d.px {
		d.px[i] = make([]float64, len(d.stencil.Coeffs))
		d.y[i] = makeSignal(d.cfg.Name)
	}
	return nil
}

func (d *derivative) UpdateConfig(ctx context.Context, config BlockConfig) error {
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

func (d *derivative) Output(ctx context.Context) []*Signal {
	return d.y
}

func (d *derivative) Config(ctx context.Context) BlockConfig {
	return d.cfg
}
