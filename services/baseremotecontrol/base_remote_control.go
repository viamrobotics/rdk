// Package baseremotecontrol implements a remote control for a base.
package baseremotecontrol

import (
	"context"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

// Constants for the system including the max speed and angle (TBD: allow to be set as config vars)
// as well as the various control modes including oneJoystick (control via a joystick), triggerSpeed
// (triggers control speed and joystick angle), button (four buttons X, Y, A, B to  control speed and
// angle) and arrow (arrows buttons used to control speed and angle). A distance ratio is used as well
// to account for the base's need for a disatnce parameter, this vlaue is arbitrarily large such that
// holding a button/joystick in position will not cause unexpected stops to quickly.
const (
	joyStickControl = controlMode(iota)
	triggerSpeedControl
	buttonControl
	arrowControl
	droneControl
	SubtypeName = resource.SubtypeName("base_remote_control")
	maxSpeed    = 500.0
	maxAngle    = 360.0
	distRatio   = 10
)

// Subtype is a constant that identifies the remote control resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Name is the BaseRemoteControlService's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

func init() {
	registry.RegisterService(Subtype, registry.Service{Constructor: New})
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
	MaxAngularVelocity  float64 `json:"max_angular"`
	MaxLinearVelocity   float64 `json:"max_linear"`
}

// RemoteService is the structure of the remote service.
type remoteService struct {
	base            base.Base
	inputController input.Controller
	controlMode     controlMode

	config *Config
	logger golog.Logger
}

// New returns a new remote control service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (interface{}, error) {
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

	remoteSvc := &remoteService{
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
func (svc *remoteService) start(ctx context.Context) error {
	buttons := make(map[input.Control]bool)
	buttons[input.ButtonNorth] = false
	buttons[input.ButtonSouth] = false
	buttons[input.ButtonEast] = false
	buttons[input.ButtonWest] = false

	arrows := make(map[input.Control]float64)
	arrows[input.AbsoluteHat0X] = 0.0
	arrows[input.AbsoluteHat0Y] = 0.0

	oldValues := []float64{}
	var lastEvent input.Event
	var onlyOneAtATime sync.Mutex

	remoteCtl := func(ctx context.Context, event input.Event) {
		onlyOneAtATime.Lock()
		defer onlyOneAtATime.Unlock()

		if event.Time.Before(lastEvent.Time) {
			return
		}
		lastEvent = event

		var err error
		var newValues []float64

		switch svc.controlMode {
		case joyStickControl:
			if len(oldValues) != 2 {
				oldValues = []float64{0, 0}
			}

			linY, angZ := svc.oneJoyStickEvent(event, oldValues[0], oldValues[1])
			linY = scaleValue(linY)
			angZ = scaleValue(angZ)

			newValues = []float64{linY, angZ}

			if similar(oldValues, newValues, .05) {
				return
			}

			err = svc.base.SetPower(ctx, r3.Vector{Y: linY}, r3.Vector{Z: angZ})
		case droneControl:
			if len(oldValues) != 4 {
				oldValues = []float64{0, 0, 0, 0}
			}

			newValues = svc.droneEvent(event, oldValues)

			if similar(oldValues, newValues, .05) {
				return
			}

			if svc.config.MaxAngularVelocity > 0 && svc.config.MaxLinearVelocity > 0 {
				err = svc.base.SetVelocity(
					ctx,
					r3.Vector{
						X: svc.config.MaxLinearVelocity * newValues[2],
						Y: svc.config.MaxLinearVelocity * newValues[3],
						Z: svc.config.MaxLinearVelocity * newValues[1],
					},
					r3.Vector{Z: svc.config.MaxAngularVelocity * newValues[0]},
				)
			} else {
				err = svc.base.SetPower(ctx, r3.Vector{X: newValues[2], Y: newValues[3], Z: newValues[1]}, r3.Vector{Z: newValues[0]})
			}
		case arrowControl, buttonControl, triggerSpeedControl:
			var mmPerSec, angleDeg float64

			// nolint:exhaustive
			switch svc.controlMode {
			case triggerSpeedControl:
				mmPerSec, angleDeg = svc.triggerSpeedEvent(event, mmPerSec, angleDeg)
			case buttonControl:
				mmPerSec, angleDeg, buttons = svc.buttonControlEvent(event, buttons)
			case arrowControl:
				mmPerSec, angleDeg, arrows = svc.arrowEvent(event, arrows)
			default:
				panic("impossible")
			}

			newValues = []float64{mmPerSec, angleDeg}
			// Skip minor adjustments in instructions as to not overload system
			if similar(oldValues, newValues, .05) {
				return
			}

			var d int
			var s float64
			var a float64

			switch {
			case math.Abs(mmPerSec) < 0.15 && math.Abs(angleDeg) < 0.25:
				// Stop
				d = int(maxSpeed * distRatio)
				s = 0.0
				a = angleDeg * maxAngle * -1
			case math.Abs(angleDeg) < 0.25:
				// Move Straight
				d = int(math.Abs(mmPerSec * maxSpeed * distRatio))
				s = mmPerSec * maxSpeed
				a = math.Abs(angleDeg * maxAngle * distRatio)
			case math.Abs(mmPerSec) < 0.15:
				// Spin
				d = int(0)
				s = angleDeg * maxSpeed
				a = math.Abs(angleDeg * maxAngle * distRatio / 2)
			default:
				// Move Arc
				d = int(math.Abs(mmPerSec * maxSpeed * distRatio))
				s = mmPerSec * maxSpeed
				a = angleDeg*maxAngle*distRatio*2 - 1
			}

			err = svc.base.MoveArc(ctx, d, s, a)
		}

		if err != nil {
			svc.logger.Errorw("error with moving base to desired position", "error", err)
		} else {
			oldValues = newValues
		}
	}

	for _, control := range svc.controllerInputs() {
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
func (svc *remoteService) Close(ctx context.Context) error {
	for _, control := range svc.controllerInputs() {
		var err error
		if svc.controlMode == buttonControl {
			err = svc.inputController.RegisterControlCallback(ctx, control, []input.EventType{input.ButtonChange}, nil)
		} else {
			err = svc.inputController.RegisterControlCallback(ctx, control, []input.EventType{input.PositionChangeAbs}, nil)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// controllerInputs returns the list of inputs from the controller that are being monitored for that control mode.
func (svc *remoteService) controllerInputs() []input.Control {
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

// triggerSpeedEvent takes inputs from the gamepad allowing the triggers to control speed and the left joystick to
// control the angle.
func (svc *remoteService) triggerSpeedEvent(event input.Event, speed float64, angle float64) (float64, float64) {
	
	switch event.Control {
	case input.AbsoluteZ:
		speed -= 0.05
		speed = math.Max(-1, speed)
	case input.AbsoluteRZ:
		speed += 0.05
		speed = math.Min(1, speed)
	case input.AbsoluteX:
		angle = event.Value
	default:
		return speed, angle
	}

	return speed, angle
}

// buttonControlEvent takes inputs from the gamepad allowing the X and B buttons to control speed and Y and A buttons to control angle.
func (svc *remoteService) buttonControlEvent(event input.Event, buttons map[input.Control]bool) (float64, float64, map[input.Control]bool) {
	var speed float64
	var angle float64

	
	switch event.Event {
	case input.ButtonPress:
		buttons[event.Control] = true
	case input.ButtonRelease:
		buttons[event.Control] = false
	default:
		return 0.0, 0.0, buttons
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
func (svc *remoteService) arrowEvent(event input.Event, arrows map[input.Control]float64) (float64, float64, map[input.Control]float64) {
	arrows[event.Control] = -1.0 * event.Value

	speed := arrows[input.AbsoluteHat0Y]
	angle := arrows[input.AbsoluteHat0X]

	return speed, angle, arrows
}

// oneJoyStickEvent (default) takes inputs from the gamepad allowing the left joystick to control speed and angle.
func (svc *remoteService) oneJoyStickEvent(event input.Event, y float64, x float64) (float64, float64) {
	
	switch event.Control {
	case input.AbsoluteY:
		y = -1.0 * event.Value
	case input.AbsoluteX:
		x = -1.0 * event.Value
	default:
		return y, x
	}

	return y, x
}

// right joystick is forward/back, strafe right/left
// left joystick is spin right/left & up/down
// order is: X, Y, RX, RY = AngZ(0), LinZ(1), LinX(2), LinY(3).
func (svc *remoteService) droneEvent(event input.Event, oldValues []float64) []float64 {
	newValues := make([]float64, len(oldValues))
	copy(newValues, oldValues)

	
	switch event.Control {
	case input.AbsoluteX:
		newValues[0] = -1.0 * event.Value
	case input.AbsoluteY:
		newValues[1] = -1.0 * event.Value
	case input.AbsoluteRX:
		newValues[2] = event.Value
	case input.AbsoluteRY:
		newValues[3] = -1.0 * event.Value
	}

	return scaleValues(newValues)
}

func similar(a, b []float64, delta float64) bool {
	if len(a) != len(b) {
		return false
	}

	for idx := range a {
		if math.Abs(a[idx]-b[idx]) > delta {
			return false
		}
	}

	return true
}

func scaleValues(a []float64) []float64 {
	n := make([]float64, len(a))
	for idx, x := range a {
		n[idx] = scaleValue(x)
	}
	return n
}

func scaleValue(a float64) float64 {
	// nolint:ifshort
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
