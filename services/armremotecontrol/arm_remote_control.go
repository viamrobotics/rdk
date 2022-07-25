// Package armremotecontrol implements a remote control for a base.
package armremotecontrol

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
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
	JointByJointControl = controlMode(iota) // control modes
	endPointControl
	SubtypeName = resource.SubtypeName("arm_remote_control") // resource name
	noop        = armEvent(iota)                             // controller events
	leftJoystick
	rightJoystick
	directionalHat
	triggerZ
	buttonPressed
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
type armEvent uint8

// Config describes how to configure the service.
type Config struct {
	ArmName             string `json:"arm"`
	InputControllerName string `json:"input_controller"`
	ControlModeName     string `json:"control_mode"`
	ArmSpeed            string `json:"arm_speed"`
}

// RemoteService is the structure of the remote service.
type remoteService struct {
	arm             arm.Arm
	inputController input.Controller
	controlMode     controlMode
	armEvent        armEvent
	config          *Config
	logger          golog.Logger
}

type controllerState struct {
	event   controllerEvent
	x       float64
	y       float64
	z       float64
	buttons map[input.Control]bool
	arrows  map[input.Control]float64
}

func (cs *controllerState) init() {
	cs.event = noop
	cs.x = 0.0
	cs.y = 0.0
	cs.z = 0.0
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

	var controlMode1 controlMode
	switch svcConfig.ControlModeName {
	case "jointByJointControl":
		controlMode1 = jointByJointControl
	case "endPointControl":
		controlMode1 = endPointControl
	default:
		controlMode1 = jointByJointControl
	}

	remoteSvc := &remoteService{
		arm:             arm1,
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
	svc.logger.Infof("Arm Controller service started with initial mode of: %v", svc.controlMode)
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

func parseEndPointEvent(event input.Event, state *controllerState) error {
	switch event.Event {
	case input.ButtonPress:
		state.buttons[event.Control] = true
	case input.ButtonRelease:
		state.buttons[event.Control] = false
	case input.PositionChangeAbs:
		state.arrows[event.Control] = event.Value
	default:
		return errors.New("invalid event")
	}
	return nil
}

func parseJointByJointEvent(event input.Event, state *controllerState) error {
	switch event.Event {
	case input.ButtonPress:
		state.buttons[event.Control] = true
	case input.ButtonRelease:
		state.buttons[event.Control] = false
		return nil
	case input.PositionChangeAbs:
		state.arrows[event.Control] = event.Value
	default:
		return errors.New("invalid event")
	}

	switch event.Control {
	case input.AbsoluteX, // joint1
		input.AbsoluteY,     // joint2
		input.AbsoluteRX,    // joint3
		input.AbsoluteRY,    // joint4
		input.AbsoluteHat0X, // joint5
		input.AbsoluteHat0Y, // joint6
		input.AbsoluteZ,     // joint7
		input.AbsoluteRZ:    // joint7
		if state.arrows[input.AbsoluteX] != 0.0 || state.arrows[input.AbsoluteY] != 0.0 {
			state.event = processXY
		} else if state.arrows[input.AbsoluteRX] != 0.0 || state.arrows[input.AbsoluteRY] != 0.0 {
			state.event = processRXRY
		} else if state.arrows[input.AbsoluteHat0X] != 0.0 || state.arrows[input.AbsoluteHat0Y] != 0.0 {
			state.event = processHat0XHat0Y
		} else if state.arrows[input.AbsoluteZ] != 0.0 || state.arrows[input.AbsoluteRZ] != 0.0 {
			state.event = processZRZ
		}
	case input.ButtonLT:
		state.event = pincerOpen
	case input.ButtonRT:
		state.event = pincerClose
	case input.ButtonSouth:
		state.event = stop
	case input.ButtonWest:
		state.event = changeMode
	case input.ButtonEast:
		state.event = namedPosePreview
	case input.ButtonNorth:
		state.event = namedPoseExecute
	case input.ButtonMenu:
		state.event = changeJointGroup
	case input.ButtonSelect, input.ButtonStart:
		state.event = changeCollisionAvoidance
	}
	return nil
}

func parseControllerEvent(mode controlMode, state *controllerState, event input.Event) error {
	switch mode {
	case endPointControl:
		return parseEndPointEvent(event, state)
	case jointByJointControl:
		return parseJointByJointEvent(event, state)
	default:
		return errors.New("invalid mode")
	}
}

func (svc *remoteService) processEvent(ctx context.Context, state *controllerState, event input.Event) error {
	err := parseControlerEvent(svc.controlMode, state, event)
	if err != nil {
		svc.logger.Errorw("error processing event", "error", err)
		return err
	}

	switch state.event {
	case setPosition:
		svc.logger.Debug("setPosition")
	case pincerClose:
		svc.logger.Debug("pincerClose")
	case pincerOpen:
		svc.logger.Debug("pincerOpen")
	case changeMode:
		svc.logger.Debug("changeMode")
	case namedPoseExecute:
		svc.logger.Debug("namedPoseExecute")
	case namedPosePreview:
		svc.logger.Debug("namedPosePreview")
	case changeCollisionAvoidance:
		svc.logger.Debug("changeCollisionAvoidance")
	case changeJointGroup:
		svc.logger.Debug("changeJointGroup")
	case processXY:
		svc.logger.Debugf("processXY(%f, %f)", state.arrows[input.AbsoluteX], state.arrows[input.AbsoluteY])
	case processRXRY:
		svc.logger.Debugf("processRXRY(%f, %f)", state.arrows[input.AbsoluteRX], state.arrows[input.AbsoluteRY])
	case processHat0XHat0Y:
		svc.logger.Debugf("processHat0XHat0Y(%f, %f)", state.arrows[input.AbsoluteHat0X], state.arrows[input.AbsoluteHat0Y])
	case processZRZ:
		svc.logger.Debugf("processZRZ(%f, %f)", state.arrows[input.AbsoluteZ], state.arrows[input.AbsoluteRZ])
	case stop:
		err := svc.arm.Stop(ctx, nil)
		if err != nil {
			svc.logger.Info("stop failed")
		}
		svc.logger.Debug("stop")
	case noop:
		return nil
	default:
		svc.logger.Debugf("bad option: %q", state.event)
	}

	// clear event
	state.event = noop
	return nil
}
