package gamepad

import (
	"context"
	"syscall"
	"time"
	"path/filepath"

	"github.com/go-errors/errors"
	"github.com/edaniels/golog"
	"go.viam.com/core/config"
	"go.viam.com/core/input"
	"go.viam.com/core/robot"

	"github.com/kenshaw/evdev"

)


type Gamepad struct {
	dev *evdev.Evdev
	Model string
	Mapping Mapping
	inputs map[input.ControlCode]input.Input
	logger golog.Logger

	callbacks map[input.ControlCode]map[input.EventType]input.ControlFunction

}

type Mapping struct {
	Buttons map[evdev.KeyType]input.ControlCode
	Axes map[evdev.AbsoluteType]input.ControlCode
}

type GamepadInput struct {
	pad *Gamepad
	controlCode input.ControlCode
	lastEvent input.Event
}

func timevaltoTime(timeVal syscall.Timeval) time.Time {
	return time.Unix(timeVal.Sec, timeVal.Usec * 1000)
}


func (g *Gamepad) EventDispatcher(ctx context.Context) {
	evChan := g.dev.Poll(ctx)
	for {
		msg := <-evChan
		if msg == nil || msg.Event.Type == evdev.EventMisc {
			continue
		}
		// fmt.Printf("%s: Type: %d, Code: %d, Value: %d\n", timevaltoTime(msg.Event.Time), msg.Event.Type, msg.Event.Code, msg.Event.Value)

		if msg.Event.Type == evdev.EventAbsolute {

			thisAxis, ok := g.Mapping.Axes[msg.Type.(evdev.AbsoluteType)]
			if !ok {
				// Unmapped axis
				continue
			}

			info := g.dev.AbsoluteTypes()[msg.Type.(evdev.AbsoluteType)]

			var scaledPos float64
			// Scale triggers 0.0 to 1.0
			if thisAxis == input.AbsoluteZ || thisAxis == input.AbsoluteRZ {
				scaledPos = float64(msg.Event.Value) / float64(info.Max - info.Min)
			}else{
				scaledPos = (2 * float64(msg.Event.Value) / float64(info.Max - info.Min)) - 1
			}
			//g.logger.Debugf("Axis Info: %+v, scaled: %f\n", info, scaledPos);

			event := input.Event{
				Time: timevaltoTime(msg.Event.Time),
				Event: input.PositionChangeAbs,
				Code: thisAxis,
				Value: scaledPos,
			}


			_, ok = g.callbacks[event.Code]
			if !ok {
				g.callbacks[event.Code] = make(map[input.EventType]input.ControlFunction)
			}
			ctrlFunc, ok := g.callbacks[event.Code][event.Event]

			if ok {
				err := ctrlFunc(ctx, g.inputs[thisAxis], event)
				if err != nil {
					g.logger.Error(err)
				}
			}

		}

	}

}


func NewGamepad(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (input.Controller, error){
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


	pad.inputs = make(map[input.ControlCode]input.Input)
	for _, v := range pad.Mapping.Axes {
		pad.inputs[v] = &GamepadInput{pad: pad, controlCode: v}
	}
	for _, v := range pad.Mapping.Buttons {
		pad.inputs[v] = &GamepadInput{pad: pad, controlCode: v}
	}

	pad.callbacks = make(map[input.ControlCode]map[input.EventType]input.ControlFunction)

	//logger.Debugf("Map: %+v", pad.Mapping)

	//go pad.eventDispatcher(ctx)

	return pad, nil

}


func (g *Gamepad) Inputs(ctx context.Context) (map[input.ControlCode]input.Input, error) {
	return g.inputs, nil
}


func (i *GamepadInput) Name(ctx context.Context) string {
	return "Gamepad"
}


func (i *GamepadInput) State(ctx context.Context) (input.Event, error) {
	return i.lastEvent, nil
}

func (i *GamepadInput) RegisterControl(ctx context.Context, ctrlFunc input.ControlFunction, trigger input.EventType) error {
	if i.pad.callbacks[i.controlCode] == nil {
		i.pad.callbacks[i.controlCode] = make(map[input.EventType]input.ControlFunction)
	}
	i.pad.callbacks[i.controlCode][trigger] = ctrlFunc
	return nil
}