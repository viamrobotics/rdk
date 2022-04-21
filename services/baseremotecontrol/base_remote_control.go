// Package baseremotecontrol implements a remote control for a base.
package baseremotecontrol

import (
	"context"
	"math"
	"fmt"

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
	SubtypeName = resource.SubtypeName("base_remote_control")
	maxSpeed    = 300.0
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
	JoyStickModeName    string `json:"joystick_mode"`
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

	controlMode1 := oneJoyStickControl

	if svcConfig.JoyStickModeName == "triggerSpeedControl" {
		controlMode1 = triggerSpeedControl
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

	remoteCtl := func(ctx context.Context, event input.Event) {
		if event.Event != input.PositionChangeAbs {
			return
		}

		switch svc.controlMode {
		case triggerSpeedControl:
			mmPerSec, angleDeg = svc.triggerSpeedEvent(event, mmPerSec, angleDeg)
		case oneJoyStickControl:
			fallthrough
		default:
			mmPerSec, angleDeg = svc.oneJoyStickEvent(event, mmPerSec, angleDeg)
		}

		fmt.Printf("Controller Input: %v | %v\n", mmPerSec, angleDeg)

		if (math.Abs(mmPerSec - oldMmPerSec) < 0.05 && math.Abs(angleDeg - oldAngleDeg) < 0.05) {
			fmt.Println("Skipping...")
			return
		}

		oldMmPerSec = mmPerSec
		oldAngleDeg = angleDeg
		// Set distance to large number as it will be overwritten (Note: could have a dependecy on speed)
		var err error
		if math.Abs(angleDeg) < 1.1 && math.Abs(mmPerSec) > 0.1 {
			// Move Arc
			d := int(math.Abs(mmPerSec*maxSpeed*distRatio))
			s := mmPerSec*maxSpeed*-1
			a := angleDeg*maxAngle*distRatio*-1
			fmt.Printf("Arc: s = %v | a = %v | Dist = %v | Speed = %v | Angle = %v\n", mmPerSec, angleDeg, d, s, a)
			err = svc.base.MoveArc(ctx, d, s, a, true)
		} else if math.Abs(angleDeg) > 0.9 && math.Abs(mmPerSec) < 0.1 {
			// Spin
			d := int(0)
			s := angleDeg*maxSpeed*-1
			a := math.Abs(angleDeg*maxAngle*distRatio)
			fmt.Printf("Spin: s = %v | a = %v | Dist = %v | Speed = %v | Angle = %v\n", mmPerSec, angleDeg, d, s, a)
			err = svc.base.MoveArc(ctx, d, s, a, true)
		} else {
			// Stop
			d := int(maxSpeed*distRatio)
			s := 0.0
			a := angleDeg*maxAngle*-1
			fmt.Printf("Stop: s = %v | a = %v | Dist = %v | Speed = %v | Angle = %v\n", mmPerSec, angleDeg, d, s, a)
			err = svc.base.MoveArc(ctx, d, s, a, true)
		}

		if err != nil {
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
	case math.Abs(speed - oldSpeed) < 0.2:
		newSpeed = oldSpeed
		newAngle = angle
	// case math.Abs(speed) < 0.25:
	// 	newSpeed = 0
	// 	newAngle = angle
	default:
		newSpeed = speed
		newAngle = angle
	}
	return newSpeed, newAngle
}
