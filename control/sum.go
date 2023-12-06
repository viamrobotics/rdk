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
	if len(x) != len(b.operation) {
		lin := 0.0
		ang := 0.0
		for i := range x {
			op, ok := b.operation[x[i].name]
			if !ok {
				return b.y, false
			}
			switch op {
			case addition:
				if i%2 == 0 {
					lin += x[i].GetSignalValueAt(0)
				} else {
					ang += x[i].GetSignalValueAt(0)
				}
			case subtraction:
				if i%2 == 0 {
					lin -= x[i].GetSignalValueAt(0)
				} else {
					ang -= x[i].GetSignalValueAt(0)
				}
			}
		}
		b.y[0].SetSignalValueAt(0, lin)
		b.y[1].SetSignalValueAt(0, ang)
	} else {
		y := 0.0
		for i := range x {
			op, ok := b.operation[x[i].name]
			if !ok {
				return b.y, false
			}
			switch op {
			case addition:
				y += x[i].GetSignalValueAt(0)
			case subtraction:
				y -= x[i].GetSignalValueAt(0)
			default:
				return b.y, false
			}
		}
		b.y[0].SetSignalValueAt(0, y)
	}
	b.logger.Errorf("SUM NEXT (LIN) = %v for block %v", b.y[0].GetSignalValueAt(0), b.cfg.Name)
	b.logger.Errorf("SUM NEXT (ANG) = %v for block %v", b.y[1].GetSignalValueAt(0), b.cfg.Name)
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
