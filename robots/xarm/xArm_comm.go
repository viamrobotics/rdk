package xarm

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"

	"go.viam.com/utils"

	"go.viam.com/core/arm"
	"go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/resource"
	"go.viam.com/core/rlog"

	"go.uber.org/multierr"
)

var servoErrorMap = map[byte]string{
	0x00: "xArm Servo: Joint Communication Error",
	0x0A: "xArm Servo: Current Detection Error",
	0x0B: "xArm Servo: Joint Overcurrent",
	0x0C: "xArm Servo: Joint Overspeed",
	0x0E: "xArm Servo: Position Command Overlimit",
	0x0F: "xArm Servo: Joints Overheat",
	0x10: "xArm Servo: Encoder Initialization Error",
	0x11: "xArm Servo: Single-turn Encoder Error",
	0x12: "xArm Servo: Multi-turn Encoder Error",
	0x13: "xArm Servo: Low Battery Voltage",
	0x14: "xArm Servo: Driver IC Hardware Error",
	0x15: "xArm Servo: Driver IC Init Error",
	0x16: "xArm Servo: Encoder Config Error",
	0x17: "xArm Servo: Large Motor Position Deviation",
	0x1A: "xArm Servo: Joint N Positive Overrun",
	0x1B: "xArm Servo: Joint N Negative Overrun",
	0x1C: "xArm Servo: Joint Commands Error",
	0x21: "xArm Servo: Drive Overloaded",
	0x22: "xArm Servo: Motor Overload",
	0x23: "xArm Servo: Motor Type Error",
	0x24: "xArm Servo: Driver Type Error",
	0x27: "xArm Servo: Joint Overvoltage",
	0x28: "xArm Servo: Joint Undervoltage",
	0x31: "xArm Servo: EEPROM RW Error",
	0x34: "xArm Servo: Initialization of Motor Angle Error",
}

var armBoxErrorMap = map[byte]string{
	0x01: "xArm: Emergency Stop Button Pushed In",
	0x02: "xArm: Emergency IO Triggered",
	0x03: "xArm: Emergency Stop 3-State Switch Pressed",
	0x0B: "xArm: Power Cycle Required",
	0x0C: "xArm: Power Cycle Required",
	0x0D: "xArm: Power Cycle Required",
	0x0E: "xArm: Power Cycle Required",
	0x0F: "xArm: Power Cycle Required",
	0x10: "xArm: Power Cycle Required",
	0x11: "xArm: Power Cycle Required",
	0x13: "xArm: Gripper Communication Error",
	0x15: "xArm: Kinematic Error",
	0x16: "xArm: Self Collision Error",
	0x17: "xArm: Joint Angle Exceeds Limit",
	0x18: "xArm: Speed Exceeds Limit",
	0x19: "xArm: Planning Error",
	0x1A: "xArm: Linux RT Error",
	0x1B: "xArm: Command Reply Error",
	0x1C: "xArm: End Module Communication Error",
	0x1D: "xArm: Other Errors",
	0x1E: "xArm: Feedback Speed Exceeds Limit",
	0x1F: "xArm: Collision Caused Abnormal Current",
	0x20: "xArm: Three-point Drawing Circle Calculation Error",
	0x21: "xArm: Abnormal Arm Current",
	0x22: "xArm: Recording Timeout",
	0x23: "xArm: Safety Boundary Limit",
	0x24: "xArm: Delay Command Limit Exceeded",
	0x25: "xArm: Abnormal Motion in Manual Mode",
	0x26: "xArm: Abnormal Joint Angle",
	0x27: "xArm: Abnormal Communication Between Power Boards",
}
var armBoxWarnMap = map[byte]string{
	0x0B: "xArm Warning: Buffer Overflow",
	0x0C: "xArm Warning: Command Parameter Abnormal",
	0x0D: "xArm Warning: Unknown Command",
	0x0E: "xArm Warning: Command No Solution",
}

var regMap = map[string]byte{
	"Version":     0x01,
	"Shutdown":    0x0A,
	"ToggleServo": 0x0B,
	"SetState":    0x0C,
	"GetState":    0x0D,
	"CmdCount":    0x0E,
	"GetError":    0x0F,
	"ClearError":  0x10,
	"ClearWarn":   0x11,
	"ToggleBrake": 0x12,
	"SetMode":     0x13,
	"MoveJoints":  0x17,
	"ZeroJoints":  0x19,
	"JointPos":    0x2A,
	"SetBound":    0x34,
	"EnableBound": 0x34,
	"SetEEModel":  0x4E,
	"ServoError":  0x6A,
}

type cmd struct {
	tid    uint16
	prot   uint16
	reg    byte
	params []byte
}

func (c *cmd) bytes() []byte {
	var bin []byte
	uintBin := make([]byte, 2)
	binary.BigEndian.PutUint16(uintBin, c.tid)
	bin = append(bin, uintBin...)
	binary.BigEndian.PutUint16(uintBin, c.prot)
	bin = append(bin, uintBin...)
	binary.BigEndian.PutUint16(uintBin, 1+uint16(len(c.params)))
	bin = append(bin, uintBin...)
	bin = append(bin, c.reg)
	bin = append(bin, c.params...)
	return bin
}

func float64fromByte32(bytes []byte) float64 {
	bits := binary.LittleEndian.Uint32(bytes)
	float := math.Float32frombits(bits)
	return float64(float)
}

func (x *xArm) newCmd(reg byte) cmd {
	x.tid++
	return cmd{tid: x.tid, prot: 2, reg: reg}
}

func (x *xArm) send(ctx context.Context, c cmd, checkError bool) (cmd, error) {

	x.moveLock.Lock()

	b := c.bytes()
	_, err := x.conn.Write(b)
	if err != nil {
		return cmd{}, err
	}

	c2, err := x.response(ctx)
	if err != nil {
		return cmd{}, err
	}
	x.moveLock.Unlock()

	if checkError {
		state := c2.params[0]
		if state&96 != 0 {
			// Error (64) and/or warning (32) bit is set
			e2 := multierr.Combine(
				x.readError(ctx),
				x.clearErrorAndWarning(ctx))
			return c2, e2
		}
		// If bit 16 is set, that just means we have not yet activated motion- this happens at startup and shutdown
	}
	return c2, err
}

func (x *xArm) response(ctx context.Context) (cmd, error) {
	// Read response header
	buf, err := utils.ReadBytes(ctx, x.conn, 7)
	if err != nil {
		return cmd{}, err
	}
	c := cmd{}
	c.tid = binary.BigEndian.Uint16(buf[0:2])
	c.prot = binary.BigEndian.Uint16(buf[2:4])
	c.reg = buf[6]
	length := binary.BigEndian.Uint16(buf[4:6])
	c.params, err = utils.ReadBytes(ctx, x.conn, int(length-1))
	if err != nil {
		return cmd{}, err
	}
	return c, err
}

// checkServoErrors will query the individual servos for any servo-specific
// errors. It may be useful for troubleshooting but as the SDK does not call
// it directly ever, we probably don't need to either during normal operation
func (x *xArm) CheckServoErrors(ctx context.Context) error {
	x.mu.RLock()
	defer x.mu.RUnlock()
	c := x.newCmd(regMap["ServoError"])
	e, err := x.send(ctx, c, false)
	if err != nil {
		return err
	}
	if len(e.params) < 18 {
		return errors.New("bad servo error query response")
	}

	// Get error codes for all (8) servos.
	// xArm 6 has 6, xArm 7 has 7, and plus one in the xArm gripper
	for i := 1; i < 9; i++ {
		errCode := e.params[i*2]
		errMsg, isErr := servoErrorMap[errCode]
		if isErr {
			err = multierr.Append(err, errors.New(errMsg))
		}
	}
	return err
}

func (x *xArm) clearErrorAndWarning(ctx context.Context) error {
	c1 := x.newCmd(regMap["ClearError"])
	c2 := x.newCmd(regMap["ClearWarn"])
	_, err1 := x.send(ctx, c1, false)
	_, err2 := x.send(ctx, c2, false)
	err3 := x.setMotionState(context.Background(), 0)
	return multierr.Combine(err1, err2, err3)
}

func (x *xArm) readError(ctx context.Context) error {
	c := x.newCmd(regMap["GetError"])
	e, err := x.send(ctx, c, false)
	if err != nil {
		return err
	}
	if len(e.params) < 3 {
		return errors.New("bad arm error query response")
	}

	errCode := e.params[1]
	warnCode := e.params[2]
	errMsg, isErr := armBoxErrorMap[errCode]
	warnMsg, isWarn := armBoxWarnMap[warnCode]
	if isErr || isWarn {
		return multierr.Combine(errors.New(errMsg),
			errors.New(warnMsg))
	}
	// Commands are returning error codes that are not mentioned in the
	// developer manual
	return errors.New("xArm: UNKNOWN ERROR")
}

// setMotionState sets the motion state of the arm.
// Useful states:
// 0: Servo motion mode
// 3: Suspend current movement
// 4: Stop all motion, restart system
func (x *xArm) setMotionState(ctx context.Context, state byte) error {
	c := x.newCmd(regMap["SetState"])
	c.params = append(c.params, state)
	_, err := x.send(ctx, c, true)
	return err
}

// toggleServos toggles the servos on or off.
// True enables servos and disengages brakes.
// False disables servos without engaging brakes.
func (x *xArm) toggleServos(ctx context.Context, enable bool) error {
	c := x.newCmd(regMap["ToggleServo"])
	var enByte byte
	if enable {
		enByte = 1
	}
	c.params = append(c.params, 8, enByte)
	_, err := x.send(ctx, c, true)
	return err
}

// toggleBrake toggles the brakes on or off.
// True disengages brakes, false engages them.
func (x *xArm) toggleBrake(ctx context.Context, disable bool) error {
	c := x.newCmd(regMap["ToggleBrake"])
	var enByte byte
	if disable {
		enByte = 1
	}
	c.params = append(c.params, 8, enByte)
	_, err := x.send(ctx, c, true)
	return err
}

func (x *xArm) start() error {
	err := x.toggleServos(context.Background(), true)
	if err != nil {
		return err
	}
	return x.setMotionState(context.Background(), 0)
}

// motionWait will block until all arm pieces have stopped moving.
func (x *xArm) motionWait(ctx context.Context) error {
	ready := false
	if !utils.SelectContextOrWait(ctx, 50*time.Millisecond) {
		return ctx.Err()
	}
	slept := 0
	for !ready {
		if !utils.SelectContextOrWait(ctx, 50*time.Millisecond) {
			return ctx.Err()
		}
		slept += 50
		// Error if we've been waiting more than 15 seconds for motion
		if slept > 25000 {
			return errors.New("motionWait continued to detect motion after 25 seconds")
		}
		c := x.newCmd(regMap["GetState"])
		sData, err := x.send(ctx, c, true)
		if err != nil {
			return err
		}
		if len(sData.params) < 2 {
			return errors.New("malformed state data response in motionWait")
		}
		if sData.params[1] != 1 {
			ready = true
		}
	}
	return nil
}

// Close shuts down the arm servos and engages brakes.
func (x *xArm) Close() error {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.close()
}

func (x *xArm) close() error {
	err := x.toggleBrake(context.Background(), false)
	if err != nil {
		return err
	}
	err = x.toggleServos(context.Background(), false)
	if err != nil {
		return err
	}
	err = x.setMotionState(context.Background(), 4)
	if err != nil {
		return err
	}
	return x.conn.Close()
}

// MoveToJointPositions moves the arm to the requested joint positions.
func (x *xArm) MoveToJointPositions(ctx context.Context, newPositions *pb.JointPositions) error {
	x.mu.RLock()
	defer x.mu.RUnlock()
	radians := arm.JointPositionsToRadians(newPositions)
	c := x.newCmd(regMap["MoveJoints"])
	jFloatBytes := make([]byte, 4)
	for _, jRad := range radians {
		binary.LittleEndian.PutUint32(jFloatBytes, math.Float32bits(float32(jRad)))
		c.params = append(c.params, jFloatBytes...)
	}
	// xarm 6 has 6 joints, but protocol needs 7- add 4 bytes for a blank 7th joint
	for dof := x.dof; dof < 7; dof++ {
		c.params = append(c.params, 0, 0, 0, 0)
	}
	// Add speed
	binary.LittleEndian.PutUint32(jFloatBytes, math.Float32bits(x.speed))
	c.params = append(c.params, jFloatBytes...)
	// Add accel
	binary.LittleEndian.PutUint32(jFloatBytes, math.Float32bits(x.accel))
	c.params = append(c.params, jFloatBytes...)

	// add motion time, 0
	c.params = append(c.params, 0, 0, 0, 0)
	_, err := x.send(ctx, c, true)
	if err != nil {
		return err
	}
	return x.motionWait(ctx)
}

// JointMoveDelta TODO
func (x *xArm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return errors.New("not done yet")
}

// CurrentPosition computes and returns the current cartesian position.
func (x *xArm) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	x.mu.RLock()
	defer x.mu.RUnlock()
	joints, err := x.CurrentJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return kinematics.ComputePosition(x.ik.Model(), joints)
}

// MoveToPosition moves the arm to the specified cartesian position.
func (x *xArm) MoveToPosition(ctx context.Context, pos *pb.ArmPosition) error {
	x.mu.RLock()
	defer x.mu.RUnlock()
	joints, err := x.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}
	solution, err := x.ik.Solve(ctx, pos, frame.JointPosToInputs(joints))
	if err != nil {
		return err
	}
	return x.MoveToJointPositions(ctx, frame.InputsToJointPos(solution))
}

// CurrentJointPositions returns the current positions of all joints.
func (x *xArm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	x.mu.RLock()
	defer x.mu.RUnlock()
	c := x.newCmd(regMap["JointPos"])

	jData, err := x.send(ctx, c, true)
	if err != nil {
		return &pb.JointPositions{}, err
	}
	var radians []float64
	for i := 0; i < x.dof; i++ {
		idx := i*4 + 1
		radians = append(radians, float64fromByte32(jData.params[idx:idx+4]))
	}

	return arm.JointPositionsFromRadians(radians), nil
}

// Reconfigure reconfigures the current resource to the resource passed in.
func (x *xArm) Reconfigure(newResource resource.Resource) {
	x.mu.Lock()
	defer x.mu.Unlock()
	actual, ok := newResource.(*xArm)
	if !ok {
		panic(fmt.Errorf("expected new resource to be %T but got %T", actual, newResource))
	}
	if err := x.close(); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	x.dof = actual.dof
	x.tid = actual.tid
	x.conn = actual.conn
	x.speed = actual.speed
	x.accel = actual.accel
	x.ik = actual.ik
}
