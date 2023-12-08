package control

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
)

// controlBlockInternal Holds internal variables to control the flow of data between blocks.
type controlBlockInternal struct {
	mu        sync.Mutex
	blockType controlBlockType
	ins       []chan []*Signal
	outs      []chan []*Signal
	blk       Block
}

// controlTicker Used to emit impulse on blocks which do not depend on inputs or are endpoints.
type controlTicker struct {
	ticker *time.Ticker
	stop   chan bool
}

// Loop holds the loop config.
type Loop struct {
	cfg                     Config
	blocks                  map[string]*controlBlockInternal
	ct                      controlTicker
	logger                  logging.Logger
	ts                      []chan time.Time
	dt                      time.Duration
	activeBackgroundWorkers sync.WaitGroup
	cancelCtx               context.Context
	cancel                  context.CancelFunc
	running                 bool
	tuning                  bool
}

// NewLoop construct a new control loop for a specific endpoint.
func NewLoop(logger logging.Logger, cfg Config, m Controllable) (*Loop, error) {
	return createLoop(logger, cfg, m)
}

func createLoop(logger logging.Logger, cfg Config, m Controllable) (*Loop, error) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	l := Loop{
		logger:    logger,
		cfg:       cfg,
		blocks:    make(map[string]*controlBlockInternal),
		cancelCtx: cancelCtx,
		cancel:    cancel,
		running:   false,
	}
	if l.cfg.Frequency == 0.0 || l.cfg.Frequency > 200 {
		return nil, errors.New("loop frequency shouldn't be 0 or above 200Hz")
	}
	l.dt = time.Duration(float64(time.Second) * (1.0 / (l.cfg.Frequency)))
	for _, bcfg := range cfg.Blocks {
		blk, err := createBlock(bcfg, logger)
		if err != nil {
			return nil, err
		}
		l.blocks[bcfg.Name] = &controlBlockInternal{blk: blk, blockType: bcfg.Type}
		if bcfg.Type == blockEndpoint {
			l.blocks[bcfg.Name].blk.(*endpoint).ctr = m
		}
	}
	for _, b := range l.blocks {
		for _, dep := range b.blk.Config(l.cancelCtx).DependsOn {
			blockDep, ok := l.blocks[dep]
			if !ok {
				return nil, errors.Errorf("block %s depends on %s but it does not exist", b.blk.Config(l.cancelCtx).Name, dep)
			}
			blockDep.outs = append(blockDep.outs, make(chan []*Signal))
			b.ins = append(b.ins, blockDep.outs[len(blockDep.outs)-1])
		}
	}
	for _, b := range l.blocks {
		if len(b.blk.Config(l.cancelCtx).DependsOn) == 0 || b.blk.Config(l.cancelCtx).Type == blockEndpoint {
			waitCh := make(chan struct{})
			l.ts = append(l.ts, make(chan time.Time, 1))
			l.activeBackgroundWorkers.Add(1)
			utils.ManagedGo(func() {
				t := l.ts[len(l.ts)-1]
				b := b
				close(waitCh)
				for {
					_, ok := <-t
					if !ok {
						b.mu.Lock()
						for _, out := range b.outs {
							close(out)
						}
						b.outs = nil
						b.mu.Unlock()
						return
					}
					v, _ := b.blk.Next(l.cancelCtx, nil, l.dt)
					for _, out := range b.outs {
						out <- v
					}
				}
			}, l.activeBackgroundWorkers.Done)
			<-waitCh
		}
		if len(b.blk.Config(l.cancelCtx).DependsOn) != 0 {
			waitCh := make(chan struct{})
			l.activeBackgroundWorkers.Add(1)
			utils.ManagedGo(func() {
				b := b
				close(waitCh)
				for {
					sw := []*Signal{}
					s := []*Signal{}
					for _, c := range b.ins {
						r, ok := <-c
						if !ok {
							b.mu.Lock()
							for _, out := range b.outs {
								close(out)
							}
							b.outs = nil
							b.mu.Unlock()
							return
						}
						for j := 0; j < len(r); j++ {
							if r[j] != nil {
								sw = append(sw, r[j])
							}
						}
						// TODO(npmenard) do we want to support multidimentional signals?
						//nolint: makezero
					}
					if strings.Contains(b.blk.Config(l.cancelCtx).Name, "PID") {
						if strings.Contains(b.blk.Config(l.cancelCtx).Name, "ang") {
							s = append(s, sw[1])
						} else {
							s = append(s, sw[0])
						}
					} else {
						s = sw
					}

					v, ok := b.blk.Next(l.cancelCtx, s, l.dt)
					if ok {
						for _, out := range b.outs {
							out <- v
						}
					}
				}
			}, l.activeBackgroundWorkers.Done)
			<-waitCh
		}
	}
	return &l, nil
}

// OutputAt returns the Signal at the block name, error when the block doesn't exist.
func (l *Loop) OutputAt(ctx context.Context, name string) ([]*Signal, error) {
	blk, ok := l.blocks[name]
	if !ok {
		return []*Signal{}, errors.Errorf("cannot return Signals for non existing block %s", name)
	}
	return blk.blk.Output(ctx), nil
}

// ConfigAt returns the Config at the block name, error when the block doesn't exist.
func (l *Loop) ConfigAt(ctx context.Context, name string) (BlockConfig, error) {
	blk, ok := l.blocks[name]
	if !ok {
		return BlockConfig{}, errors.Errorf("cannot return Config for non existing block %s", name)
	}
	return blk.blk.Config(ctx), nil
}

// ConfigAtType returns the Config(s) at the block type, error when the block doesn't exist.
func (l *Loop) ConfigAtType(ctx context.Context, bType string) ([]BlockConfig, error) {
	var blocks []BlockConfig
	l.logger.Errorf("l.blocks = %v", l.blocks)
	for _, b := range l.blocks {
		l.logger.Errorf("b = %v", b)
		if b.blockType == controlBlockType(bType) {
			l.logger.Error("appending to blocks")
			blocks = append(blocks, b.blk.Config(ctx))
		}
	}
	if len(blocks) > 0 {
		return blocks, nil
	}
	return []BlockConfig{}, errors.Errorf("cannot return Configs for non existing block type %s", bType)
}

// SetConfigAt returns the Configl at the block name, error when the block doesn't exist.
func (l *Loop) SetConfigAt(ctx context.Context, name string, config BlockConfig) error {
	blk, ok := l.blocks[name]
	if !ok {
		return errors.Errorf("cannot return Config for non existing block %s", name)
	}
	return blk.blk.UpdateConfig(ctx, config)
}

// BlockList returns the list of blocks in a control loop error when the list is empty.
func (l *Loop) BlockList(ctx context.Context) ([]string, error) {
	var out []string
	for k := range l.blocks {
		out = append(out, k)
	}
	return out, nil
}

// Frequency returns the loop's frequency.
func (l *Loop) Frequency(ctx context.Context) (float64, error) {
	return l.cfg.Frequency, nil
}

// Start starts the loop.
func (l *Loop) Start() error {
	if len(l.ts) == 0 {
		return errors.New("cannot start the control loop if there are no blocks depending on an impulse")
	}
	l.logger.Infof("Running loop on %1.4f %+v\r\n", l.cfg.Frequency, l.dt)
	l.ct = controlTicker{
		ticker: time.NewTicker(l.dt),
		stop:   make(chan bool, 1),
	}
	waitCh := make(chan struct{})
	l.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		ct := l.ct
		ts := l.ts
		close(waitCh)
		for {
			if l.cancelCtx.Err() != nil {
				for _, c := range ts {
					close(c)
				}
				return
			}
			select {
			case t := <-ct.ticker.C:
				for _, c := range ts {
					c <- t
				}
			case <-ct.stop:
				for _, c := range ts {
					close(c)
				}
				return
			case <-l.cancelCtx.Done():
				for _, c := range ts {
					close(c)
				}
				return
			}
		}
	}, l.activeBackgroundWorkers.Done)
	<-waitCh
	l.running = true
	return nil
}

// StartBenchmark special start function to benchmark speed of complex loop configurations.
func (l *Loop) startBenchmark(loops int) error {
	if len(l.ts) == 0 {
		return errors.New("cannot start the control loop if there are no blocks depending on an impulse")
	}
	waitCh := make(chan struct{})
	l.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		ts := l.ts
		close(waitCh)
		for i := 0; i < loops; i++ {
			if l.cancelCtx.Err() != nil {
				for _, c := range ts {
					close(c)
				}
				return
			}
			for _, c := range ts {
				c <- time.Now()
			}
		}
		for _, c := range ts {
			close(c)
		}
	}, l.activeBackgroundWorkers.Done)
	<-waitCh
	return nil
}

// Stop stops then loop.
func (l *Loop) Stop() {
	if l.running {
		l.logger.Debug("closing loop")
		l.ct.ticker.Stop()
		close(l.ct.stop)
		l.activeBackgroundWorkers.Wait()
		l.running = false
	}
}

// GetConfig return the control loop config.
func (l *Loop) GetConfig(ctx context.Context) Config {
	return l.cfg
}

// GetTuning returns the current tuning value.
func (l *Loop) GetTuning(ctx context.Context) bool {
	return l.tuning
}

// SetTuning sets the tuning variable.
func (l *Loop) SetTuning(ctx context.Context, val bool) {
	l.tuning = val
}
