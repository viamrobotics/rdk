// Package limo implements the AgileX Limo base
package limo

import (
	"context"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/jacobsa/go-serial/serial"
	"github.com/pkg/errors"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

var (
	globalMu    sync.Mutex
	controllers map[string]*controller
)

func init() {
	controllers = make(map[string]*controller)

	limoBaseComp := registry.Component{
		Constructor: func(
			ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger,
		) (interface{}, error) {
			return CreateLimoBase(ctx, r, config.ConvertedAttributes.(*Config), logger)
		},
	}

	registry.RegisterComponent(base.Subtype, "agilex-limo", limoBaseComp)
	config.RegisterComponentAttributeMapConverter(
		base.SubtypeName,
		"agilex-limo",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&Config{})
}

// controller is common across all limo instances sharing a controller.
type controller struct {
	mu           sync.Mutex
	port         io.ReadWriteCloser
	serialDevice string
	logger       golog.Logger
	testChan     chan []uint8
}

type limoFrame struct {
	stamp float64
	id    uint16
	data  []uint8
	count uint8
}

type limoState struct {
	controlThreadStarted                    bool
	velocityLinearGoal, velocityAngularGoal r3.Vector
}
type limoBase struct {
	generic.Unimplemented
	driveMode          string
	controller         *controller
	state              limoState
	stateMutex         sync.Mutex
	opMgr              operation.SingleOperationManager
	cancel             context.CancelFunc
	waitGroup          sync.WaitGroup
	width              int
	wheelbase          int
	maxInnerAngle      float64
	rightAngleScale    float64
	maxLinearVelocity  int
	maxAngularVelocity int
}

// Config is how you configure a limo base.
type Config struct {
	DriveMode    string `json:"drive_mode"`
	SerialDevice string `json:"serial_device"` // path to /dev/ttyXXXX file
	// TestChan is a fake "serial" path for test use only
	TestChan chan []uint8
}

// CreateLimoBase returns a AgileX limo base
func CreateLimoBase(ctx context.Context, r robot.Robot, config *Config, logger golog.Logger) (base.LocalBase, error) {

	logger.Debugf("creating limo base with config %+v", config)

	if config.DriveMode == "" {
		return nil, errors.Errorf("drive mode must be defined and one of differential, ackermann, or omni")
	}

	globalMu.Lock()
	sDevice := config.SerialDevice
	if sDevice == "" {
		sDevice = "/dev/ttyTHS1"
	}
	ctrl, controllerExists := controllers[sDevice]
	if !controllerExists {
		logger.Debug("creating controller")
		newCtrl, err := newController(sDevice, config.TestChan, logger)
		if err != nil {
			return nil, err
		}

		controllers[sDevice] = newCtrl
		ctrl = newCtrl

		// enable commanded mode
		frame := new(limoFrame)
		frame.id = 0x421
		frame.data = make([]uint8, 8)
		frame.data[0] = 0x01
		frame.data[1] = 0
		frame.data[2] = 0
		frame.data[3] = 0
		frame.data[4] = 0
		frame.data[5] = 0
		frame.data[6] = 0
		frame.data[7] = 0

		logger.Debug("Will send init frame")
		err = ctrl.sendFrame(frame)
		if err != nil && !strings.HasPrefix(err.Error(), "error enabling commanded mode") {
			return nil, err
		}
	}
	globalMu.Unlock()

	base := &limoBase{
		driveMode:          config.DriveMode,
		controller:         ctrl,
		width:              172,
		wheelbase:          200,
		maxLinearVelocity:  1200,
		maxAngularVelocity: 180,
		maxInnerAngle:      .48869, // 28 degrees in radians
		rightAngleScale:    1.64,
	}

	base.stateMutex.Lock()
	if !base.state.controlThreadStarted {
		// nolint:contextcheck
		err := base.startControlThread()
		if err != nil {
			return base, err
		}
		base.state.controlThreadStarted = true
	}
	base.stateMutex.Unlock()

	logger.Debug("Base initialized")

	return base, nil
}

func newController(sDevice string, testChan chan []uint8, logger golog.Logger) (*controller, error) {
	ctrl := new(controller)
	ctrl.serialDevice = sDevice
	ctrl.logger = logger

	if testChan == nil {
		logger.Debug("opening serial connection")
		serialOptions := serial.OpenOptions{
			PortName:          sDevice,
			BaudRate:          460800,
			DataBits:          8,
			StopBits:          1,
			MinimumReadSize:   1,
			RTSCTSFlowControl: true,
		}

		port, err := serial.Open(serialOptions)
		if err != nil {
			logger.Error(err)
			return nil, err
		}
		ctrl.port = port
	} else {
		ctrl.testChan = testChan
	}

	return ctrl, nil
}

// this rover requires messages to be sent continously or the motors will shut down
func (b *limoBase) startControlThread() error {
	var ctx context.Context
	ctx, b.cancel = context.WithCancel(context.Background())
	b.controller.logger.Debug("Starting control thread")

	b.waitGroup.Add(1)
	go func() {
		defer b.waitGroup.Done()

		for {
			utils.SelectContextOrWait(ctx, time.Duration(float64(time.Millisecond)*10))
			err := b.controlThreadLoop(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				b.controller.logger.Warn(err)
			}
		}
	}()

	return nil
}

func (base *limoBase) controlThreadLoop(ctx context.Context) error {
	var err error
	if base.driveMode == "differential" {
		err = base.setMotionCommand(ctx, base.state.velocityLinearGoal.Y, -base.state.velocityAngularGoal.Z, 0, 0)
	} else if base.driveMode == "ackermann" {
		r := base.state.velocityLinearGoal.Y / base.state.velocityAngularGoal.Z
		if math.Abs(r) < float64(base.width)/2.0 {
			if r == 0 {
				r = base.state.velocityAngularGoal.Z / math.Abs(base.state.velocityAngularGoal.Z) * (float64(base.width)/2.0 + 10)
			} else {
				r = r / math.Abs(r) * (float64(base.width)/2.0 + 10)
			}
		}
		central_angle := math.Atan(float64(base.wheelbase) / r)
		inner_angle := math.Atan((2 * float64(base.wheelbase) * math.Sin(central_angle) / (2*float64(base.wheelbase)*math.Cos(math.Abs(central_angle)) - float64(base.width)*math.Sin(math.Abs(central_angle)))))

		if inner_angle > base.maxInnerAngle {
			inner_angle = base.maxInnerAngle
		}
		if inner_angle < -base.maxInnerAngle {
			inner_angle = -base.maxInnerAngle
		}

		steering_angle := inner_angle / base.rightAngleScale
		// steering angle is in unit of .001 radians
		err = base.setMotionCommand(ctx, base.state.velocityLinearGoal.Y, 0, 0, -steering_angle*1000)
	} else if base.driveMode == "omni" {
		err = base.setMotionCommand(ctx, base.state.velocityLinearGoal.Y, -base.state.velocityAngularGoal.Z, base.state.velocityLinearGoal.X, 0)
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// Must be run inside a lock.
func (c *controller) sendFrame(frame *limoFrame) error {
	var checksum uint32 = 0
	var frame_len uint8 = 0x0e
	var data = make([]uint8, 14)
	data[0] = 0x55
	data[1] = frame_len // frame length
	data[2] = uint8(frame.id >> 8)
	data[3] = uint8(frame.id & 0xff)
	for i := 0; i < 8; i++ {
		data[i+4] = frame.data[i]
		checksum += uint32(frame.data[i])
	}
	data[frame_len-1] = uint8(checksum & 0xff)

	if c.testChan != nil {
		c.logger.Debug("writing to test chan")
		c.testChan <- data
	} else {
		_, err := c.port.Write(data)
		if err != nil {
			return err
		}
	}

	return nil
}

func (base *limoBase) setMotionCommand(ctx context.Context, linear_vel float64, angular_vel float64, lateral_vel float64, steering_angle float64) error {
	frame := new(limoFrame)
	frame.id = 0x111
	linear_cmd := int16(linear_vel)
	angular_cmd := int16(angular_vel)
	lateral_cmd := int16(lateral_vel)
	steering_cmd := int16(steering_angle)

	frame.data = make([]uint8, 8)
	frame.data[0] = uint8(linear_cmd >> 8)
	frame.data[1] = uint8(linear_cmd & 0x00ff)
	frame.data[2] = uint8(angular_cmd >> 8)
	frame.data[3] = uint8(angular_cmd & 0x00ff)
	frame.data[4] = uint8(lateral_cmd >> 8)
	frame.data[5] = uint8(lateral_cmd & 0x00ff)
	frame.data[6] = uint8(steering_cmd >> 8)
	frame.data[7] = uint8(steering_cmd & 0x00ff)

	base.controller.mu.Lock()
	err := base.controller.sendFrame(frame)
	if err != nil {
		base.controller.logger.Error(err)
	}
	base.controller.mu.Unlock()

	return err
}

func (base *limoBase) Spin(ctx context.Context, angleDeg float64, degsPerSec float64) error {
	base.controller.logger.Debugf("Spin(%f, %f)", angleDeg, degsPerSec)
	secsToRun := math.Abs(angleDeg / degsPerSec)
	var err error
	if base.driveMode == "differential" || base.driveMode == "omni" {
		// angular velocity is expressed in units of .001 radians/sec
		err = base.SetVelocity(ctx, r3.Vector{}, r3.Vector{Z: degsPerSec})
	} else if base.driveMode == "ackermann" {
		// TODO: this is not the correct math
		linear := float64(base.maxLinearVelocity) * (degsPerSec / 360) * math.Pi
		angular := float64(base.maxAngularVelocity)
		if angleDeg < 0 {
			angular = -angular
		}
		// max angular translates to max steering angle for ackermann+
		err = base.SetVelocity(ctx, r3.Vector{Y: linear}, r3.Vector{Z: angular})
	}

	if err != nil {
		return err
	}
	// stop base after calculated time
	timeToRun := time.Millisecond * time.Duration(secsToRun*1000)
	base.controller.logger.Debugf("Will run for duration %f", timeToRun)
	utils.SelectContextOrWait(ctx, timeToRun)
	return base.Stop(ctx)
}

func (base *limoBase) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64) error {
	base.controller.logger.Debugf("MoveStraight(%d, %f)", distanceMm, mmPerSec)
	base.SetVelocity(ctx, r3.Vector{Y: mmPerSec}, r3.Vector{})

	// stop base after calculated time
	timeToRun := time.Millisecond * time.Duration(math.Abs(float64(distanceMm)/float64(mmPerSec))*1000)
	base.controller.logger.Debugf("Will run for duration %f", timeToRun)
	utils.SelectContextOrWait(ctx, timeToRun)
	return base.Stop(ctx)
}

// linear is in mm/sec, angular in degrees/sec
func (base *limoBase) SetVelocity(ctx context.Context, linear, angular r3.Vector) error {
	base.controller.logger.Debugf("Will set linear velocity %f angular velocity %f", linear, angular)

	_, done := base.opMgr.New(ctx)
	defer done()

	// this base expects angular velocity to be expressed in .001 radians/sec, convert
	angular.Z = (angular.Z / 57.2958) * 1000

	base.stateMutex.Lock()
	base.state.velocityLinearGoal = linear
	base.state.velocityAngularGoal = angular
	base.stateMutex.Unlock()

	return nil
}

func (base *limoBase) SetPower(ctx context.Context, linear, angular r3.Vector) error {
	base.controller.logger.Debugf("Will set power linear %f angular %f", linear, angular)
	linY := linear.Y * float64(base.maxLinearVelocity)
	angZ := angular.Z * float64(base.maxAngularVelocity)
	err := base.SetVelocity(ctx, r3.Vector{Y: linY}, r3.Vector{Z: -angZ})
	if err != nil {
		return err
	}
	return nil
}

func (base *limoBase) Stop(ctx context.Context) error {
	base.controller.logger.Debug("Stop()")
	err := base.SetVelocity(ctx, r3.Vector{}, r3.Vector{})
	if err != nil {
		return err
	}
	base.opMgr.CancelRunning(ctx)
	return nil
}

// Do executes additional commands beyond the Base{} interface.
func (base *limoBase) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	name, ok := cmd["command"]
	if !ok {
		return nil, errors.New("missing 'command' value")
	}
	switch name {
	case "drive_mode":
		modeRaw, ok := cmd["mode"]
		if !ok {
			return nil, errors.New("mode must be set, one of differential|ackermann|omni")
		}
		mode, ok := modeRaw.(string)
		if !ok {
			return nil, errors.New("mode value must be a string")
		}
		if !((mode == "differential") || (mode == "ackermann") || (mode == "omni")) {
			return nil, errors.New("mode value must be one of differential|ackermann|omni")
		}
		base.driveMode = mode
		return nil, nil
	default:
		return nil, fmt.Errorf("no such command: %s", name)
	}
}

func (base *limoBase) Close(ctx context.Context) error {
	base.controller.logger.Debug("Close()")
	base.Stop(ctx)
	if base.cancel != nil {
		base.controller.logger.Debug("calling cancel()")
		base.cancel()
		base.cancel = nil
		base.waitGroup.Wait()
		base.controller.logger.Debug("done waiting on cancel")
	}
	return nil
}

func (base *limoBase) GetWidth(ctx context.Context) (int, error) {
	return base.width, nil
}
