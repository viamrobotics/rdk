// Package gpio implements a gpio/adc based input.Controller.
package gpio

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bep/debounce"
	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

const modelName = "gpio"

func init() {
	registry.RegisterComponent(input.Subtype, modelName, registry.Component{Constructor: NewGPIOController})

	config.RegisterComponentAttributeMapConverter(
		input.SubtypeName,
		modelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&Config{})
}

// NewGPIOController returns a new input.Controller.
func NewGPIOController(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
	var c Controller
	c.logger = logger
	ctx, cancel := context.WithCancel(ctx)
	c.cancelFunc = cancel
	c.callbacks = make(map[input.Control]map[input.EventType]input.ControlFunction)
	c.lastEvents = make(map[input.Control]input.Event)

	cfg, ok := config.ConvertedAttributes.(*Config)
	if !ok {
		return nil, errors.New("type assertion failed on input/gpio config")
	}

	brd, err := board.FromDependencies(deps, cfg.Board)
	if err != nil {
		return nil, err
	}

	for interrupt, control := range cfg.Buttons {
		err := c.newButton(ctx, brd, interrupt, control)
		if err != nil {
			return nil, err
		}
	}

	for reader, axis := range cfg.Axes {
		err := c.newAxis(ctx, brd, reader, axis)
		if err != nil {
			return nil, err
		}
	}

	c.sendConnectionStatus(ctx, true)

	return &c, nil
}

// Config is the overall config.
type Config struct {
	Board   string                  `json:"board"`
	Buttons map[string]ButtonConfig `json:"buttons"`
	Axes    map[string]AxisConfig   `json:"axes"`
}

// AxisConfig is a subconfig for axes.
type AxisConfig struct {
	Control       input.Control `json:"control"`
	Min           int           `json:"min"`
	Max           int           `json:"max"`
	Bidirectional bool          `json:"bidirectional"`
	Deadzone      int           `json:"deadzone"`
	MinChange     int           `json:"min_change"`
	PollHz        float64       `json:"poll_hz"`
	Invert        bool          `json:"invert"`
}

// ButtonConfig is a subconfig for buttons.
type ButtonConfig struct {
	Control    input.Control `json:"control"`
	Invert     bool          `json:"invert"`
	DebounceMs int           `json:"debounce_ms"` // set to -1 to disable, default=5
}

// A Controller creates an input.Controller from DigitalInterrupts and AnalogReaders.
type Controller struct {
	mu                      sync.RWMutex
	controls                []input.Control
	lastEvents              map[input.Control]input.Event
	logger                  golog.Logger
	activeBackgroundWorkers sync.WaitGroup
	cancelFunc              func()
	callbacks               map[input.Control]map[input.EventType]input.ControlFunction
	generic.Unimplemented
}

// GetControls lists the inputs.
func (c *Controller) GetControls(ctx context.Context) ([]input.Control, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := append([]input.Control(nil), c.controls...)
	return out, nil
}

// GetEvents returns the last input.Event (the current state) of each control.
func (c *Controller) GetEvents(ctx context.Context) (map[input.Control]input.Event, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[input.Control]input.Event)
	for key, value := range c.lastEvents {
		out[key] = value
	}
	return out, nil
}

// RegisterControlCallback registers a callback function to be executed on the specified trigger Event.
func (c *Controller) RegisterControlCallback(
	ctx context.Context,
	control input.Control,
	triggers []input.EventType,
	ctrlFunc input.ControlFunction,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.callbacks[control] == nil {
		c.callbacks[control] = make(map[input.EventType]input.ControlFunction)
	}

	for _, trigger := range triggers {
		if trigger == input.ButtonChange {
			c.callbacks[control][input.ButtonRelease] = ctrlFunc
			c.callbacks[control][input.ButtonPress] = ctrlFunc
		} else {
			c.callbacks[control][trigger] = ctrlFunc
		}
	}
	return nil
}

// Close terminates background worker threads.
func (c *Controller) Close() {
	c.cancelFunc()
	c.activeBackgroundWorkers.Wait()
}

func (c *Controller) makeCallbacks(ctx context.Context, eventOut input.Event) {
	c.mu.Lock()
	c.lastEvents[eventOut.Control] = eventOut
	c.mu.Unlock()

	c.mu.RLock()
	_, ok := c.callbacks[eventOut.Control]
	c.mu.RUnlock()
	if !ok {
		c.mu.Lock()
		c.callbacks[eventOut.Control] = make(map[input.EventType]input.ControlFunction)
		c.mu.Unlock()
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	ctrlFunc, ok := c.callbacks[eventOut.Control][eventOut.Event]
	if ok && ctrlFunc != nil {
		c.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer c.activeBackgroundWorkers.Done()
			ctrlFunc(ctx, eventOut)
		})
	}

	ctrlFuncAll, ok := c.callbacks[eventOut.Control][input.AllEvents]
	if ok && ctrlFuncAll != nil {
		c.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer c.activeBackgroundWorkers.Done()
			ctrlFuncAll(ctx, eventOut)
		})
	}
}

func (c *Controller) sendConnectionStatus(ctx context.Context, connected bool) {
	evType := input.Disconnect
	now := time.Now()
	if connected {
		evType = input.Connect
	}

	for _, control := range c.controls {
		if c.lastEvents[control].Event != evType {
			eventOut := input.Event{
				Time:    now,
				Event:   evType,
				Control: control,
				Value:   0,
			}
			c.makeCallbacks(ctx, eventOut)
		}
	}
}

func (c *Controller) newButton(ctx context.Context, brd board.Board, intName string, cfg ButtonConfig) error {
	interrupt, ok := brd.DigitalInterruptByName(intName)
	if !ok {
		return fmt.Errorf("can't find DigitalInterrupt (%s)", intName)
	}
	intChan := make(chan bool)
	interrupt.AddCallback(intChan)

	if cfg.DebounceMs == 0 {
		cfg.DebounceMs = 5
	}

	c.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		debounced := debounce.New(time.Millisecond * time.Duration(cfg.DebounceMs))
		for {
			var val bool
			select {
			case <-ctx.Done():
				return
			case val = <-intChan:
			}

			if cfg.Invert {
				val = !val
			}

			evt := input.ButtonPress
			outVal := 1.0
			if !val {
				evt = input.ButtonRelease
				outVal = 0
			}

			eventOut := input.Event{
				Time:    time.Now(),
				Event:   evt,
				Control: cfg.Control,
				Value:   outVal,
			}
			if cfg.DebounceMs < 0 {
				c.makeCallbacks(ctx, eventOut)
			} else {
				debounced(func() { c.makeCallbacks(ctx, eventOut) })
			}
		}
	}, c.activeBackgroundWorkers.Done)
	c.controls = append(c.controls, cfg.Control)
	return nil
}

func (c *Controller) newAxis(ctx context.Context, brd board.Board, analogName string, cfg AxisConfig) error {
	reader, ok := brd.AnalogReaderByName(analogName)
	if !ok {
		return fmt.Errorf("can't find AnalogReader (%s)", analogName)
	}
	if cfg.MinChange < 1 {
		cfg.MinChange = 1
	}
	if cfg.PollHz <= 0 {
		cfg.PollHz = 10
	}
	if cfg.Min >= cfg.Max {
		return fmt.Errorf("min (%d) must be less than max (%d)", cfg.Min, cfg.Max)
	}
	c.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		var prevVal int
		ticker := time.NewTicker(time.Second / time.Duration(cfg.PollHz))
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			rawVal, err := reader.Read(ctx, nil)
			if err != nil {
				c.logger.Error(err)
			}

			if rawVal > cfg.Max {
				rawVal = cfg.Max
			} else if rawVal < cfg.Min {
				rawVal = cfg.Min
			}

			var outVal float64
			if cfg.Bidirectional {
				center := (cfg.Min + cfg.Max) / 2
				if abs(rawVal-center) < cfg.Deadzone {
					rawVal = center
					outVal = 0.0
				} else {
					outVal = scaleAxis(rawVal, cfg.Min, cfg.Max, -1, 1)
				}
			} else {
				if abs(rawVal-cfg.Min) < cfg.Deadzone {
					rawVal = cfg.Min
				}
				outVal = scaleAxis(rawVal, cfg.Min, cfg.Max, 0, 1)
			}

			if abs(rawVal-prevVal) < cfg.MinChange {
				continue
			}

			if cfg.Invert {
				outVal *= -1
			}

			prevVal = rawVal
			eventOut := input.Event{
				Time:    time.Now(),
				Event:   input.PositionChangeAbs,
				Control: cfg.Control,
				Value:   outVal,
			}
			c.makeCallbacks(ctx, eventOut)
		}
	}, c.activeBackgroundWorkers.Done)
	c.controls = append(c.controls, cfg.Control)
	return nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func scaleAxis(x int, inMin int, inMax int, outMin float64, outMax float64) float64 {
	return float64(x-inMin)*(outMax-outMin)/float64(inMax-inMin) + outMin
}
