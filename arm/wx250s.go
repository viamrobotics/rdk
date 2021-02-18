package arm

import (
	"fmt"

	"github.com/edaniels/golog"
	"github.com/jacobsa/go-serial/serial"
	"github.com/viamrobotics/dynamixel/network"
	"github.com/viamrobotics/dynamixel/servo"
	"github.com/viamrobotics/dynamixel/servo/s_model"

	"strconv"
	"sync"
	"time"
)


type Wx250s struct {
	Joints   map[string][]*servo.Servo
	moveLock sync.Mutex
	kin      *Kinematics
}

// servoPosToRadians takes a 360 degree 0-4096 servo position, centered at 2048,
// and converts it to radians, centered at 0
//~ func servoPosToRadians(pos float64) float64 {
//~ return (pos - 2048) * (math.Pi / 2048)
//~ }

// radiansToServoPos takes a 0-centered radian and converts to a 360 degree 0-4096 servo position, centered at 2048
//~ func radiansToServoPos(pos float64) int {
//~ return int(2048 + (pos/math.Pi)*2048)
//~ }

// servoPosToDegrees takes a 360 degree 0-4096 servo position, centered at 2048,
// and converts it to degrees, centered at 0
func servoPosToDegrees(pos float64) float64 {
	return ((pos - 2048) * 180) / 2048
}

// degreeToServoPos takes a 0-centered radian and converts to a 360 degree 0-4096 servo position, centered at 2048
func degreeToServoPos(pos float64) int {
	return int(2048 + (pos/180)*2048)
}

func NewWx250s(attributes map[string]string) (*Wx250s, error) {
	servos := findServos(attributes["usbPort"], attributes["baudRate"], attributes["armServoCount"])
	kin, err := NewRobot(attributes["modelXML"])
	if err != nil{
		golog.Global.Errorf("Could not initialize kinematics: %s", err)
	}
	
	newArm := Wx250s{
		kin: kin,
		Joints: map[string][]*servo.Servo{
			"Waist":       {servos[0]},
			"Shoulder":    {servos[1], servos[2]},
			"Elbow":       {servos[3], servos[4]},
			"Forearm_rot": {servos[5]},
			"Wrist":       {servos[6]},
			"Wrist_rot":   {servos[7]},
		},
	}
	err = newArm.SetVelocity(2000)
	if err != nil {
		golog.Global.Errorf("Could not set arm velocity: %s", err)
	}
	err = newArm.TorqueOn()
	if err != nil {
		golog.Global.Errorf("Could not set arm torque: %s", err)
	}
	return &newArm, err
}

func (a *Wx250s) CurrentPosition() (Position, error) {

	ci := Position{}
	setJointTelNums := []float64{}

	// Update kinematics model with current robot location
	curPos, err := a.CurrentJointPositions()
	if err != nil {
		return ci, err
	}
	setJointTelNums = append(setJointTelNums, curPos.Degrees[0:6]...)

	// HACK my joint angles are reversed for these joints. Fix.
	setJointTelNums[1] *= -1
	setJointTelNums[2] *= -1
	
	a.kin.SetJointPositions(setJointTelNums)
	ci = a.kin.GetForwardPosition()
	ci.X /= 1000
	ci.Y /= 1000
	ci.Z /= 1000
	return ci, nil
}

//TODO: Motion planning rather than just setting the position
func (a *Wx250s) MoveToPosition(c Position) error {
	c.X *= 1000
	c.Y *= 1000
	c.Z *= 1000
	
	err := a.kin.SetForwardPosition(c)
	if err != nil {
		return err
	}

	servoPosList := a.kin.GetJointPositions()
	// HACK my joint angles are reversed for these joints. Fix.
	servoPosList[1] *= -1
	servoPosList[2] *= -1
	return a.MoveToJointPositions(JointPositions{servoPosList})
}

// MoveToJointPositions takes a list of degrees and sets the corresponding joints to that position
func (a *Wx250s) MoveToJointPositions(jp JointPositions) error {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()

	if len(jp.Degrees) > len(a.JointOrder()) {
		return fmt.Errorf("passed in too many positions")
	}

	// TODO: make block configurable
	block := false
	for i, pos := range jp.Degrees {
		a.JointTo(a.JointOrder()[i], degreeToServoPos(pos), block)
	}
	return nil
}

// CurrentJointPositions returns a sorted (from base outwards) slice of joint angles in degrees
func (a *Wx250s) CurrentJointPositions() (JointPositions, error) {

	angleMap, err := a.GetAllAngles()
	if err != nil {
		return JointPositions{}, err
	}

	var positions []float64
	for _, jointName := range a.JointOrder() {
		//2048 is the halfway position for Dynamixel servos
		// TODO: Function for servo pos/degree/radian conversion
		positions = append(positions, servoPosToDegrees(angleMap[jointName]))
	}
	return JointPositions{positions}, nil
}

func (a *Wx250s) JointMoveDelta(joint int, amount float64) error {
	return fmt.Errorf("not done yet")
}

// Close will get the arm ready to be turned off
func (a *Wx250s) Close() {
	err := a.HomePosition()
	if err != nil {
		golog.Global.Errorf("Home position error: %s", err)
	}
	err = a.SleepPosition()
	if err != nil {
		golog.Global.Errorf("Sleep pos error: %s", err)
	}
	golog.Global.Errorf("Torque off error: %s", a.TorqueOff())
}

// GetAllAngles will return a map of the angles of each joint, denominated in servo position
func (a *Wx250s) GetAllAngles() (map[string]float64, error) {
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

func (a *Wx250s) JointOrder() []string {
	return []string{"Waist", "Shoulder", "Elbow", "Forearm_rot", "Wrist", "Wrist_rot"}
}

// Note: there are a million and a half different ways to move servos
// GoalPosition, GoalCurrent, GoalPWM, etc
// To start with I'll just use GoalPosition
// TODO: Support other movement types
// TODO: Configurable waiting for movement to complete or not
// TODO: write more TODOS for nice-to-have features

// Grippers are special because they use PWM by default rather than position control
// Note that goal PWM values not in [-350:350] may cause the servo to overload, necessitating an arm reboot
// TODO: Track position or something rather than just have a timer
//~ func (a *Wx250s) CloseGripper(block bool) error {
//~ err := a.Joints["Gripper"][0].SetGoalPWM(-350)
//~ if block {
//~ err = servo.WaitForMovementVar(a.Joints["Gripper"][0])
//~ }
//~ return err
//~ }

//~ // See CloseGripper()
//~ func (a *Wx250s) OpenGripper() error {
//~ err := a.Joints["Gripper"][0].SetGoalPWM(250)
//~ if err != nil {
//~ return err
//~ }

//~ // We don't want to over-open
//~ atPos := false
//~ for !atPos {
//~ var pos int
//~ pos, err = a.Joints["Gripper"][0].PresentPosition()
//~ if err != nil {
//~ return err
//~ }
//~ // TODO: Don't harcode
//~ if pos < 3000 {
//~ time.Sleep(50 * time.Millisecond)
//~ } else {
//~ atPos = true
//~ }
//~ }
//~ return err
//~ }

// Print positions of all servos
// TODO: Print joint names, not just servo numbers
func (a *Wx250s) PrintPositions() error {
	posString := ""
	for i, s := range a.GetAllServos() {
		pos, err := s.PresentPosition()
		if err != nil {
			return err
		}
		posString = fmt.Sprintf("%s || %d : %d", posString, i, pos)
	}
	return nil
}

// Return a slice containing all servos in the arm
func (a *Wx250s) GetAllServos() []*servo.Servo {
	var servos []*servo.Servo
	for _, joint := range(a.JointOrder()){
		servos = append(servos, a.Joints[joint]...)
	}
	return servos
}

// Return a slice containing all servos in the named joint
func (a *Wx250s) GetServos(jointName string) []*servo.Servo {
	var servos []*servo.Servo
	servos = append(servos, a.Joints[jointName]...)
	return servos
}

// Set Acceleration for servos
func (a *Wx250s) SetAcceleration(accel int) error {
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
func (a *Wx250s) SetVelocity(veloc int) error {
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
func (a *Wx250s) TorqueOn() error {
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
func (a *Wx250s) TorqueOff() error {
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
func (a *Wx250s) JointTo(jointName string, pos int, block bool) {

	if pos > 4095 {
		pos = 4095
	} else if pos < 0 {
		pos = 0
	}

	err := servo.GoalAndTrack(pos, block, a.GetServos(jointName)...)
	if err != nil {
		golog.Global.Errorf("jointTo error: %s", err)
	}
}

//Go back to the sleep position, ready to turn off torque
func (a *Wx250s) SleepPosition() error {
	a.moveLock.Lock()
	sleepWait := false
	a.JointTo("Waist", 2048, sleepWait)
	a.JointTo("Shoulder", 840, sleepWait)
	a.JointTo("Wrist_rot", 2048, sleepWait)
	a.JointTo("Wrist", 2509, sleepWait)
	a.JointTo("Forearm_rot", 2048, sleepWait)
	a.JointTo("Elbow", 3090, sleepWait)
	a.moveLock.Unlock()
	return a.WaitForMovement()
}

//Go to the home position
func (a *Wx250s) HomePosition() error {
	a.moveLock.Lock()

	wait := false
	for jointName := range a.Joints {
		a.JointTo(jointName, 2048, wait)
	}
	a.moveLock.Unlock()
	return a.WaitForMovement()
}

// WaitForMovement takes some servos, and will block until the servos are done moving
func (a *Wx250s) WaitForMovement() error {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	allAtPos := false

	for !allAtPos {
		time.Sleep(200 * time.Millisecond)
		allAtPos = true
		for _, s := range a.GetAllServos() {
			isMoving, err := s.Moving()
			if err != nil {
				return err
			}
			// TODO: Make this configurable
			if isMoving != 0 {
				allAtPos = false
			}
		}
	}
	return nil
}

// Find the specified number of Dynamixel servos on the specified USB port
// We're going to hardcode some USB parameters that we will literally never want to change
func findServos(usbPort, baudRateStr, armServoCountStr string) []*servo.Servo {
	baudRate, err := strconv.Atoi(baudRateStr)
	if err != nil {
		golog.Global.Fatalf("Mangled baudrate: %v\n", err)
	}
	armServoCount, err := strconv.Atoi(armServoCountStr)
	if err != nil {
		golog.Global.Fatalf("Mangled servo count: %v\n", err)
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
		golog.Global.Fatalf("error opening serial port: %v\n", err)
	}

	var servos []*servo.Servo

	network := network.New(serial)

	// By default, Dynamixel servos come 1-indexed out of the box because reasons
	for i := 1; i <= armServoCount; i++ {
		//Get model ID of each servo
		newServo, err := s_model.New(network, i)
		if err != nil {
			golog.Global.Fatalf("error initializing servo %d: %v\n", i, err)
		}
		// Set some nice-to-have settings
		err = newServo.SetMovingThreshold(0)
		if err != nil {
			golog.Global.Fatalf("error initializing servo %d: %v\n", i, err)
		}
		err = newServo.SetPGain(1800)
		if err != nil {
			golog.Global.Fatalf("error initializing servo %d: %v\n", i, err)
		}
		err = newServo.SetIGain(10)
		if err != nil {
			golog.Global.Fatalf("error initializing servo %d: %v\n", i, err)
		}
		servos = append(servos, newServo)
	}

	return servos
}
