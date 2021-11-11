package remotecontrol

import (
	"context"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"github.com/mitchellh/mapstructure"
	"go.viam.com/utils"

	"go.viam.com/core/base"
	"go.viam.com/core/config"
	"go.viam.com/core/input"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	// Register package
	_ "go.viam.com/core/input/gamepad"
)

// Type is the type of service.
const Type = config.ServiceType("remote-control")

// Initialize remote-control service with main server
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

// Config describes how to configure the service.
type Config struct {
	BaseName            string `json:"base"`
	InputControllerName string `json:"input_controller"`
}

// Validate ensures all parts of the config are valid.
func (config *Config) Validate(path string) error {
	// if err := config.Store.Validate(fmt.Sprintf("%s.%s", path, "store")); err != nil {
	// 	return err
	// }
	if config.BaseName == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "base")
	}
	if config.InputControllerName == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "input_controller")
	}
	return nil
}

// --------------------------------------------------------------------------------------------------

// A Service controls the navigation for a robot.
type Service interface {
	Start(context.Context) error
	//Mode(ctx context.Context) (Mode, error)
	//SetMode(ctx context.Context, mode Mode) error
	Close() error
}

// Mode describes what mode to operate the service in.
type Mode uint8

// The set of known modes.
// const (
// 	ModeManual = Mode(iota)
// 	ModeRemote
// )

// func (svc *remoteService) Mode(ctx context.Context) (Mode, error) {
// 	svc.mu.RLock()
// 	defer svc.mu.RUnlock()
// 	return svc.mode, nil
// }

// func (svc *remoteService) SetMode(ctx context.Context, mode Mode) error {
// 	svc.mu.Lock()
// 	defer svc.mu.Unlock()
// 	if svc.mode == mode {
// 		return nil
// 	}

// 	// switch modes
// 	svc.cancelFunc()
// 	svc.activeBackgroundWorkers.Wait()
// 	cancelCtx, cancelFunc := context.WithCancel(context.Background())
// 	svc.cancelCtx = cancelCtx
// 	svc.cancelFunc = cancelFunc

// 	svc.mode = ModeManual
// 	switch mode {
// 	case ModeRemote:
// 		if err := svc.startRemote(ctx); err != nil {
// 			return err
// 		}
// 		svc.mode = mode
// 	}
// 	return nil
// }

// Close out all remote control related systems
func (svc *remoteService) Close() error {
	if svc.cancelFunc != nil {
		svc.cancelFunc()
		svc.cancelFunc = nil
	}
	svc.activeBackgroundWorkers.Wait()
	return nil
}

type remoteService struct {
	//mu   sync.RWMutex
	r robot.Robot
	//mode Mode

	base            base.Base
	inputController input.Controller

	logger                  golog.Logger
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// New returns a new remote control service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	svcConfig := config.ConvertedAttributes.(*Config)
	base1, ok := r.BaseByName(svcConfig.BaseName)
	if !ok {
		return nil, errors.Errorf("no base named %q", svcConfig.BaseName)
	}
	controller, ok := r.InputControllerByName(svcConfig.InputControllerName)
	if !ok {
		return nil, errors.Errorf("no input controller named %q", svcConfig.InputControllerName)
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	remoteSvc := &remoteService{
		r:               r,
		base:            base1,
		inputController: controller,
		logger:          logger,
		cancelCtx:       cancelCtx,
		cancelFunc:      cancelFunc,
	}

	err := remoteSvc.Start(ctx)

	if err != nil {
		remoteSvc.logger.Errorw("error with starting remote control service", "error", err)
	}

	return remoteSvc, nil
}

// Starts background process of remote control
func (svc *remoteService) Start(ctx context.Context) error {
	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		if err := svc.StartRemote(ctx); err != nil {
			svc.logger.Errorw("error with remote control", "error", err)
		}
	})
	return nil
}

// Main control loops for sending events from controller to base
func (svc *remoteService) StartRemote(ctx context.Context) error {

	var speed float64
	var angle float64
	var millisPerSec float64
	var degPerSec float64

	maxSpeed := 100.0
	maxAngle := 40.0

	remoteCtl := func(ctx context.Context, event input.Event) {

		if event.Event != input.PositionChangeAbs {
			return
		}

		switch event.Control {
		case input.AbsoluteY:
			speed = event.Value

		case input.AbsoluteX:
			angle = event.Value
		}

		// Joystick angle + speed to millisPerSec and degPerSec (two variations)
		millisPerSec, degPerSec = speedAndAngleMathMag(speed, angle)
		//millisPerSec, degPerSec = speedAndAngleMathSquare(speed, angle)

		//fmt.SPrintf("Event = %v | Speed = %v Angle = %v | millisPerSec = %v degPerSec = %v     \n", event.Control, speed, angle, millisPerSec*-30, degPerSec*1)

		// Set distance to large number as it will be overwritten (Note: could have a dependecy on speed)
		_, err := svc.base.MoveArc(ctx, 1000, millisPerSec*maxSpeed*-1, degPerSec*maxAngle, true) //300 | 40

		if err != nil {
			svc.logger.Errorw("error with moving base to desired position", "error", err)
		}
	}

	for _, control := range []input.Control{input.AbsoluteY, input.AbsoluteX} {
		err := svc.inputController.RegisterControlCallback(ctx, control, []input.EventType{input.PositionChangeAbs}, remoteCtl)
		if err != nil {
			return err
		}
	}

	return nil
}

// Utilizes a cut-off and the magnitude of the speed and angle to dictate millisPerSec and degPerSec
func speedAndAngleMathMag(speed float64, angle float64) (float64, float64) {

	var millisPerSec float64
	var degPerSec float64

	mag := math.Sqrt(speed*speed + angle*angle)

	if math.Abs(speed) > 0.5 {
		millisPerSec = speed
		degPerSec = angle

	} else {
		if mag > 0.5 {
			degPerSec = angle
		} else {
			millisPerSec = speed
			degPerSec = angle
		}
	}
	return millisPerSec, degPerSec
}

// Utilizes a bounding box with hard cut-offs to dictate millisPerSec and degPerSec
func speedAndAngleMathSquare(speed float64, angle float64) (float64, float64) {

	var millisPerSec float64
	var degPerSec float64

	if math.Abs(speed) > 0.5 {
		if speed > 0 {
			millisPerSec = 0.5
		} else {
			millisPerSec = -0.5
		}
	} else {
		millisPerSec = speed
	}

	if math.Abs(angle) > 0.5 {
		if angle > 0 {
			degPerSec = 0.5
		} else {
			degPerSec = -0.5
		}
	} else {
		degPerSec = angle
	}

	return millisPerSec, degPerSec
}

// Identity for speed and angle to millisPerSec and degPerSec
func speedAndAngleMathIdentity(speed float64, angle float64) (float64, float64) {

	return speed, angle
}

// Control loops varaiation that uses left and right triggers to control speed
func (svc *remoteService) StartRemoteLR(ctx context.Context) error {

	var speed float64
	var angle float64
	var millisPerSec float64
	var degPerSec float64

	maxSpeed := 100.0
	maxAngle := 40.0

	remoteCtl := func(ctx context.Context, event input.Event) {

		if event.Event != input.PositionChangeAbs {
			return
		}

		switch event.Control {
		case input.AbsoluteZ:
			speed -= 0.05
			speed = math.Max(-1, speed)
		case input.AbsoluteRZ:
			speed += 0.05
			speed = math.Min(1, speed)

		case input.AbsoluteX:
			angle = event.Value
		}

		if math.Abs(speed) < 0.1 {
			millisPerSec, degPerSec = speedAndAngleMathIdentity(0, angle)
		} else {
			millisPerSec, degPerSec = speedAndAngleMathIdentity(speed, angle)
		}
		millisPerSec, degPerSec = speedAndAngleMathMag(speed, angle)
		millisPerSec, degPerSec = speedAndAngleMathSquare(speed, angle)

		// Set distance to large number
		_, err := svc.base.MoveArc(ctx, 1000, millisPerSec*maxSpeed*-1, degPerSec*maxAngle, true) // 300 | 30

		if err != nil {
			svc.logger.Errorw("error with moving base to desired position", "error", err)
		}

	}

	for _, control := range []input.Control{input.AbsoluteX, input.AbsoluteZ, input.AbsoluteRZ} {
		err := svc.inputController.RegisterControlCallback(ctx, control, []input.EventType{input.PositionChangeAbs}, remoteCtl)
		if err != nil {
			return err
		}
	}

	return nil
}
