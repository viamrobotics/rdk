package baseremotecontrol

import (
	"context"
	"math"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/base"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

// Type is the type of service, set of implmented control modes and maxSpeed and maxAngle parameters
const (
	oneJoyStickControl = controlMode(iota)
	triggerSpeedControl
	Type      = config.ServiceType("base_remote_control")
	maxSpeed  = 1000.0
	maxAngle  = 360.0
	distRatio = 10
)

func init() {
	registry.RegisterService(Type, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
		},
	},
	)

	config.RegisterServiceAttributeMapConverter(Type, func(attributes config.AttributeMap) (interface{}, error) {
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

// ControlMode is the control type for the remote control
type controlMode uint8

// Config describes how to configure the service.
type Config struct {
	BaseName            string `json:"base"`
	InputControllerName string `json:"input_controller"`
	JoyStickModeName    string `json:"joystick_mode"`
}

// RemoteService is the structure of the remote service
type remoteService struct {
	base            base.Base
	inputController input.Controller
	controlMode     controlMode

	logger golog.Logger
}

// New returns a new remote control service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (interface{}, error) {
	svcConfig := config.ConvertedAttributes.(*Config)
	base1, ok := r.BaseByName(svcConfig.BaseName)
	if !ok {
		return nil, errors.Errorf("no base named %q", svcConfig.BaseName)
	}
	controller, ok := r.InputControllerByName(svcConfig.InputControllerName)
	if !ok {
		return nil, errors.Errorf("no input controller named %q", svcConfig.InputControllerName)
	}

	controlMode1 := oneJoyStickControl

	switch svcConfig.JoyStickModeName {
	case "triggerSpeedControl":
		controlMode1 = triggerSpeedControl
	}

	remoteSvc := &remoteService{
		base:            base1,
		inputController: controller,
		controlMode:     controlMode1,
		logger:          logger,
	}

	err := remoteSvc.start(ctx)

	if err != nil {
		return nil, errors.Errorf("error with starting remote control service: %q", err)
	}

	return remoteSvc, nil
}

// Start is the main control loops for sending events from controller to base
func (svc *remoteService) start(ctx context.Context) error {

	var millisPerSec float64
	var degPerSec float64

	remoteCtl := func(ctx context.Context, event input.Event) {

		if event.Event != input.PositionChangeAbs {
			return
		}

		switch svc.controlMode {
		case triggerSpeedControl:
			millisPerSec, degPerSec = svc.triggerSpeedEvent(event, millisPerSec, degPerSec)
		default:
			millisPerSec, degPerSec = svc.oneJoyStickEvent(event, millisPerSec, degPerSec)
		}

		// Set distance to large number as it will be overwritten (Note: could have a dependecy on speed)
		var err error
		if math.Abs(degPerSec) < 0.99 && math.Abs(millisPerSec) > 0.1 {
			err = svc.base.MoveArc(context.Background(), maxSpeed*distRatio, millisPerSec*maxSpeed*-1, degPerSec*maxAngle, true)

		} else {
			err = svc.base.MoveArc(context.Background(), maxSpeed*distRatio, 0, degPerSec*maxAngle, true)
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

// Close out of all remote control related systems
func (svc *remoteService) Close() error {
	for _, control := range svc.controllerInputs() {
		err := svc.inputController.RegisterControlCallback(context.Background(), control, []input.EventType{input.PositionChangeAbs}, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

// controllerInputs returns the list of inputs from the controller that are being monitored for that control mode
func (svc *remoteService) controllerInputs() []input.Control {
	switch svc.controlMode {
	case triggerSpeedControl:
		return []input.Control{input.AbsoluteX, input.AbsoluteZ, input.AbsoluteRZ}
	default:
		return []input.Control{input.AbsoluteX, input.AbsoluteY}
	}
}

// triggerSpeedEvent takes inputs from the gamepad allowing the triggers to control speed and the left jostick to
// control the angle
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
	}

	return svc.speedAndAngleMathMag(speed, angle, oldSpeed, oldAngle)
}

// oneJoyStickEvent (default) takes inputs from the gamepad allowing the left joystick to control speed and angle
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
	}

	return svc.speedAndAngleMathMag(speed, angle, oldSpeed, oldAngle)
}

// SpeedAndAngleMathMag utilizes a cut-off and the magnitude of the speed and angle to dictate millisPerSec and
// degPerSec
func (svc *remoteService) speedAndAngleMathMag(speed float64, angle float64, oldSpeed float64, oldAngle float64) (float64, float64) {

	var newSpeed float64
	var newAngle float64

	mag := math.Sqrt(speed*speed + angle*angle)

	if math.Abs(speed) < 0.25 && mag > 0.25 {
		newSpeed = oldSpeed
		newAngle = angle
	} else if math.Abs(speed) < 0.25 {
		newSpeed = 0
		newAngle = angle
	} else {
		newSpeed = speed
		newAngle = angle

	}
	return newSpeed, newAngle
}
