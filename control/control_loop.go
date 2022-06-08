package control

import (
	"context"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"
)

// controlBlockInternal Holds internal variables to control the flow of data between blocks.
type controlBlockInternal struct {
	mu        sync.Mutex
	blockType controlBlockType
	ins       []chan []Signal
	outs      []chan []Signal
	blk       ControlBlock
}

// controlTicker Used to emit impulse on blocks which do not depend on inputs or are endpoints.
type controlTicker struct {
	ticker *time.Ticker
	stop   chan bool
}

// ControlLoop holds the loop config
// nolint: revive
type ControlLoop struct {
	cfg                     ControlConfig
	blocks                  map[string]*controlBlockInternal
	ct                      controlTicker
	logger                  golog.Logger
	ts                      []chan time.Time
	dt                      time.Duration
	activeBackgroundWorkers *sync.WaitGroup
	cancelCtx               context.Context
	cancel                  context.CancelFunc
	running                 bool
}

// NewControlLoop construct a new control loop for a specific endpoint.
func NewControlLoop(logger golog.Logger, cfg ControlConfig, m Controllable) (*ControlLoop, error) {
	return createControlLoop(logger, cfg, m)
}

func createControlLoop(logger golog.Logger, cfg ControlConfig, m Controllable) (*ControlLoop, error) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	c := ControlLoop{
		logger:                  logger,
		activeBackgroundWorkers: &sync.WaitGroup{},
		cfg:                     cfg,
		blocks:                  make(map[string]*controlBlockInternal),
		cancelCtx:               cancelCtx,
		cancel:                  cancel,
		running:                 false,
	}
	if c.cfg.Frequency == 0.0 || c.cfg.Frequency > 200 {
		return nil, errors.New("loop frequency shouldn't be 0 or above 200Hz")
	}
	c.dt = time.Duration(float64(time.Second) * (1.0 / (c.cfg.Frequency)))
	for _, bcfg := range cfg.Blocks {
		blk, err := createControlBlock(bcfg, logger)
		if err != nil {
			return nil, err
		}
		c.blocks[bcfg.Name] = &controlBlockInternal{blk: blk, blockType: bcfg.Type}
		if bcfg.Type == blockEndpoint {
			c.blocks[bcfg.Name].blk.(*endpoint).ctr = m
		}
	}
	for _, b := range c.blocks {
		for _, dep := range b.blk.Config(c.cancelCtx).DependsOn {
			blockDep, ok := c.blocks[dep]
			if !ok {
				return nil, errors.Errorf("block %s depends on %s but it does not exist", b.blk.Config(c.cancelCtx).Name, dep)
			}
			blockDep.outs = append(blockDep.outs, make(chan []Signal))
			b.ins = append(b.ins, blockDep.outs[len(blockDep.outs)-1])
		}
	}
	for _, b := range c.blocks {
		if len(b.blk.Config(c.cancelCtx).DependsOn) == 0 || b.blk.Config(c.cancelCtx).Type == blockEndpoint {
			waitCh := make(chan struct{})
			c.ts = append(c.ts, make(chan time.Time, 1))
			c.activeBackgroundWorkers.Add(1)
			utils.ManagedGo(func() {
				t := c.ts[len(c.ts)-1]
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
					v, _ := b.blk.Next(c.cancelCtx, nil, c.dt)
					for _, out := range b.outs {
						out <- v
					}
				}
			}, c.activeBackgroundWorkers.Done)
			<-waitCh
		}
		if len(b.blk.Config(c.cancelCtx).DependsOn) != 0 {
			waitCh := make(chan struct{})
			c.activeBackgroundWorkers.Add(1)
			utils.ManagedGo(func() {
				b := b
				nInputs := len(b.ins)
				close(waitCh)
				for {
					sw := make([]Signal, nInputs)
					for i, c := range b.ins {
						r, ok := <-c
						if !ok {
							b.mu.Lock()
							for _, out := range b.outs {
								close(out)
							}
							// logger.Debugf("Closing outs for block %s %+v\r\n", b.blk.Config(ctx).Name, r)
							b.outs = nil
							b.mu.Unlock()
							return
						}
						if len(r) == 1 {
							sw[i] = r[0]
						} else {
							// TODO(npmenard) do we want to support multidimentional signals?
							// nolint: makezero
							sw = append(sw, r...)
						}
					}
					v, ok := b.blk.Next(c.cancelCtx, sw, c.dt)
					if ok {
						for _, out := range b.outs {
							out <- v
						}
					}
				}
			}, c.activeBackgroundWorkers.Done)
			<-waitCh
		}
	}
	return &c, nil
}

// OutputAt returns the Signal at the block name, error when the block doesn't exist.
func (c *ControlLoop) OutputAt(ctx context.Context, name string) ([]Signal, error) {
	blk, ok := c.blocks[name]
	if !ok {
		return []Signal{}, errors.Errorf("cannot return Signals for non existing block %s", name)
	}
	return blk.blk.Output(ctx), nil
}

// ConfigAt returns the Configl at the block name, error when the block doesn't exist.
func (c *ControlLoop) ConfigAt(ctx context.Context, name string) (ControlBlockConfig, error) {
	blk, ok := c.blocks[name]
	if !ok {
		return ControlBlockConfig{}, errors.Errorf("cannot return Config for non existing block %s", name)
	}
	return blk.blk.Config(ctx), nil
}

// SetConfigAt returns the Configl at the block name, error when the block doesn't exist.
func (c *ControlLoop) SetConfigAt(ctx context.Context, name string, config ControlBlockConfig) error {
	blk, ok := c.blocks[name]
	if !ok {
		return errors.Errorf("cannot return Config for non existing block %s", name)
	}
	return blk.blk.UpdateConfig(ctx, config)
}

// BlockList returns the list of blocks in a control loop error when the list is empty.
func (c *ControlLoop) BlockList(ctx context.Context) ([]string, error) {
	var out []string
	for k := range c.blocks {
		out = append(out, k)
	}
	return out, nil
}

// Frequency returns the loop's frequency.
func (c *ControlLoop) Frequency(ctx context.Context) (float64, error) {
	return c.cfg.Frequency, nil
}

// Start starts the loop.
func (c *ControlLoop) Start() error {
	if len(c.ts) == 0 {
		return errors.New("cannot start the control loop if there are no blocks depending on an impulse")
	}
	c.logger.Debugf("Running loop on %1.4f %+v\r\n", c.cfg.Frequency, c.dt)
	c.ct = controlTicker{
		ticker: time.NewTicker(c.dt),
		stop:   make(chan bool, 1),
	}
	waitCh := make(chan struct{})
	c.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		ct := c.ct
		ts := c.ts
		close(waitCh)
		for {
			if c.cancelCtx.Err() != nil {
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
			case <-c.cancelCtx.Done():
				for _, c := range ts {
					close(c)
				}
				return
			}
		}
	}, c.activeBackgroundWorkers.Done)
	<-waitCh
	c.running = true
	return nil
}

// StartBenchmark special start function to benchmark speed of complex loop configurations.
func (c *ControlLoop) startBenchmark(loops int) error {
	if len(c.ts) == 0 {
		return errors.New("cannot start the control loop if there are no blocks depending on an impulse")
	}
	waitCh := make(chan struct{})
	c.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		ts := c.ts
		close(waitCh)
		for i := 0; i < loops; i++ {
			if c.cancelCtx.Err() != nil {
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
	}, c.activeBackgroundWorkers.Done)
	<-waitCh
	return nil
}

// Stop stops then loop.
func (c *ControlLoop) Stop() {
	if c.running {
		c.ct.ticker.Stop()
		close(c.ct.stop)
		c.activeBackgroundWorkers.Wait()
		c.running = false
	}
}

// GetConfig return the control loop config.
func (c *ControlLoop) GetConfig(ctx context.Context) ControlConfig {
	return c.cfg
}
