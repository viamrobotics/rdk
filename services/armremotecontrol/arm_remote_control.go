// Package armremotecontrol implements a remote control for a arm.
package armremotecontrol

import (
	"context"
	"math"
	"strconv"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	spatial "go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

// constants, joints are constants till we can can get joint name from model.
const (
	noop = controllerEvent(iota) // controller events
	jointEvent
	endPointEvent
	buttonPressed
	jointMode                    = mode("joints") // arm modes supported
	endpointMode                 = mode("endpoints")
	defaultJointStep             = 10.0
	defaultMMStep                = 0.1
	defaultDegreeStep            = 5.0
	defaultControllerSensitivity = 5.0
	SubtypeName                  = resource.SubtypeName("arm_remote_control") // resource name
)

// Subtype is a constant that identifies the remote control resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Name is the ArmRemoteControlService's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

func init() {
	registry.RegisterService(Subtype, registry.Service{Constructor: New})
	cType := config.ServiceType(SubtypeName)
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
	})

	config.RegisterServiceAttributeMapConverter(cType, func(attributes config.AttributeMap) (interface{}, error) {
		var conf ServiceConfig
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &conf})
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(attributes); err != nil {
			return nil, err
		}
		return &conf, nil
	}, &ServiceConfig{})
}

type (
	controllerEvent uint8
	mode            string
)

// ServiceConfig describes how to configure the service.
type ServiceConfig struct {
	ArmName               string  `json:"arm"`
	InputControllerName   string  `json:"input_controller"`
	JointStep             float64 `json:"joint_step,omitempty"`   // scale Joint movement in degrees (defaults to 10)
	DegreeStep            float64 `json:"degree_step,omitempty"`  // scale roll, pitch, yaw default 0.1
	MMStep                float64 `json:"mm_step,omitempty"`      // scale x, y, z in millimeters (deafaults to 5)
	ControllerSensitivity float64 `json:"controller_sensitivity"` // joystick sensitivity
	// only respond to events where: abs(+-1) - sensitivity > 0
	ControllerModes []ControllerMode `json:"controller_modes"` // modes of operation for arm (joint or endpoint/pose control)
}

// ControllerMode supports mapping in joint or endpoint configuration.
type ControllerMode struct {
	ModeName       mode                     `json:"mode_name"`
	ControlMapping map[string]input.Control `json:"control_mapping"`
}

// controllerState.
type controllerState struct {
	event      controllerEvent        // type of event to execute
	curModeIdx int                    // current controller mode
	buttons    map[input.Control]bool // controller button pressed - which command to execute
}

// Validate ensures configuration is valid.
func (config *ServiceConfig) Validate(path string) ([]string, error) {
	deps, err := config.validate()
	if err != nil {
		err = utils.NewConfigValidationError(path, err)
	}
	return deps, err
}

func (config *ServiceConfig) validate() ([]string, error) {
	var deps []string

	if len(config.ArmName) == 0 {
		return nil, errors.New("a configured arm name must be provided")
	}
	deps = append(deps, config.ArmName)

	if len(config.InputControllerName) == 0 {
		return nil, errors.New("a configured input controller name must be provided")
	}
	deps = append(deps, config.InputControllerName)

	if config.JointStep <= 0 {
		return nil, errors.New("Joint Step needs to be greater than 0")
	}

	if config.DegreeStep <= 0 {
		return nil, errors.New("Degree step needs to be greater than 0")
	}

	if config.MMStep <= 0 {
		return nil, errors.New("MM step must be greater than 0")
	}

	if config.ControllerSensitivity <= 0 {
		return nil, errors.New("Controller sensitivity cannot be 0")
	}

	if len(config.ControllerModes) == 0 {
		return nil, errors.New("At least one arm controller mode needs to be provided")
	}

	return deps, nil
}

// state of control, event, axis, mode, command.
func (cs *controllerState) init() {
	cs.event = noop
	cs.curModeIdx = 0
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

func (cs *controllerState) set(event input.Event, svcConfig ServiceConfig) {
	//exhaustive:ignore
	switch event.Event {
	case input.ButtonPress:
		cs.event = buttonPressed
		cs.buttons[event.Control] = !cs.buttons[event.Control]
	case input.ButtonRelease:
		cs.event = noop
		cs.buttons[event.Control] = !cs.buttons[event.Control]
	case input.PositionChangeAbs:
		modeName := svcConfig.ControllerModes[cs.curModeIdx].ModeName
		switch modeName {
		case jointMode:
			cs.event = jointEvent
		case endpointMode:
			cs.event = endPointEvent
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
	for k := range cs.buttons {
		cs.buttons[k] = false
	}
}

// armRemoteService is the structure of the arm remote service.
type armRemoteService struct {
	arm             arm.Arm
	inputController input.Controller
	config          *ServiceConfig
	logger          golog.Logger
}

var _ = resource.Reconfigurable(&reconfigurableArmRemoteControl{})

// New returns a new remote control service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (interface{}, error) {
	svcConfig, ok := config.ConvertedAttributes.(*ServiceConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(svcConfig, config.ConvertedAttributes)
	}

	arm1, err := arm.FromRobot(r, svcConfig.ArmName)
	if err != nil {
		return nil, err
	}
	controller, err := input.FromRobot(r, svcConfig.InputControllerName)
	if err != nil {
		return nil, err
	}

	// setup defaults for omitempty requirements
	if svcConfig.ControllerSensitivity == 0.0 {
		svcConfig.ControllerSensitivity = defaultControllerSensitivity
	}

	if svcConfig.DegreeStep == 0.0 {
		svcConfig.DegreeStep = defaultDegreeStep
	}

	if svcConfig.JointStep == 0.0 {
		svcConfig.JointStep = defaultJointStep
	}

	if svcConfig.MMStep == 0.0 {
		svcConfig.MMStep = defaultMMStep
	}

	// ensure we are mapped to degree of freedoms
	dofLen := len(arm1.ModelFrame().DoF())
	for _, mode := range svcConfig.ControllerModes {
		if mode.ModeName == jointMode {
			if len(mode.ControlMapping) != dofLen {
				return nil, errors.New("Degree of Freedom mapping not correct")
			}
		}
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

// Start is the main control loops for sending events from controller to arm.
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

	controls, err := svc.inputController.GetControls(ctx)
	if err != nil {
		return err
	}

	for _, control := range controls {
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
	controls, err := svc.inputController.GetControls(ctx)
	if err != nil {
		return err
	}

	for _, control := range controls {
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

func (svc *armRemoteService) processEvent(ctx context.Context, state *controllerState, event input.Event) error {
	// set state to be executed
	state.set(event, *svc.config)
	defer state.reset()
	// execute stated arm control
	if err := processArmControllerEvent(ctx, svc, state, event); err != nil {
		return err
	}
	return nil
}

type reconfigurableArmRemoteControl struct {
	mu     sync.RWMutex
	actual *armRemoteService
}

func (svc *reconfigurableArmRemoteControl) Close(ctx context.Context) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return utils.TryClose(ctx, svc.actual)
}

func (svc *reconfigurableArmRemoteControl) Reconfigure(ctx context.Context, newSvc resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newSvc.(*reconfigurableArmRemoteControl)
	if !ok {
		return rdkutils.NewUnexpectedTypeError(svc, newSvc)
	}
	if err := utils.TryClose(ctx, &svc.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// WrapWithReconfigurable wraps a ArmRemoteControl as a Reconfigurable.
func WrapWithReconfigurable(s interface{}) (resource.Reconfigurable, error) {
	if reconfigurable, ok := s.(*reconfigurableArmRemoteControl); ok {
		return reconfigurable, nil
	}

	svc, ok := s.(armRemoteService)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(&armRemoteService{}, s)
	}

	return &reconfigurableArmRemoteControl{actual: &svc}, nil
}

// isInvalid: currently assume sensitivity is 0-5.
func isInvalid(sensitivity float64, val float64) bool {
	sensitivity = (94 + sensitivity) * 0.01
	return math.Abs(val)-sensitivity <= 0
}

// processCommandEvent should properly map to arm control functions.
func processCommandEvent(ctx context.Context, svc *armRemoteService, state *controllerState) error {
	switch {
	case state.buttons[input.ButtonSouth]:
		svc.logger.Debug("stopping arm")
		return svc.arm.Stop(ctx, nil)
	case state.buttons[input.ButtonWest]:
		// move through state
		state.curModeIdx = (state.curModeIdx + 1) % len(svc.config.ControllerModes)
		svc.logger.Debug("switched joint control")
	default:
		return nil
	}
	return nil
}

func getEventValue(event input.Event, control input.Control, step float64) float64 {
	if event.Control == control {
		return math.Round(event.Value) * step
	}
	return 0.0
}

func processArmEndPointEvent(ctx context.Context, svc *armRemoteService, state *controllerState, event input.Event) error {
	if isInvalid(svc.config.ControllerSensitivity, event.Value) {
		return nil
	}

	currentPoseBuf, err := svc.arm.GetEndPosition(ctx, nil)
	if err != nil {
		return err
	}

	mappings := svc.config.ControllerModes[state.curModeIdx].ControlMapping
	mmStep := svc.config.MMStep
	degStep := svc.config.DegreeStep

	offSetPoseVector := r3.Vector{}
	offSetEulerAngles := spatial.NewEulerAngles()

	for key, control := range mappings {
		switch key {
		case "x":
			offSetPoseVector.X = getEventValue(event, control, mmStep)
		case "y":
			offSetPoseVector.Y = getEventValue(event, control, mmStep)
		case "z":
			offSetPoseVector.Z = getEventValue(event, control, mmStep)
		case "roll":
			offSetEulerAngles.Roll = getEventValue(event, control, degStep)
		case "pitch":
			offSetEulerAngles.Pitch = getEventValue(event, control, degStep)
		case "yaw":
			offSetEulerAngles.Yaw = getEventValue(event, control, degStep)
		default:
			return errors.New("Invalid endpoint key")
		}
	}

	currentPose := spatial.NewPoseFromProtobuf(currentPoseBuf)
	offsetPose := spatial.NewPoseFromOrientation(offSetPoseVector, offSetEulerAngles)
	newPose := spatial.Compose(currentPose, offsetPose)

	err = svc.arm.MoveToPosition(ctx, spatial.PoseToProtobuf(newPose), nil, nil)
	if err != nil {
		return err
	}
	return nil
}

func processArmJointEvent(ctx context.Context, svc *armRemoteService, state *controllerState, event input.Event) error {
	if isInvalid(svc.config.ControllerSensitivity, event.Value) {
		return nil
	}

	mappings := svc.config.ControllerModes[state.curModeIdx].ControlMapping
	jointStep := svc.config.JointStep

	jointPositions, err := svc.arm.GetJointPositions(ctx, nil)
	if err != nil {
		return err
	}

	for key, control := range mappings {
		keyInt, err := strconv.Atoi(key)
		if err != nil {
			return errors.New("cannot convert joint key to integer")
		}
		jointPositions.Values[keyInt] += getEventValue(event, control, jointStep)
	}

	err = svc.arm.MoveToJointPositions(ctx, jointPositions, nil)
	if err != nil {
		return err
	}
	return nil
}

func processArmControllerEvent(ctx context.Context, svc *armRemoteService, state *controllerState, event input.Event) error {
	//exhaustive:ignore
	switch state.event {
	case endPointEvent:
		return processArmEndPointEvent(ctx, svc, state, event)
	case jointEvent:
		return processArmJointEvent(ctx, svc, state, event)
	case buttonPressed:
		return processCommandEvent(ctx, svc, state)
	}
	return nil
}
