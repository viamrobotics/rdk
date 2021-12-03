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
	"go.viam.com/core/component/motor"
	"go.viam.com/core/config"
	"go.viam.com/core/kinematics"
	"go.viam.com/core/motionplan"
	commonpb "go.viam.com/core/proto/api/common/v1"
	componentpb "go.viam.com/core/proto/api/component/v1"
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
			raw, err := newArmV1(ctx, r, logger)
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

func testJointLimit(ctx context.Context, m motor.Motor, dir int64, logger golog.Logger) (float64, error) {
	logger.Debugf("testJointLimit dir: %v", dir)
	err := m.GoFor(ctx, float64(dir)*TestingRPM, 0)
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
				err := m.Go(ctx, float64(dir)*TestingForce)
				if err != nil {
					return math.NaN(), motorOffError(ctx, m, err)
				}

			}
		}

	}

	return math.NaN(), motorOffError(ctx, m, errors.New("testing joint limit timed out"))
}

func newArmV1(ctx context.Context, r robot.Robot, logger golog.Logger) (arm.Arm, error) {
	var err error
	newArm := &armV1{}

	newArm.model, err = frame.ParseJSON(v1modeljson, "")
	if err != nil {
		return nil, err
	}
	newArm.mp, err = motionplan.NewCBiRRTMotionPlanner(newArm.model, 4, logger)
	if err != nil {
		return nil, err
	}
	opt := motionplan.NewDefaultPlannerOptions()
	opt.SetMetric(motionplan.NewPositionOnlyMetric())
	newArm.mp.SetOptions(opt)

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

	newArm.j0.posMax, err = testJointLimit(ctx, newArm.j0Motor, 1, logger)
	if err != nil {
		return nil, err
	}

	newArm.j0.posMin, err = testJointLimit(ctx, newArm.j0Motor, -1, logger)
	if err != nil {
		return nil, err
	}

	newArm.j1.posMin, err = testJointLimit(ctx, newArm.j1Motor, -1, logger)
	if err != nil {
		return nil, err
	}

	newArm.j1.posMax = newArm.j1.posMin + 3.417 // TODO(erh): this is super gross

	logger.Debugf("%#v", newArm)

	return newArm, multierr.Combine(newArm.j0.validate(), newArm.j1.validate())
}

type armV1 struct {
	j0Motor, j1Motor motor.Motor

	j0, j1 joint
	mp     motionplan.MotionPlanner
	model  *frame.Model
}

// CurrentPosition computes and returns the current cartesian position.
func (a *armV1) CurrentPosition(ctx context.Context) (*commonpb.Pose, error) {
	joints, err := a.CurrentJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return kinematics.ComputePosition(a.mp.Frame(), joints)
}

// MoveToPosition moves the arm to the specified cartesian position.
func (a *armV1) MoveToPosition(ctx context.Context, pos *commonpb.Pose) error {
	joints, err := a.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}
	solution, err := a.mp.Plan(ctx, pos, frame.JointPosToInputs(joints))
	if err != nil {
		return err
	}
	return arm.GoToWaypoints(ctx, a, solution)
}

func (a *armV1) moveJointToDegrees(ctx context.Context, m motor.Motor, j joint, curDegrees, gotoDegrees float64) error {
	curPos := j.degreesToPosition(curDegrees)
	gotoPos := j.degreesToPosition(gotoDegrees)

	delta := gotoPos - curPos

	if math.Abs(delta) < .001 {
		return nil
	}

	return m.GoFor(ctx, 10.0, delta)

}

// MoveToJointPositions TODO
func (a *armV1) MoveToJointPositions(ctx context.Context, pos *componentpb.ArmJointPositions) error {
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
func (a *armV1) IsOn(ctx context.Context) (bool, error) {
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
func (a *armV1) CurrentJointPositions(ctx context.Context) (*componentpb.ArmJointPositions, error) {
	var e1, e2 error
	joints := &componentpb.ArmJointPositions{Degrees: make([]float64, 2)}
	joints.Degrees[0], e1 = jointToDegrees(ctx, a.j0Motor, a.j0)
	joints.Degrees[1], e2 = jointToDegrees(ctx, a.j1Motor, a.j1)

	joints.Degrees[1] = (joints.Degrees[1] - joints.Degrees[0])
	return joints, multierr.Combine(e1, e2)
}

// JointMoveDelta TODO
func (a *armV1) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
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

func (a *armV1) ModelFrame() *frame.Model {
	return a.model
}

func (a *armV1) CurrentInputs(ctx context.Context) ([]frame.Input, error) {
	res, err := a.CurrentJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return frame.JointPosToInputs(res), nil
}

func (a *armV1) GoToInputs(ctx context.Context, goal []frame.Input) error {
	return a.MoveToJointPositions(ctx, frame.InputsToJointPos(goal))
}

func computeInnerJointAngle(j0, j1 float64) float64 {
	return j0 + j1
}
