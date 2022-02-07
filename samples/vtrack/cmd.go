package main

import (
	"context"
	"errors"
	"flag"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc/client"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/vision/segmentation"
)

var logger = golog.NewDevelopmentLogger("vtrack")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	flag.Parse()

	cfg, err := config.Read(ctx, flag.Arg(0))
	if err != nil {
		return err
	}

	myRobot, err := robotimpl.New(ctx, cfg, logger, client.WithDialOptions(rpc.WithInsecure()))
	if err != nil {
		return err
	}
	defer myRobot.Close(ctx)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fc, ok := camera.FromRobot(myRobot, "front-composed")
	if !ok {
		return errors.New("no front-composed camera")
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		start := time.Now()
		pc, err := fc.NextPointCloud(ctx)
		if err != nil {
			return err
		}
		logger.Debugw("NextPointCloud", "took", time.Since(start).String())
		startInner := time.Now()
		config := segmentation.ObjectConfig{50000, 500, 10}
		_, err = segmentation.NewObjectSegmentation(ctx, pc, config)
		if err != nil {
			return err
		}
		logger.Debugw("NewObjectSegmentation", "took", time.Since(startInner).String())
		logger.Debugw("frame", "took", time.Since(start).String())
	}
}
