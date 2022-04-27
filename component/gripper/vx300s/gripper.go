// Package vx300s implements a vx300s gripper.
package vx300s

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

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

func init() {
	registry.RegisterComponent(gripper.Subtype, "vx300s", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
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

// vx300s TODO.
type vx300s struct {
	jServo   *servo.Servo
	moveLock *sync.Mutex
	generic.Unimplemented
}

// newGripper TODO.
func newGripper(attributes config.AttributeMap, logger golog.Logger) (*vx300s, error) {
	usbPort := attributes.String("usb_port")
	jServo := findServo(usbPort, attributes.String("baud_rate"), logger)
	err := jServo.SetTorqueEnable(true)
	newGripper := vx300s{
		jServo:   jServo,
		moveLock: getPortMutex(usbPort),
	}
	return &newGripper, err
}

// GetMoveLock TODO.
func (g *vx300s) GetMoveLock() *sync.Mutex {
	return g.moveLock
}

// Open TODO.
func (g *vx300s) Open(ctx context.Context) error {
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
func (g *vx300s) Grab(ctx context.Context) (bool, error) {
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

// Close closes the connection, not the gripper.
func (g *vx300s) Close() error {
	return g.jServo.SetTorqueEnable(false)
}

// findServo finds the gripper numbered Dynamixel servo on the specified USB port
// we are going to hardcode some USB parameters that we will literally never want to change.
func findServo(usbPort, baudRateStr string, logger golog.Logger) *servo.Servo {
	GripperServoNum := 9
	baudRate, err := strconv.Atoi(baudRateStr)
	if err != nil {
		logger.Fatalf("Mangled baudrate: %v\n", err)
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
		logger.Fatalf("error opening serial port: %v\n", err)
	}

	network := network.New(serial)

	// By default, Dynamixel servos come 1-indexed out of the box because reasons
	// Get model ID of servo
	newServo, err := s_model.New(network, GripperServoNum)
	if err != nil {
		logger.Fatalf("error initializing servo %d: %v\n", GripperServoNum, err)
	}

	return newServo
}
