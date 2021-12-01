package wx250s

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

	"go.viam.com/core/component/gripper"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
)

func init() {
	registry.RegisterComponent(gripper.Subtype, "wx250s", registry.Component{
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

// wx250s TODO
type wx250s struct {
	jServo   *servo.Servo
	moveLock *sync.Mutex
}

// newGripper TODO
func newGripper(attributes config.AttributeMap, logger golog.Logger) (*wx250s, error) {
	usbPort := attributes.String("usbPort")
	jServo := findServo(usbPort, attributes.String("baudRate"), logger)
	err := jServo.SetTorqueEnable(true)
	newGripper := wx250s{
		jServo:   jServo,
		moveLock: getPortMutex(usbPort),
	}
	return &newGripper, err
}

// GetMoveLock TODO
func (g *wx250s) GetMoveLock() *sync.Mutex {
	return g.moveLock
}

// Open TODO
func (g *wx250s) Open(ctx context.Context) error {
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

// Grab TODO
func (g *wx250s) Grab(ctx context.Context) (bool, error) {
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
func (g *wx250s) Close() error {
	err := g.jServo.SetTorqueEnable(false)
	return err
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
	//Get model ID of servo
	newServo, err := s_model.New(network, GripperServoNum)
	if err != nil {
		logger.Fatalf("error initializing servo %d: %v\n", GripperServoNum, err)
	}

	return newServo
}
