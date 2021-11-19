// Package main allows one to play around with a robotic arm.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/action"
	commonpb "go.viam.com/core/proto/api/common/v1"
	"go.viam.com/core/robot"
	"go.viam.com/core/motionplan"
	spatial "go.viam.com/core/spatialmath"
	webserver "go.viam.com/core/web/server"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

var logger = golog.NewDevelopmentLogger("armplay")

func init() {
	action.RegisterAction("play", func(ctx context.Context, r robot.Robot) {
		err := play(ctx, r)
		if err != nil {
			logger.Errorf("error playing: %s", err)
		}
	})

	action.RegisterAction("chrisCirlce", func(ctx context.Context, r robot.Robot) {
		err := chrisCirlce(ctx, r)
		if err != nil {
			logger.Errorf("error: %s", err)
		}
	})

	action.RegisterAction("upAndDown", func(ctx context.Context, r robot.Robot) {
		err := upAndDown(ctx, r)
		if err != nil {
			logger.Errorf("error upAndDown: %s", err)
		}
	})

}

func chrisCirlce(ctx context.Context, r robot.Robot) error {
	if len(r.ArmNames()) != 1 {
		return errors.New("need 1 arm name")
	}

	arm, ok := r.ArmByName(r.ArmNames()[0])
	if !ok {
		return fmt.Errorf("failed to find arm %q", r.ArmNames()[0])
	}

	return multierr.Combine(
		arm.MoveToPosition(ctx, &commonpb.Pose{X: -600, Z: 480}),
		arm.MoveToPosition(ctx, &commonpb.Pose{X: -200, Z: 480}),
		arm.MoveToPosition(ctx, &commonpb.Pose{X: -200, Z: 300}),
		arm.MoveToPosition(ctx, &commonpb.Pose{X: -600, Z: 300}),
	)
}

func upAndDown(ctx context.Context, r robot.Robot) error {
	if len(r.ArmNames()) != 1 {
		return errors.New("need 1 arm name")
	}

	arm, ok := r.ArmByName(r.ArmNames()[0])
	if !ok {
		return fmt.Errorf("failed to find arm %q", r.ArmNames()[0])
	}

	for i := 0; i < 5; i++ {
		logger.Debugf("upAndDown loop %d", i)
		pos, err := arm.CurrentPosition(ctx)
		if err != nil {
			return err
		}

		pos.Y += 550
		err = arm.MoveToPosition(ctx, pos)
		if err != nil {
			return err
		}

		pos.Y -= 550
		err = arm.MoveToPosition(ctx, pos)
		if err != nil {
			return err
		}
	}

	return nil
}

func play(ctx context.Context, r robot.Robot) error {
	if len(r.ArmNames()) != 1 {
		return errors.New("need 1 arm name")
	}

	arm, ok := r.ArmByName(r.ArmNames()[0])
	if !ok {
		return fmt.Errorf("failed to find arm %q", r.ArmNames()[0])
	}

	start, err := arm.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}

	for i := 0; i < 180; i += 10 {
		start.Degrees[0] = float64(i)
		err := arm.MoveToJointPositions(ctx, start)
		if err != nil {
			return err
		}

		if !utils.SelectContextOrWait(ctx, time.Second) {
			return ctx.Err()
		}
	}

	return nil
}

func main() {
	utils.ContextualMain(webserver.RunServer, logger)
}

// DontHitPetersWallConstraint defines some obstacles that nothing should not intersect with
// TODO(pl): put this somewhere else, maybe in an example file or something
func DontHitPetersWallConstraint() motionplan.Constraint {

	f := func(ci *motionplan.ConstraintInput) (bool, float64) {
		checkPt := func(pose spatial.Pose) bool {
			pt := pose.Point()

			// wall in Peter's office
			if pt.Y < -536.8 {
				return false
			}
			if pt.X < -600 {
				return false
			}
			// shelf in Peter's office
			if pt.Z < 5 && pt.Y < 260 && pt.X < 140 {
				return false
			}

			return true
		}
		if ci.StartPos != nil {
			if !checkPt(ci.StartPos) {
				return false, 0
			}
		} else if ci.StartInput != nil {
			pos, err := ci.Frame.Transform(ci.StartInput)
			if err != nil {
				return false, 0
			}
			if !checkPt(pos) {
				return false, 0
			}
		}
		if ci.EndPos != nil {
			if !checkPt(ci.EndPos) {
				return false, 0
			}
		} else if ci.EndInput != nil {
			pos, err := ci.Frame.Transform(ci.EndInput)
			if err != nil {
				return false, 0
			}
			if !checkPt(pos) {
				return false, 0
			}
		}
		return true, 0
	}
	return motionplan.NewFlexibleConstraint(f)
}
