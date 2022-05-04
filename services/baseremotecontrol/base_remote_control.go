// Package baseremotecontrol implements a remote control for a base.
package baseremotecontrol

import (
	"context"
	"math"

	"github.com/edaniels/golog"
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

// Type is the type of service, set of implmented control modes and maxSpeed and maxAngle parameters.
// Note: these constants are flexible and may be tweaking.
const (
	oneJoyStickControl = controlMode(iota)
	triggerSpeedControl
	buttonControl
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
	BaseName            string `json:"base"`
	InputControllerName string `json:"input_controller"`
	ControlModeName     string `json:"joystick_mode"`
}

// RemoteService is the structure of the remote service.
type remoteService struct {
	base            base.Base
	inputController input.Controller
	controlMode     controlMode

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
	default:
		controlMode1 = oneJoyStickControl
	}

	remoteSvc := &remoteService{
		base:            base1,
		inputController: controller,
		controlMode:     controlMode1,
		logger:          logger,
	}

	if err := remoteSvc.start(ctx); err != nil {
		return nil, errors.Errorf("error with starting remote control service: %q", err)
	}

	return remoteSvc, nil
}

// Start is the main control loops for sending events from controller to base.
func (svc *remoteService) start(ctx context.Context) error {
	var mmPerSec float64
	var angleDeg float64
	var oldMmPerSec float64
	var oldAngleDeg float64

	var buttons map[input.Control]bool
	buttons[input.ButtonNorth] = false
	buttons[input.ButtonSouth] = false
	buttons[input.ButtonEast] = false
	buttons[input.ButtonWest] = false

	remoteCtl := func(ctx context.Context, event input.Event) {
		// if event.Event != input.PositionChangeAbs && svc.controlMode != buttonControl {
		// 	return
		// }

		switch svc.controlMode {
		case triggerSpeedControl:
			mmPerSec, angleDeg = svc.triggerSpeedEvent(event, mmPerSec, angleDeg)
		case buttonControl:
			mmPerSec, angleDeg, buttons = svc.buttonControlEvent(event, buttons)
		case oneJoyStickControl:
			fallthrough
		default:
			mmPerSec, angleDeg = svc.oneJoyStickEvent(event, mmPerSec, angleDeg)
		}

		if math.Abs(mmPerSec-oldMmPerSec) < 0.05 && math.Abs(angleDeg-oldAngleDeg) < 0.05 {
			return
		}

		oldMmPerSec = mmPerSec
		oldAngleDeg = angleDeg

		var d int
		var s float64
		var a float64
		if mmPerSec == 0 && angleDeg == 0 {
			// Stop
			d = int(maxSpeed * distRatio)
			s = 0.0
			a = angleDeg * maxAngle * -1
		} else if mmPerSec == 0 {
			// Spin
			d = int(0)
			s = angleDeg * maxSpeed
			a = math.Abs(angleDeg * maxAngle * distRatio / 2)
		} else if angleDeg == 0 {
			// Move Straight
			d = int(math.Abs(mmPerSec * maxSpeed * distRatio))
			s = mmPerSec * maxSpeed
			a = math.Abs(angleDeg * maxAngle * distRatio)
		} else {
			// Move Arc
			d = int(math.Abs(mmPerSec * maxSpeed * distRatio))
			s = mmPerSec * maxSpeed
			a = angleDeg*maxAngle*distRatio*2 - 1
		}

		if err := svc.base.MoveArc(ctx, d, s, a, true); err != nil {
			svc.logger.Errorw("error with moving base to desired position", "error", err)
		}
	}

	for _, control := range svc.controllerInputs() {
		err := svc.inputController.RegisterControlCallback(ctx, control, []input.EventType{input.PositionChangeAbs}, remoteCtl)
		if err != nil {
			return err
		}
	}
	return nil
}

// Close out of all remote control related systems.
func (svc *remoteService) Close(ctx context.Context) error {
	for _, control := range svc.controllerInputs() {
		err := svc.inputController.RegisterControlCallback(ctx, control, []input.EventType{input.PositionChangeAbs}, nil)
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
	case buttonControl:
		return []input.Control{input.ButtonNorth, input.ButtonSouth, input.ButtonEast, input.ButtonWest}
	case oneJoyStickControl:
		fallthrough
	default:
		return []input.Control{input.AbsoluteX, input.AbsoluteY}
	}
}

// triggerSpeedEvent takes inputs from the gamepad allowing the triggers to control speed and the left jostick to
// control the angle.
func (svc *remoteService) triggerSpeedEvent(event input.Event, speed float64, angle float64) (float64, float64) {
	oldSpeed := speed
	oldAngle := angle

	switch event.Control {
	case input.AbsoluteZ:
		speed -= 0.05
		speed = math.Max(-1, speed)
		angle = oldAngle
	case input.AbsoluteRZ:
		speed += 0.05
		speed = math.Min(1, speed)
		angle = oldAngle
	case input.AbsoluteX:
		angle = event.Value
		speed = oldSpeed
	case input.AbsoluteY, input.AbsoluteHat0X, input.AbsoluteHat0Y, input.AbsoluteRX, input.AbsoluteRY,
		input.ButtonEStop, input.ButtonEast, input.ButtonLT,
		input.ButtonLThumb, input.ButtonMenu, input.ButtonNorth, input.ButtonRT, input.ButtonRThumb,
		input.ButtonRecord, input.ButtonSelect, input.ButtonSouth, input.ButtonStart, input.ButtonWest:
	}

	return svc.speedAndAngleMathMag(speed, angle, oldSpeed, oldAngle)
}

// oneJoyStickEvent (default) takes inputs from the gamepad allowing the left joystick to control speed and angle.
func (svc *remoteService) oneJoyStickEvent(event input.Event, speed float64, angle float64) (float64, float64) {
	oldSpeed := speed
	oldAngle := angle

	switch event.Control {
	case input.AbsoluteY:
		speed = event.Value
		angle = oldAngle
	case input.AbsoluteX:
		angle = event.Value
		speed = oldSpeed
	case input.AbsoluteHat0X, input.AbsoluteHat0Y, input.AbsoluteRX, input.AbsoluteRY,
		input.AbsoluteRZ, input.AbsoluteZ, input.ButtonEStop, input.ButtonEast, input.ButtonLT,
		input.ButtonLThumb, input.ButtonMenu, input.ButtonNorth, input.ButtonRT, input.ButtonRThumb,
		input.ButtonRecord, input.ButtonSelect, input.ButtonSouth, input.ButtonStart, input.ButtonWest:
	}

	return svc.speedAndAngleMathMag(speed, angle, oldSpeed, oldAngle)
}

func (svc *remoteService) buttonControlEvent(event input.Event, buttons map[input.Control]bool) (float64, float64, map[input.Control]bool) {
	var newSpeed float64
	var newAngle float64

	if event.Event == input.ButtonPress {
		buttons[event.Control] = true
	}

	if event.Event == input.ButtonRelease {
		buttons[event.Control] = false
	}

	if buttons[input.ButtonNorth] == buttons[input.ButtonSouth] {
		newSpeed = 0.0
	} else if buttons[input.ButtonNorth] {
		newSpeed = 1.0
	} else {
		newSpeed = -1.0
	}

	if buttons[input.ButtonEast] == buttons[input.ButtonWest] {
		newAngle = 0.0
	} else if buttons[input.ButtonEast] {
		newAngle = -1.0
	} else {
		newAngle = 1.0
	}

	return newSpeed, newAngle, buttons
}

// SpeedAndAngleMathMag utilizes a cut-off and the magnitude of the speed and angle to dictate mmPerSec and
// angleDeg.
func (svc *remoteService) speedAndAngleMathMag(speed float64, angle float64, oldSpeed float64, oldAngle float64) (float64, float64) {
	var newSpeed float64
	var newAngle float64

	mag := math.Sqrt(speed*speed + angle*angle)

	switch {
	case math.Abs(speed) < 0.25 && mag > 0.25:
		newSpeed = oldSpeed
		newAngle = angle
	case math.Abs(speed-oldSpeed) < 0.2:
		newSpeed = oldSpeed
		newAngle = angle
	default:
		newSpeed = speed
		newAngle = angle
	}
	return newSpeed, newAngle
}
