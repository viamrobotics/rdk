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
	commonpb "go.viam.com/rdk/proto/api/common/v1"
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
	ArmName               string            `json:"arm"`
	InputControllerName   string            `json:"input_controller"`
	ArmSpeed              string            `json:"arm_speed"`
	ArmStep               float64           `json:"arm_step"`
	StartingArmMode       string            `json:"starting_arm_mode"`
	DefaultJointStep      float64           `json:"default_joint_step"`
	ControllerSensitivity int               `json:"controller_sensitivity"`
	ControllerModes       []controllerModes `json:"controller_modes"`
}

type controllerModes struct {
	ModeName string             `json:"mode_name"`
	Mappings controllerMappings `json:"mappings"`
}

type controllerMappings struct {
	JointOne   string `json:"joint_one"`
	JointTwo   string `json:"joint_two"`
	JointThree string `json:"joint_three"`
	JointFour  string `json:"joint_four"`
	JointFive  string `json:"joint_five"`
	JointSix   string `json:"joint_six"`
	LinearX    string `json:"linear_x"`
	LinearY    string `json:"linear_y"`
	LinearZ    string `json:"linear_z"`
	RotationX  string `json:"rotation_x"`
	RotationY  string `json:"rotation_y"`
	RotationZ  string `json:"rotation_z"`
}

// RemoteService is the structure of the remote service.
type remoteService struct {
	arm             arm.Arm
	inputController input.Controller
	config          *Config
	logger          golog.Logger
}

// controllerState used to manage controller for arm
// TODO: can we remove button & arrow maps
type controllerState struct {
	event   controllerEvent
	mode    controlMode
	axis    r3.Vector // capture joystick events
	buttons map[input.Control]bool
	arrows  map[input.Control]float64
}

func (cs *controllerState) init() {
	cs.event = noop
	cs.axis = r3.Vector{}
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
}

// reset state
func (cs *controllerState) reset() {
	cs.event = noop
	cs.axis = r3.Vector{}

	for k, _ := range cs.buttons {
		cs.buttons[k] = false
	}
}

// isInvalid: currently assume sensitivity is 0-5
func (cs *controllerState) isInvalid(sensitivity float64) bool {
	sensitivity = (94 + sensitivity) * 0.01
	if math.Abs(cs.axis.X)-sensitivity > 0 {
		return false
	}
	if math.Abs(cs.axis.Y)-sensitivity > 0 {
		return false
	}
	if math.Abs(cs.axis.Z)-sensitivity > 0 {
		return false
	}

	return true
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

func processCommand(ctx context.Context, svc *remoteService, state *controllerState) error {
	if state.buttons[input.ButtonSouth] {
		svc.logger.Info("arm stopping")
		return svc.arm.Stop(ctx, nil)
	} else if state.buttons[input.ButtonEast] {
		svc.logger.Debug("attempting named pose preview")
	} else if state.buttons[input.ButtonWest] {
		svc.logger.Info("switching joint control mode")
		if state.mode == jointByJointControl {
			state.mode = endPointControl
		} else if state.mode == endPointControl {
			state.mode = jointByJointControl
		}
	} else if state.buttons[input.ButtonNorth] {
		svc.logger.Debug("attempting named pose execute")
	} else if state.buttons[input.ButtonLT] {
		svc.logger.Debug("attempting to close pincer")
	} else if state.buttons[input.ButtonRT] {
		svc.logger.Debug("attempting to open pincer")
	} else if state.buttons[input.ButtonSelect] {
		svc.logger.Debug("attempting disable collision avoidance")
	} else if state.buttons[input.ButtonStart] {
		svc.logger.Debug("attempting enable collision avoidance")
	} else if state.buttons[input.ButtonMenu] {
		svc.logger.Debug("attempting to change joint group")
	}
	return nil
}

func processArmEndPoint(ctx context.Context, svc *remoteService, state *controllerState) error {
	if state.isInvalid(5) {
		return nil
	}

	currentPose, err := svc.arm.GetEndPosition(ctx, nil)
	if err != nil {
		return err
	}
	newPose := &commonpb.Pose{}

	switch state.event {
	case leftJoystick:
		newPose.Z = currentPose.Z + (math.Round(state.axis.Z) * 10)
	case rightJoystick:
		newPose.Y = currentPose.Y + (math.Round(state.axis.X) * 10)
		newPose.X = currentPose.X + (math.Round(state.axis.Y) * 10)
	case directionalHat:
		newPose.OZ = currentPose.OZ + (math.Round(state.axis.X) * 10)
		newPose.OY = currentPose.OY + (math.Round(state.axis.Y) * 10)
	case triggerZ:
		newPose.OX = currentPose.OX + (math.Round(state.axis.Z) * 10)
	}

	err = svc.arm.MoveToPosition(ctx, newPose, nil, nil)
	if err != nil {
		return err
	}
	return nil
}

func processArmJoint(ctx context.Context, svc *remoteService, state *controllerState) error {
	if state.isInvalid(5) {
		return nil
	}

	jointPositions, err := svc.arm.GetJointPositions(ctx, nil)
	if err != nil {
		return err
	}

	switch state.event {
	case leftJoystick:
		jointPositions.Values[0] += (math.Round(state.axis.X) * 10)
		jointPositions.Values[1] += (math.Round(state.axis.Y) * 10)
	case rightJoystick:
		jointPositions.Values[2] += (math.Round(state.axis.Y) * 10)
		jointPositions.Values[3] += (math.Round(state.axis.X) * 10)
	case directionalHat:
		jointPositions.Values[4] += (math.Round(state.axis.X) * 10)
		jointPositions.Values[5] += (math.Round(state.axis.Y) * 10)
	case triggerZ:
		break
	}

	err = svc.arm.MoveToJointPositions(ctx, jointPositions, nil)
	if err != nil {
		return err
	}
	return nil
}

func processArmControl(ctx context.Context, svc *remoteService, state *controllerState) error {
	switch state.event {
	case leftJoystick, rightJoystick, directionalHat, triggerZ:
		switch state.mode {
		case endPointControl:
			return processArmEndPoint(ctx, svc, state)
		case jointByJointControl:
			return processArmJoint(ctx, svc, state)
		}
	case buttonPressed:
		return processCommand(ctx, svc, state)
	}
	return nil
}

// processControllerEvent sets up controller state based on event
func processControllerEvent(state *controllerState, event input.Event) {
	switch event.Event {
	case input.ButtonPress:
		state.buttons[event.Control] = true
	case input.ButtonRelease:
		state.buttons[event.Control] = false
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
		state.axis.Z = -1 * event.Value
		state.event = triggerZ
	case input.AbsoluteRZ:
		state.axis.Z = event.Value
		state.event = triggerZ
	case input.ButtonLT, input.ButtonRT, input.ButtonSouth,
		input.ButtonWest, input.ButtonEast, input.ButtonNorth,
		input.ButtonMenu, input.ButtonSelect, input.ButtonStart:
		state.event = buttonPressed
	}
}

func (svc *remoteService) processEvent(ctx context.Context, state *controllerState, event input.Event) error {
	// setup controller state to execute new arm control
	processControllerEvent(state, event)
	// execute stated arm control
	if err := processArmControl(ctx, svc, state); err != nil {
		return err
	}
	// reset state
	state.reset()
	return nil
}
