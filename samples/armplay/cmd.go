// Package main allows one to play around with a robotic arm.
package main

import (
	"context"
	"flag"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/core/action"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/robot"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/utils"
	"go.viam.com/core/web"

	_ "go.viam.com/core/board/detector"         // load boards
	_ "go.viam.com/core/robots/eva"             // load arm
	_ "go.viam.com/core/robots/universalrobots" // load arm
	_ "go.viam.com/core/robots/varm"            // load arm
	_ "go.viam.com/core/robots/vx300s"          // load arm

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

	arm := r.ArmByName(r.ArmNames()[0])

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

	arm := r.ArmByName(r.ArmNames()[0])

	for i := 0; i < 1000; i++ {
		logger.Debugf("upAndDown loop %d", i)
		pos, err := arm.CurrentPosition(ctx)
		if err != nil {
			return err
		}

		pos.Z += 100
		err = arm.MoveToPosition(ctx, pos)
		if err != nil {
			return err
		}

		pos.Z -= 100
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

	arm := r.ArmByName(r.ArmNames()[0])

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
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	flag.Parse()

	cfg, err := config.Read(flag.Arg(0))
	if err != nil {
		return err
	}

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close()

	return web.RunWeb(ctx, myRobot, web.NewOptions(), logger)
}
