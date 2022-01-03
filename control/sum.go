package control

import (
	"context"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
)

type sumOperand rune

const (
	addition    sumOperand = '+'
	subtraction sumOperand = '-'
)

type sum struct {
	mu        sync.Mutex
	cfg       ControlBlockConfig
	y         []Signal
	operation map[string]sumOperand
	logger    golog.Logger
}

func newSum(config ControlBlockConfig, logger golog.Logger) (ControlBlock, error) {
	s := &sum{cfg: config, logger: logger}
	if err := s.reset(); err != nil {
		return nil, err
	}
	return s, nil
}

func (b *sum) Next(ctx context.Context, x []Signal, dt time.Duration) ([]Signal, bool) {
	if len(x) != len(b.operation) {
		return b.y, false
	}
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
	return b.y, true
}

func (b *sum) reset() error {
	if !b.cfg.Attribute.Has("sum_string") {
		return errors.Errorf("sum block %s doesn't have a sum_string", b.cfg.Name)
	}
	if len(b.cfg.DependsOn) != len(b.cfg.Attribute.String("sum_string")) {
		return errors.Errorf("invalid number of inputs for sum block %s expected %d got %d",
			b.cfg.Name, len(b.cfg.Attribute.String("sum_string")),
			len(b.cfg.DependsOn))
	}
	b.operation = make(map[string]sumOperand)
	for idx, c := range b.cfg.Attribute.String("sum_string") {
		if c != '+' && c != '-' {
			return errors.Errorf("expected +/- for sum block %s got %c", b.cfg.Name, c)
		}
		b.operation[b.cfg.DependsOn[idx]] = sumOperand(c)
	}
	b.y = make([]Signal, 1)
	b.y[0] = makeSignal(b.cfg.Name, 1)
	return nil
}

func (b *sum) Reset(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reset()
}

func (b *sum) UpdateConfig(ctx context.Context, config ControlBlockConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cfg = config
	return b.reset()
}

func (b *sum) Output(ctx context.Context) []Signal {
	return b.y
}

func (b *sum) Config(ctx context.Context) ControlBlockConfig {
	return b.cfg
}
