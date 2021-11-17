package baseremotecontrol

import (
	"context"
	"math"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"github.com/mitchellh/mapstructure"

	"go.viam.com/core/base"
	"go.viam.com/core/config"
	"go.viam.com/core/input"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
)

// Type is the type of service, set of implmented control modes and maxSpeed and maxAngle parameters
const (
	oneJoyStickControl = controlMode(iota)
	triggerSpeedControl
	Type     = config.ServiceType("base_remote_control")
	maxSpeed = 100.0
	maxAngle = 40.0
)

func init() {
	registry.RegisterService(Type, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
		},
		AttributeMapConverter: func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &conf})
			if err != nil {
				return nil, err
			}
			if err := decoder.Decode(attributes); err != nil {
				return nil, err
			}
			return &conf, nil
		},
	},
	)
}

// ControlMode is the control type for the remote control
type controlMode uint8

// Config describes how to configure the service.
type Config struct {
	BaseName            string `json:"base"`
	InputControllerName string `json:"input_controller"`
	JoyStickModeName    string `json:"joystick_mode"`
}

// controllerInputs returns the list of inputs from the controller that are being monitored for that control mode
func (svc *RemoteService) controllerInputs() []input.Control {
	switch svc.controlMode {
	case triggerSpeedControl:
		return []input.Control{input.AbsoluteX, input.AbsoluteZ, input.AbsoluteRZ}
	default:
		return []input.Control{input.AbsoluteX, input.AbsoluteY}
	}
}

// A Service controls the navigation for a robot.
type Service interface {
	Close(ctx context.Context) error
}

// Close out all remote control related systems
func (svc *RemoteService) Close(ctx context.Context) error {
	for _, control := range svc.controllerInputs() {
		err := svc.inputController.RegisterControlCallback(ctx, control, []input.EventType{input.PositionChangeAbs}, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoteService is the structure of the remote service
type RemoteService struct {
	base            base.Base
	inputController input.Controller
	controlMode     controlMode

	logger golog.Logger
}

// New returns a new remote control service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (*RemoteService, error) {
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

	remoteSvc := &RemoteService{
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
func (svc *RemoteService) start(ctx context.Context) error {

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
		_, err := svc.base.MoveArc(context.Background(), 1000, millisPerSec*maxSpeed*-1, degPerSec*maxAngle, true)

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

// triggerSpeedEvent takes inputs from the gamepad allowing the triggers to control speed and the left jostick to
// control the angle
func (svc *RemoteService) triggerSpeedEvent(event input.Event, speed float64, angle float64) (float64, float64) {

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
func (svc *RemoteService) oneJoyStickEvent(event input.Event, speed float64, angle float64) (float64, float64) {

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
func (svc *RemoteService) speedAndAngleMathMag(speed float64, angle float64, oldSpeed float64, oldAngle float64) (float64, float64) {

	var newSpeed float64
	var newAngle float64

	mag := math.Sqrt(speed*speed + angle*angle)

	if math.Abs(speed) < 0.5 && mag > 0.5 {
		newSpeed = oldSpeed
		newAngle = angle
	} else {
		newSpeed = speed
		newAngle = angle

	}
	return newSpeed, newAngle
}
