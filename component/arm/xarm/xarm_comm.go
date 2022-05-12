package xarm

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"time"

	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/motionplan"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/referenceframe"
	rutils "go.viam.com/rdk/utils"
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
	"MoveJoints":  0x1D,
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

func (x *xArm) newCmd(reg byte) cmd {
	x.tid++
	return cmd{tid: x.tid, prot: 2, reg: reg}
}

func (x *xArm) send(ctx context.Context, c cmd, checkError bool) (cmd, error) {
	x.moveLock.Lock()

	b := c.bytes()
	_, err := x.conn.Write(b)
	if err != nil {
		x.moveLock.Unlock()
		return cmd{}, err
	}

	c2, err := x.response(ctx)
	x.moveLock.Unlock()
	if err != nil {
		return cmd{}, err
	}

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
// it directly ever, we probably don't need to either during normal operation.
func (x *xArm) CheckServoErrors(ctx context.Context) error {
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
	err3 := x.setMotionMode(ctx, 1)
	err4 := x.setMotionState(ctx, 0)
	return multierr.Combine(err1, err2, err3, err4)
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

// setMotionMode sets the motion mode of the arm.
// 0: Position Control Mode, i.e. "normal" mode
// 1: Servoj mode. This mode will immediately execute joint positions at the fastest available speed and is intended
// for streaming large numbers of joint positions to the arm.
// 2: Joint teaching mode, not useful right now
func (x *xArm) setMotionMode(ctx context.Context, state byte) error {
	c := x.newCmd(regMap["SetMode"])
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

func (x *xArm) start(ctx context.Context) error {
	err := x.toggleServos(ctx, true)
	if err != nil {
		return err
	}
	err = x.setMotionMode(ctx, 1)
	if err != nil {
		return err
	}
	return x.setMotionState(ctx, 0)
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
func (x *xArm) Close(ctx context.Context) error {
	err := x.toggleBrake(ctx, false)
	if err != nil {
		return err
	}
	err = x.toggleServos(ctx, false)
	if err != nil {
		return err
	}
	err = x.setMotionState(ctx, 4)
	if err != nil {
		return err
	}
	return x.conn.Close()
}

// MoveToJointPositions moves the arm to the requested joint positions.
func (x *xArm) MoveToJointPositions(ctx context.Context, newPositions *pb.JointPositions) error {
	ctx, done := x.opMgr.New(ctx)
	defer done()
	to := referenceframe.JointPosToInputs(newPositions)
	curPos, err := x.GetJointPositions(ctx)
	if err != nil {
		return err
	}
	from := referenceframe.JointPosToInputs(curPos)

	diff := getMaxDiff(from, to)
	nSteps := int((diff / float64(x.speed)) * x.moveHZ)
	for i := 1; i <= nSteps; i++ {
		step := referenceframe.InputsToFloats(referenceframe.InterpolateInputs(from, to, float64(i)/float64(nSteps)))

		c := x.newCmd(regMap["MoveJoints"])
		jFloatBytes := make([]byte, 4)
		for _, jRad := range step {
			binary.LittleEndian.PutUint32(jFloatBytes, math.Float32bits(float32(jRad)))
			c.params = append(c.params, jFloatBytes...)
		}
		// xarm 6 has 6 joints, but protocol needs 7- add 4 bytes for a blank 7th joint
		for dof := x.dof; dof < 7; dof++ {
			c.params = append(c.params, 0, 0, 0, 0)
		}
		// When in servoj mode, motion time, speed, and acceleration are not handled by the control box
		c.params = append(c.params, 0, 0, 0, 0)
		c.params = append(c.params, 0, 0, 0, 0)
		c.params = append(c.params, 0, 0, 0, 0)
		_, err := x.send(ctx, c, true)
		if err != nil {
			return err
		}
		if !utils.SelectContextOrWait(ctx, time.Duration(1000000./x.moveHZ)*time.Microsecond) {
			return ctx.Err()
		}
	}
	return nil
}

// GetEndPosition computes and returns the current cartesian position.
func (x *xArm) GetEndPosition(ctx context.Context) (*commonpb.Pose, error) {
	joints, err := x.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return motionplan.ComputePosition(x.mp.Frame(), joints)
}

// MoveToPosition moves the arm to the specified cartesian position.
func (x *xArm) MoveToPosition(ctx context.Context, pos *commonpb.Pose, worldState *commonpb.WorldState) error {
	ctx, done := x.opMgr.New(ctx)
	defer done()
	joints, err := x.GetJointPositions(ctx)
	if err != nil {
		return err
	}
	solution, err := x.mp.Plan(ctx, pos, referenceframe.JointPosToInputs(joints), nil)
	if err != nil {
		return err
	}
	err = arm.GoToWaypoints(ctx, x, solution)
	if err != nil {
		return err
	}
	return x.motionWait(ctx)
}

// GetJointPositions returns the current positions of all joints.
func (x *xArm) GetJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	c := x.newCmd(regMap["JointPos"])

	jData, err := x.send(ctx, c, true)
	if err != nil {
		return &pb.JointPositions{}, err
	}
	var radians []float64
	for i := 0; i < x.dof; i++ {
		idx := i*4 + 1
		radians = append(radians, float64(rutils.Float32FromBytesLE((jData.params[idx : idx+4]))))
	}

	return referenceframe.JointPositionsFromRadians(radians), nil
}

func getMaxDiff(from, to []referenceframe.Input) float64 {
	maxDiff := 0.
	for i, fromI := range from {
		diff := math.Abs(fromI.Value - to[i].Value)
		if diff > maxDiff {
			maxDiff = diff
		}
	}
	return maxDiff
}
