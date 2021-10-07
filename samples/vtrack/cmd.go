package main

import (
	"context"
	"errors"
	"flag"
	"time"

	"go.viam.com/utils"

	"go.viam.com/core/config"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/vision/segmentation"

	_ "go.viam.com/core/rimage/imagesource"

	"github.com/edaniels/golog"
)

var logger = golog.NewDevelopmentLogger("vtrack")

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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fc, ok := myRobot.CameraByName("front-composed")
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
