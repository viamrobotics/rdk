// Package trossen implements arms from Trossen Robotics.
package trossen

import (
	"context"
	// for embedding model file.
	_ "embed"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/jacobsa/go-serial/serial"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/dynamixel/network"
	"go.viam.com/dynamixel/servo"
	"go.viam.com/dynamixel/servo/s_model"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

// This is an implementation of the Arm API for Trossen arm models
// vx300s and wx250s ONLY (these codenames can be found at https://docs.trossenrobotics.com/interbotix_xsarms_docs/)
// Specifications for vx300s (ViperX-300 6DOF):
// https://docs.trossenrobotics.com/interbotix_xsarms_docs/specifications/vx300s.html
// Specifications for wx250s (WidowX-250 6DOF):
// https://docs.trossenrobotics.com/interbotix_xsarms_docs/specifications/wx250s.html
// WARNING: This implementation is experimental and not currently stable.

const servoCount = 9

var (
	modelNameWX250s = resource.NewDefaultModel("trossen-wx250s")
	modelNameVX300s = resource.NewDefaultModel("trossen-vx300s")
)

// SleepAngles are the angles we go to prepare to turn off torque.
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
	generic.Unimplemented
	Joints   map[string][]*servo.Servo
	moveLock *sync.Mutex
	logger   golog.Logger
	robot    robot.Robot
	model    referenceframe.Model
	opMgr    operation.SingleOperationManager
}

// servoPosToValues takes a 360 degree 0-4096 servo position, centered at 2048,
// and converts it to degrees, centered at 0.
func servoPosToValues(pos float64) float64 {
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

// AttrConfig is used for converting Arm config attributes.
type AttrConfig struct {
	UsbPort  string `json:"serial_path"`
	BaudRate int    `json:"serial_baud_rate,omitempty"`
	// NOTE: ArmServoCount is currently unused because both
	// supported arms are 9 servo arms - GV
	ArmServoCount int `json:"arm_servo_count,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) error {
	if len(config.UsbPort) == 0 {
		return utils.NewConfigValidationFieldRequiredError(path, "serial_path")
	}

	return nil
}

//go:embed trossen_wx250s_kinematics.json
var wx250smodeljson []byte

//go:embed trossen_vx300s_kinematics.json
var vx300smodeljson []byte

func init() {
	registry.RegisterComponent(arm.Subtype, modelNameWX250s, registry.Component{
		RobotConstructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewArm(r, config, logger, wx250smodeljson)
		},
	})

	config.RegisterComponentAttributeMapConverter(arm.Subtype, modelNameWX250s,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})

	registry.RegisterComponent(arm.Subtype, modelNameVX300s, registry.Component{
		RobotConstructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewArm(r, config, logger, vx300smodeljson)
		},
	})

	config.RegisterComponentAttributeMapConverter(arm.Subtype, modelNameVX300s,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

// NewArm returns an instance of Arm given a model json.
func NewArm(r robot.Robot, cfg config.Component, logger golog.Logger, json []byte) (arm.LocalArm, error) {
	attributes, ok := cfg.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attributes, cfg.ConvertedAttributes)
	}
	usbPort := attributes.UsbPort
	baudRate := attributes.BaudRate
	if baudRate == 0 {
		baudRate = 1000000
	}
	servos, err := findServos(usbPort, baudRate)
	if err != nil {
		return nil, err
	}

	model, err := referenceframe.UnmarshalModelJSON(json, cfg.Name)
	if err != nil {
		return nil, err
	}

	a := &Arm{
		Joints: map[string][]*servo.Servo{
			"Waist":       {servos[0]},
			"Shoulder":    {servos[1], servos[2]},
			"Elbow":       {servos[3], servos[4]},
			"Forearm_rot": {servos[5]},
			"Wrist":       {servos[6]},
			"Wrist_rot":   {servos[7]},
			"Gripper":     {servos[8]},
		},
		moveLock: getPortMutex(usbPort),
		logger:   logger,
		robot:    r,
		model:    model,
	}
	// start the arm in an open gripper state
	err = a.OpenGripper(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "trossen arm failed to initialize, could not open gripper")
	}
	return a, nil
}

// EndPosition computes and returns the current cartesian position.
func (a *Arm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	joints, err := a.JointPositions(ctx, extra)
	if err != nil {
		return nil, err
	}
	return motionplan.ComputeOOBPosition(a.model, joints)
}

// MoveToPosition moves the arm to the specified cartesian position.
func (a *Arm) MoveToPosition(
	ctx context.Context,
	pos spatialmath.Pose,
	worldState *referenceframe.WorldState,
	extra map[string]interface{},
) error {
	ctx, done := a.opMgr.New(ctx)
	defer done()
	return arm.Move(ctx, a.robot, a, pos, worldState)
}

// MoveToJointPositions takes a list of degrees and sets the corresponding joints to that position.
func (a *Arm) MoveToJointPositions(ctx context.Context, jp *pb.JointPositions, extra map[string]interface{}) error {
	// check that joint positions are not out of bounds
	if err := arm.CheckDesiredJointPositions(ctx, a, jp.Values); err != nil {
		return err
	}
	ctx, done := a.opMgr.New(ctx)
	defer done()
	if len(jp.Values) > len(a.jointOrder()) {
		return errors.New("passed in too many positions")
	}

	a.moveLock.Lock()
	defer a.moveLock.Unlock()

	// TODO(pl): make block configurable
	block := false
	for i, pos := range jp.Values {
		a.JointTo(a.jointOrder()[i], degreeToServoPos(pos), block)
	}
	return a.waitForMovement(ctx)
}

// JointPositions returns an empty struct, because the wx250s should use joint angles from kinematics.
func (a *Arm) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	angleMap, err := a.GetAllAngles()
	if err != nil {
		return &pb.JointPositions{}, err
	}

	numJoints := len(a.jointOrder())
	positions := make([]float64, 0, numJoints)
	for _, jointName := range a.jointOrder() {
		positions = append(positions, servoPosToValues(angleMap[jointName]))
	}

	return &pb.JointPositions{Values: positions}, nil
}

// OpenGripper opens the gripper.
func (a *Arm) OpenGripper(ctx context.Context) error {
	ctx, done := a.opMgr.New(ctx)
	defer done()
	a.moveLock.Lock()
	defer a.moveLock.Unlock()

	pos, err := a.Joints["Gripper"][0].PresentPosition()
	if err != nil {
		return errors.Wrap(err, "position retrieval failed when opening gripper")
	}
	if pos >= 2800 {
		a.logger.Debug("gripper already open, returning")
		return nil
	}

	err = a.Joints["Gripper"][0].SetGoalPWM(150)
	if err != nil {
		return err
	}
	a.logger.Debug("gripper pwm set to 150")

	// We don't want to over-open
	atPos := false
	for !atPos {
		var pos int
		pos, err = a.Joints["Gripper"][0].PresentPosition()
		if err != nil {
			return errors.Wrap(err, "position retrieval failed when opening gripper")
		}
		if pos < 2800 {
			if !utils.SelectContextOrWait(ctx, 50*time.Millisecond) {
				return ctx.Err()
			}
		} else {
			atPos = true
			a.logger.Debug("reached open gripper position")
		}
	}
	err = a.Joints["Gripper"][0].SetGoalPWM(0)
	if err != nil {
		return errors.Wrap(err, "failed to set gripper PWM to 0")
	}
	return nil
}

// Grab closes the gripper.
func (a *Arm) Grab(ctx context.Context) (bool, error) {
	_, done := a.opMgr.New(ctx)
	defer done()
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	err := a.Joints["Gripper"][0].SetGoalPWM(-350)
	if err != nil {
		return false, err
	}
	a.logger.Debug("gripper pwm set to -350")
	err = servo.WaitForMovementVar(a.Joints["Gripper"][0])
	if err != nil {
		setPWMErr := a.Joints["Gripper"][0].SetGoalPWM(0)
		return false, multierr.Combine(err, setPWMErr)
	}
	pos, err := a.Joints["Gripper"][0].PresentPosition()
	if err != nil {
		return false, err
	}
	a.logger.Debug(fmt.Sprintf("gripper position at %d (after grab)", pos))
	didGrab := true

	// If servo position is less than 1500, it's closed and we grabbed nothing
	if pos < 1500 {
		didGrab = false
	}
	return didGrab, nil
}

// StopGripper stops the gripper servo.
func (a *Arm) StopGripper(ctx context.Context) error {
	a.opMgr.CancelRunning(ctx)
	err := a.Joints["Gripper"][0].SetTorqueEnable(false)
	if err != nil {
		return err
	}
	return a.Joints["Gripper"][0].SetTorqueEnable(true)
}

// GripperIsMoving returns whether the gripper servo is moving.
func (a *Arm) GripperIsMoving(ctx context.Context) (bool, error) {
	isMovingInt, err := a.Joints["Gripper"][0].Moving()
	if err != nil {
		return false, err
	}
	return (isMovingInt == 1), nil
}

// Stop stops the servos of the arm.
func (a *Arm) Stop(ctx context.Context, extra map[string]interface{}) error {
	a.opMgr.CancelRunning(ctx)
	return multierr.Combine(
		a.TorqueOff(),
		a.TorqueOn(),
	)
}

// IsMoving returns whether the arm is moving.
func (a *Arm) IsMoving(ctx context.Context) (bool, error) {
	return a.opMgr.OpRunning(), nil
}

// Close will get the arm ready to be turned off.
func (a *Arm) Close(ctx context.Context) error {
	// First, check if we are approximately in the sleep position
	// If so, we can just turn off torque
	// If not, let's move through the home position first
	angles, err := a.GetAllAngles()
	if err != nil {
		return errors.Wrap(err, "failed to get angles on component close")
	}
	alreadyAtSleep := true
	gripperIsOpen := true
	for _, joint := range a.jointOrder() {
		if !within(angles[joint], SleepAngles[joint], 15) && !within(angles[joint], OffAngles[joint], 15) {
			alreadyAtSleep = false
		}
	}
	gripperPos, err := a.Joints["Gripper"][0].PresentPosition()
	if err != nil {
		a.logger.Errorf("failed to get gripper position on close: %s", err)
		gripperIsOpen = true
	} else if gripperPos >= 2800 {
		gripperIsOpen = false
	}
	if !alreadyAtSleep {
		err = a.HomePosition(context.Background())
		if err != nil {
			return errors.Wrap(err, "home position error")
		}
		err = a.SleepPosition(context.Background())
		if err != nil {
			return errors.Wrap(err, "sleep position err")
		}
	} else {
		a.logger.Debug("trossen arm already at sleep, proceeding to close component")
	}
	if !gripperIsOpen {
		err = a.OpenGripper(context.Background())
		if err != nil {
			a.logger.Errorf("gripper failed to open on component c,ose: %s", err)
		}
	} else {
		a.logger.Debug("gripper already open, proceeding to close component")
	}
	err = a.TorqueOff()
	if err != nil {
		return errors.Wrap(err, "torque off error in close")
	}
	return nil
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
		angleMean := float64(angleSum) / float64(len(servos))
		angles[jointName] = angleMean
	}
	return angles, nil
}

func (a *Arm) jointOrder() []string {
	return []string{"Waist", "Shoulder", "Elbow", "Forearm_rot", "Wrist", "Wrist_rot"}
}

// PrintPositions prints positions of all servos.
// TODO(pl): Print joint names, not just servo numbers.
func (a *Arm) PrintPositions() error {
	posString := ""
	for i, s := range a.GetAllServos(true) {
		pos, err := s.PresentPosition()
		if err != nil {
			return err
		}
		posString = fmt.Sprintf("%s || %d : %d, %f degrees", posString, i, pos, servoPosToValues(float64(pos)))
	}
	return nil
}

// GetAllServos returns a slice containing all servos in the arm.
func (a *Arm) GetAllServos(includeGripper bool) []*servo.Servo {
	var servos []*servo.Servo
	for _, joint := range a.jointOrder() {
		servos = append(servos, a.Joints[joint]...)
	}
	if includeGripper {
		servos = append(servos, a.Joints["Gripper"]...)
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
	for _, s := range a.GetAllServos(false) {
		err := s.SetProfileAcceleration(accel)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetVelocity sets velocity for servos in travel time;
// recommended value 1000.
func (a *Arm) SetVelocity(veloc int) error {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	for _, s := range a.GetAllServos(false) {
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
	for _, s := range a.GetAllServos(true) {
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
	for _, s := range a.GetAllServos(true) {
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
	defer a.moveLock.Unlock()
	sleepWait := false
	a.JointTo("Waist", 2048, sleepWait)
	a.JointTo("Shoulder", 840, sleepWait)
	a.JointTo("Wrist_rot", 2048, sleepWait)
	a.JointTo("Wrist", 2509, sleepWait)
	a.JointTo("Forearm_rot", 2048, sleepWait)
	a.JointTo("Elbow", 3090, sleepWait)
	return a.waitForMovement(ctx)
}

// GetMoveLock TODO.
func (a *Arm) GetMoveLock() *sync.Mutex {
	return a.moveLock
}

// HomePosition goes to the home position.
func (a *Arm) HomePosition(ctx context.Context) error {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()

	wait := false
	for jointName := range a.Joints {
		if jointName != "Gripper" {
			a.JointTo(jointName, 2048, wait)
		}
	}
	return a.waitForMovement(ctx)
}

// CurrentInputs TODO.
func (a *Arm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := a.JointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	return a.model.InputFromProtobuf(res), nil
}

// GoToInputs TODO.
func (a *Arm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	positionDegs := a.model.ProtobufFromInput(goal)
	if err := arm.CheckDesiredJointPositions(ctx, a, positionDegs.Values); err != nil {
		return err
	}
	return a.MoveToJointPositions(ctx, positionDegs, nil)
}

// waitForMovement takes some servos, and will block until the servos are done moving.
// The arm's moveLock MUST be locked before calling this function.
func (a *Arm) waitForMovement(ctx context.Context) error {
	allAtPos := false

	for !allAtPos {
		if !utils.SelectContextOrWait(ctx, 200*time.Millisecond) {
			return ctx.Err()
		}
		allAtPos = true
		for _, s := range a.GetAllServos(true) {
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
	// Set some nice-to-have settings
	err := newServo.SetTorqueEnable(false)
	if err != nil {
		return err
	}
	err = newServo.SetMovingThreshold(0)
	if err != nil {
		return errors.Wrapf(err, "error SetMovingThreshold servo %d", newServo.ID)
	}
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

// findServos finds the specified number of Dynamixel servos on the specified USB port
// we are going to hardcode some USB parameters that we will literally never want to change.
func findServos(usbPort string, baudRate int) ([]*servo.Servo, error) {
	if baudRate == 0 {
		return nil, errors.New("non-zero serial_baud_rate expected")
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

	// By default, Dynamixel servo IDs in the trossen arm come 1-indexed
	// from the waist joint upwards
	for i := 1; i <= servoCount; i++ {
		// Get model ID of each servo
		newServo, err := s_model.New(network, i)
		if err != nil {
			return nil, errors.Wrapf(err, "error initializing servo %d", i)
		}
		// Don't set the defaults for the gripper servo. REVISIT: don't set defaults
		// for any servo? The arm should be shipped with the servos pre-configured
		if i != servoCount {
			err = setServoDefaults(newServo)
		} else {
			err = newServo.SetTorqueEnable(true)
		}
		if err != nil {
			return nil, err
		}

		servos = append(servos, newServo)
		time.Sleep(500 * time.Millisecond)
	}

	return servos, nil
}

func within(a, b, c float64) bool {
	return math.Abs(a-b) <= c
}
