// Package builtin implements a remote control for a arm.
package builtin

import (
	"context"
	"math"
	"strconv"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/armremotecontrol"
	"go.viam.com/rdk/session"
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

func init() {
	registry.RegisterService(armremotecontrol.Subtype, resource.DefaultModelName, registry.Service{
		Constructor: func(ctx context.Context, deps registry.Dependencies, c config.Service, logger golog.Logger) (interface{}, error) {
			return NewBuiltIn(ctx, deps, c, logger)
		},
	})
	cType := config.ServiceType(armremotecontrol.SubtypeName)
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
	DegreeStep            float64 `json:"step_degs,omitempty"`    // scale roll, pitch, yaw default 0.1
	MMStep                float64 `json:"step_mm,omitempty"`      // scale x, y, z in millimeters (deafaults to 5)
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
	mu         sync.Mutex
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

	// setup defaults for omitempty and validate entries
	if config.ControllerSensitivity == 0.0 {
		config.ControllerSensitivity = defaultControllerSensitivity
	} else if config.ControllerSensitivity <= 0.0 {
		return nil, errors.New("Controller sensitivity must be greater than 0")
	}

	if config.DegreeStep == 0.0 {
		config.DegreeStep = defaultDegreeStep
	} else if config.DegreeStep <= 0.0 {
		return nil, errors.New("Degree step needs to be greater than 0")
	}

	if config.JointStep == 0.0 {
		config.JointStep = defaultJointStep
	} else if config.JointStep <= 0.0 {
		return nil, errors.New("Joint Step needs to be greater than 0")
	}

	if config.MMStep == 0.0 {
		config.MMStep = defaultMMStep
	} else if config.MMStep <= 0 {
		return nil, errors.New("MM step must be greater than 0")
	}

	if len(config.ControllerModes) == 0 {
		return nil, errors.New("At least one arm controller mode needs to be provided")
	}

	return deps, nil
}

// state of control, event, axis, mode, command.
func (cs *controllerState) init() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
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
	cs.mu.Lock()
	defer cs.mu.Unlock()
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
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.event = noop
	for k := range cs.buttons {
		cs.buttons[k] = false
	}
}

// BuiltIn is the structure of the arm remote service.
type builtIn struct {
	arm             arm.Arm
	inputController input.Controller
	config          *ServiceConfig
	logger          golog.Logger

	cancel                  func()
	cancelCtx               context.Context
	activeBackgroundWorkers sync.WaitGroup
}

// NewDefault returns a new remote control service for the given robot.
func NewBuiltIn(
	ctx context.Context,
	deps registry.Dependencies,
	config config.Service,
	logger golog.Logger,
) (armremotecontrol.Service, error) {
	svcConfig, ok := config.ConvertedAttributes.(*ServiceConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(svcConfig, config.ConvertedAttributes)
	}

	armComponent, err := arm.FromDependencies(deps, svcConfig.ArmName)
	if err != nil {
		return nil, err
	}

	controller, err := input.FromDependencies(deps, svcConfig.InputControllerName)
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

	modelFrame := armComponent.ModelFrame()
	if modelFrame == nil {
		return nil, errors.New("arm modelframe not found, validate config")
	}

	// ensure we are mapped to degree of freedoms
	dofLen := len(armComponent.ModelFrame().DoF())
	for _, mode := range svcConfig.ControllerModes {
		if mode.ModeName == jointMode {
			if len(mode.ControlMapping) != dofLen {
				return nil, errors.New("Degree of Freedom mapping not correct")
			}
		}
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	armRemoteSvc := &builtIn{
		arm:             armComponent,
		inputController: controller,
		config:          svcConfig,
		logger:          logger,
		cancelCtx:       cancelCtx,
		cancel:          cancel,
	}

	if err := armRemoteSvc.start(ctx); err != nil {
		return nil, errors.Errorf("error with starting remote control service: %q", err)
	}

	return armRemoteSvc, nil
}

// Start is the main control loops for sending events from controller to arm.
func (svc *builtIn) start(ctx context.Context) error {
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

		svc.processEvent(ctx, state, event)
	}

	controls, err := svc.inputController.Controls(ctx, map[string]interface{}{})
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
			map[string]interface{}{},
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// Close out of all remote control related systems.
func (svc *builtIn) Close(ctx context.Context) error {
	svc.cancel()
	svc.activeBackgroundWorkers.Wait()

	controls, err := svc.inputController.Controls(ctx, map[string]interface{}{})
	if err != nil {
		return err
	}

	for _, control := range controls {
		err := svc.inputController.RegisterControlCallback(
			ctx,
			control,
			[]input.EventType{input.ButtonChange, input.PositionChangeAbs},
			nil,
			map[string]interface{}{},
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (svc *builtIn) processEvent(ctx context.Context, state *controllerState, event input.Event) {
	// This would be better deeper down in the processing but it's not trivial to move
	// the defer of state.reset around right now. That means it assumes any event we register
	// is considered safety monitor which is not 100% true.
	session.SafetyMonitor(ctx, svc.arm)

	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		// set state to be executed
		state.set(event, *svc.config)
		defer state.reset()

		// execute stated arm control
		if err := processArmControllerEvent(ctx, svc, state, event); err != nil {
			svc.logger.Errorw("error with moving arm to desired position", "error", err)
		}
	})
}

// isInvalid: currently assume sensitivity is 0-5.
func isInvalid(sensitivity, val float64) bool {
	sensitivity = (94 + sensitivity) * 0.01
	return math.Abs(val)-sensitivity <= 0
}

// processCommandEvent should properly map to arm control functions.
func processCommandEvent(ctx context.Context, svc *builtIn, state *controllerState) error {
	state.mu.Lock()
	switch {
	case state.buttons[input.ButtonSouth]:
		state.mu.Unlock()
		svc.logger.Debug("stopping arm")
		return svc.arm.Stop(ctx, nil)
	case state.buttons[input.ButtonWest]:
		// move through state
		state.curModeIdx = (state.curModeIdx + 1) % len(svc.config.ControllerModes)
		state.mu.Unlock()
		svc.logger.Debug("switched joint control")
	default:
		state.mu.Unlock()
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

func processArmEndPointEvent(ctx context.Context, svc *builtIn, state *controllerState, event input.Event) error {
	if isInvalid(svc.config.ControllerSensitivity, event.Value) {
		return nil
	}

	currentPose, err := svc.arm.EndPosition(ctx, nil)
	if err != nil {
		return err
	}

	state.mu.Lock()
	mappings := svc.config.ControllerModes[state.curModeIdx].ControlMapping
	state.mu.Unlock()
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

	offsetPose := spatial.NewPoseFromOrientation(offSetPoseVector, offSetEulerAngles)
	newPose := spatial.Compose(currentPose, offsetPose)

	err = svc.arm.MoveToPosition(ctx, newPose, nil, nil)
	if err != nil {
		return err
	}
	return nil
}

func processArmJointEvent(ctx context.Context, svc *builtIn, state *controllerState, event input.Event) error {
	if isInvalid(svc.config.ControllerSensitivity, event.Value) {
		return nil
	}

	state.mu.Lock()
	mappings := svc.config.ControllerModes[state.curModeIdx].ControlMapping
	state.mu.Unlock()
	jointStep := svc.config.JointStep

	jointPositions, err := svc.arm.JointPositions(ctx, nil)
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

func processArmControllerEvent(ctx context.Context, svc *builtIn, state *controllerState, event input.Event) error {
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
