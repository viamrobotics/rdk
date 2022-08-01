// Package armremotecontrol implements a remote control for a base.
package armremotecontrol

import (
	"context"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

// constants.
const (
	noop = controllerEvent(iota) // controller events
	jointEvent
	endPointEvent
	buttonPressed
	SubtypeName = resource.SubtypeName("arm_remote_control") // resource name
	jointOne    = armPart("jointOne")
	jointTwo    = armPart("jointTwo")
	jointThree  = armPart("jointThree")
	jointFour   = armPart("jointFour")
	jointFive   = armPart("jointFive")
	jointSix    = armPart("jointSix")
	x           = armPart("x")
	y           = armPart("y")
	z           = armPart("z")
	ox          = armPart("ox")
	oy          = armPart("oy")
	oz          = armPart("oz")
	theta       = armPart("theta")
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
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
	})

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

type (
	controllerEvent uint8
	armPart         string
)

// Config describes how to configure the service.
type Config struct {
	ArmName               string           `json:"arm"`
	InputControllerName   string           `json:"input_controller"`
	DefaultJointStep      float64          `json:"default_joint_step"`
	DefaultPoseStep       float64          `json:"default_pose_step"`
	ControllerSensitivity float64          `json:"controller_sensitivity"`
	ControllerModes       []controllerMode `json:"controller_modes"`
}

type controllerMode struct {
	ModeName string                    `json:"mode_name"`
	Mappings map[input.Control]armPart `json:"mappings"`
}

// controllerState used to manage controller for arm
// TODO: can we remove button & arrow maps.
type controllerState struct {
	event      controllerEvent
	curModeIdx int
	joints     map[armPart]float64
	endpoints  map[armPart]float64
	buttons    map[input.Control]bool
}

// state of control, event, axis, mode, command.
func (cs *controllerState) init() {
	cs.event = noop
	cs.curModeIdx = 0
	cs.endpoints = map[armPart]float64{
		x:     0.0,
		y:     0.0,
		z:     0.0,
		ox:    0.0,
		oy:    0.0,
		oz:    0.0,
		theta: 0.0,
	}
	cs.joints = map[armPart]float64{
		jointOne:   0.0,
		jointTwo:   0.0,
		jointThree: 0.0,
		jointFour:  0.0,
		jointFive:  0.0,
		jointSix:   0.0,
	}
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

// how to do mapping.
func (cs *controllerState) set(event input.Event, remoteConfig Config) {
	mappings := remoteConfig.ControllerModes[cs.curModeIdx].Mappings
	//exhaustive:ignore
	switch event.Event {
	case input.ButtonPress:
		cs.event = buttonPressed
		cs.buttons[event.Control] = !cs.buttons[event.Control]
	case input.ButtonRelease:
		cs.event = noop
		cs.buttons[event.Control] = !cs.buttons[event.Control]
	case input.PositionChangeAbs:
		ap := mappings[event.Control]
		switch ap {
		case jointOne, jointTwo, jointThree, jointFour, jointFive, jointSix:
			cs.event = jointEvent
			cs.joints[ap] = event.Value
		case x, y, z, ox, oy, oz, theta:
			cs.event = endPointEvent
			cs.endpoints[ap] = event.Value
		default:
			cs.event = noop
		}
	default:
		cs.event = noop
	}
}

// reset state.
func (cs *controllerState) reset() {
	cs.event = noop
	for k := range cs.endpoints {
		cs.endpoints[k] = 0.0
	}
	for k := range cs.joints {
		cs.joints[k] = 0.0
	}
	for k := range cs.buttons {
		cs.buttons[k] = false
	}
}

// isInvalid: currently assume sensitivity is 0-5.
func (cs *controllerState) isInvalid(sensitivity float64) bool {
	sensitivity = (94 + sensitivity) * 0.01
	//exhaustive:ignore
	switch cs.event {
	case jointEvent:
		for _, val := range cs.joints {
			if math.Abs(val)-sensitivity > 0 {
				return false
			}
		}
		return true
	case endPointEvent:
		for _, val := range cs.endpoints {
			if math.Abs(val)-sensitivity > 0 {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// RemoteService is the structure of the remote service.
type armRemoteService struct {
	arm             arm.Arm
	inputController input.Controller
	config          *Config
	logger          golog.Logger
}

var _ = resource.Reconfigurable(&reconfigurableArmRemoteControl{})

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

	armRemoteSvc := &armRemoteService{
		arm:             arm1,
		inputController: controller,
		config:          svcConfig,
		logger:          logger,
	}

	if err := armRemoteSvc.start(ctx); err != nil {
		return nil, errors.Errorf("error with starting remote control service: %q", err)
	}

	return armRemoteSvc, nil
}

// Start is the main control loops for sending events from controller to base.
func (svc *armRemoteService) start(ctx context.Context) error {
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
func (svc *armRemoteService) Close(ctx context.Context) error {
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
func (svc *armRemoteService) controllerInputs() []input.Control {
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

func (svc *armRemoteService) processEvent(ctx context.Context, state *controllerState, event input.Event) error {
	// set state to be executed
	state.set(event, *svc.config)
	// execute stated arm control
	if err := processArmControllerEvent(ctx, svc, state); err != nil {
		state.reset()
		return err
	}
	// reset state
	state.reset()
	return nil
}

type reconfigurableArmRemoteControl struct {
	mu     sync.RWMutex
	actual *armRemoteService
}

func (svc *reconfigurableArmRemoteControl) Close(ctx context.Context) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return viamutils.TryClose(ctx, svc.actual)
}

func (svc *reconfigurableArmRemoteControl) Reconfigure(ctx context.Context, newSvc resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newSvc.(*reconfigurableArmRemoteControl)
	if !ok {
		return utils.NewUnexpectedTypeError(svc, newSvc)
	}
	if err := viamutils.TryClose(ctx, &svc.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// WrapWithReconfigurable wraps a BaseRemoteControl as a Reconfigurable.
func WrapWithReconfigurable(s interface{}) (resource.Reconfigurable, error) {
	if reconfigurable, ok := s.(*reconfigurableArmRemoteControl); ok {
		return reconfigurable, nil
	}

	svc, ok := s.(armRemoteService)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(&armRemoteService{}, s)
	}

	return &reconfigurableArmRemoteControl{actual: &svc}, nil
}

// processCommandEvent should properly map to arm control functions.
func processCommandEvent(ctx context.Context, svc *armRemoteService, state *controllerState) error {
	switch {
	case state.buttons[input.ButtonSouth]:
		svc.logger.Info("stopping arm")
		return svc.arm.Stop(ctx, nil)
	case state.buttons[input.ButtonEast]:
		svc.logger.Debug("previewing pose [TODO]")
	case state.buttons[input.ButtonWest]:
		// move through state
		prevMode := svc.config.ControllerModes[state.curModeIdx].ModeName
		state.curModeIdx = (state.curModeIdx + 1) % len(svc.config.ControllerModes)
		svc.logger.Infof("switched joint control(from:%s,to:%s)", prevMode, svc.config.ControllerModes[state.curModeIdx].ModeName)
	case state.buttons[input.ButtonNorth]:
		svc.logger.Debug("executing named pose [TODO]")
	case state.buttons[input.ButtonLT]:
		svc.logger.Debug("closing pincer [TODO]")
	case state.buttons[input.ButtonRT]:
		svc.logger.Debug("opening pincer [TODO]")
	case state.buttons[input.ButtonSelect]:
		svc.logger.Debug("disable collision avoidance [TODO]")
	case state.buttons[input.ButtonStart]:
		svc.logger.Debug("enable collision avoidance [TODO]")
	case state.buttons[input.ButtonMenu]:
		svc.logger.Debug("change joint group [TODO]")
	default:
		return errors.New("invalid button option")
	}
	return nil
}

func processArmEndPointEvent(ctx context.Context, svc *armRemoteService, state *controllerState) error {
	if state.isInvalid(svc.config.ControllerSensitivity) {
		return nil
	}

	currentPose, err := svc.arm.GetEndPosition(ctx, nil)
	if err != nil {
		return err
	}

	poseStep := svc.config.DefaultPoseStep
	currentPose.Y += (math.Round(state.endpoints[x]) * poseStep)
	currentPose.X += (math.Round(state.endpoints[y]) * poseStep)
	currentPose.Z += (math.Round(state.endpoints[z]) * poseStep)
	currentPose.OX += (math.Round(state.endpoints[ox]) * poseStep)
	currentPose.OY += (math.Round(state.endpoints[oy]) * poseStep)
	currentPose.OZ += (math.Round(state.endpoints[oz]) * poseStep)
	currentPose.Theta += (math.Round(state.endpoints[theta]) * poseStep)

	err = svc.arm.MoveToPosition(ctx, currentPose, nil, nil)
	if err != nil {
		return err
	}
	return nil
}

func processArmJointEvent(ctx context.Context, svc *armRemoteService, state *controllerState) error {
	if state.isInvalid(svc.config.ControllerSensitivity) {
		return nil
	}

	jointPositions, err := svc.arm.GetJointPositions(ctx, nil)
	if err != nil {
		return err
	}

	jointStep := svc.config.DefaultJointStep
	jointPositions.Values[0] += (math.Round(state.joints[jointOne]) * jointStep)
	jointPositions.Values[1] += (math.Round(state.joints[jointTwo]) * jointStep)
	jointPositions.Values[2] += (math.Round(state.joints[jointThree]) * jointStep)
	jointPositions.Values[3] += (math.Round(state.joints[jointFour]) * jointStep)
	jointPositions.Values[4] += (math.Round(state.joints[jointFive]) * jointStep)
	jointPositions.Values[5] += (math.Round(state.joints[jointSix]) * jointStep)

	err = svc.arm.MoveToJointPositions(ctx, jointPositions, nil)
	if err != nil {
		return err
	}
	return nil
}

func processArmControllerEvent(ctx context.Context, svc *armRemoteService, state *controllerState) error {
	//exhaustive:ignore
	switch state.event {
	case endPointEvent:
		return processArmEndPointEvent(ctx, svc, state)
	case jointEvent:
		return processArmJointEvent(ctx, svc, state)
	case buttonPressed:
		return processCommandEvent(ctx, svc, state)
	}
	return nil
}
