// Package vx300s implements the ViperX 300 Robot Arm from Trossen Robotics.
package vx300s

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/jacobsa/go-serial/serial"

	"go.viam.com/dynamixel/network"
	"go.viam.com/dynamixel/servo"
	"go.viam.com/dynamixel/servo/s_model"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/kinematics"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/utils"
)

func init() {
	api.RegisterArm("vx300s", func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (api.Arm, error) {
		mut, err := api.RobotAsMutable(r)
		if err != nil {
			return nil, err
		}
		return NewArm(config.Attributes, getProviderOrCreate(mut).moveLock, logger)
	})
}

// SleepAngles are the angles we go to to prepare to turn off torque
var SleepAngles = map[string]float64{
	"Waist":       2048,
	"Shoulder":    840,
	"Elbow":       3090,
	"Forearm_rot": 2048,
	"Wrist":       2509,
	"Wrist_rot":   2048,
}

// OffAngles are the angles the arm falls into after torque is off
var OffAngles = map[string]float64{
	"Waist":       2048,
	"Shoulder":    795,
	"Elbow":       3091,
	"Forearm_rot": 2048,
	"Wrist":       2566,
	"Wrist_rot":   2048,
}

type Arm struct {
	Joints   map[string][]*servo.Servo
	moveLock *sync.Mutex
	logger   golog.Logger
}

// servoPosToDegrees takes a 360 degree 0-4096 servo position, centered at 2048,
// and converts it to degrees, centered at 0
func servoPosToDegrees(pos float64) float64 {
	return ((pos - 2048) * 180) / 2048
}

// degreeToServoPos takes a 0-centered radian and converts to a 360 degree 0-4096 servo position, centered at 2048
func degreeToServoPos(pos float64) int {
	return int(2048 + (pos/180)*2048)
}

func NewArm(attributes api.AttributeMap, mutex *sync.Mutex, logger golog.Logger) (api.Arm, error) {
	servos, err := findServos(attributes.String("usbPort"), attributes.String("baudRate"), attributes.String("armServoCount"))
	if err != nil {
		return nil, err
	}

	if mutex == nil {
		mutex = &sync.Mutex{}
	}

	newArm := &Arm{
		Joints: map[string][]*servo.Servo{
			"Waist":       {servos[0]},
			"Shoulder":    {servos[1], servos[2]},
			"Elbow":       {servos[3], servos[4]},
			"Forearm_rot": {servos[5]},
			"Wrist":       {servos[6]},
			"Wrist_rot":   {servos[7]},
		},
		moveLock: mutex,
		logger:   logger,
	}

	return kinematics.NewArmJSONFile(newArm, attributes.String("modelJSON"), 4, logger)
}

func (a *Arm) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	return nil, fmt.Errorf("vx300s dosn't support kinematics")
}

func (a *Arm) MoveToPosition(ctx context.Context, c *pb.ArmPosition) error {
	return fmt.Errorf("vx300s dosn't support kinematics")
}

// MoveToJointPositions takes a list of degrees and sets the corresponding joints to that position
func (a *Arm) MoveToJointPositions(ctx context.Context, jp *pb.JointPositions) error {
	if len(jp.Degrees) > len(a.JointOrder()) {
		return fmt.Errorf("passed in too many positions")
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

// CurrentJointPositions returns an empty struct, because the vx300s should use joint angles from kinematics
func (a *Arm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	return &pb.JointPositions{}, nil
}

func (a *Arm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	return fmt.Errorf("not done yet")
}

// Close will get the arm ready to be turned off
func (a *Arm) Close() error {
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
		err = a.SleepPosition(context.Background())
		if err != nil {
			a.logger.Errorf("Sleep pos error: %s", err)
		}
	}
	err = a.TorqueOff()
	if err != nil {
		a.logger.Errorf("Torque off error: %s", err)
	}
	return nil
}

// GetAllAngles will return a map of the angles of each joint, denominated in servo position
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

func (a *Arm) JointOrder() []string {
	return []string{"Waist", "Shoulder", "Elbow", "Forearm_rot", "Wrist", "Wrist_rot"}
}

// Print positions of all servos
// TODO(pl): Print joint names, not just servo numbers
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

// Return a slice containing all servos in the arm
func (a *Arm) GetAllServos() []*servo.Servo {
	var servos []*servo.Servo
	for _, joint := range a.JointOrder() {
		servos = append(servos, a.Joints[joint]...)
	}
	return servos
}

// Return a slice containing all servos in the named joint
func (a *Arm) GetServos(jointName string) []*servo.Servo {
	var servos []*servo.Servo
	servos = append(servos, a.Joints[jointName]...)
	return servos
}

// Set Acceleration for servos
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

// Set Velocity for servos in travel time
// Recommended value 1000
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

//Turn on torque for all servos
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

//Turn off torque for all servos
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

// Set a joint to a position
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

//Go back to the sleep position, ready to turn off torque
func (a *Arm) SleepPosition(ctx context.Context) error {
	a.moveLock.Lock()
	sleepWait := false
	a.JointTo("Waist", 2048, sleepWait)
	a.JointTo("Shoulder", 839, sleepWait)
	a.JointTo("Wrist_rot", 2048, sleepWait)
	a.JointTo("Wrist", 2509, sleepWait)
	a.JointTo("Forearm_rot", 2048, sleepWait)
	a.JointTo("Elbow", 3090, sleepWait)
	a.moveLock.Unlock()
	return a.WaitForMovement(ctx)
}

func (a *Arm) GetMoveLock() *sync.Mutex {
	return a.moveLock
}

//Go to the home position
func (a *Arm) HomePosition(ctx context.Context) error {
	a.moveLock.Lock()

	wait := false
	for jointName := range a.Joints {
		a.JointTo(jointName, 2048, wait)
	}
	a.moveLock.Unlock()
	return a.WaitForMovement(ctx)
}

// WaitForMovement takes some servos, and will block until the servos are done moving
func (a *Arm) WaitForMovement(ctx context.Context) error {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	allAtPos := false

	for !allAtPos {
		if !utils.SelectContextOrWait(ctx, 100*time.Millisecond) {
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

// TODO: Map out *all* servo defaults so that they are always set correctly
func setServoDefaults(newServo *servo.Servo) error {
	// Set some nice-to-have settings
	err := newServo.SetTorqueEnable(false)
	if err != nil {
		return err
	}
	err = newServo.SetMovingThreshold(0)
	if err != nil {
		return fmt.Errorf("error SetMovingThreshold servo %d: %v", newServo.ID, err)
	}
	dm, err := newServo.DriveMode()
	if err != nil {
		return fmt.Errorf("error DriveMode servo %d: %v", newServo.ID, err)
	}
	if dm == 4 {
		err = newServo.SetDriveMode(0)
		if err != nil {
			return fmt.Errorf("error SetDriveMode0 servo %d: %v", newServo.ID, err)
		}
	}
	if dm == 5 {
		err = newServo.SetDriveMode(1)
		if err != nil {
			return fmt.Errorf("error DriveMode1 servo %d: %v", newServo.ID, err)
		}
	}
	err = newServo.SetPGain(2800)
	if err != nil {
		return fmt.Errorf("error SetPGain servo %d: %v", newServo.ID, err)
	}
	err = newServo.SetIGain(50)
	if err != nil {
		return fmt.Errorf("error SetIGain servo %d: %v", newServo.ID, err)
	}
	err = newServo.SetTorqueEnable(true)
	if err != nil {
		return fmt.Errorf("error SetTorqueEnable servo %d: %v", newServo.ID, err)
	}
	err = newServo.SetProfileVelocity(50)
	if err != nil {
		return fmt.Errorf("error SetProfileVelocity servo %d: %v", newServo.ID, err)
	}
	err = newServo.SetProfileAcceleration(10)
	if err != nil {
		return fmt.Errorf("error SetProfileAcceleration servo %d: %v", newServo.ID, err)
	}
	return nil
}

// Find the specified number of Dynamixel servos on the specified USB port
// We're going to hardcode some USB parameters that we will literally never want to change
func findServos(usbPort, baudRateStr, armServoCountStr string) ([]*servo.Servo, error) {
	baudRate, err := strconv.Atoi(baudRateStr)
	if err != nil {
		return nil, fmt.Errorf("mangled baudrate: %v", err)
	}
	armServoCount, err := strconv.Atoi(armServoCountStr)
	if err != nil {
		return nil, fmt.Errorf("mangled servo count: %v", err)
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
		return nil, fmt.Errorf("error opening serial port: %v", err)
	}

	var servos []*servo.Servo

	network := network.New(serial)

	// By default, Dynamixel servos come 1-indexed out of the box because reasons
	for i := 1; i <= armServoCount; i++ {
		//Get model ID of each servo
		newServo, err := s_model.New(network, i)
		if err != nil {
			return nil, fmt.Errorf("error initializing servo %d: %v", i, err)
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
