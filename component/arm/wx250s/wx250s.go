// Package wx250s implements the WidowX 250 Robot Arm from Trossen Robotics.
package wx250s

import (
	"context"

	// for embedding model file.
	_ "embed"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/jacobsa/go-serial/serial"
	"github.com/pkg/errors"
	"go.viam.com/dynamixel/network"
	"go.viam.com/dynamixel/servo"
	"go.viam.com/dynamixel/servo/s_model"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

//go:embed wx250s_kinematics.json
var wx250smodeljson []byte

func init() {
	registry.RegisterComponent(arm.Subtype, "wx250s", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewArm(ctx, config.Attributes, logger)
		},
	})
}

// SleepAngles are the angles we go to to prepare to turn off torque.
var SleepAngles = map[string]float64{
	"Waist":       2048,
	"Shoulder":    840,
	"Elbow":       3090,
	"Forearm_rot": 2048,
	"Wrist":       2509,
	"Wrist_rot":   2048,
}

// OffAngles are the angles the arm falls into after torque is off.
var OffAngles = map[string]float64{
	"Waist":       2048,
	"Shoulder":    795,
	"Elbow":       3091,
	"Forearm_rot": 2048,
	"Wrist":       2566,
	"Wrist_rot":   2048,
}

// Arm TODO.
type Arm struct {
	Joints   map[string][]*servo.Servo
	moveLock *sync.Mutex
	logger   golog.Logger
	mp       motionplan.MotionPlanner
	model    referenceframe.Model
}

// servoPosToDegrees takes a 360 degree 0-4096 servo position, centered at 2048,
// and converts it to degrees, centered at 0.
func servoPosToDegrees(pos float64) float64 {
	return ((pos - 2048) * 180) / 2048
}

// degreeToServoPos takes a 0-centered radian and converts to a 360 degree 0-4096 servo position, centered at 2048.
func degreeToServoPos(pos float64) int {
	return int(2048 + (pos/180)*2048)
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

// NewArm TODO.
func NewArm(ctx context.Context, attributes config.AttributeMap, logger golog.Logger) (arm.Arm, error) {
	usbPort := attributes.String("usbPort")
	servos, err := findServos(usbPort, attributes.String("baudRate"), attributes.String("armServoCount"))
	if err != nil {
		return nil, err
	}

	model, err := referenceframe.UnmarshalModelJSON(wx250smodeljson, "")
	if err != nil {
		return nil, err
	}
	mp, err := motionplan.NewCBiRRTMotionPlanner(model, 4, logger)
	if err != nil {
		return nil, err
	}

	return &Arm{
		Joints: map[string][]*servo.Servo{
			"Waist":       {servos[0]},
			"Shoulder":    {servos[1], servos[2]},
			"Elbow":       {servos[3], servos[4]},
			"Forearm_rot": {servos[5]},
			"Wrist":       {servos[6]},
			"Wrist_rot":   {servos[7]},
		},
		moveLock: getPortMutex(usbPort),
		logger:   logger,
		mp:       mp,
		model:    model,
	}, nil
}

// GetEndPosition computes and returns the current cartesian position.
func (a *Arm) GetEndPosition(ctx context.Context) (*commonpb.Pose, error) {
	joints, err := a.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return motionplan.ComputePosition(a.mp.Frame(), joints)
}

// MoveToPosition moves the arm to the specified cartesian position.
func (a *Arm) MoveToPosition(ctx context.Context, pos *commonpb.Pose, worldState *commonpb.WorldState) error {
	joints, err := a.GetJointPositions(ctx)
	if err != nil {
		return err
	}
	solution, err := a.mp.Plan(ctx, pos, referenceframe.JointPosToInputs(joints), nil)
	if err != nil {
		return err
	}
	return arm.GoToWaypoints(ctx, a, solution)
}

// MoveToJointPositions takes a list of degrees and sets the corresponding joints to that position.
func (a *Arm) MoveToJointPositions(ctx context.Context, jp *pb.JointPositions) error {
	if len(jp.Degrees) > len(a.JointOrder()) {
		return errors.New("passed in too many positions")
	}

	a.moveLock.Lock()

	// TODO(pl): make block configurable
	block := false
	for i, pos := range jp.Degrees {
		a.JointTo(a.JointOrder()[i], degreeToServoPos(pos), block)
	}

	a.moveLock.Unlock()
	return a.WaitForMovement(ctx)
}

// GetJointPositions returns an empty struct, because the wx250s should use joint angles from kinematics.
func (a *Arm) GetJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	return &pb.JointPositions{}, nil
}

// Close will get the arm ready to be turned off.
func (a *Arm) Close() {
	// First, check if we are approximately in the sleep position
	// If so, we can just turn off torque
	// If not, let's move through the home position first
	angles, err := a.GetAllAngles()
	if err != nil {
		a.logger.Errorf("failed to get angles: %s", err)
	}
	alreadyAtSleep := true
	for _, joint := range a.JointOrder() {
		if !within(angles[joint], SleepAngles[joint], 15) && !within(angles[joint], OffAngles[joint], 15) {
			alreadyAtSleep = false
		}
	}
	if !alreadyAtSleep {
		err = a.HomePosition(context.Background())
		if err != nil {
			a.logger.Errorf("Home position error: %s", err)
		}
	}
	err = a.SleepPosition(context.Background())
	if err != nil {
		a.logger.Errorf("Sleep pos error: %s", err)
	}
	err = a.TorqueOff()
	if err != nil {
		a.logger.Errorf("Torque off error: %s", err)
	}
}

// GetAllAngles will return a map of the angles of each joint, denominated in servo position.
func (a *Arm) GetAllAngles() (map[string]float64, error) {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	angles := make(map[string]float64)
	for jointName, servos := range a.Joints {
		angleSum := 0
		for _, s := range servos {
			pos, err := s.PresentPosition()
			if err != nil {
				return angles, err
			}
			angleSum += pos
		}
		angleMean := float64(angleSum / len(servos))
		angles[jointName] = angleMean
	}
	return angles, nil
}

// JointOrder TODO.
func (a *Arm) JointOrder() []string {
	return []string{"Waist", "Shoulder", "Elbow", "Forearm_rot", "Wrist", "Wrist_rot"}
}

// PrintPositions print positions of all servos.
// TODO(pl): Print joint names, not just servo numbers.
func (a *Arm) PrintPositions() error {
	posString := ""
	for i, s := range a.GetAllServos() {
		pos, err := s.PresentPosition()
		if err != nil {
			return err
		}
		posString = fmt.Sprintf("%s || %d : %d, %f degrees", posString, i, pos, servoPosToDegrees(float64(pos)))
	}
	return nil
}

// GetAllServos return a slice containing all servos in the arm.
func (a *Arm) GetAllServos() []*servo.Servo {
	var servos []*servo.Servo
	for _, joint := range a.JointOrder() {
		servos = append(servos, a.Joints[joint]...)
	}
	return servos
}

// GetServos returns a slice containing all servos in the named joint.
func (a *Arm) GetServos(jointName string) []*servo.Servo {
	var servos []*servo.Servo
	servos = append(servos, a.Joints[jointName]...)
	return servos
}

// SetAcceleration sets acceleration for servos.
func (a *Arm) SetAcceleration(accel int) error {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	for _, s := range a.GetAllServos() {
		err := s.SetProfileAcceleration(accel)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetVelocity set velocity for servos in travel time;
// recommended value 1000.
func (a *Arm) SetVelocity(veloc int) error {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	for _, s := range a.GetAllServos() {
		err := s.SetProfileVelocity(veloc)
		if err != nil {
			return err
		}
	}
	return nil
}

// TorqueOn turns on torque for all servos.
func (a *Arm) TorqueOn() error {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	for _, s := range a.GetAllServos() {
		err := s.SetTorqueEnable(true)
		if err != nil {
			return err
		}
	}
	return nil
}

// TorqueOff turns off torque for all servos.
func (a *Arm) TorqueOff() error {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	for _, s := range a.GetAllServos() {
		err := s.SetTorqueEnable(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// JointTo sets a joint to a position.
func (a *Arm) JointTo(jointName string, pos int, block bool) {
	if pos > 4095 {
		pos = 4095
	} else if pos < 0 {
		pos = 0
	}

	err := servo.GoalAndTrack(pos, block, a.GetServos(jointName)...)
	if err != nil {
		a.logger.Errorf("%s jointTo error: %s", jointName, err)
	}
}

// SleepPosition goes back to the sleep position, ready to turn off torque.
func (a *Arm) SleepPosition(ctx context.Context) error {
	a.moveLock.Lock()
	sleepWait := false
	a.JointTo("Waist", 2048, sleepWait)
	a.JointTo("Shoulder", 840, sleepWait)
	a.JointTo("Wrist_rot", 2048, sleepWait)
	a.JointTo("Wrist", 2509, sleepWait)
	a.JointTo("Forearm_rot", 2048, sleepWait)
	a.JointTo("Elbow", 3090, sleepWait)
	a.moveLock.Unlock()
	return a.WaitForMovement(ctx)
}

// GetMoveLock TODO.
func (a *Arm) GetMoveLock() *sync.Mutex {
	return a.moveLock
}

// HomePosition goes to the home position.
func (a *Arm) HomePosition(ctx context.Context) error {
	a.moveLock.Lock()

	wait := false
	for jointName := range a.Joints {
		a.JointTo(jointName, 2048, wait)
	}
	a.moveLock.Unlock()
	return a.WaitForMovement(ctx)
}

// CurrentInputs TODO.
func (a *Arm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := a.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.JointPosToInputs(res), nil
}

// GoToInputs TODO.
func (a *Arm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return a.MoveToJointPositions(ctx, referenceframe.InputsToJointPos(goal))
}

// WaitForMovement takes some servos, and will block until the servos are done moving.
func (a *Arm) WaitForMovement(ctx context.Context) error {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	allAtPos := false

	for !allAtPos {
		if !utils.SelectContextOrWait(ctx, 200*time.Millisecond) {
			return ctx.Err()
		}
		allAtPos = true
		for _, s := range a.GetAllServos() {
			isMoving, err := s.Moving()
			if err != nil {
				return err
			}
			// TODO(pl): Make this configurable
			if isMoving != 0 {
				allAtPos = false
			}
		}
	}
	return nil
}

// ModelFrame TODO.
func (a *Arm) ModelFrame() referenceframe.Model {
	return a.model
}

func setServoDefaults(newServo *servo.Servo) error {
	dm, err := newServo.DriveMode()
	if err != nil {
		return errors.Wrapf(err, "error DriveMode servo %d", newServo.ID)
	}
	if dm == 4 {
		err = newServo.SetDriveMode(0)
		if err != nil {
			return errors.Wrapf(err, "error SetDriveMode0 servo %d", newServo.ID)
		}
	}
	if dm == 5 {
		err = newServo.SetDriveMode(1)
		if err != nil {
			return errors.Wrapf(err, "error DriveMode1 servo %d", newServo.ID)
		}
	}
	err = newServo.SetPGain(2800)
	if err != nil {
		return errors.Wrapf(err, "error SetPGain servo %d", newServo.ID)
	}
	err = newServo.SetIGain(50)
	if err != nil {
		return errors.Wrapf(err, "error SetIGain servo %d", newServo.ID)
	}
	err = newServo.SetTorqueEnable(true)
	if err != nil {
		return errors.Wrapf(err, "error SetTorqueEnable servo %d", newServo.ID)
	}
	err = newServo.SetProfileVelocity(50)
	if err != nil {
		return errors.Wrapf(err, "error SetProfileVelocity servo %d", newServo.ID)
	}
	err = newServo.SetProfileAcceleration(10)
	if err != nil {
		return errors.Wrapf(err, "error SetProfileAcceleration servo %d", newServo.ID)
	}
	return nil
}

// findServos find the specified number of Dynamixel servos on the specified USB port
// we are going to hardcode some USB parameters that we will literally never want to change.
func findServos(usbPort, baudRateStr, armServoCountStr string) ([]*servo.Servo, error) {
	baudRate, err := strconv.Atoi(baudRateStr)
	if err != nil {
		return nil, errors.Wrap(err, "mangled baudrate")
	}
	armServoCount, err := strconv.Atoi(armServoCountStr)
	if err != nil {
		return nil, errors.Wrap(err, "mangled servo count")
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
		return nil, errors.Wrap(err, "error opening serial port")
	}

	var servos []*servo.Servo

	network := network.New(serial)

	// By default, Dynamixel servos come 1-indexed out of the box because reasons
	for i := 1; i <= armServoCount; i++ {
		// Get model ID of each servo
		newServo, err := s_model.New(network, i)
		if err != nil {
			return nil, errors.Wrapf(err, "error initializing servo %d", i)
		}

		err = setServoDefaults(newServo)
		if err != nil {
			return nil, err
		}

		servos = append(servos, newServo)
	}

	return servos, nil
}

func within(a, b, c float64) bool {
	return math.Abs(a-b) <= c
}
