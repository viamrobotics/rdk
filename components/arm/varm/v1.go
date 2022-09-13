// Package varm implements versions of the Viam arm.
package varm

import (
	"context"
	// for embedding model file.
	_ "embed"
	"math"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"gonum.org/v1/gonum/stat"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	componentpb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
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
 * ++ on joint 1 should lift.
 */
const (
	TestingForce = .5
	TestingRPM   = 10.0
)

//go:embed v1.json
var v1modeljson []byte

func init() {
	registry.RegisterComponent(arm.Subtype, "varm1", registry.Component{
		RobotConstructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return newArmV1(ctx, r, logger)
		},
	})
}

type joint struct {
	posMin, posMax float64
	degMin, degMax float64
}

func (j joint) positionToValues(pos float64) float64 {
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
	m, err := motor.FromRobot(r, name)
	if err != nil {
		return nil, err
	}

	supportedProperties, err := m.GetProperties(ctx, nil)
	if err != nil {
		return nil, err
	}
	pok := supportedProperties[motor.PositionReporting]
	if !pok {
		return nil, motor.NewFeatureUnsupportedError(motor.PositionReporting, name)
	}

	return m, nil
}

func motorOffError(ctx context.Context, m motor.Motor, other error) error {
	return multierr.Combine(other, m.Stop(ctx, nil))
}

func testJointLimit(ctx context.Context, m motor.Motor, dir int64, logger golog.Logger) (float64, error) {
	logger.Debugf("testJointLimit dir: %v", dir)
	err := m.GoFor(ctx, float64(dir)*TestingRPM, 0, nil)
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
		pos, err := m.GetPosition(ctx, nil)
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
					return pos, m.Stop(ctx, nil)
				}
				bigger = true
				positions = []float64{}
				err := m.SetPower(ctx, float64(dir)*TestingForce, nil)
				if err != nil {
					return math.NaN(), motorOffError(ctx, m, err)
				}
			}
		}
	}

	return math.NaN(), motorOffError(ctx, m, errors.New("testing joint limit timed out"))
}

func newArmV1(ctx context.Context, r robot.Robot, logger golog.Logger) (arm.LocalArm, error) {
	var err error
	newArm := &armV1{}
	newArm.robot = r

	newArm.model, err = referenceframe.UnmarshalModelJSON(v1modeljson, "")
	if err != nil {
		return nil, err
	}

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
	generic.Unimplemented
	j0Motor, j1Motor motor.Motor

	j0, j1 joint
	model  referenceframe.Model
	robot  robot.Robot

	opMgr operation.SingleOperationManager
}

// GetEndPosition computes and returns the current cartesian position.
func (a *armV1) GetEndPosition(ctx context.Context, extra map[string]interface{}) (*commonpb.Pose, error) {
	joints, err := a.GetJointPositions(ctx, extra)
	if err != nil {
		return nil, err
	}
	return motionplan.ComputePosition(a.model, joints)
}

// MoveToPosition moves the arm to the specified cartesian position.
func (a *armV1) MoveToPosition(
	ctx context.Context,
	pos *commonpb.Pose,
	worldState *commonpb.WorldState,
	extra map[string]interface{},
) error {
	ctx, done := a.opMgr.New(ctx)
	defer done()
	return arm.Move(ctx, a.robot, a, pos, worldState)
}

func (a *armV1) moveJointToValues(ctx context.Context, m motor.Motor, j joint, curValues, gotoValues float64) error {
	curPos := j.degreesToPosition(curValues)
	gotoPos := j.degreesToPosition(gotoValues)

	delta := gotoPos - curPos

	if math.Abs(delta) < .001 {
		return nil
	}

	return m.GoFor(ctx, 10.0, delta, nil)
}

// MoveToJointPositions TODO.
func (a *armV1) MoveToJointPositions(ctx context.Context, pos *componentpb.JointPositions, extra map[string]interface{}) error {
	ctx, done := a.opMgr.New(ctx)
	defer done()

	if len(pos.Values) != 2 {
		return errors.New("need exactly 2 joints")
	}

	cur, err := a.GetJointPositions(ctx, extra)
	if err != nil {
		return err
	}

	err = multierr.Combine(
		a.moveJointToValues(ctx, a.j0Motor, a.j0, cur.Values[0], pos.Values[0]),
		a.moveJointToValues(ctx, a.j1Motor, a.j1, cur.Values[1], pos.Values[1]),
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

// IsOn TODO.
func (a *armV1) IsOn(ctx context.Context) (bool, error) {
	on0, err0 := a.j0Motor.IsPowered(ctx, nil)
	on1, err1 := a.j0Motor.IsPowered(ctx, nil)

	return on0 || on1, multierr.Combine(err0, err1)
}

func jointToValues(ctx context.Context, m motor.Motor, j joint) (float64, error) {
	pos, err := m.GetPosition(ctx, nil)
	if err != nil {
		return 0, err
	}

	return j.positionToValues(pos), nil
}

// GetJointPositions TODO.
func (a *armV1) GetJointPositions(ctx context.Context, extra map[string]interface{}) (*componentpb.JointPositions, error) {
	var e1, e2 error
	joints := &componentpb.JointPositions{Values: make([]float64, 2)}
	joints.Values[0], e1 = jointToValues(ctx, a.j0Motor, a.j0)
	joints.Values[1], e2 = jointToValues(ctx, a.j1Motor, a.j1)

	joints.Values[1] = (joints.Values[1] - joints.Values[0])
	return joints, multierr.Combine(e1, e2)
}

func (a *armV1) Stop(ctx context.Context, extra map[string]interface{}) error {
	// RSDK-374: Implement Stop
	return arm.ErrStopUnimplemented
}

func (a *armV1) IsMoving(ctx context.Context) (bool, error) {
	return a.opMgr.OpRunning(), nil
}

func (a *armV1) ModelFrame() referenceframe.Model {
	return a.model
}

func (a *armV1) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := a.GetJointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	return a.model.InputFromProtobuf(res), nil
}

func (a *armV1) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return a.MoveToJointPositions(ctx, a.model.ProtobufFromInput(goal), nil)
}

func computeInnerJointAngle(j0, j1 float64) float64 {
	return j0 + j1
}
