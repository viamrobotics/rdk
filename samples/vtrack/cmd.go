package main

import (
	"context"
	"flag"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/vision/segmentation"
)

var logger = golog.NewDevelopmentLogger("vtrack")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	flag.Parse()

	cfg, err := config.Read(ctx, flag.Arg(0), logger)
	if err != nil {
		return err
	}

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close(ctx)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fc, err := camera.FromRobot(myRobot, "front-composed")
	if err != nil {
		return err
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
		config := &segmentation.RadiusClusteringConfig{50000, 500, 10, 50}
		_, err = segmentation.RadiusClusteringOnPointCloud(ctx, pc, config)
		if err != nil {
			return err
		}
		logger.Debugw("NewObjectSegmentation", "took", time.Since(startInner).String())
		logger.Debugw("frame", "took", time.Since(start).String())
	}
}
