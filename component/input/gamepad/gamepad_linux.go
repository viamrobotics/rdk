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

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"github.com/viamrobotics/evdev"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

const (
	modelname      = "gamepad"
	defaultMapping = "Microsoft X-Box 360 pad"
)

// Config is used for converting config attributes.
type Config struct {
	DevFile       string `json:"dev_file"`
	AutoReconnect bool   `json:"auto_reconnect"`
}

func init() {
	registry.RegisterComponent(input.Subtype, modelname, registry.Component{Constructor: NewController})

	config.RegisterComponentAttributeMapConverter(
		input.SubtypeName,
		modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&Config{})
}

func createController(ctx context.Context, logger golog.Logger, devFile string, reconnect bool) input.Controller {
	var g gamepad
	g.logger = logger
	g.reconnect = reconnect
	ctxWithCancel, cancel := context.WithCancel(ctx)
	g.cancelFunc = cancel
	g.devFile = devFile
	g.callbacks = make(map[input.Control]map[input.EventType]input.ControlFunction)
	g.lastEvents = make(map[input.Control]input.Event)

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
					g.logger.Fatal(err)
					return
				}
			}
			g.eventDispatcher(ctxWithCancel)
		}
	})
	return &g
}

// NewController creates a new gamepad.
func NewController(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
	return createController(ctx, logger, config.ConvertedAttributes.(*Config).DevFile, config.ConvertedAttributes.(*Config).AutoReconnect), nil
}

// gamepad is an input.Controller.
type gamepad struct {
	dev                     *evdev.Evdev
	Model                   string
	Mapping                 Mapping
	controls                []input.Control
	lastEvents              map[input.Control]input.Event
	logger                  golog.Logger
	mu                      sync.RWMutex
	activeBackgroundWorkers sync.WaitGroup
	cancelFunc              func()
	callbacks               map[input.Control]map[input.EventType]input.ControlFunction
	devFile                 string
	reconnect               bool
	generic.Unimplemented
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

func scaleAxis(x int32, inMin int32, inMax int32, outMin float64, outMax float64) float64 {
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
			if eventIn == nil || eventIn.Event.Type == evdev.EventMisc || (eventIn.Event.Type == evdev.EventSync && eventIn.Event.Code == 0) {
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
				g.logger.Debugf("unhandled event: %+v", eventIn)

			case evdev.EventAbsolute:
				thisAxis, ok := g.Mapping.Axes[eventIn.Type.(evdev.AbsoluteType)]
				if !ok {
					// Unmapped axis
					continue
				}

				info := g.dev.AbsoluteTypes()[eventIn.Type.(evdev.AbsoluteType)]

				var scaledPos float64
				if thisAxis == input.AbsoluteZ || thisAxis == input.AbsoluteRZ {
					// Scale triggers 0.0 to 1.0
					scaledPos = scaleAxis(eventIn.Event.Value, info.Min, info.Max, 0, 1.0)
				} else {
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

				if eventIn.Event.Value == 1 {
					eventOut.Event = input.ButtonPress
				} else if eventIn.Event.Value == 0 {
					eventOut.Event = input.ButtonRelease
				} else if eventIn.Event.Value == 2 {
					eventOut.Event = input.ButtonHold
				}
			case evdev.EventEffect, evdev.EventEffectStatus, evdev.EventLED, evdev.EventMisc,
				evdev.EventPower, evdev.EventRelative, evdev.EventRepeat, evdev.EventSound,
				evdev.EventSwitch:
				fallthrough
			default:
				g.logger.Debugf("unhandled event: %+v", eventIn)
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
		mapping, ok := GamepadMappings[name]
		if ok {
			g.logger.Infof("found known gamepad: '%s' at %s", name, n)
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
				g.logger.Infof("found gamepad: '%s' at %s", name, n)
				g.logger.Infof("no button mapping for '%s', using default: '%s'", name, defaultMapping)
				g.dev = dev
				g.Model = g.dev.Name()
				g.Mapping = GamepadMappings[defaultMapping]
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
func (g *gamepad) Close() {
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()
	if g.dev != nil {
		if err := g.dev.Close(); err != nil {
			g.logger.Error(err)
		}
	}
}

// GetControls lists the inputs of the gamepad.
func (g *gamepad) GetControls(ctx context.Context) ([]input.Control, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.dev == nil && len(g.controls) == 0 {
		return nil, errors.New("no controller connected")
	}
	out := append([]input.Control(nil), g.controls...)
	return out, nil
}

// GetEvents returns the last input.Event (the current state).
func (g *gamepad) GetEvents(ctx context.Context) (map[input.Control]input.Event, error) {
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
