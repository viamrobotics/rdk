//go:build linux
// +build linux

// Package gamepad implements a linux gamepad as an input controller.
package gamepad

import (
	"context"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/viamrobotics/evdev"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

const defaultMapping = "Microsoft X-Box 360 pad"

var model = resource.DefaultModelFamily.WithModel("gamepad")

// Config is used for converting config attributes.
type Config struct {
	resource.TriviallyValidateConfig
	DevFile       string `json:"dev_file,omitempty"`
	AutoReconnect bool   `json:"auto_reconnect,omitempty"`
}

func init() {
	resource.RegisterComponent(input.API, model, resource.Registration[input.Controller, *Config]{
		Constructor: NewController,
	})
}

func createController(_ context.Context, name resource.Name, logger logging.Logger, devFile string, reconnect bool) input.Controller {
	ctxWithCancel, cancel := context.WithCancel(context.Background())
	g := gamepad{
		Named:      name.AsNamed(),
		logger:     logger,
		reconnect:  reconnect,
		devFile:    devFile,
		cancelFunc: cancel,
		callbacks:  map[input.Control]map[input.EventType]input.ControlFunction{},
		lastEvents: map[input.Control]input.Event{},
	}

	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer g.activeBackgroundWorkers.Done()
		for {
			if !utils.SelectContextOrWait(ctxWithCancel, 250*time.Millisecond) {
				return
			}
			err := g.connectDev(ctxWithCancel)
			if err != nil {
				if g.reconnect {
					if !strings.Contains(err.Error(), "no gamepad found") {
						g.logger.Error(err)
					}
					continue
				} else {
					g.logger.Error(err)
					return
				}
			}
			g.eventDispatcher(ctxWithCancel)
		}
	})
	return &g
}

// NewController creates a new gamepad.
func NewController(
	ctx context.Context, _ resource.Dependencies, conf resource.Config, logger logging.Logger,
) (input.Controller, error) {
	return createController(
		ctx,
		conf.ResourceName(),
		logger,
		conf.ConvertedAttributes.(*Config).DevFile,
		conf.ConvertedAttributes.(*Config).AutoReconnect,
	), nil
}

// gamepad is an input.Controller.
type gamepad struct {
	resource.Named
	resource.AlwaysRebuild
	dev                     *evdev.Evdev
	Model                   string
	Mapping                 Mapping
	controls                []input.Control
	lastEvents              map[input.Control]input.Event
	logger                  logging.Logger
	mu                      sync.RWMutex
	activeBackgroundWorkers sync.WaitGroup
	cancelFunc              func()
	callbacks               map[input.Control]map[input.EventType]input.ControlFunction
	devFile                 string
	reconnect               bool
}

// Mapping represents the evdev code to input.Control mapping for a given gamepad model.
type Mapping struct {
	Buttons map[evdev.KeyType]input.Control
	Axes    map[evdev.AbsoluteType]input.Control
}

func timevaltoTime(timeVal syscall.Timeval) time.Time {
	//nolint:unconvert
	return time.Unix(int64(timeVal.Sec), int64(timeVal.Usec*1000))
}

func scaleAxis(x, inMin, inMax int32, outMin, outMax float64) float64 {
	return float64(x-inMin)*(outMax-outMin)/float64(inMax-inMin) + outMin
}

func (g *gamepad) eventDispatcher(ctx context.Context) {
	evChan := g.dev.Poll(ctx)
	for {
		select {
		case <-ctx.Done():
			err := g.dev.Close()
			if err != nil {
				g.logger.Error(err)
			}
			return
		case eventIn := <-evChan:
			if eventIn == nil || eventIn.Event.Type == evdev.EventMisc ||
				(eventIn.Event.Type == evdev.EventSync && eventIn.Event.Code == 0) {
				continue
			}
			// Use debug line below when developing new controller mappings
			// g.logger.Debugf(
			// 	"%s: Type: %d, Code: %d, Value: %d\n",
			// 	 timevaltoTime(eventIn.Event.Time), eventIn.Event.Type, eventIn.Event.Control, eventIn.Event.Value)

			var eventOut input.Event
			switch eventIn.Event.Type {
			case evdev.EventSync:
				if evdev.SyncType(eventIn.Event.Code) == 4 {
					g.sendConnectionStatus(ctx, false)
					err := g.dev.Close()
					if err != nil {
						g.logger.Error(err)
					}
					g.dev = nil
					return
				}
				g.logger.CDebugf(ctx, "unhandled event: %+v", eventIn)

			case evdev.EventAbsolute:
				thisAxis, ok := g.Mapping.Axes[eventIn.Type.(evdev.AbsoluteType)]
				if !ok {
					// Unmapped axis
					continue
				}

				info := g.dev.AbsoluteTypes()[eventIn.Type.(evdev.AbsoluteType)]

				var scaledPos float64
				//nolint:exhaustive
				switch thisAxis {
				case input.AbsolutePedalAccelerator, input.AbsolutePedalBrake, input.AbsolutePedalClutch:
					// Scale pedals 1.0 to 0
					// We invert the values because resting state is the high value and we'd like it to be zero.
					scaledPos = 1 - scaleAxis(eventIn.Event.Value, info.Min, info.Max, 0, 1.0)
				case input.AbsoluteZ, input.AbsoluteRZ:
					// Scale triggers 0.0 to 1.0
					scaledPos = scaleAxis(eventIn.Event.Value, info.Min, info.Max, 0, 1.0)
				default:
					// Scale normal axes -1.0 to 1.0
					scaledPos = scaleAxis(eventIn.Event.Value, info.Min, info.Max, -1.0, 1.0)
				}

				// Use evDev provided deadzone
				if math.Abs(scaledPos) <= float64(info.Flat)/float64(info.Max-info.Min) {
					scaledPos = 0.0
				}

				eventOut = input.Event{
					Time:    timevaltoTime(eventIn.Event.Time),
					Event:   input.PositionChangeAbs,
					Control: thisAxis,
					Value:   scaledPos,
				}

			case evdev.EventKey:
				thisButton, ok := g.Mapping.Buttons[eventIn.Type.(evdev.KeyType)]
				if !ok {
					// Unmapped button
					continue
				}

				eventOut = input.Event{
					Time:    timevaltoTime(eventIn.Event.Time),
					Event:   input.ButtonChange,
					Control: thisButton,
					Value:   float64(eventIn.Event.Value),
				}

				switch eventIn.Event.Value {
				case 0:
					eventOut.Event = input.ButtonRelease
				case 1:
					eventOut.Event = input.ButtonPress
				case 2:
					eventOut.Event = input.ButtonHold
				}
			case evdev.EventEffect, evdev.EventEffectStatus, evdev.EventLED, evdev.EventMisc,
				evdev.EventPower, evdev.EventRelative, evdev.EventRepeat, evdev.EventSound,
				evdev.EventSwitch:
				fallthrough
			default:
				g.logger.CDebugf(ctx, "unhandled event: %+v", eventIn)
			}

			g.makeCallbacks(ctx, eventOut)
		}
	}
}

func (g *gamepad) sendConnectionStatus(ctx context.Context, connected bool) {
	evType := input.Disconnect
	now := time.Now()
	if connected {
		evType = input.Connect
	}

	for _, control := range g.controls {
		if g.lastEvents[control].Event != evType {
			eventOut := input.Event{
				Time:    now,
				Event:   evType,
				Control: control,
				Value:   0,
			}
			g.makeCallbacks(ctx, eventOut)
		}
	}
}

func (g *gamepad) makeCallbacks(ctx context.Context, eventOut input.Event) {
	g.mu.Lock()
	g.lastEvents[eventOut.Control] = eventOut
	g.mu.Unlock()

	g.mu.RLock()
	_, ok := g.callbacks[eventOut.Control]
	g.mu.RUnlock()
	if !ok {
		g.mu.Lock()
		g.callbacks[eventOut.Control] = make(map[input.EventType]input.ControlFunction)
		g.mu.Unlock()
	}
	g.mu.RLock()
	defer g.mu.RUnlock()

	ctrlFunc, ok := g.callbacks[eventOut.Control][eventOut.Event]
	if ok && ctrlFunc != nil {
		g.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer g.activeBackgroundWorkers.Done()
			ctrlFunc(ctx, eventOut)
		})
	}

	ctrlFuncAll, ok := g.callbacks[eventOut.Control][input.AllEvents]
	if ok && ctrlFuncAll != nil {
		g.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer g.activeBackgroundWorkers.Done()
			ctrlFuncAll(ctx, eventOut)
		})
	}
}

func (g *gamepad) connectDev(ctx context.Context) error {
	g.mu.Lock()
	var devs []string
	devs = []string{g.devFile}

	if len(devs) != 1 || devs[0] == "" {
		var err error
		devs, err = filepath.Glob("/dev/input/event*")
		if err != nil {
			g.mu.Unlock()
			return err
		}
	}

	for _, n := range devs {
		dev, err := evdev.OpenFile(n)
		if err != nil {
			continue
		}
		name := dev.Name()
		name = strings.TrimSpace(name)
		mapping, ok := MappingForModel(name)
		if ok {
			g.logger.CInfof(ctx, "found known gamepad: '%s' at %s", name, n)
			g.dev = dev
			g.Model = g.dev.Name()
			g.Mapping = mapping
			break
		} else {
			if err := dev.Close(); err != nil {
				return err
			}
		}
	}

	// Fallback to a default mapping
	if g.dev == nil {
		for _, n := range devs {
			dev, err := evdev.OpenFile(n)
			if err != nil {
				continue
			}
			if isGamepad(dev) {
				name := dev.Name()
				g.logger.CInfof(ctx, "found gamepad: '%s' at %s", name, n)
				g.logger.CInfof(ctx, "no button mapping for '%s', using default: '%s'", name, defaultMapping)
				g.dev = dev
				g.Model = g.dev.Name()
				g.Mapping, _ = MappingForModel(defaultMapping)
				break
			} else {
				if err := dev.Close(); err != nil {
					return err
				}
			}
		}
	}

	if g.dev == nil {
		g.mu.Unlock()
		return errors.New("no gamepad found (check /dev/input/eventXX permissions)")
	}

	for _, v := range g.Mapping.Axes {
		g.controls = append(g.controls, v)
	}
	for _, v := range g.Mapping.Buttons {
		g.controls = append(g.controls, v)
	}

	g.mu.Unlock()
	g.sendConnectionStatus(ctx, true)

	return nil
}

// Close terminates background worker threads.
func (g *gamepad) Close(ctx context.Context) error {
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()
	if g.dev != nil {
		if err := g.dev.Close(); err != nil {
			g.logger.Error(err)
		}
	}
	return nil
}

// Controls lists the inputs of the gamepad.
func (g *gamepad) Controls(ctx context.Context, extra map[string]interface{}) ([]input.Control, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.dev == nil && len(g.controls) == 0 {
		return nil, errors.New("no controller connected")
	}
	out := append([]input.Control(nil), g.controls...)
	return out, nil
}

// Events returns the last input.Event (the current state).
func (g *gamepad) Events(ctx context.Context, extra map[string]interface{}) (map[input.Control]input.Event, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make(map[input.Control]input.Event)
	for key, value := range g.lastEvents {
		out[key] = value
	}
	return out, nil
}

// RegisterControlCallback registers a callback function to be executed on the specified control's trigger Events.
func (g *gamepad) RegisterControlCallback(
	ctx context.Context,
	control input.Control,
	triggers []input.EventType,
	ctrlFunc input.ControlFunction,
	extra map[string]interface{},
) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.callbacks[control] == nil {
		g.callbacks[control] = make(map[input.EventType]input.ControlFunction)
	}

	for _, trigger := range triggers {
		if trigger == input.ButtonChange {
			g.callbacks[control][input.ButtonRelease] = ctrlFunc
			g.callbacks[control][input.ButtonPress] = ctrlFunc
		} else {
			g.callbacks[control][trigger] = ctrlFunc
		}
	}
	return nil
}

func isGamepad(dev *evdev.Evdev) bool {
	if dev.IsJoystick() {
		axes := dev.AbsoluteTypes()
		_, x := axes[evdev.AbsoluteX]
		_, y := axes[evdev.AbsoluteY]
		return x && y
	}
	return false
}
