package action

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/edaniels/gostream"

	"go.viam.com/robotcore/artifact"
	"go.viam.com/robotcore/base"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/utils"
)

// init registers the RandomWalk action.
func init() {
	RegisterAction("RandomWalk", RandomWalk)
}

// A RandomWalk instructs the robot to traverse a space randomly for a bounded
// amount of time.
func RandomWalk(ctx context.Context, theRobot robot.Robot) {
	defer func() { theRobot.Logger().Debug("RandomWalk done") }()

	ctx, cancelFunc := context.WithTimeout(ctx, 60*time.Second)
	defer cancelFunc()
	for {
		err := randomWalkIncrement(ctx, theRobot)

		if err != nil {
			theRobot.Logger().Debugf("error doing random walk, trying again: %s", err)
			if !utils.SelectContextOrWait(ctx, 2*time.Second) {
				return
			}
			continue
		}
	}
}

func randomWalkIncrement(ctx context.Context, theRobot robot.Robot) error {

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
		return errors.New("no depth data")
	}
	pc, err = pc.CropToDepthData()

	if err != nil || pc.Depth.Width() < 10 || pc.Depth.Height() < 10 {
		theRobot.Logger().Debugf("error getting depth info: %s, backing up", err)
		_, err := base.MoveStraight(ctx, -200, 60, true)
		return err
	}

	_, points := roverWalk(pc, false, theRobot.Logger())
	if points < 200 {
		theRobot.Logger().Debug("safe to move forward")
		_, err := base.MoveStraight(ctx, 200, 50, true)
		return err
	}

	fn := artifact.MustNewPath(fmt.Sprintf("robot/actions/rover-cannot-walk-%d.both.gz", time.Now().Unix()))
	err = pc.WriteTo(fn)
	if err != nil {
		return err
	}

	theRobot.Logger().Debugf("not safe, let's spin, wrote debug img to: %s", fn)
	_, err = base.Spin(ctx, -15, 60, true)
	return err
}

func setup(theRobot robot.Robot) (base.Base, gostream.ImageSource, error) {
	baseNames := theRobot.BaseNames()
	if len(baseNames) == 0 {
		return nil, nil, errors.New("no bases, can't drive")
	}

	cameraNames := theRobot.CameraNames()
	if len(cameraNames) == 0 {
		return nil, nil, errors.New("no cameras, can't drive")
	}

	return theRobot.BaseByName(baseNames[0]), theRobot.CameraByName(cameraNames[0]), nil
}
