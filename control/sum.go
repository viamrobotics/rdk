package control

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

type sumOperand rune

const (
	addition    sumOperand = '+'
	subtraction sumOperand = '-'
)

type sum struct {
	mu        sync.Mutex
	cfg       BlockConfig
	y         []*Signal
	operation map[string]sumOperand
	logger    logging.Logger
}

func newSum(config BlockConfig, logger logging.Logger) (Block, error) {
	s := &sum{cfg: config, logger: logger}
	if err := s.reset(); err != nil {
		return nil, err
	}
	return s, nil
}

func (b *sum) Next(ctx context.Context, x []*Signal, dt time.Duration) ([]*Signal, bool) {
	// sum blocks only support signals with the same number of inputs
	if len(x)%2 != 0 {
		return b.y, false
	}
	y := make([]float64, len(x)/2)
	half := len(x) / 2

	// loop through the first set of inputs and add or subract from the corresponding output
	for i := 0; i < half; i++ {
		op, ok := b.operation[x[i].name]
		if !ok {
			return b.y, false
		}
		switch op {
		case addition:
			y[i] += x[i].GetSignalValueAt(0)
		case subtraction:
			y[i] -= x[i].GetSignalValueAt(0)
		}
	}

	// loop through the second set of inputs and add or subract from the corresponding output
	for i := half; i < len(x); i++ {
		op, ok := b.operation[x[i].name]
		if !ok {
			return b.y, false
		}
		switch op {
		case addition:
			y[i-half] += x[i].GetSignalValueAt(0)
		case subtraction:
			y[i-half] -= x[i].GetSignalValueAt(0)
		}
	}

	// loop through the output and set the signal
	for i := range y {
		b.y[i].SetSignalValueAt(0, y[i])
	}

	return b.y, true
}

func (b *sum) reset() error {
	if !b.cfg.Attribute.Has("sum_string") {
		return errors.Errorf("sum block %s doesn't have a sum_string", b.cfg.Name)
	}
	if len(b.cfg.DependsOn) != len(b.cfg.Attribute["sum_string"].(string)) {
		return errors.Errorf("invalid number of inputs for sum block %s expected %d got %d",
			b.cfg.Name, len(b.cfg.Attribute["sum_string"].(string)),
			len(b.cfg.DependsOn))
	}
	b.operation = make(map[string]sumOperand)
	for idx, c := range b.cfg.Attribute["sum_string"].(string) {
		if c != '+' && c != '-' {
			return errors.Errorf("expected +/- for sum block %s got %c", b.cfg.Name, c)
		}
		b.operation[b.cfg.DependsOn[idx]] = sumOperand(c)
	}
	b.y = make([]*Signal, len(b.cfg.DependsOn))
	b.y[0] = makeSignal(b.cfg.DependsOn[0])
	if len(b.operation) == 3 {
		b.y[1] = makeSignal(b.cfg.DependsOn[1])
	}
	return nil
}

func (b *sum) Reset(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reset()
}

func (b *sum) UpdateConfig(ctx context.Context, config BlockConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cfg = config
	return b.reset()
}

func (b *sum) Output(ctx context.Context) []*Signal {
	return b.y
}

func (b *sum) Config(ctx context.Context) BlockConfig {
	return b.cfg
}
