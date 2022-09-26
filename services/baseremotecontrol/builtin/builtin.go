// Package builtin implements a remote control for a base.
package builtin

import (
	"context"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/baseremotecontrol"
	"go.viam.com/rdk/utils"
)

// Constants for the system including the max speed and angle (TBD: allow to be set as config vars)
// as well as the various control modes including oneJoystick (control via a joystick), triggerSpeed
// (triggers control speed and joystick angle), button (four buttons X, Y, A, B to  control speed and
// angle) and arrow (arrows buttons used to control speed and angle).
const (
	joyStickControl = controlMode(iota)
	triggerSpeedControl
	buttonControl
	arrowControl
	droneControl
	SubtypeName = resource.SubtypeName("base_remote_control")
)

func init() {
	registry.RegisterService(baseremotecontrol.Subtype, resource.DefaultModelName, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return NewBuiltIn(ctx, r, c, logger)
		},
	})
	cType := config.ServiceType(SubtypeName)
	config.RegisterServiceAttributeMapConverter(cType, func(attributes config.AttributeMap) (interface{}, error) {
		var conf Config
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &conf})
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(attributes); err != nil {
			return nil, err
		}
		return &conf, nil
	}, &Config{})
}

// ControlMode is the control type for the remote control.
type controlMode uint8

// Config describes how to configure the service.
type Config struct {
	BaseName            string  `json:"base"`
	InputControllerName string  `json:"input_controller"`
	ControlModeName     string  `json:"control_mode"`
	MaxAngularVelocity  float64 `json:"max_angular_deg_per_sec"`
	MaxLinearVelocity   float64 `json:"max_linear_mm_per_sec"`
}

// builtIn is the structure of the remote service.
type builtIn struct {
	base            base.Base
	inputController input.Controller
	controlMode     controlMode
	closed          bool

	config *Config
	logger golog.Logger
}

// NewDefault returns a new remote control service for the given robot.
func NewBuiltIn(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (baseremotecontrol.Service, error) {
	svcConfig, ok := config.ConvertedAttributes.(*Config)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(svcConfig, config.ConvertedAttributes)
	}
	base1, err := base.FromRobot(r, svcConfig.BaseName)
	if err != nil {
		return nil, err
	}
	controller, err := input.FromRobot(r, svcConfig.InputControllerName)
	if err != nil {
		return nil, err
	}

	var controlMode1 controlMode
	switch svcConfig.ControlModeName {
	case "triggerSpeedControl":
		controlMode1 = triggerSpeedControl
	case "buttonControl":
		controlMode1 = buttonControl
	case "joystickControl":
		controlMode1 = joyStickControl
	case "droneControl":
		controlMode1 = droneControl
	default:
		controlMode1 = arrowControl
	}

	remoteSvc := &builtIn{
		base:            base1,
		inputController: controller,
		controlMode:     controlMode1,
		config:          svcConfig,
		logger:          logger,
	}

	if err := remoteSvc.start(ctx); err != nil {
		return nil, errors.Errorf("error with starting remote control service: %q", err)
	}

	return remoteSvc, nil
}

// Start is the main control loops for sending events from controller to base.
func (svc *builtIn) start(ctx context.Context) error {
	state := &throttleState{}
	state.init()

	var lastEvent input.Event
	var onlyOneAtATime sync.Mutex

	remoteCtl := func(ctx context.Context, event input.Event) {
		onlyOneAtATime.Lock()
		defer onlyOneAtATime.Unlock()

		if svc.closed {
			return
		}

		if event.Time.Before(lastEvent.Time) {
			return
		}
		lastEvent = event

		err := svc.processEvent(ctx, state, event)
		if err != nil {
			svc.logger.Errorw("error with moving base to desired position", "error", err)
		}
	}

	for _, control := range svc.ControllerInputs() {
		var err error
		if svc.controlMode == buttonControl {
			err = svc.inputController.RegisterControlCallback(ctx, control, []input.EventType{input.ButtonChange}, remoteCtl)
		} else {
			err = svc.inputController.RegisterControlCallback(ctx, control, []input.EventType{input.PositionChangeAbs}, remoteCtl)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Close out of all remote control related systems.
func (svc *builtIn) Close(ctx context.Context) error {
	svc.closed = true
	return nil
}

// ControllerInputs returns the list of inputs from the controller that are being monitored for that control mode.
func (svc *builtIn) ControllerInputs() []input.Control {
	switch svc.controlMode {
	case triggerSpeedControl:
		return []input.Control{input.AbsoluteX, input.AbsoluteZ, input.AbsoluteRZ}
	case arrowControl:
		return []input.Control{input.AbsoluteHat0X, input.AbsoluteHat0Y}
	case buttonControl:
		return []input.Control{input.ButtonNorth, input.ButtonSouth, input.ButtonEast, input.ButtonWest}
	case joyStickControl:
		return []input.Control{input.AbsoluteX, input.AbsoluteY}
	case droneControl:
		return []input.Control{input.AbsoluteX, input.AbsoluteY, input.AbsoluteRX, input.AbsoluteRY}
	}
	return []input.Control{}
}

func (svc *builtIn) processEvent(ctx context.Context, state *throttleState, event input.Event) error {
	newLinear, newAngular := parseEvent(svc.controlMode, state, event)

	if similar(newLinear, state.linearThrottle, .05) && similar(newAngular, state.angularThrottle, .05) {
		return nil
	}

	if svc.config.MaxAngularVelocity > 0 && svc.config.MaxLinearVelocity > 0 {
		if err := svc.base.SetVelocity(
			ctx,
			r3.Vector{
				X: svc.config.MaxLinearVelocity * newLinear.X,
				Y: svc.config.MaxLinearVelocity * newLinear.Y,
				Z: svc.config.MaxLinearVelocity * newLinear.Z,
			},
			r3.Vector{
				X: svc.config.MaxAngularVelocity * newAngular.X,
				Y: svc.config.MaxAngularVelocity * newAngular.Y,
				Z: svc.config.MaxAngularVelocity * newAngular.Z,
			},
			nil,
		); err != nil {
			return err
		}
	} else {
		if err := svc.base.SetPower(ctx, newLinear, newAngular, nil); err != nil {
			return err
		}
	}

	state.linearThrottle = newLinear
	state.angularThrottle = newAngular
	return nil
}

// triggerSpeedEvent takes inputs from the gamepad allowing the triggers to control speed and the left joystick to
// control the angle.
func triggerSpeedEvent(event input.Event, speed, angle float64) (float64, float64) {
	switch event.Control {
	case input.AbsoluteZ:
		speed -= 0.05
		speed = math.Max(-1, speed)
	case input.AbsoluteRZ:
		speed += 0.05
		speed = math.Min(1, speed)
	case input.AbsoluteX:
		angle = event.Value
	case input.AbsoluteHat0X, input.AbsoluteHat0Y, input.AbsoluteRX, input.AbsoluteRY, input.AbsoluteY,
		input.ButtonEStop, input.ButtonEast, input.ButtonLT, input.ButtonLT2, input.ButtonLThumb, input.ButtonMenu,
		input.ButtonNorth, input.ButtonRT, input.ButtonRT2, input.ButtonRThumb, input.ButtonRecord,
		input.ButtonSelect, input.ButtonSouth, input.ButtonStart, input.ButtonWest:
		fallthrough
	default:
	}

	return speed, angle
}

// buttonControlEvent takes inputs from the gamepad allowing the X and B buttons to control speed and Y and A buttons to control angle.
func buttonControlEvent(event input.Event, buttons map[input.Control]bool) (float64, float64, map[input.Control]bool) {
	var speed float64
	var angle float64

	switch event.Event {
	case input.ButtonPress:
		buttons[event.Control] = true
	case input.ButtonRelease:
		buttons[event.Control] = false
	case input.AllEvents, input.ButtonChange, input.ButtonHold, input.Connect, input.Disconnect,
		input.PositionChangeAbs, input.PositionChangeRel:
		fallthrough
	default:
	}

	if buttons[input.ButtonNorth] == buttons[input.ButtonSouth] {
		speed = 0.0
	} else {
		if buttons[input.ButtonNorth] {
			speed = 1.0
		} else {
			speed = -1.0
		}
	}

	if buttons[input.ButtonEast] == buttons[input.ButtonWest] {
		angle = 0.0
	} else {
		if buttons[input.ButtonEast] {
			angle = -1.0
		} else {
			angle = 1.0
		}
	}

	return speed, angle, buttons
}

// arrowControlEvent takes inputs from the gamepad allowing the arrow buttons to control speed and angle.
func arrowEvent(event input.Event, arrows map[input.Control]float64) (float64, float64, map[input.Control]float64) {
	arrows[event.Control] = -1.0 * event.Value

	speed := arrows[input.AbsoluteHat0Y]
	angle := arrows[input.AbsoluteHat0X]

	return speed, angle, arrows
}

// oneJoyStickEvent (default) takes inputs from the gamepad allowing the left joystick to control speed and angle.
func oneJoyStickEvent(event input.Event, y, x float64) (float64, float64) {
	switch event.Control {
	case input.AbsoluteY:
		y = -1.0 * event.Value
	case input.AbsoluteX:
		x = -1.0 * event.Value
	case input.AbsoluteHat0X, input.AbsoluteHat0Y, input.AbsoluteRX, input.AbsoluteRY, input.AbsoluteRZ,
		input.AbsoluteZ, input.ButtonEStop, input.ButtonEast, input.ButtonLT, input.ButtonLT2, input.ButtonLThumb,
		input.ButtonMenu, input.ButtonNorth, input.ButtonRT, input.ButtonRT2, input.ButtonRThumb,
		input.ButtonRecord, input.ButtonSelect, input.ButtonSouth, input.ButtonStart, input.ButtonWest:
		fallthrough
	default:
	}

	return scaleThrottle(y), scaleThrottle(x)
}

// right joystick is forward/back, strafe right/left
// left joystick is spin right/left & up/down.
func droneEvent(event input.Event, linear, angular r3.Vector) (r3.Vector, r3.Vector) {
	switch event.Control {
	case input.AbsoluteX:
		angular.Z = scaleThrottle(-1.0 * event.Value)
	case input.AbsoluteY:
		linear.Z = scaleThrottle(-1.0 * event.Value)
	case input.AbsoluteRX:
		linear.X = scaleThrottle(event.Value)
	case input.AbsoluteRY:
		linear.Y = scaleThrottle(-1.0 * event.Value)
	case input.AbsoluteHat0X, input.AbsoluteHat0Y, input.AbsoluteRZ, input.AbsoluteZ, input.ButtonEStop,
		input.ButtonEast, input.ButtonLT, input.ButtonLT2, input.ButtonLThumb, input.ButtonMenu, input.ButtonNorth,
		input.ButtonRT, input.ButtonRT2, input.ButtonRThumb, input.ButtonRecord, input.ButtonSelect,
		input.ButtonSouth, input.ButtonStart, input.ButtonWest:
		fallthrough
	default:
	}

	return linear, angular
}

func similar(a, b r3.Vector, deltaThreshold float64) bool {
	if math.Abs(a.X-b.X) > deltaThreshold {
		return false
	}

	if math.Abs(a.Y-b.Y) > deltaThreshold {
		return false
	}

	if math.Abs(a.Z-b.Z) > deltaThreshold {
		return false
	}

	return true
}

func scaleThrottle(a float64) float64 {
	//nolint:ifshort
	neg := a < 0

	a = math.Abs(a)
	if a <= .27 {
		return 0
	}

	a = math.Ceil(a*10) / 10.0

	if neg {
		a *= -1
	}

	return a
}

type throttleState struct {
	linearThrottle, angularThrottle r3.Vector
	buttons                         map[input.Control]bool
	arrows                          map[input.Control]float64
}

func (ts *throttleState) init() {
	ts.buttons = map[input.Control]bool{
		input.ButtonNorth: false,
		input.ButtonSouth: false,
		input.ButtonEast:  false,
		input.ButtonWest:  false,
	}

	ts.arrows = map[input.Control]float64{
		input.AbsoluteHat0X: 0.0,
		input.AbsoluteHat0Y: 0.0,
	}
}

func parseEvent(mode controlMode, state *throttleState, event input.Event) (r3.Vector, r3.Vector) {
	newLinear := state.linearThrottle
	newAngular := state.angularThrottle

	switch mode {
	case joyStickControl:
		newLinear.Y, newAngular.Z = oneJoyStickEvent(event, state.linearThrottle.Y, state.angularThrottle.Z)
	case droneControl:
		newLinear, newAngular = droneEvent(event, state.linearThrottle, state.angularThrottle)
	case triggerSpeedControl:
		newLinear.Y, newAngular.Z = triggerSpeedEvent(event, state.linearThrottle.Y, state.angularThrottle.Z)
	case buttonControl:
		newLinear.Y, newAngular.Z, state.buttons = buttonControlEvent(event, state.buttons)
	case arrowControl:
		newLinear.Y, newAngular.Z, state.arrows = arrowEvent(event, state.arrows)
	}

	return newLinear, newAngular
}
