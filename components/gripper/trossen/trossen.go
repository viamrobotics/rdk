// Package trossen implements a trossen gripper.
package trossen

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/jacobsa/go-serial/serial"
	"go.viam.com/dynamixel/network"
	"go.viam.com/dynamixel/servo"
	"go.viam.com/dynamixel/servo/s_model"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterComponent(gripper.Subtype, "wx250s", registry.Component{
		Constructor: func(ctx context.Context, _ registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			return newGripper(config.Attributes, logger)
		},
	})

	registry.RegisterComponent(gripper.Subtype, "vx300s", registry.Component{
		Constructor: func(ctx context.Context, _ registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			return newGripper(config.Attributes, logger)
		},
	})
}

var (
	portMapping   = map[string]*sync.Mutex{}
	portMappingMu sync.Mutex
)

func getPortMutex(port string) *sync.Mutex {
	portMappingMu.Lock()
	defer portMappingMu.Unlock()
	mu, ok := portMapping[port]
	if !ok {
		mu = &sync.Mutex{}
		portMapping[port] = mu
	}
	return mu
}

// Gripper TODO.
type Gripper struct {
	jServo   *servo.Servo
	moveLock *sync.Mutex
	opMgr    operation.SingleOperationManager
	generic.Unimplemented
}

// newGripper TODO.
func newGripper(attributes config.AttributeMap, logger golog.Logger) (gripper.LocalGripper, error) {
	usbPort := attributes.String("usb_port")
	jServo, err := findServo(usbPort, attributes.String("baud_rate"), logger)
	if err != nil {
		return nil, err
	}
	if err := jServo.SetTorqueEnable(true); err != nil {
		return nil, err
	}
	newGripper := Gripper{
		jServo:   jServo,
		moveLock: getPortMutex(usbPort),
	}
	return &newGripper, nil
}

// GetMoveLock TODO.
func (g *Gripper) GetMoveLock() *sync.Mutex {
	return g.moveLock
}

// Open TODO.
func (g *Gripper) Open(ctx context.Context) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()
	g.moveLock.Lock()
	defer g.moveLock.Unlock()
	err := g.jServo.SetGoalPWM(150)
	if err != nil {
		return err
	}

	// We don't want to over-open
	atPos := false
	for !atPos {
		var pos int
		pos, err = g.jServo.PresentPosition()
		if err != nil {
			return err
		}
		if pos < 2800 {
			if !utils.SelectContextOrWait(ctx, 50*time.Millisecond) {
				return ctx.Err()
			}
		} else {
			atPos = true
		}
	}
	err = g.jServo.SetGoalPWM(0)
	return err
}

// Grab TODO.
func (g *Gripper) Grab(ctx context.Context) (bool, error) {
	_, done := g.opMgr.New(ctx)
	defer done()
	g.moveLock.Lock()
	defer g.moveLock.Unlock()
	err := g.jServo.SetGoalPWM(-350)
	if err != nil {
		return false, err
	}
	err = servo.WaitForMovementVar(g.jServo)
	if err != nil {
		return false, err
	}
	pos, err := g.jServo.PresentPosition()
	if err != nil {
		return false, err
	}
	didGrab := true

	// If servo position is less than 1500, it's closed and we grabbed nothing
	if pos < 1500 {
		didGrab = false
	}
	return didGrab, nil
}

// Stop is unimplemented for Gripper.
func (g *Gripper) Stop(ctx context.Context) error {
	// RSDK-388: Implement Stop
	return gripper.ErrStopUnimplemented
}

// IsMoving returns whether the gripper is moving.
func (g *Gripper) IsMoving(ctx context.Context) (bool, error) {
	return g.opMgr.OpRunning(), nil
}

// Close closes the connection, not the gripper.
func (g *Gripper) Close() error {
	return g.jServo.SetTorqueEnable(false)
}

// ModelFrame is unimplemented for Gripper.
func (g *Gripper) ModelFrame() referenceframe.Model {
	return nil
}

// findServo finds the gripper numbered Dynamixel servo on the specified USB port
// we are going to hardcode some USB parameters that we will literally never want to change.
func findServo(usbPort, baudRateStr string, logger golog.Logger) (*servo.Servo, error) {
	GripperServoNum := 9
	baudRate, err := strconv.Atoi(baudRateStr)
	if err != nil {
		logger.Errorf("Mangled baudrate: %v\n", err)
		return nil, err
	}
	options := serial.OpenOptions{
		PortName:              usbPort,
		BaudRate:              uint(baudRate),
		DataBits:              8,
		StopBits:              1,
		MinimumReadSize:       0,
		InterCharacterTimeout: 100,
	}

	serial, err := serial.Open(options)
	if err != nil {
		logger.Errorf("error opening serial port: %v\n", err)
		return nil, err
	}

	network := network.New(serial)

	// By default, Dynamixel servos come 1-indexed out of the box because reasons
	// Get model ID of servo
	newServo, err := s_model.New(network, GripperServoNum)
	if err != nil {
		logger.Errorf("error initializing servo %d: %v\n", GripperServoNum, err)
		return nil, err
	}

	return newServo, nil
}
