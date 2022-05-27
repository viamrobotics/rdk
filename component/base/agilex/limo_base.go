// Package limo implements the AgileX Limo base
package limo

import (
	"context"
	"io"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/jacobsa/go-serial/serial"
	"github.com/pkg/errors"

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

type limoBase struct {
	generic.Unimplemented
	driveMode  string
	controller *controller
	opMgr      operation.SingleOperationManager
}

// Config is how you configure a limo base.
type Config struct {
	DriveMode    string `json:"driveMode"`
	SerialDevice string `json:"serial_device" default:"/dev/ttyTHS1"` // path to /dev/ttyXXXX file
	// TestChan is a fake "serial" path for test use only
	TestChan chan string `json:"-"`
}

// CreateLimoBase returns a AgileX limo base
func CreateLimoBase(ctx context.Context, r robot.Robot, config *Config, logger golog.Logger) (base.LocalBase, error) {

	globalMu.Lock()
	ctrl, ok := controllers[config.SerialDevice]
	if !ok {
		newCtrl, err := newController(config, logger)
		if err != nil {
			return nil, err
		}
		controllers[config.SerialDevice] = newCtrl
		ctrl = newCtrl
	}
	globalMu.Unlock()

	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()

	base := &limoBase{
		driveMode:  config.DriveMode,
		controller: ctrl,
	}

	// enable commanded mode
	frame := new(limoFrame)
	frame.id = 0x421
	frame.data[0] = 0x01
	frame.data[1] = 0
	frame.data[2] = 0
	frame.data[3] = 0
	frame.data[4] = 0
	frame.data[5] = 0
	frame.data[6] = 0
	frame.data[7] = 0

	err := ctrl.sendFrame(frame)
	if err != nil && !strings.HasPrefix(err.Error(), "error enabling commanded mode") {
		return nil, err
	}

	return base, nil
}

func newController(c *Config, logger golog.Logger) (*controller, error) {
	ctrl := new(controller)
	ctrl.serialDevice = c.SerialDevice
	ctrl.logger = logger

	if c.TestChan == nil {
		serialOptions := serial.OpenOptions{
			PortName:          c.SerialDevice,
			BaudRate:          0010004,
			DataBits:          8,
			StopBits:          1,
			MinimumReadSize:   1,
			RTSCTSFlowControl: true,
		}

		port, err := serial.Open(serialOptions)
		if err != nil {
			return nil, err
		}
		ctrl.port = port
	}

	return ctrl, nil
}

// Must be run inside a lock.
func (c *controller) sendFrame(frame *limoFrame) error {
	var checksum uint8 = 0
	var frame_len uint8 = 0x0e
	frame.data[14] = 0x55
	frame.data[15] = frame_len

	frame.data[2] = uint8(frame.id >> 8)
	frame.data[3] = uint8(frame.id & 0xff)
	for i := 0; i < 8; i++ {
		frame.data[i+3] = frame.data[i-1]
		checksum += frame.data[i-1]
	}
	frame.data[frame_len-1] = uint8(checksum & 0xff)

	if c.testChan != nil {
		c.testChan <- frame.data
	} else {
		_, err := c.port.Write(frame.data)
		if err != nil {
			return err
		}
	}
	return nil
}

func (base *limoBase) setMotionCommand(linear_vel float64, angular_vel float64, lateral_velocity float64, steering_angle float64) error {
	base.controller.mu.Lock()
	defer base.controller.mu.Unlock()

	// enable commanded mode
	frame := new(limoFrame)
	frame.id = 0x111
	linear_cmd := uint16(linear_vel * 1000)
	angular_cmd := uint16(angular_vel * 1000)
	lateral_cmd := uint16(lateral_velocity * 1000)
	steering_cmd := uint16(steering_angle * 1000)

	frame.data[0] = uint8(linear_cmd >> 8)
	frame.data[1] = uint8(linear_cmd & 0x00ff)
	frame.data[2] = uint8(angular_cmd >> 8)
	frame.data[3] = uint8(angular_cmd & 0x00ff)
	frame.data[4] = uint8(lateral_cmd >> 8)
	frame.data[5] = uint8(lateral_cmd & 0x00ff)
	frame.data[6] = uint8(steering_cmd >> 8)
	frame.data[7] = uint8(steering_cmd & 0x00ff)
	err := base.controller.sendFrame(frame)
	return err
}

func (base *limoBase) Spin(ctx context.Context, angleDeg float64, degsPerSec float64) error {
	ctx, done := base.opMgr.New(ctx)
	defer done()

	// Stop the motors if the speed is 0
	if math.Abs(degsPerSec) < 0.0001 {
		err := base.Stop(ctx)
		if err != nil {
			return errors.Errorf("error when trying to spin at a speed of 0: %v", err)
		}
		return err
	}

	return nil
}

func (base *limoBase) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64) error {
	ctx, done := base.opMgr.New(ctx)
	defer done()

	// stop motors after calculated distance
	timeInSecs := float64(distanceMm) / mmPerSec
	timer := time.AfterFunc(time.Second*time.Duration(timeInSecs), func() {
		base.setMotionCommand(0, 0, 0, 0)
	})
	defer timer.Stop()

	base.setMotionCommand(mmPerSec, 0, 0, 0)

	return nil
}

func (base *limoBase) MoveArc(ctx context.Context, distanceMm int, mmPerSec float64, angleDeg float64) error {
	ctx, done := base.opMgr.New(ctx)
	defer done()

	// Stop the motors if the speed is 0
	if math.Abs(mmPerSec) < 0.0001 {
		err := base.Stop(ctx)
		if err != nil {
			return errors.Errorf("error when trying to arc at a speed of 0: %v", err)
		}
		return err
	}

	return nil
}

func (base *limoBase) SetPower(ctx context.Context, linear, angular r3.Vector) error {
	return nil
}

func (base *limoBase) WaitForMotorsToStop(ctx context.Context) error {
	return base.Stop(ctx)
}

func (base *limoBase) Stop(ctx context.Context) error {
	err := base.setMotionCommand(0, 0, 0, 0)
	return err
}

func (base *limoBase) Close(ctx context.Context) error {
	return base.Stop(ctx)
}

func (base *limoBase) GetWidth(ctx context.Context) (int, error) {
	return 172, nil
}
