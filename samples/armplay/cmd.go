// Package main allows one to play around with a robotic arm.
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/action"
	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/motionplan"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/framesystem"
	spatial "go.viam.com/rdk/spatialmath"
	webserver "go.viam.com/rdk/web/server"
)

var (
	logger      = golog.NewDevelopmentLogger("armplay")
	whiteboardY = -529.
)

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

	action.RegisterAction("writeViam", func(ctx context.Context, r robot.Robot) {
		err := followPoints(ctx, r, viamPoints, "marker")
		if err != nil {
			logger.Errorf("error writeViam: %s", err)
		}
	})

	action.RegisterAction("writeViamLogo", func(ctx context.Context, r robot.Robot) {
		err := followPoints(ctx, r, viamLogo, "marker")
		if err != nil {
			logger.Errorf("error writeViam: %s", err)
		}
	})

	action.RegisterAction("eraseViam", func(ctx context.Context, r robot.Robot) {
		err := followPoints(ctx, r, eraserPoints, "eraser")
		if err != nil {
			logger.Errorf("error writeViam: %s", err)
		}
	})
}

func chrisCirlce(ctx context.Context, r robot.Robot) error {
	if len(arm.NamesFromRobot(r)) != 1 {
		return errors.New("need 1 arm name")
	}

	a, err := arm.FromRobot(r, arm.NamesFromRobot(r)[0])
	if err != nil {
		return err
	}

	return multierr.Combine(
		a.MoveToPosition(ctx, &commonpb.Pose{X: -600, Z: 480}, &commonpb.WorldState{}),
		a.MoveToPosition(ctx, &commonpb.Pose{X: -200, Z: 480}, &commonpb.WorldState{}),
		a.MoveToPosition(ctx, &commonpb.Pose{X: -200, Z: 300}, &commonpb.WorldState{}),
		a.MoveToPosition(ctx, &commonpb.Pose{X: -600, Z: 300}, &commonpb.WorldState{}),
	)
}

func upAndDown(ctx context.Context, r robot.Robot) error {
	if len(arm.NamesFromRobot(r)) != 1 {
		return errors.New("need 1 arm name")
	}

	a, err := arm.FromRobot(r, arm.NamesFromRobot(r)[0])
	if err != nil {
		return err
	}

	for i := 0; i < 5; i++ {
		logger.Debugf("upAndDown loop %d", i)
		pos, err := a.GetEndPosition(ctx)
		if err != nil {
			return err
		}

		pos.Y += 550
		err = a.MoveToPosition(ctx, pos, &commonpb.WorldState{})
		if err != nil {
			return err
		}

		pos.Y -= 550
		err = a.MoveToPosition(ctx, pos, &commonpb.WorldState{})
		if err != nil {
			return err
		}
	}

	return nil
}

func play(ctx context.Context, r robot.Robot) error {
	if len(arm.NamesFromRobot(r)) != 1 {
		return errors.New("need 1 arm name")
	}

	a, err := arm.FromRobot(r, arm.NamesFromRobot(r)[0])
	if err != nil {
		return err
	}

	start, err := a.GetJointPositions(ctx)
	if err != nil {
		return err
	}

	for i := 0; i < 180; i += 10 {
		start.Degrees[0] = float64(i)
		err := a.MoveToJointPositions(ctx, start)
		if err != nil {
			return err
		}

		if !utils.SelectContextOrWait(ctx, time.Second) {
			return ctx.Err()
		}
	}

	return nil
}

func followPoints(ctx context.Context, r robot.Robot, points []spatial.Pose, moveFrameName string) error {
	resources, err := getInputEnabled(ctx, r)
	if err != nil {
		return err
	}
	fs, err := framesystem.RobotFrameSystem(ctx, r)
	if err != nil {
		return err
	}
	armFrame := fs.GetFrame(arm.NamesFromRobot(r)[0])

	markerOffFrame, err := referenceframe.NewStaticFrame(
		"marker_offset", spatial.NewPoseFromOrientation(r3.Vector{}, &spatial.OrientationVectorDegrees{OY: 1, OZ: 1}))
	if err != nil {
		return err
	}
	markerFrame, err := referenceframe.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0, 0, 160}))
	if err != nil {
		return err
	}

	eraserOffFrame, err := referenceframe.NewStaticFrame(
		"eraser_offset", spatial.NewPoseFromOrientation(r3.Vector{}, &spatial.OrientationVectorDegrees{OY: -1, OZ: 1}))
	if err != nil {
		return err
	}
	eraserFrame, err := referenceframe.NewStaticFrame("eraser", spatial.NewPoseFromPoint(r3.Vector{0, 0, 160}))
	if err != nil {
		return err
	}
	err = fs.AddFrame(eraserOffFrame, armFrame)
	if err != nil {
		return err
	}
	err = fs.AddFrame(eraserFrame, eraserOffFrame)
	if err != nil {
		return err
	}
	err = fs.AddFrame(markerOffFrame, armFrame)
	if err != nil {
		return err
	}
	err = fs.AddFrame(markerFrame, markerOffFrame)
	if err != nil {
		return err
	}

	moveFrame := fs.GetFrame(moveFrameName)
	if moveFrame == nil {
		return fmt.Errorf("frame does not exist %s", moveFrameName)
	}

	fss := motionplan.NewSolvableFrameSystem(fs, logger)

	goal := spatial.NewPoseFromProtobuf(&commonpb.Pose{
		X:  480,
		Y:  whiteboardY + 80,
		Z:  600,
		OY: -1,
	})

	seedMap, err := getCurrentInputs(ctx, resources)
	if err != nil {
		return err
	}

	curPos, err := fs.TransformFrame(seedMap, moveFrameName, referenceframe.World)
	if err != nil {
		return err
	}

	opt := motionplan.NewDefaultPlannerOptions()
	opt.AddConstraint("officewall", DontHitPetersWallConstraint(whiteboardY-15))
	steps, err := fss.SolvePoseWithOptions(ctx, seedMap, goal, moveFrameName, referenceframe.World, opt)
	if err != nil {
		return err
	}

	pathD := 0.05
	// orientation distance wiggle allowable
	pathO := 0.3
	destO := 0.2
	// No orientation wiggle for eraser
	if moveFrameName == "eraser" {
		pathO = 0.01
		destO = 0.
	}

	done := make(chan struct{})
	waypoints := make(chan map[string][]referenceframe.Input, 9999)

	validOV := &spatial.OrientationVector{OX: 0, OY: -1, OZ: 0}

	goToGoal := func(seedMap map[string][]referenceframe.Input, goal spatial.Pose) map[string][]referenceframe.Input {
		curPos, err = fs.TransformFrame(seedMap, moveFrameName, referenceframe.World)

		validFunc, gradFunc := motionplan.NewLineConstraint(curPos.Pose().Point(), goal.Point(), validOV, pathO, pathD)
		destGrad := motionplan.NewPoseFlexOVMetric(goal, destO)

		// update constraints
		opt := motionplan.NewDefaultPlannerOptions()
		opt.AddConstraint("officewall", DontHitPetersWallConstraint(whiteboardY))

		opt.SetPathDist(gradFunc)
		opt.SetMetric(destGrad)
		opt.AddConstraint("whiteboard", validFunc)

		waysteps, err := fss.SolvePoseWithOptions(ctx, seedMap, goal, moveFrame.Name(), fs.World().Name(), opt)
		if err != nil {
			return map[string][]referenceframe.Input{}
		}
		for _, waystep := range waysteps {
			waypoints <- waystep
		}
		return waysteps[len(waysteps)-1]
	}

	finish := func(seedMap map[string][]referenceframe.Input) map[string][]referenceframe.Input {
		goal := spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 260, Y: whiteboardY + 150, Z: 520, OY: -1})

		curPos, _ = fs.TransformFrame(seedMap, moveFrameName, referenceframe.World)

		// update constraints
		opt := motionplan.NewDefaultPlannerOptions()
		opt.AddConstraint("officewall", DontHitPetersWallConstraint(whiteboardY))

		waysteps, err := fss.SolvePoseWithOptions(ctx, seedMap, goal, moveFrameName, fs.World().Name(), opt)
		if err != nil {
			return map[string][]referenceframe.Input{}
		}
		for _, waystep := range waysteps {
			waypoints <- waystep
		}
		return waysteps[len(waysteps)-1]
	}

	go func() {
		seed := steps[len(steps)-1]
		for _, goal = range points {
			seed = goToGoal(seed, goal)
		}
		finish(seed)
		close(done)
	}()

	for _, step := range steps {
		goToInputs(ctx, resources, step)
	}
	for {
		select {
		case waypoint := <-waypoints:
			goToInputs(ctx, resources, waypoint)
		default:
			select {
			case <-done:
				return nil
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

func main() {
	utils.ContextualMain(webserver.RunServer, logger)
}

func getInputEnabled(ctx context.Context, r robot.Robot) (map[string]referenceframe.InputEnabled, error) {
	fs, err := framesystem.RobotFrameSystem(ctx, r)
	if err != nil {
		return nil, err
	}
	input := referenceframe.StartPositions(fs)
	resources := map[string]referenceframe.InputEnabled{}

	for k := range input {
		if strings.HasSuffix(k, "_offset") {
			continue
		}

		all := robot.AllResourcesByName(r, k)
		if len(all) != 1 {
			return nil, fmt.Errorf("got %d resources instead of 1 for (%s)", len(all), k)
		}

		ii, ok := all[0].(referenceframe.InputEnabled)
		if !ok {
			return nil, fmt.Errorf("%v(%T) is not InputEnabled", k, all[0])
		}

		resources[k] = ii
	}
	return resources, nil
}

func goToInputs(ctx context.Context, res map[string]referenceframe.InputEnabled, dest map[string][]referenceframe.Input) error {
	for name, inputFrame := range res {
		err := inputFrame.GoToInputs(ctx, dest[name])
		if err != nil {
			return err
		}
	}
	return nil
}

func getCurrentInputs(ctx context.Context, res map[string]referenceframe.InputEnabled) (map[string][]referenceframe.Input, error) {
	posMap := map[string][]referenceframe.Input{}
	for name, inputFrame := range res {
		pos, err := inputFrame.CurrentInputs(ctx)
		if err != nil {
			return nil, err
		}
		posMap[name] = pos
	}
	return posMap, nil
}

// Write out the word "VIAM".
var viamPoints = []spatial.Pose{
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 480, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 440, Y: whiteboardY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 400, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 400, Y: whiteboardY + 10, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 380, Y: whiteboardY + 10, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 380, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 380, Y: whiteboardY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 380, Y: whiteboardY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 360, Y: whiteboardY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 360, Y: whiteboardY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 320, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 280, Y: whiteboardY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 280, Y: whiteboardY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 340, Y: whiteboardY + 10, Z: 550, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 340, Y: whiteboardY, Z: 550, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 300, Y: whiteboardY, Z: 550, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 300, Y: whiteboardY + 10, Z: 550, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 260, Y: whiteboardY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 260, Y: whiteboardY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 230, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 200, Y: whiteboardY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 170, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 140, Y: whiteboardY, Z: 500, OY: -1}),
}

// ViamLogo writes out the word "VIAM" in a stylized font.
var viamLogo = []spatial.Pose{
	// V
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 480, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 440, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 400, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 420, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 440, Y: whiteboardY, Z: 560, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 460, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 480, Y: whiteboardY, Z: 600, OY: -1}),

	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 480, Y: whiteboardY + 10, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 390, Y: whiteboardY + 10, Z: 600, OY: -1}),

	// I
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 390, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 390, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 370, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 370, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 390, Y: whiteboardY, Z: 600, OY: -1}),

	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 390, Y: whiteboardY + 10, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 360, Y: whiteboardY + 10, Z: 520, OY: -1}),

	// A
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 360, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 320, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 280, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 300, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 320, Y: whiteboardY, Z: 560, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 340, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 360, Y: whiteboardY, Z: 520, OY: -1}),

	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 360, Y: whiteboardY + 10, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 270, Y: whiteboardY + 10, Z: 520, OY: -1}),

	// M
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 260, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 260, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 240, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 220, Y: whiteboardY, Z: 560, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 200, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 180, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 180, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 195, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 195, Y: whiteboardY, Z: 560, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 220, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 245, Y: whiteboardY, Z: 560, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 245, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 260, Y: whiteboardY, Z: 520, OY: -1}),
}

// Erase where VIAM was written.
var eraserPoints = []spatial.Pose{
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 480, Y: whiteboardY + 1.5, Z: 595, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 120, Y: whiteboardY + 1.5, Z: 595, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 120, Y: whiteboardY + 1.5, Z: 555, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 480, Y: whiteboardY + 1.5, Z: 555, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 480, Y: whiteboardY + 1.5, Z: 515, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 120, Y: whiteboardY + 1.5, Z: 515, OY: -1}),
}

// DontHitPetersWallConstraint defines some obstacles that nothing should intersect with.
func DontHitPetersWallConstraint(wbY float64) func(ci *motionplan.ConstraintInput) (bool, float64) {
	f := func(ci *motionplan.ConstraintInput) (bool, float64) {
		checkPt := func(pose spatial.Pose) bool {
			pt := pose.Point()

			// wall in Peter's office
			if pt.Y < wbY-10 {
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
	return f
}
