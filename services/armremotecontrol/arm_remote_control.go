// Package baseremotecontrol implements a remote control for a base.
// TODO:
// - reintroduce throttle state
// - movement calculations
// - button map changing
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

// constants for arm controll
// TODO: determine controls types needed
const (
	jointByJointControl = controlMode(iota)
	endPointControl
	SubtypeName = resource.SubtypeName("arm_remote_control")
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

	config *Config
	logger golog.Logger
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
		controlMode1 = jointByJoint
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
			ctx, control,
			[]input.EventType{input.ButtonChange, input.PositionChangeAbs},
			nil,
		)
		if err != nil {
			return err
		}
	}
	svc.logger.Infof("Arm Controller service started with intial mode of: %s", svc.controlMode)
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
		input.ButtonLThumb,
		input.ButtonRThumb,
		input.ButtonSelect,
		input.ButtonStart,
		input.ButtonMenu,
	}
}

func parseEndPointEvent(event input.Event) (msg, bool, err) {
	switch event.Control {
	case input.AbsoluteX:
		return "[end-point] Z Rotation", false, nil
	case input.AbsoluteY:
		return "[end-point] Y Rotation", false, nil
	case input.AbsoluteZ:
		return "[end-point] X Rotation", false, nil
	case input.AbsoluteRX:
		return "[end-point] X Linear", false, nil
	case input.AbsoluteRY:
		return "[end-point] Y Linear", false, nil
	case input.AbsoluteRZ:
		return "[end-point] X Rotation", false, nil
	case input.AbsoluteHat0X:
		return "[end-point] NOTHING", false, nil
	case input.AbsoluteHat0Y:
		return "[end-point] Z Linear", false, nil
	case input.ButtonSouth:
		return "[end-point] STOP", false, nil
	case input.ButtonEast:
		return "[end-point] Named Pose Preview", false, nil
	case input.ButtonWest:
		return "[end-point] control mode pressed, will switch modes", true, nil
	case input.ButtonNorth:
		return "[end-point] Named Pose Execute", false, nil
	case input.ButtonLT:
		return "[end-point] X Rotation Possibly ()", false, nil
	case input.ButtonRT:
		return "[end-point] X Rotation Possibly ()", false, nil
	case input.ButtonLThumb:
		return "[end-point] Pincer Close (trigger)", false, nil
	case input.ButtonRThumb:
		return "[end-point] Pincer Open (trigger)", false, nil
	case input.ButtonSelect:
		return "[end-point] Disable collision avoidance", false, nil
	case input.ButtonStart:
		return "[end-point] Change Joint Groups", false, nil
	case input.ButtonMenu:
		return "[end-point] Enable collision avoidance", false, nil
	default:
		return nil, nil, errors.New("invalid button for mode")
	}

	return msg, controlMode, nil

}

func parseJointByJointEvent(event input.Event) bool {
	switch event.Control {
	case input.AbsoluteX:
		return "[joint-by-joint] Drive 5", false, nil
	case input.AbsoluteY:
		return "[joint-by-joint] Drive 6", false, nil
	case input.AbsoluteZ:
		return "[joint-by-joint] Z Pincer Close (Trigger Maybe)", false, nil
	case input.AbsoluteRX:
		return "[joint-by-joint] Drive 4", false, nil
	case input.AbsoluteRY:
		return "[joint-by-joint] Drive 3", false, nil
	case input.AbsoluteRZ:
		return "[joint-by-joint] RZ Pincer Open (Trigger Maybe)", false, nil
	case input.AbsoluteHat0X:
		return "[joint-by-joint] Drive 1", false, nil
	case input.AbsoluteHat0Y:
		return "[joint-by-joint] Drive 2", false, nil
	case input.ButtonSouth:
		return "[joint-by-joint] STOP", false, nil
	case input.ButtonEast:
		return "[joint-by-joint] Named Pose Preview", false, nil
	case input.ButtonWest:
		return "[joint-by-joint] control mode pressed, will switch modes", true, nil
	case input.ButtonNorth:
		return "[joint-by-joint] Named Pose Execute", false, nil
	case input.ButtonLT:
		return "[joint-by-joint] Pincer Open (trigger)", false, nil
	case input.ButtonRT:
		return "[joint-by-joint] Pincer Close (trigger)", false, nil
	case input.ButtonLThumb:
		return "[joint-by-joint] Drive 7", false, nil
	case input.ButtonRThumb:
		return "[joint-by-joint] Drive 7", false, nil
	case input.ButtonSelect:
		return "[joint-by-joint] enable collision detection", false, nil
	case input.ButtonStart:
		return "[joint-by-joint] change joint groups", false, nil
	case input.ButtonMenu:
		return "[joint-by-joint] disable collision avoidance", false, nil
	default:
		return nil, nil, errors.New("invalid button for mode")
	}
}

// TODO: This code is terrible clean it up
func (svc *remoteService) processEvent(ctx context.Context, event input.Event) error {
	var msg string
	var switchMode bool
	var err error
	switch svc.controlMode {
	case endPointControl:
		msg, switchMode, err = parseEndPointEvent(event)
	case jointByJointControl:
		msg, switchMode, err = parseJointByJointEvent(event)
	}

	if err != nil {
		svc.logger.Errorw("error processing event", "error", err)
		return err
	}

	svc.logger.Info("%s", msg)
	if switchMode {
		svc.logger.Info("switch arm control mode")
		if svc.controlMode == endPointControl {
			svc.controlMode = jointByJointControl
		} else {
			svc.controlMode = endPointControl
		}
	}

	return nil
}
