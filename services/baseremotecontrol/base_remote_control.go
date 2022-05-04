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
	maxSpeed    = 500.0
	maxAngle    = 425.0
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
	var ButtonNorth bool
	var ButtonEast bool
	var ButtonSouth bool
	var ButtonWest bool

	var mmPerSec float64
	var angleDeg float64

	remoteCtl := func(ctx context.Context, event input.Event) {
		//fmt.Printf("%s:%s: %.4f\n", event.Control, event.Event, event.Value)
		if event.Event != input.ButtonPress && event.Event != input.ButtonRelease {
			return
		}

		if event.Event == input.ButtonPress {
			switch event.Control {
			case input.ButtonNorth:
				ButtonNorth = true
			case input.ButtonSouth:
				ButtonSouth = true
			case input.ButtonEast:
				ButtonEast = true
			case input.ButtonWest:
				ButtonWest = true
			}
		}

		if event.Event == input.ButtonRelease {
			switch event.Control {
			case input.ButtonNorth:
				ButtonNorth = false
			case input.ButtonSouth:
				ButtonSouth = false
			case input.ButtonEast:
				ButtonEast = false
			case input.ButtonWest:
				ButtonWest = false
			}
		}

		// Speed
		if ButtonNorth && ButtonSouth {
			mmPerSec = 0.0
		} else if ButtonNorth {
			mmPerSec = 1.0
		} else if ButtonSouth {
			mmPerSec = -1.0			
		} else {
			mmPerSec = 0.0		
		}

		// Angle
		if ButtonEast && ButtonWest {
			angleDeg = 0.0
		} else if ButtonEast {
			angleDeg = -1.0
		} else if ButtonWest {
			angleDeg = 1.0			
		} else {
			angleDeg = 0.0		
		}

		var err error
		if mmPerSec == 0 && angleDeg == 0 {
			// Stop
			d := int(maxSpeed*distRatio)
			s := 0.0
			a := angleDeg*maxAngle*-1
			fmt.Printf("Stop: s = %v | a = %v | Dist = %v | Speed = %v | Angle = %v\n", mmPerSec, angleDeg, d, s, a)
			err = svc.base.MoveArc(ctx, d, s, a, true)
		} else if mmPerSec == 0 {
			// Spin
			d := int(0)
			s := angleDeg*maxSpeed
			a := math.Abs(angleDeg*maxAngle*distRatio/2)
			fmt.Printf("Spin: s = %v | a = %v | Dist = %v | Speed = %v | Angle = %v\n", mmPerSec, angleDeg, d, s, a)
			err = svc.base.MoveArc(ctx, d, s, a, true)
		} else if angleDeg == 0 {
			// Move Straight
			d := int(math.Abs(mmPerSec*maxSpeed*distRatio))
			s := mmPerSec*maxSpeed
			a := math.Abs(angleDeg*maxAngle*distRatio)
			fmt.Printf("Straight: s = %v | a = %v | Dist = %v | Speed = %v | Angle = %v\n", mmPerSec, angleDeg, d, s, a)
			err = svc.base.MoveArc(ctx, d, s, a, true)		
		} else {
			// Move Arc
			d := int(math.Abs(mmPerSec*maxSpeed*distRatio))
			s := mmPerSec*maxSpeed
			a := angleDeg*maxAngle*distRatio*2-1
			fmt.Printf("Arc: s = %v | a = %v | Dist = %v | Speed = %v | Angle = %v\n", mmPerSec, angleDeg, d, s, a)
			err = svc.base.MoveArc(ctx, d, s, a, true)
		}

		if err != nil {
			svc.logger.Errorw("error with moving base to desired position", "error", err)
		}
	}

	for _, control := range []input.Control{input.ButtonSouth, input.ButtonEast, input.ButtonWest, input.ButtonNorth} {
		err := svc.inputController.RegisterControlCallback(ctx, control, []input.EventType{input.ButtonChange}, remoteCtl)
		if err != nil {
			return err
		}
	}
	return nil
}

// Close out of all remote control related systems.
func (svc *remoteService) Close(ctx context.Context) error {
	for _, control := range []input.Control{input.ButtonSouth, input.ButtonEast, input.ButtonWest, input.ButtonNorth}  {
		err := svc.inputController.RegisterControlCallback(ctx, control, []input.EventType{input.ButtonChange}, nil)
		if err != nil {
			return err
		}
	}
	return nil
}
