// Package limo implements the AgileX Limo base
package limo

import (
	"context"
	"io"
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
	velocityThreadStarted                   bool
	velocityLinearGoal, velocityAngularGoal r3.Vector
}
type limoBase struct {
	generic.Unimplemented
	driveMode  string
	controller *controller
	state      limoState
	stateMutex sync.Mutex
	opMgr      operation.SingleOperationManager
	cancel     context.CancelFunc
	waitGroup  sync.WaitGroup
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

	if config.SerialDevice == "" {
		config.SerialDevice = "/dev/ttyTHS1"
	}
	if config.DriveMode == "" {
		return nil, errors.Errorf("drive mode must be defined and one of differential, ackermann, or omni")
	}

	globalMu.Lock()
	ctrl, controllerExists := controllers[config.SerialDevice]
	if !controllerExists {
		logger.Debug("creating controller")
		newCtrl, err := newController(config, logger)
		if err != nil {
			return nil, err
		}
		controllers[config.SerialDevice] = newCtrl
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
		driveMode:  config.DriveMode,
		controller: ctrl,
	}

	base.stateMutex.Lock()
	if !base.state.velocityThreadStarted {
		// nolint:contextcheck
		err := base.startVelocityThread()
		if err != nil {
			return base, err
		}
		base.state.velocityThreadStarted = true
	}
	base.stateMutex.Unlock()

	logger.Debug("Base initialized")

	return base, nil
}

func newController(c *Config, logger golog.Logger) (*controller, error) {
	ctrl := new(controller)
	ctrl.serialDevice = c.SerialDevice
	ctrl.logger = logger

	if c.TestChan == nil {
		logger.Debug("opening serial connection")
		serialOptions := serial.OpenOptions{
			PortName:          c.SerialDevice,
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
		ctrl.testChan = c.TestChan
	}

	return ctrl, nil
}

func (b *limoBase) startVelocityThread() error {
	var ctx context.Context
	ctx, b.cancel = context.WithCancel(context.Background())
	b.controller.logger.Debug("Starting velocity thread")

	b.waitGroup.Add(1)
	go func() {
		defer b.waitGroup.Done()

		for {
			utils.SelectContextOrWait(ctx, time.Duration(float64(time.Millisecond)*100))
			err := b.velocityThreadLoop(ctx)
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

func (base *limoBase) velocityThreadLoop(ctx context.Context) error {
	err := base.setMotionCommand(ctx, base.state.velocityLinearGoal.Y, base.state.velocityAngularGoal.Z, 0, 0)
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
		n, err := c.port.Write(data)
		if err != nil {
			return err
		}
		c.logger.Debugf("Sent %d bytes %v", n, data)
	}

	return nil
}

func (base *limoBase) setMotionCommand(ctx context.Context, linear_vel float64, angular_vel float64, lateral_velocity float64, steering_angle float64) error {
	frame := new(limoFrame)
	frame.id = 0x111
	linear_cmd := int16(linear_vel)
	angular_cmd := int16(angular_vel)
	lateral_cmd := int16(lateral_velocity)
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
	millis := 1000 * (angleDeg / degsPerSec)
	err := base.SetVelocity(ctx, r3.Vector{}, r3.Vector{Z: -1 * degsPerSec})
	if err != nil {
		return err
	}
	utils.SelectContextOrWait(ctx, time.Duration(float64(time.Millisecond)*millis))
	return base.Stop(ctx)
}

func (base *limoBase) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64) error {
	base.controller.logger.Debugf("MoveStraight(%d, %f)", distanceMm, mmPerSec)
	base.SetVelocity(ctx, r3.Vector{Y: mmPerSec}, r3.Vector{})

	// stop base after calculated distance
	timeInSecs := time.Second * time.Duration(float64(distanceMm)/mmPerSec)
	utils.SelectContextOrWait(ctx, timeInSecs)
	return base.Stop(ctx)
}

func (base *limoBase) SetVelocity(ctx context.Context, linear, angular r3.Vector) error {
	base.controller.logger.Debugf("Will set linear velocity %f angular velocity %f", linear, angular)

	_, done := base.opMgr.New(ctx)
	defer done()

	base.stateMutex.Lock()

	base.state.velocityLinearGoal = linear
	base.state.velocityAngularGoal = angular
	base.stateMutex.Unlock()

	return nil
}

func (base *limoBase) SetPower(ctx context.Context, linear, angular r3.Vector) error {
	return nil
}

func (base *limoBase) Stop(ctx context.Context) error {
	base.controller.logger.Debug("Stop()")
	base.SetVelocity(ctx, r3.Vector{}, r3.Vector{})
	base.opMgr.CancelRunning(ctx)
	return nil
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
	return 172, nil
}
