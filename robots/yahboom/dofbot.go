package yahboom

import (
	"context"
	_ "embed" // for embedding model file
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/core/board"
	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/utils"

	gutils "go.viam.com/utils"

	"github.com/edaniels/golog"
)

//go:embed dofbot.json
var modeljson []byte

func dofbotModel() (*kinematics.Model, error) {
	return kinematics.ParseJSON(modeljson, "yahboom-dofbot")
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

func (jc jointConfig) toDegrees(n int) float64 {
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

func init() {
	registry.RegisterComponent(arm.Subtype, "yahboom-dofbot", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return newDofBot(ctx, r, config, logger)
		},
	})

	registry.RegisterGripper("yahboom-dofbot", registry.Gripper{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gripper.Gripper, error) {
			armName := config.Attributes.String("arm")
			if armName == "" {
				return nil, errors.New("yahboom-dofbot gripper needs an arm")
			}
			myArm, ok := r.ArmByName(armName)
			if !ok {
				return nil, errors.New("yahboom-dofbot gripper can't find arm")
			}

			goodArm, ok := utils.UnwrapProxy(myArm).(*dofBot)
			if !ok {
				return nil, fmt.Errorf("yahboom-dofbot gripper got not a dofbot arm, got %T", myArm)
			}

			return goodArm, nil
		},
	})

}

type dofBot struct {
	handle board.I2CHandle
	model  *kinematics.Model
	ik     kinematics.InverseKinematics
	mu     sync.Mutex
	muMove sync.Mutex
}

func createDofBotSolver(logger golog.Logger) (*kinematics.Model, kinematics.InverseKinematics, error) {
	model, err := dofbotModel()
	if err != nil {
		return nil, nil, err
	}
	ik, err := kinematics.CreateCombinedIKSolver(model, logger, 4)
	if err != nil {
		return nil, nil, err
	}
	ik.SetSolveWeights(model.SolveWeights)
	return model, ik, nil
}

func newDofBot(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (arm.Arm, error) {
	var err error

	a := dofBot{}

	b, ok := r.BoardByName(config.Attributes.String("board"))
	if !ok {
		return nil, fmt.Errorf("no board for yahboom-dofbot arm %s", config.Name)
	}

	i2c, ok := b.I2CByName(config.Attributes.String("i2c"))
	if !ok {
		return nil, fmt.Errorf("no i2c for yahboom-dofbot arm %s", config.Name)
	}

	a.handle, err = i2c.OpenHandle(0x15)
	if err != nil {
		return nil, err
	}

	a.model, a.ik, err = createDofBotSolver(logger)
	if err != nil {
		return nil, err
	}

	return &a, nil
}

// CurrentPosition returns the current position of the arm.
func (a *dofBot) CurrentPosition(ctx context.Context) (*pb.Pose, error) {
	joints, err := a.CurrentJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return kinematics.ComputePosition(a.ik.Model(), joints)
}

// MoveToPosition moves the arm to the given absolute position.
func (a *dofBot) MoveToPosition(ctx context.Context, pos *pb.Pose) error {
	joints, err := a.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}
	solution, err := a.ik.Solve(ctx, pos, frame.JointPosToInputs(joints))
	if err != nil {
		return err
	}
	return a.MoveToJointPositions(ctx, frame.InputsToJointPos(solution))
}

// MoveToJointPositions moves the arm's joints to the given positions.
func (a *dofBot) MoveToJointPositions(ctx context.Context, pos *pb.JointPositions) error {
	a.muMove.Lock()
	defer a.muMove.Unlock()
	if len(pos.Degrees) > 5 {
		return fmt.Errorf("yahboom wrong number of degrees got %d, need at most 5", len(pos.Degrees))
	}

	for j := 0; j < 100; j++ {
		success, err := func() (bool, error) {
			a.mu.Lock()
			defer a.mu.Unlock()

			current, err := a.currentJointPositionsInLock(ctx)
			if err != nil {
				return false, err
			}

			movedAny := false

			for i, d := range pos.Degrees {
				delta := math.Abs(current.Degrees[i] - d)

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

func (a *dofBot) moveJointInLock(ctx context.Context, joint int, degrees float64) error {
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

// CurrentJointPositions returns the current joint positions of the arm.
func (a *dofBot) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.currentJointPositionsInLock(ctx)
}

func (a *dofBot) currentJointPositionsInLock(ctx context.Context) (*pb.JointPositions, error) {
	pos := pb.JointPositions{}
	for i := 1; i <= 5; i++ {
		x, err := a.readJointInLock(ctx, i)
		if err != nil {
			return nil, err
		}
		pos.Degrees = append(pos.Degrees, x)
	}

	return &pos, nil
}

func (a *dofBot) readJointInLock(ctx context.Context, joint int) (float64, error) {
	reg := byte(0x30 + joint)
	err := a.handle.WriteByteData(ctx, reg, 0)
	if err != nil {
		return 0, err
	}

	time.Sleep(3 * time.Millisecond)

	res, err := a.handle.ReadWordData(ctx, reg)
	if err != nil {
		return 0, err
	}

	time.Sleep(3 * time.Millisecond)

	res = (res >> 8 & 0xff) | (res << 8 & 0xff00)
	return joints[joint-1].toDegrees(int(res)), nil
}

// JointMoveDelta moves a specific joint of the arm by the given amount.
func (a *dofBot) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	return errors.New("yahboom dofBot doesn't support JointMoveDelta")
}

// ModelFrame returns all the information necessary for including the arm in a FrameSystem
func (a *dofBot) ModelFrame() *kinematics.Model {
	return a.model
}

// Open opens the gripper.
func (a *dofBot) Open(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.moveJointInLock(ctx, 6, 100)
}

const grabAngle = 240.0

// Grab makes the gripper grab.
func (a *dofBot) Grab(ctx context.Context) (bool, error) {

	a.mu.Lock()
	defer a.mu.Unlock()
	err := a.moveJointInLock(ctx, 6, 360)
	if err != nil {
		return false, err
	}

	// wait a moment to get moving
	if !gutils.SelectContextOrWait(ctx, 200*time.Millisecond) {
		return false, errors.New("timeout while grabbing")
	}

	// wait till we stop moving
	last := -1.0

	for i := 0; i < 10; i++ {

		if !gutils.SelectContextOrWait(ctx, 50*time.Millisecond) {
			return false, errors.New("timeout while grabbing")
		}

		current, err := a.readJointInLock(ctx, 6)
		if err != nil {
			return false, err
		}

		if math.Abs(last-current) < 5 || current > grabAngle {
			break
		}

		last = current
	}

	return last < grabAngle, a.moveJointInLock(ctx, 6, last+20) // squeeze a tiny bit
}

func (a *dofBot) Close() error {
	return a.handle.Close()
}
