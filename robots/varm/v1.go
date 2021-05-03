package varm

import (
	"context"
	_ "embed" // for embedding model file
	"fmt"
	"math"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/kinematics"
	pb "go.viam.com/robotcore/proto/api/v1"

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
 */
const (
	TestingForce = .5
)

//go:embed v1.json
var v1modeljson []byte

func init() {
	api.RegisterArm("varm1", func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (api.Arm, error) {
		b := r.BoardByName("local")
		if b == nil {
			return nil, fmt.Errorf("viam arm requires a board called local")
		}
		raw, err := NewArmV1(ctx, b, logger)
		if err != nil {
			return nil, err
		}

		return kinematics.NewArm(raw, v1modeljson, 4, logger)
	})
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
		return fmt.Errorf("difference between posMin and posMax is not enough %#v", j)
	}
	if j.degMax-j.degMin < 30 {
		return fmt.Errorf("difference between degMin and degMin is not enough %#v", j)
	}

	return nil
}

func getMotor(ctx context.Context, theBoard board.Board, name string) (board.Motor, error) {
	m := theBoard.Motor(name)
	if m == nil {
		return nil, fmt.Errorf("no motor with name: %s", name)
	}

	pok, err := m.PositionSupported(ctx)
	if err != nil {
		return nil, err
	}

	if !pok {
		return nil, fmt.Errorf("motor %s doesn't support position", name)
	}

	return m, nil
}

func motorOffError(ctx context.Context, m board.Motor, other error) error {
	return multierr.Combine(other, m.Off(ctx))
}

func testJointLimit(ctx context.Context, m board.Motor, dir pb.DirectionRelative, logger golog.Logger) (float64, error) {
	logger.Debugf("testJointLimit dir: %v", dir)
	err := m.Go(ctx, dir, TestingForce)
	if err != nil {
		return 0.0, err
	}

	time.Sleep(500 * time.Millisecond)

	positions := []float64{}

	bigger := false

	for i := 0; i < 500; i++ {
		time.Sleep(25 * time.Millisecond)
		pos, err := m.Position(ctx)
		if err != nil {
			return 0.0, motorOffError(ctx, m, err)
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
				err := m.Go(ctx, dir, TestingForce*2)
				if err != nil {
					return 0.0, motorOffError(ctx, m, err)
				}

			}
		}

	}

	return 0.0, motorOffError(ctx, m, fmt.Errorf("testing joint limit timed out"))
}

func NewArmV1(ctx context.Context, theBoard board.Board, logger golog.Logger) (api.Arm, error) {
	var err error
	arm := &ArmV1{}

	arm.j0.degMin = -135.0
	arm.j0.degMax = 75.0

	arm.j1.degMin = -142.0
	arm.j1.degMax = 0.0

	arm.j0Motor, err = getMotor(ctx, theBoard, "m-j0")
	if err != nil {
		return nil, err
	}

	arm.j1Motor, err = getMotor(ctx, theBoard, "m-j1")
	if err != nil {
		return nil, err
	}

	arm.j0.posMax, err = testJointLimit(ctx, arm.j0Motor, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, logger)
	if err != nil {
		return nil, err
	}
	/*
		arm.j1.posMax, err = testJointLimit(ctx, arm.j1Motor, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, logger)
		if err != nil {
			return nil, err
		}
	*/
	arm.j0.posMin, err = testJointLimit(ctx, arm.j0Motor, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, logger)
	if err != nil {
		return nil, err
	}

	arm.j1.posMin, err = testJointLimit(ctx, arm.j1Motor, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, logger)
	if err != nil {
		return nil, err
	}

	arm.j1.posMax = arm.j1.posMin + .72 // TODO(erh): this is super gross

	logger.Debugf("%#v", arm)

	return arm, multierr.Combine(arm.j0.validate(), arm.j1.validate())
}

type ArmV1 struct {
	j0Motor, j1Motor board.Motor

	j0, j1 joint
}

func (a *ArmV1) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	return nil, fmt.Errorf("no CurrentPosition support")
}

func (a *ArmV1) MoveToPosition(ctx context.Context, c *pb.ArmPosition) error {
	return fmt.Errorf("no MoveToPosition support")
}

func (a *ArmV1) moveJointToDegrees(ctx context.Context, m board.Motor, j joint, curDegrees, gotoDegrees float64) error {
	curPos := j.degreesToPosition(curDegrees)
	gotoPos := j.degreesToPosition(gotoDegrees)

	delta := gotoPos - curPos

	if math.Abs(delta) < .001 {
		return nil
	}

	return m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 30.0, delta)
}

func (a *ArmV1) MoveToJointPositions(ctx context.Context, pos *pb.JointPositions) error {
	if len(pos.Degrees) != 2 {
		return fmt.Errorf("need exactly 2 joints")
	}

	cur, err := a.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}

	return multierr.Combine(
		a.moveJointToDegrees(ctx, a.j0Motor, a.j0, cur.Degrees[0], pos.Degrees[0]),
		a.moveJointToDegrees(ctx, a.j1Motor, a.j1, cur.Degrees[1], pos.Degrees[1]),
	)
}

func jointToDegrees(ctx context.Context, m board.Motor, j joint) (float64, error) {
	pos, err := m.Position(ctx)
	if err != nil {
		return 0, err
	}

	return j.positionToDegrees(pos), nil
}

func (a *ArmV1) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	var e1, e2 error
	joints := &pb.JointPositions{Degrees: make([]float64, 2)}
	joints.Degrees[0], e1 = jointToDegrees(ctx, a.j0Motor, a.j0)
	joints.Degrees[1], e2 = jointToDegrees(ctx, a.j1Motor, a.j1)
	return joints, multierr.Combine(e1, e2)
}

func (a *ArmV1) JointMoveDelta(ctx context.Context, joint int, amount float64) error {
	joints, err := a.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}

	if joint >= len(joints.Degrees) {
		return fmt.Errorf("invalid joint number (%d) len: %d", joint, len(joints.Degrees))
	}

	joints.Degrees[joint] += amount

	return a.MoveToJointPositions(ctx, joints)
}

func computeInnerJointAngle(j0, j1 float64) float64 {
	return j0 + j1
}
