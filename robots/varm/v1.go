// Package varm implements versions of the Viam arm.
package varm

import (
	"context"
	_ "embed" // for embedding model file
	"math"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/kinematics"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"gonum.org/v1/gonum/stat"
)

/**
 * ---------
 *      |  |
 *      |  |
 *      |  | 450mm
 *      |  |
 *      |  |
 *      150
 * that position is 0 degrees for joint 0, and -90 degrees for joint 1 and -90 for the inner joint
 * ++ on joint 0 should lift
 * ++ on joint 1 should lift
 */
const (
	TestingForce = .5
	TestingRPM   = 10.0
)

//go:embed v1.json
var v1modeljson []byte

func init() {
	registry.RegisterComponent(arm.Subtype, "varm1", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			model, err := kinematics.ParseJSON(v1modeljson, "")
			if err != nil {
				return nil, err
			}
			ik, err := kinematics.CreateCombinedIKSolver(model, logger, 4)
			if err != nil {
				return nil, err
			}
			raw, err := NewArmV1(ctx, r, logger, ik)
			if err != nil {
				return nil, err
			}

			return raw, nil
		}})
}

type joint struct {
	posMin, posMax float64
	degMin, degMax float64
}

func (j joint) positionToDegrees(pos float64) float64 {
	pos = (pos - j.posMin) / (j.posMax - j.posMin) // now is 0 -> 1 in percent
	pos *= (j.degMax - j.degMin)
	pos += j.degMin
	return pos
}

func (j joint) degreesToPosition(deg float64) float64 {
	deg = (deg - j.degMin) / (j.degMax - j.degMin) // now is 0 -> 1 in percent
	deg *= (j.posMax - j.posMin)
	deg += j.posMin
	return deg
}

func (j joint) validate() error {
	if j.posMax-j.posMin < .2 {
		return errors.Errorf("difference between posMin and posMax is not enough %#v", j)
	}
	if j.degMax-j.degMin < 30 {
		return errors.Errorf("difference between degMin and degMin is not enough %#v", j)
	}

	return nil
}

func getMotor(ctx context.Context, r robot.Robot, name string) (motor.Motor, error) {
	m, ok := r.MotorByName(name)
	if !ok {
		return nil, errors.Errorf("no motor with name: %s", name)
	}

	pok, err := m.PositionSupported(ctx)
	if err != nil {
		return nil, err
	}

	if !pok {
		return nil, errors.Errorf("motor %s doesn't support position", name)
	}

	return m, nil
}

func motorOffError(ctx context.Context, m motor.Motor, other error) error {
	return multierr.Combine(other, m.Off(ctx))
}

func testJointLimit(ctx context.Context, m motor.Motor, dir pb.DirectionRelative, logger golog.Logger) (float64, error) {
	logger.Debugf("testJointLimit dir: %v", dir)
	err := m.GoFor(ctx, dir, TestingRPM, 0)
	if err != nil {
		return 0.0, err
	}

	if !utils.SelectContextOrWait(ctx, 500*time.Millisecond) {
		return math.NaN(), ctx.Err()
	}

	positions := []float64{}

	bigger := false

	for i := 0; i < 500; i++ {
		if !utils.SelectContextOrWait(ctx, 25*time.Millisecond) {
			return math.NaN(), ctx.Err()
		}
		pos, err := m.Position(ctx)
		if err != nil {
			return math.NaN(), motorOffError(ctx, m, err)
		}

		positions = append(positions, pos)

		if len(positions) > 5 {
			positions = positions[len(positions)-5:]
			avg, stdDev := stat.MeanStdDev(positions, nil)
			logger.Debugf("pos: %v avg: %v stdDev: %v", pos, avg, stdDev)

			if stdDev < .0001 {
				if bigger {
					return pos, m.Off(ctx)
				}
				bigger = true
				positions = []float64{}
				err := m.Go(ctx, dir, TestingForce)
				if err != nil {
					return math.NaN(), motorOffError(ctx, m, err)
				}

			}
		}

	}

	return math.NaN(), motorOffError(ctx, m, errors.New("testing joint limit timed out"))
}

// NewArmV1 TODO
func NewArmV1(ctx context.Context, r robot.Robot, logger golog.Logger, ik kinematics.InverseKinematics) (arm.Arm, error) {
	var err error
	newArm := &ArmV1{}

	newArm.j0.degMin = -135.0
	newArm.j0.degMax = 75.0

	newArm.j1.degMin = -142.0
	newArm.j1.degMax = 0.0

	newArm.j0Motor, err = getMotor(ctx, r, "m-j0")
	if err != nil {
		return nil, err
	}

	newArm.j1Motor, err = getMotor(ctx, r, "m-j1")
	if err != nil {
		return nil, err
	}

	newArm.j0.posMax, err = testJointLimit(ctx, newArm.j0Motor, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, logger)
	if err != nil {
		return nil, err
	}
	/*
		newArm.j1.posMax, err = testJointLimit(ctx, newArm.j1Motor, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, logger)
		if err != nil {
			return nil, err
		}
	*/
	newArm.j0.posMin, err = testJointLimit(ctx, newArm.j0Motor, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, logger)
	if err != nil {
		return nil, err
	}

	newArm.j1.posMin, err = testJointLimit(ctx, newArm.j1Motor, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, logger)
	if err != nil {
		return nil, err
	}

	newArm.j1.posMax = newArm.j1.posMin + 3.417 // TODO(erh): this is super gross

	logger.Debugf("%#v", newArm)

	return newArm, multierr.Combine(newArm.j0.validate(), newArm.j1.validate())
}

// ArmV1 TODO
type ArmV1 struct {
	j0Motor, j1Motor motor.Motor

	j0, j1 joint
	ik     kinematics.InverseKinematics
}

// CurrentPosition computes and returns the current cartesian position.
func (a *ArmV1) CurrentPosition(ctx context.Context) (*pb.Pose, error) {
	joints, err := a.CurrentJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return kinematics.ComputePosition(a.ik.Model(), joints)
}

// MoveToPosition moves the arm to the specified cartesian position.
func (a *ArmV1) MoveToPosition(ctx context.Context, pos *pb.Pose) error {
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

func (a *ArmV1) moveJointToDegrees(ctx context.Context, m motor.Motor, j joint, curDegrees, gotoDegrees float64) error {
	curPos := j.degreesToPosition(curDegrees)
	gotoPos := j.degreesToPosition(gotoDegrees)

	delta := gotoPos - curPos

	if math.Abs(delta) < .001 {
		return nil
	}

	return m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 10.0, delta)

}

// MoveToJointPositions TODO
func (a *ArmV1) MoveToJointPositions(ctx context.Context, pos *pb.JointPositions) error {
	if len(pos.Degrees) != 2 {
		return errors.New("need exactly 2 joints")
	}

	cur, err := a.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}

	err = multierr.Combine(
		a.moveJointToDegrees(ctx, a.j0Motor, a.j0, cur.Degrees[0], pos.Degrees[0]),
		a.moveJointToDegrees(ctx, a.j1Motor, a.j1, cur.Degrees[1], pos.Degrees[1]),
	)
	if err != nil {
		return err
	}

	for i := 0; i < 100; i++ {
		if !utils.SelectContextOrWait(ctx, 25*time.Millisecond) {
			return ctx.Err()
		}

		on, err := a.IsOn(ctx)
		if err != nil {
			return err
		}

		if !on {
			return nil
		}
	}

	return errors.Errorf("arm moved timed out, wanted: %v", pos)
}

// IsOn TODO
func (a *ArmV1) IsOn(ctx context.Context) (bool, error) {
	on0, err0 := a.j0Motor.IsOn(ctx)
	on1, err1 := a.j0Motor.IsOn(ctx)

	return on0 || on1, multierr.Combine(err0, err1)
}

func jointToDegrees(ctx context.Context, m motor.Motor, j joint) (float64, error) {
	pos, err := m.Position(ctx)
	if err != nil {
		return 0, err
	}

	return j.positionToDegrees(pos), nil
}

// CurrentJointPositions TODO
func (a *ArmV1) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	var e1, e2 error
	joints := &pb.JointPositions{Degrees: make([]float64, 2)}
	joints.Degrees[0], e1 = jointToDegrees(ctx, a.j0Motor, a.j0)
	joints.Degrees[1], e2 = jointToDegrees(ctx, a.j1Motor, a.j1)

	joints.Degrees[1] = (joints.Degrees[1] - joints.Degrees[0])
	return joints, multierr.Combine(e1, e2)
}

// JointMoveDelta TODO
func (a *ArmV1) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	joints, err := a.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}

	if joint >= len(joints.Degrees) {
		return errors.Errorf("invalid joint number (%d) len: %d", joint, len(joints.Degrees))
	}

	joints.Degrees[joint] += amountDegs

	return a.MoveToJointPositions(ctx, joints)
}

func computeInnerJointAngle(j0, j1 float64) float64 {
	return j0 + j1
}
