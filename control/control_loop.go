package control

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/go-errors/errors"

	"github.com/edaniels/golog"
	"go.viam.com/utils"
)

type signalMapper struct {
	uid int32
	s   []Signal
}

// ControlBlockInternal Holds internal variables to control de flow of signals between blocks
type controlBlockInternal struct {
	mu        sync.Mutex
	blockType controlBlockType
	ins       []chan []Signal
	outs      []chan []Signal
	blk       ControlBlock
}

type controlTicker struct {
	ticker *time.Ticker
	stop   chan bool
}

// ControlLoop holds the loop config
type ControlLoop struct {
	cfg    ControlConfig
	blocks map[string]*controlBlockInternal
	ct     controlTicker
	logger golog.Logger
	ts     []chan time.Time
	dt     time.Duration
}

func createControlLoop(ctx context.Context, logger golog.Logger, cfg ControlConfig, m Controllable) (*ControlLoop, error) {
	var c ControlLoop
	c.logger = logger
	c.blocks = make(map[string]*controlBlockInternal)
	c.cfg = cfg
	c.dt = time.Duration(float64(time.Second) * (1.0 / (c.cfg.Frequency)))
	for _, bcfg := range cfg.Blocks {
		blk, err := createControlBlock(ctx, bcfg)
		if err != nil {
			return nil, err
		}
		c.blocks[bcfg.Name] = &controlBlockInternal{blk: blk, blockType: bcfg.Type}
	}
	for _, b := range c.blocks {
		for _, dep := range b.blk.Config(ctx).DependsOn {
			blockDep, ok := c.blocks[dep]
			if !ok {
				return nil, errors.Errorf("block %s depends on %s but it doesn't exists!", b.blk.Config(ctx).Name, dep)
			}
			blockDep.outs = append(blockDep.outs, make(chan []Signal))
			b.ins = append(b.ins, blockDep.outs[len(blockDep.outs)-1])
		}
	}
	for _, b := range c.blocks {
		if len(b.blk.Config(ctx).DependsOn) == 0 || b.blk.Config(ctx).Type == blockEndpoint {
			waitCh := make(chan struct{})
			c.ts = append(c.ts, make(chan time.Time))
			utils.PanicCapturingGo(func() {
				t := c.ts[len(c.ts)-1]
				b := b
				ctx := ctx
				close(waitCh)
				for d := range t {
					logger.Debugf("Impulse on BLOCK %s %+v \r\n", b.blk.Config(ctx).Name, d)
					v, _ := b.blk.Next(ctx, nil, c.dt)
					for _, out := range b.outs {
						out <- v
					}
				}
				b.mu.Lock()
				defer b.mu.Unlock()
				for _, out := range b.outs {
					close(out)
				}
				b.outs = nil
			})
			<-waitCh
		}
		if len(b.blk.Config(ctx).DependsOn) != 0 {
			waitCh := make(chan struct{})
			utils.PanicCapturingGo(func() {
				b := b
				nInputs := len(b.ins)
				i := 0
				ctx := ctx
				cases := make([]reflect.SelectCase, nInputs)
				for i, ch := range b.ins {
					cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
				}
				sw := make([]Signal, nInputs)
				close(waitCh)
				for {
					ci, s, ok := reflect.Select(cases)
					logger.Debugf("Running Block %s %+v %+v OK : %+v\r\n", b.blk.Config(ctx).Name, s.IsZero(), s, ok)
					if s.IsZero() {
						b.mu.Lock()
						for _, out := range b.outs {
							close(out)
						}
						b.outs = nil
						b.mu.Unlock()
						return
					}
					unwraped := s.Interface().([]Signal)
					if len(unwraped) == 1 {
						sw[ci] = unwraped[0]
					} else {
						sw = append(sw, unwraped...)
					}
					i++
					if i == nInputs {
						v, ok := b.blk.Next(ctx, sw, c.dt)
						logger.Debugf("Have signals for Block %s ins %+v returned %v\r\n", b.blk.Config(ctx).Name, sw, ok)
						if ok {
							for _, out := range b.outs {
								out <- v
							}
						}
						i = 0
						sw = make([]Signal, nInputs)
					}
				}
			})
			<-waitCh
		}
	}
	return &c, nil
}

// OutputAt returns the Signal at the block name, error when the block doesn't exi// OutputAt returns the Signal at the block name, error when the block doesn't exist
func (c *ControlLoop) OutputAt(ctx context.Context, name string) ([]Signal, error) {
	blk, ok := c.blocks[name]
	if !ok {
		return []Signal{}, errors.Errorf("cannot return Signals for non existing block %s", name)
	}
	return blk.blk.Output(ctx), nil
}

// ConfigAt returns the Configl at the block name, error when the block doesn't exist
func (c *ControlLoop) ConfigAt(ctx context.Context, name string) (ControlBlockConfig, error) {
	blk, ok := c.blocks[name]
	if !ok {
		return ControlBlockConfig{}, errors.Errorf("cannot return Config for non existing block %s", name)
	}
	return blk.blk.Config(ctx), nil
}

//BlockList returns the list of blocks in a control loop error when the list is empty
func (c *ControlLoop) BlockList(ctx context.Context) ([]string, error) {
	var out []string
	for k := range c.blocks {
		out = append(out, k)
	}
	return out, nil
}

//Frequency returns the loop's frequency
func (c *ControlLoop) Frequency(ctx context.Context) (float64, error) {
	return c.cfg.Frequency, nil
}

//Start starts the loop
func (c *ControlLoop) Start(ctx context.Context) error {
	if len(c.ts) == 0 {
		return errors.New("cannot start the control loop if there are no blocks depending on an impulse")
	}
	c.logger.Debugf("Running loop on %1.4f %+v\r\n", c.cfg.Frequency, c.dt)
	c.ct = controlTicker{
		ticker: time.NewTicker(c.dt),
		stop:   make(chan bool, 1),
	}
	waitCh := make(chan struct{})
	utils.PanicCapturingGo(func() {
		ct := c.ct
		ctx := ctx
		ts := c.ts
		close(waitCh)
		for {
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
			case <-ctx.Done():
				for _, c := range ts {
					close(c)
				}
				return
			}
		}
	})
	<-waitCh
	return nil
}

//Stop stops then loop
func (c *ControlLoop) Stop(ctx context.Context) error {
	c.ct.ticker.Stop()
	close(c.ct.stop)
	return nil
}
