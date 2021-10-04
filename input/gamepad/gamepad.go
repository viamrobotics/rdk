//go:build linux

package gamepad

import (
	"context"
	"math"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"go.viam.com/core/config"
	"go.viam.com/core/input"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"github.com/kenshaw/evdev"
)

type Gamepad struct {
	dev     *evdev.Evdev
	Model   string
	Mapping Mapping
	inputs  map[input.ControlCode]*GamepadInput
	logger  golog.Logger

	callbacks map[input.ControlCode]map[input.EventType]input.ControlFunction
}

type Mapping struct {
	Buttons map[evdev.KeyType]input.ControlCode
	Axes    map[evdev.AbsoluteType]input.ControlCode
}

type GamepadInput struct {
	pad         *Gamepad
	controlCode input.ControlCode
	mu          *sync.Mutex
	lastEvent   input.Event
}

func timevaltoTime(timeVal syscall.Timeval) time.Time {
	return time.Unix(timeVal.Sec, timeVal.Usec*1000)
}

func scaleAxis(x int32, inMin int32, inMax int32, outMin float64, outMax float64) float64 {
	return float64(x-inMin)*(outMax-outMin)/float64(inMax-inMin) + outMin
}

func (g *Gamepad) eventDispatcher(ctx context.Context) {
	evChan := g.dev.Poll(ctx)
	for {
		eventIn := <-evChan
		if eventIn == nil || eventIn.Event.Type == evdev.EventMisc || (eventIn.Event.Type == evdev.EventSync && eventIn.Event.Code == 0) {
			continue
		}
		//g.logger.Debugf("%s: Type: %d, Code: %d, Value: %d\n", timevaltoTime(eventIn.Event.Time), eventIn.Event.Type, eventIn.Event.Code, eventIn.Event.Value)

		var eventOut input.Event
		if eventIn.Event.Type == evdev.EventAbsolute {
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
				Time:  timevaltoTime(eventIn.Event.Time),
				Event: input.PositionChangeAbs,
				Code:  thisAxis,
				Value: scaledPos,
			}

		} else if eventIn.Event.Type == evdev.EventKey {

			thisButton, ok := g.Mapping.Buttons[eventIn.Type.(evdev.KeyType)]
			if !ok {
				// Unmapped button
				continue
			}

			eventOut = input.Event{
				Time:  timevaltoTime(eventIn.Event.Time),
				Event: input.ButtonChange,
				Code:  thisButton,
				Value: float64(eventIn.Event.Value),
			}

			if eventIn.Event.Value == 1 {
				eventOut.Event = input.ButtonDown
			} else if eventIn.Event.Value == 0 {
				eventOut.Event = input.ButtonUp
			}

		} else {
			g.logger.Debugf("unhandled event: %+v", eventIn)
		}

		g.inputs[eventOut.Code].mu.Lock()
		g.inputs[eventOut.Code].lastEvent = eventOut
		g.inputs[eventOut.Code].mu.Unlock()

		//g.logger.Debugf("EventOut: %+v", eventOut)
		_, ok := g.callbacks[eventOut.Code]
		if !ok {
			g.callbacks[eventOut.Code] = make(map[input.EventType]input.ControlFunction)
		}
		ctrlFunc, ok := g.callbacks[eventOut.Code][eventOut.Event]
		if ok {
			go ctrlFunc(ctx, g.inputs[eventOut.Code], eventOut)
		}

		ctrlFuncAll, ok := g.callbacks[eventOut.Code][input.AllEvents]
		if ok {
			go ctrlFuncAll(ctx, g.inputs[eventOut.Code], eventOut)
		}

	}

}

func NewGamepad(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (input.Controller, error) {
	devs, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, err
	}

	pad := &Gamepad{}
	pad.logger = logger

	for _, n := range devs {
		dev, err := evdev.OpenFile(n)
		if err != nil {
			continue
		}
		name := dev.Name()
		logger.Infof("found gamepad: %s", name)
		mapping, ok := GamepadModels[name]
		if ok {
			pad.dev = dev
			pad.Model = pad.dev.Name()
			pad.Mapping = mapping
			break
		}
	}

	if pad.dev == nil {
		return nil, errors.New("no gamepad found (check /dev/input/eventXX permissions)")
	}

	pad.inputs = make(map[input.ControlCode]*GamepadInput)
	for _, v := range pad.Mapping.Axes {
		pad.inputs[v] = &GamepadInput{pad: pad, mu: &sync.Mutex{}, controlCode: v}
	}
	for _, v := range pad.Mapping.Buttons {
		pad.inputs[v] = &GamepadInput{pad: pad, mu: &sync.Mutex{}, controlCode: v}
	}

	pad.callbacks = make(map[input.ControlCode]map[input.EventType]input.ControlFunction)

	//logger.Debugf("Map: %+v", pad.Mapping)

	go pad.eventDispatcher(ctx)

	return pad, nil

}

func (g *Gamepad) Inputs(ctx context.Context) (map[input.ControlCode]input.Input, error) {
	ret := make(map[input.ControlCode]input.Input)
	for k, v := range g.inputs {
		ret[k] = v
	}
	return ret, nil
}

func (i *GamepadInput) Name(ctx context.Context) string {
	return i.controlCode.String()
}

func (i *GamepadInput) State(ctx context.Context) (input.Event, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.lastEvent, nil
}

func (i *GamepadInput) RegisterControl(ctx context.Context, ctrlFunc input.ControlFunction, trigger input.EventType) error {

	if i.pad.callbacks[i.controlCode] == nil {
		i.pad.callbacks[i.controlCode] = make(map[input.EventType]input.ControlFunction)
	}

	if trigger == input.ButtonChange {
		i.pad.callbacks[i.controlCode][input.ButtonUp] = ctrlFunc
		i.pad.callbacks[i.controlCode][input.ButtonDown] = ctrlFunc
	} else {
		i.pad.callbacks[i.controlCode][trigger] = ctrlFunc
	}
	return nil
}
