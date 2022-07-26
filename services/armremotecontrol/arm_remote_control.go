// Package armremotecontrol implements a remote control for a base.
package armremotecontrol

import (
	"context"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

// constants
const (
	jointByJointControl = controlMode(iota) // control modes
	endPointControl
	noop = controllerEvent(iota) // controller events
	leftJoystick
	rightJoystick
	directionalHat
	triggerZ
	buttonPressed
	SubtypeName = resource.SubtypeName("arm_remote_control") // resource name
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
type controllerEvent uint8

// Config describes how to configure the service.
type Config struct {
	ArmName             string `json:"arm"`
	InputControllerName string `json:"input_controller"`
	ArmSpeed            string `json:"arm_speed"`
}

// RemoteService is the structure of the remote service.
type remoteService struct {
	arm             arm.Arm
	inputController input.Controller
	config          *Config
	logger          golog.Logger
}

// controllerState used to manage controller for arm
type controllerState struct {
	event   controllerEvent
	mode    controlMode
	axis    r3.Vector
	buttons map[input.Control]bool
	arrows  map[input.Control]float64
}

func (cs *controllerState) init() {
	cs.event = noop
	cs.mode = jointByJointControl
	cs.buttons = map[input.Control]bool{
		input.ButtonSouth:  false,
		input.ButtonEast:   false,
		input.ButtonWest:   false,
		input.ButtonNorth:  false,
		input.ButtonLT:     false,
		input.ButtonRT:     false,
		input.ButtonSelect: false,
		input.ButtonStart:  false,
		input.ButtonMenu:   false,
	}

	// should we group?
	cs.arrows = map[input.Control]float64{
		input.AbsoluteX:     0.0,
		input.AbsoluteY:     0.0,
		input.AbsoluteZ:     0.0,
		input.AbsoluteRX:    0.0,
		input.AbsoluteRY:    0.0,
		input.AbsoluteRZ:    0.0,
		input.AbsoluteHat0X: 0.0,
		input.AbsoluteHat0Y: 0.0,
	}
}

// New returns a new remote control service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (interface{}, error) {
	svcConfig, ok := config.ConvertedAttributes.(*Config)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(svcConfig, config.ConvertedAttributes)
	}
	arm1, err := arm.FromRobot(r, svcConfig.ArmName)
	if err != nil {
		return nil, err
	}
	controller, err := input.FromRobot(r, svcConfig.InputControllerName)
	if err != nil {
		return nil, err
	}

	remoteSvc := &remoteService{
		arm:             arm1,
		inputController: controller,
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
	state := &controllerState{}
	state.init()

	var lastEvent input.Event
	var onlyOneAtATime sync.Mutex

	remoteCtl := func(ctx context.Context, event input.Event) {
		onlyOneAtATime.Lock()
		defer onlyOneAtATime.Unlock()

		if event.Time.Before(lastEvent.Time) {
			return
		}
		lastEvent = event

		err := svc.processEvent(ctx, state, event)
		if err != nil {
			svc.logger.Errorw("error with moving arm to desired position", "error", err)
		}
	}

	for _, control := range svc.controllerInputs() {
		// Register button changes & joystick modes
		err := svc.inputController.RegisterControlCallback(
			ctx,
			control,
			[]input.EventType{input.ButtonChange, input.PositionChangeAbs},
			remoteCtl,
		)
		if err != nil {
			return err
		}
	}
	svc.logger.Infof("Arm Controller service started with initial mode of: %v", state.mode)
	return nil
}

// Close out of all remote control related systems.
func (svc *remoteService) Close(ctx context.Context) error {
	for _, control := range svc.controllerInputs() {
		err := svc.inputController.RegisterControlCallback(
			ctx,
			control,
			[]input.EventType{input.ButtonChange, input.PositionChangeAbs},
			nil,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// controllerInputs returns the list of inputs from the controller that are being monitored for that control mode.
func (svc *remoteService) controllerInputs() []input.Control {
	return []input.Control{
		input.AbsoluteX,
		input.AbsoluteY,
		input.AbsoluteZ,
		input.AbsoluteRX,
		input.AbsoluteRY,
		input.AbsoluteRZ,
		input.AbsoluteHat0X,
		input.AbsoluteHat0Y,
		input.ButtonSouth,
		input.ButtonEast,
		input.ButtonWest,
		input.ButtonNorth,
		input.ButtonLT,
		input.ButtonRT,
		input.ButtonSelect,
		input.ButtonStart,
		input.ButtonMenu,
	}
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

func processCommand(ctx context.Context, svc *remoteService, state *controllerState) error {
	// there is probably a better way
	if state.buttons[input.ButtonSouth] {
		svc.logger.Debug("attempting stop")
		return svc.arm.Stop(ctx, nil)
	} else if state.buttons[input.ButtonEast] {
		svc.logger.Debug("attempting named pose preview")
		return nil
	} else if state.buttons[input.ButtonWest] {
		svc.logger.Debug("switching control mode")
		if state.mode == jointByJointControl {
			svc.logger.Debug("switching control mode JBJ -> EP")
			state.mode = endPointControl
		} else if state.mode == endPointControl {
			svc.logger.Debug("switching control mode: EP -> JBJ")
			state.mode = jointByJointControl
		}
		return nil
	} else if state.buttons[input.ButtonNorth] {
		svc.logger.Debug("attempting named pose execute")
		return nil
	} else if state.buttons[input.ButtonLT] {
		svc.logger.Debug("attempting to close pincer")
		return nil
	} else if state.buttons[input.ButtonRT] {
		svc.logger.Debug("attempting to open pincer")
		return nil
	} else if state.buttons[input.ButtonSelect] {
		svc.logger.Debug("attempting disable collision avoidance")
		return nil
	} else if state.buttons[input.ButtonStart] {
		svc.logger.Debug("attempting enable collision avoidance")
		return nil
	} else if state.buttons[input.ButtonMenu] {
		svc.logger.Debug("attempting to change joint group")
		return nil
	}
	return nil
}

func processArmEndPoint(ctx context.Context, svc *remoteService, state *controllerState) error {
	var zeroAxis r3.Vector
	if similar(state.axis, zeroAxis, .001) {
		return nil
	}
	svc.logger.Debugf("EP  processing(%s), Joystick mode: %d", state.axis, state.mode)
	return nil
}

func processArmJoint(ctx context.Context, svc *remoteService, state *controllerState) error {
	var zeroAxis r3.Vector
	if similar(state.axis, zeroAxis, .001) {
		return nil
	}
	svc.logger.Debugf("JBJ  processing(%s), Joystick mode: %d", state.axis, state.mode)
	return nil
}

func processArmControl(ctx context.Context, svc *remoteService, state *controllerState) error {
	switch state.event {
	case leftJoystick, rightJoystick, directionalHat, triggerZ:
		switch state.mode {
		case jointByJointControl:
			return processArmJoint(ctx, svc, state)
		case endPointControl:
			return processArmJoint(ctx, svc, state)
		}
	case buttonPressed:
		return processCommand(ctx, svc, state)
	}
	return nil
}

// parseControllerEvent sets up controller state based on event
func processControllerEvent(state *controllerState, event input.Event) {
	switch event.Event {
	case input.ButtonPress:
		state.buttons[event.Control] = true
	case input.ButtonRelease:
		state.buttons[event.Control] = false
		return
	case input.PositionChangeAbs:
		state.arrows[event.Control] = event.Value
	}

	switch event.Control {
	case input.AbsoluteX:
		state.axis.X = event.Value
		state.event = leftJoystick
	case input.AbsoluteY:
		state.axis.Y = event.Value
		state.event = leftJoystick
	case input.AbsoluteRX:
		state.axis.X = event.Value
		state.event = rightJoystick
	case input.AbsoluteRY:
		state.axis.Y = event.Value
		state.event = rightJoystick
	case input.AbsoluteHat0X:
		state.axis.X = event.Value
		state.event = directionalHat
	case input.AbsoluteHat0Y:
		state.axis.Y = event.Value
		state.event = directionalHat
	case input.AbsoluteZ:
		state.axis.Z = event.Value
		state.event = triggerZ
	case input.AbsoluteRZ:
		state.axis.Z = event.Value
		state.event = triggerZ
	case input.ButtonLT,
		input.ButtonRT,
		input.ButtonSouth,
		input.ButtonWest,
		input.ButtonEast,
		input.ButtonNorth,
		input.ButtonMenu,
		input.ButtonSelect,
		input.ButtonStart:
		state.event = buttonPressed
		state.axis.X = 0.0
		state.axis.Y = 0.0
		state.axis.Y = 0.0
	}
}

func (svc *remoteService) processEvent(ctx context.Context, state *controllerState, event input.Event) error {
	// setup controller to execute arm control
	processControllerEvent(state, event)
	if err := processArmControl(ctx, svc, state); err != nil {
		return err
	}
	return nil
}
