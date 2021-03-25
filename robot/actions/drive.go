package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/edaniels/gostream"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"
)

func init() {
	RegisterAction("RandomWalk", RandomWalk)
}

func setup(theRobot api.Robot) (api.Base, gostream.ImageSource, error) {
	baseNames := theRobot.BaseNames()
	if len(baseNames) == 0 {
		return nil, nil, fmt.Errorf("no bases, can't drive")
	}

	cameraNames := theRobot.CameraNames()
	if len(cameraNames) == 0 {
		return nil, nil, fmt.Errorf("no cameras, can't drive")
	}

	return theRobot.BaseByName(baseNames[0]), theRobot.CameraByName(cameraNames[0]), nil
}

func randomWalkIncrement(ctx context.Context, theRobot api.Robot) error {

	base, camera, err := setup(theRobot)
	if err != nil {
		return err
	}

	raw, release, err := camera.Next(ctx)
	if err != nil {
		return err
	}
	defer release()

	pc := rimage.ConvertToImageWithDepth(raw)
	if pc.Depth == nil {
		return fmt.Errorf("no depth data")
	}
	pc, err = pc.CropToDepthData()

	if err != nil || pc.Depth.Width() < 10 || pc.Depth.Height() < 10 {
		theRobot.Logger().Debugf("error getting depth info: %s, backing up", err)
		return base.MoveStraight(ctx, -200, 60, true)
	}

	_, points := roverWalk(pc, false, theRobot.Logger())
	if points < 200 {
		theRobot.Logger().Debugf("safe to move forward")
		return base.MoveStraight(ctx, 200, 50, true)
	}

	fn := fmt.Sprintf("data/rover-cannot-walk-%d.both.gz", time.Now().Unix())
	err = pc.WriteTo(fn)
	if err != nil {
		return err
	}

	theRobot.Logger().Debugf("not safe, let's spin, wrote debug img to: %s", fn)
	return base.Spin(ctx, -15, 60, true)
}

func RandomWalk(theRobot api.Robot) {
	defer func() { theRobot.Logger().Debugf("RandomWalk done") }()

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*time.Duration(60))
	defer cancelFunc()
	for {
		err := randomWalkIncrement(ctx, theRobot)

		if err != nil {
			theRobot.Logger().Debugf("error doing random walk, trying again: %s", err)
			time.Sleep(2000 * time.Millisecond)
			continue
		}
	}
}
