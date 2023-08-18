// Package yahboom implements a yahboom based robot.
// code with commands found at http://www.yahboom.net/study/Dofbot-Pi
package yahboom

import (
	"context"
	// for embedding model file.
	_ "embed"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	componentpb "go.viam.com/api/component/arm/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// Model is the model used to refer to the yahboom model.
var Model = resource.DefaultModelFamily.WithModel("yahboom-dofbot")

//go:embed dofbot.json
var modeljson []byte

// MakeModelFrame returns the kinematics model of the yahboom arm, also has all Frame information.
func MakeModelFrame(name string) (referenceframe.Model, error) {
	return referenceframe.UnmarshalModelJSON(modeljson, name)
}

type jointConfig struct {
	x, y, z float64
	offset  float64
}

var joints = []jointConfig{
	{2200, 180, 100, 150},
	{2200, 180, 100, 240},
	{2200, 180, 100, 158},
	{2200, 180, 100, 150},
	{2200, 180, 100, 110},
	{2200, 180, 100, 0},
}

func (jc jointConfig) toValues(n int) float64 {
	d := float64(n) - jc.z
	d /= jc.x
	d *= jc.y
	return d - jc.offset
}

func (jc jointConfig) toHw(degrees float64) int {
	degrees = math.Max(-270, degrees)
	degrees = math.Min(270, degrees)
	hw := int((jc.x * ((degrees + jc.offset) / jc.y)) + jc.z)
	if hw < 0 {
		hw = 0
	}
	return hw
}

// Config is the config for a yahboom arm.
type Config struct {
	Board string `json:"board"`
	I2C   string `json:"i2c"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	if conf.Board == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
	}

	if conf.I2C == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "i2c")
	}
	return nil, nil
}

func init() {
	resource.RegisterComponent(arm.API, Model, resource.Registration[arm.Arm, *Config]{
		Constructor: NewDofBot,
	})
}

// Dofbot implements a yahboom dofbot arm.
// It would be nice to reconfigure atomically but this just rebuilds right now until
// someone implements it.
type Dofbot struct {
	resource.Named
	resource.AlwaysRebuild
	handle  board.I2CHandle
	model   referenceframe.Model
	mu      sync.Mutex
	muMove  sync.Mutex
	logger  golog.Logger
	opMgr   *operation.SingleOperationManager
	stopped bool
}

// NewDofBot is a constructor to create a new dofbot arm.
func NewDofBot(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (arm.Arm, error) {
	var err error

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	a := Dofbot{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
		opMgr:  operation.NewSingleOperationManager(),
	}

	b, err := board.FromDependencies(deps, newConf.Board)
	if err != nil {
		return nil, err
	}
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, fmt.Errorf("board %s is not local", newConf.Board)
	}
	i2c, ok := localB.I2CByName(newConf.I2C)
	if !ok {
		return nil, fmt.Errorf("no i2c for yahboom-dofbot arm %s", conf.Name)
	}
	a.handle, err = i2c.OpenHandle(0x15)
	if err != nil {
		return nil, err
	}

	a.model, err = MakeModelFrame(conf.Name)
	if err != nil {
		return nil, err
	}

	// sanity check if init succeeded
	var pos *componentpb.JointPositions
	pos, err = a.JointPositions(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("error reading joint positions during init: %w", err)
	}
	logger.Debug("Current joint positions: %v", pos)

	return &a, nil
}

// EndPosition returns the current position of the arm.
func (a *Dofbot) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	joints, err := a.JointPositions(ctx, extra)
	if err != nil {
		return nil, fmt.Errorf("error getting joint positions: %w", err)
	}
	return motionplan.ComputeOOBPosition(a.model, joints)
}

// MoveToPosition moves the arm to the given absolute position.
func (a *Dofbot) MoveToPosition(ctx context.Context, pos spatialmath.Pose, extra map[string]interface{}) error {
	ctx, done := a.opMgr.New(ctx)
	defer done()
	return arm.Move(ctx, a.logger, a, pos)
}

// MoveToJointPositions moves the arm's joints to the given positions.
func (a *Dofbot) MoveToJointPositions(ctx context.Context, pos *componentpb.JointPositions, extra map[string]interface{}) error {
	// check that joint positions are not out of bounds
	if err := arm.CheckDesiredJointPositions(ctx, a, pos); err != nil {
		return err
	}

	ctx, done := a.opMgr.New(ctx)
	defer done()

	a.muMove.Lock()
	defer a.muMove.Unlock()
	if a.stopped {
		err := a.turnOnTorque(ctx)
		a.logger.Warnf("error turning on torque %s: ", err)
	}
	if len(pos.Values) > 5 {
		return fmt.Errorf("yahboom wrong number of degrees got %d, need at most 5", len(pos.Values))
	}

	for j := 0; j < 100; j++ {
		success, err := func() (bool, error) {
			a.mu.Lock()
			defer a.mu.Unlock()

			current, err := a.getJointPositionsInLock(ctx)
			if err != nil {
				return false, err
			}

			movedAny := false

			for i, d := range pos.Values {
				delta := math.Abs(current.Values[i] - d)

				if delta < .5 {
					continue
				}

				if j > 5 && delta < 2 {
					// good enough
					continue
				}

				movedAny = true

				err := a.moveJointInLock(ctx, i+1, d)
				if err != nil {
					return false, fmt.Errorf("error moving joint %d: %w", i+1, err)
				}
				sleepFor := time.Duration(4+int(delta)) * time.Millisecond

				time.Sleep(sleepFor)
			}

			return !movedAny, nil
		}()
		if err != nil {
			return err
		}

		if success {
			return nil
		}
	}

	return errors.New("dofbot MoveToJointPositions timed out")
}

func (a *Dofbot) moveJointInLock(ctx context.Context, joint int, degrees float64) error {
	pos := joints[joint-1].toHw(degrees)

	buf := make([]byte, 5)
	buf[0] = byte(0x10 + joint)
	buf[1] = byte((pos >> 8) & 0xFF)
	buf[2] = byte(pos & 0xFF)

	// time
	// TODO(erh): make configurable?
	buf[3] = 0
	buf[4] = 0xFF

	return a.handle.Write(ctx, buf)
}

// JointPositions returns the current joint positions of the arm.
func (a *Dofbot) JointPositions(ctx context.Context, extra map[string]interface{}) (*componentpb.JointPositions, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.getJointPositionsInLock(ctx)
}

func (a *Dofbot) getJointPositionsInLock(ctx context.Context) (*componentpb.JointPositions, error) {
	pos := componentpb.JointPositions{}
	for i := 1; i <= 5; i++ {
		x, err := a.readJointInLock(ctx, i)
		if err != nil {
			return nil, err
		}
		pos.Values = append(pos.Values, x)
	}

	return &pos, nil
}

func (a *Dofbot) readJointInLock(ctx context.Context, joint int) (float64, error) {
	reg := byte(0x30 + joint)
	err := a.handle.WriteByteData(ctx, reg, 0)
	if err != nil {
		return 0, fmt.Errorf("error requesting joint %v from register %v: %w", joint, reg, err)
	}

	time.Sleep(3 * time.Millisecond)

	rd, err := a.handle.ReadBlockData(ctx, reg, 2)
	if err != nil {
		return 0, fmt.Errorf("error reading joint %v from register %v: %w", joint, reg, err)
	}

	time.Sleep(3 * time.Millisecond)

	res := binary.BigEndian.Uint16(rd)
	return joints[joint-1].toValues(int(res)), nil
}

// Stop is unimplemented for the dofbot.
func (a *Dofbot) Stop(ctx context.Context, extra map[string]interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopped = true
	return a.turnOffTorque(ctx)
}

func (a *Dofbot) turnOffTorque(ctx context.Context) error {
	buf := make([]byte, 2)

	buf[0] = byte(0x1A)
	buf[1] = byte(0x00)
	return a.handle.Write(ctx, buf)
}

func (a *Dofbot) turnOnTorque(ctx context.Context) error {
	buf := make([]byte, 2)

	buf[0] = byte(0x1A)
	buf[1] = byte(0x01)

	return a.handle.Write(ctx, buf)
}

// GripperStop is unimplemented for the dofbot.
func (a *Dofbot) GripperStop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.Stop(ctx, nil)
}

// IsMoving returns whether the arm is moving.
func (a *Dofbot) IsMoving(ctx context.Context) (bool, error) {
	return a.opMgr.OpRunning(), nil
}

// ModelFrame returns all the information necessary for including the arm in a FrameSystem.
func (a *Dofbot) ModelFrame() referenceframe.Model {
	return a.model
}

// Open opens the gripper.
func (a *Dofbot) Open(ctx context.Context) error {
	ctx, done := a.opMgr.New(ctx)
	defer done()

	a.mu.Lock()
	defer a.mu.Unlock()

	gripperPosition, err := a.readJointInLock(ctx, 6)
	if err != nil {
		return err
	}
	a.logger.Debug("In Open. Starting gripper position: ", gripperPosition)

	return a.moveJointInLock(ctx, 6, 100)
}

const (
	grabAngle   = 240.0
	minMovement = 5.0
)

// Grab makes the gripper grab.
// Approach: Move to close, poll until gripper reaches the closed state
// (position > grabAngle) or the position changes little (< minMovement)
// between iterations.
func (a *Dofbot) Grab(ctx context.Context) (bool, error) {
	ctx, done := a.opMgr.New(ctx)
	defer done()

	a.mu.Lock()
	defer a.mu.Unlock()

	startingGripperPos, err := a.readJointInLock(ctx, 6)
	if err != nil {
		return false, err
	}
	a.logger.Debug("In Grab. Starting gripper position: ", startingGripperPos)

	err = a.moveJointInLock(ctx, 6, grabAngle)
	if err != nil {
		return false, err
	}

	// wait a moment to get moving
	if !utils.SelectContextOrWait(ctx, 200*time.Millisecond) {
		return false, ctx.Err()
	}

	// wait till we stop moving
	last := -1.0

	for {
		current, err := a.readJointInLock(ctx, 6)
		if err != nil {
			return false, err
		}

		if math.Abs(last-current) < minMovement || current > grabAngle {
			last = current // last is used after the loop
			break
		}
		last = current

		if !utils.SelectContextOrWait(ctx, 20*time.Millisecond) {
			return false, ctx.Err()
		}
	}

	gripperPositionEnd, err := a.readJointInLock(ctx, 6)
	if err != nil {
		return false, err
	}
	a.logger.Debug("In Grab. Ending gripper position: ", gripperPositionEnd)

	return last < grabAngle, a.moveJointInLock(ctx, 6, last+10) // squeeze a tiny bit
}

// CurrentInputs returns the current inputs of the arm.
func (a *Dofbot) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := a.JointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	return a.model.InputFromProtobuf(res), nil
}

// GoToInputs moves the arm to the specified goal inputs.
func (a *Dofbot) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	// check that joint positions are not out of bounds
	positionDegs := a.model.ProtobufFromInput(goal)
	if err := arm.CheckDesiredJointPositions(ctx, a, positionDegs); err != nil {
		return err
	}
	return a.MoveToJointPositions(ctx, a.model.ProtobufFromInput(goal), nil)
}

// Close closes the arm.
func (a *Dofbot) Close(ctx context.Context) error {
	return a.handle.Close()
}

// Geometries returns the list of geometries associated with the resource, in any order. The poses of the geometries reflect their
// current location relative to the frame of the resource.
func (a *Dofbot) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	inputs, err := a.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	gif, err := a.model.Geometries(inputs)
	if err != nil {
		return nil, err
	}
	return gif.Geometries(), nil
}
