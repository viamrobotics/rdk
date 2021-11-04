// Package main allows one to play around with a robotic arm.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/action"
	"go.viam.com/core/kinematics"
	"go.viam.com/core/motionplan"
	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/robot"
	u "go.viam.com/core/utils"
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

	action.RegisterAction("plan1", func(ctx context.Context, r robot.Robot) {
		err := plan1(ctx, r)
		if err != nil {
			logger.Errorf("error plan: %s", err)
		}
	})

	action.RegisterAction("plan2", func(ctx context.Context, r robot.Robot) {
		err := plan2(ctx, r)
		if err != nil {
			logger.Errorf("error plan: %s", err)
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
		arm.MoveToPosition(ctx, &pb.ArmPosition{X: -600, Z: 480}),
		arm.MoveToPosition(ctx, &pb.ArmPosition{X: -200, Z: 480}),
		arm.MoveToPosition(ctx, &pb.ArmPosition{X: -200, Z: 300}),
		arm.MoveToPosition(ctx, &pb.ArmPosition{X: -600, Z: 300}),
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

	arm.MoveToPosition(ctx, &pb.ArmPosition{X: 300, Z: 250})

	for i := 0; i < 5; i++ {
		logger.Debugf("upAndDown loop %d", i)
		pos, err := arm.CurrentPosition(ctx)
		if err != nil {
			return err
		}

		pos.Z += 250
		err = arm.MoveToPosition(ctx, pos)
		if err != nil {
			return err
		}

		pos.Z -= 250
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

func plan1(ctx context.Context, r robot.Robot) error {
	if len(r.ArmNames()) != 1 {
		return errors.New("need 1 arm name")
	}

	arm, ok := r.ArmByName(r.ArmNames()[0])
	if !ok {
		return fmt.Errorf("failed to find arm %q", r.ArmNames()[0])
	}

	m, err := kinematics.ParseJSONFile(u.ResolveFile("robots/xarm/xArm7_kinematics.json"))
	if err != nil {
		return err
	}
	mp, err := motionplan.NewLinearMotionPlanner(m, logger, 8)
	if err != nil {
		return err
	}

	// Test ability to arrive at another position
	pos := &pb.ArmPosition{
		X:  100,
		Y:  200,
		Z:  500,
		OX: -1,
	}

	start, err := arm.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}
	solutions, err := mp.Plan(context.Background(), pos, frame.JointPosToInputs(start))
	if err != nil {
		return err
	}

	for _, solution := range solutions {
		err := arm.MoveToJointPositions(ctx, frame.InputsToJointPos(solution))
		if err != nil {
			return err
		}
	}
	
	
	// Test ability to arrive at another position
	pos = &pb.ArmPosition{
		X:  -400,
		Y:  0,
		Z:  200,
		OX: -1,
	}

	start, err = arm.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}
	solutions, err = mp.Plan(context.Background(), pos, frame.JointPosToInputs(start))
	if err != nil {
		return err
	}

	for _, solution := range solutions {
		err := arm.MoveToJointPositions(ctx, frame.InputsToJointPos(solution))
		if err != nil {
			return err
		}
	}

	return nil
}

func plan2(ctx context.Context, r robot.Robot) error {
	if len(r.ArmNames()) != 1 {
		return errors.New("need 1 arm name")
	}

	arm, ok := r.ArmByName(r.ArmNames()[0])
	if !ok {
		return fmt.Errorf("failed to find arm %q", r.ArmNames()[0])
	}

	m, err := kinematics.ParseJSONFile(u.ResolveFile("robots/xarm/xArm7_kinematics.json"))
	if err != nil {
		return err
	}
	mp, err := motionplan.NewLinearMotionPlanner(m, logger, 8)
	if err != nil {
		return err
	}

	// Test ability to arrive at another position
	pos := &pb.ArmPosition{
		X:  250,
		Y:  300,
		Z:  200,
		OY: 1,
	}

	start, err := arm.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}
	solutions, err := mp.Plan(context.Background(), pos, frame.JointPosToInputs(start))
	if err != nil {
		return err
	}

	for _, solution := range solutions {
		err := arm.MoveToJointPositions(ctx, frame.InputsToJointPos(solution))
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	utils.ContextualMain(webserver.RunServer, logger)
}
