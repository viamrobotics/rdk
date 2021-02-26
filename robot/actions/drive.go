package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/vision"
)

func randomWalkIncrement(theRobot *robot.Robot) error {

	if len(theRobot.Bases) == 0 {
		return fmt.Errorf("no bases, can't drive")
	}

	if len(theRobot.Cameras) == 0 {
		return fmt.Errorf("no cameras, can't drive")
	}

	img, dm, err := theRobot.Cameras[0].NextImageDepthPair(context.TODO())
	if err != nil {
		return err
	}

	pc := vision.PointCloud{dm, vision.NewImage(img)}
	pc, err = pc.CropToDepthData()

	if err != nil || pc.Depth.Width() < 10 || pc.Depth.Height() < 10 {
		golog.Global.Debugf("error getting depth info: %s, backing up", err)
		return theRobot.Bases[0].MoveStraight(-200, 60, true)
	}

	_, points := roverWalk(&pc, false)
	if points < 200 {
		golog.Global.Debugf("safe to move forward")
		return theRobot.Bases[0].MoveStraight(200, 50, true)
	}

	fn := fmt.Sprintf("data/rover-cannot-walk-%d.both.gz", time.Now().Unix())
	err = pc.WriteTo(fn)
	if err != nil {
		return err
	}

	golog.Global.Debugf("not safe, let's spin, wrote debug img to: %s", fn)
	return theRobot.Bases[0].Spin(-15, 60, true)
}

func RandomWalk(theRobot *robot.Robot, numSeconds int64) {
	start := time.Now().Unix()

	defer func() { golog.Global.Debugf("RandomWalk done") }()

	for {
		now := time.Now().Unix()
		if now-start > numSeconds {
			break
		}

		err := randomWalkIncrement(theRobot)

		if err != nil {
			golog.Global.Debugf("error doing random walk, trying again: %s", err)
			time.Sleep(2000 * time.Millisecond)
			continue
		}
	}
}
