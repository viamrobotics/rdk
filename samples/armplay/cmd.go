// Package main allows one to play around with a robotic arm.
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/action"
	"go.viam.com/core/motionplan"
	commonpb "go.viam.com/core/proto/api/common/v1"
	pb "go.viam.com/core/proto/api/common/v1"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/robot"
	spatial "go.viam.com/core/spatialmath"
	webserver "go.viam.com/core/web/server"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.uber.org/multierr"
)

var (
	logger      = golog.NewDevelopmentLogger("armplay")
	whiteboardY = -509.
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

func mpFuncBasic(f frame.Frame, ncpu int, logger golog.Logger) (motionplan.MotionPlanner, error) {
	mp, err := motionplan.NewCBiRRTMotionPlanner(f, 4, logger)
	opt := motionplan.NewDefaultPlannerOptions()
	opt.AddConstraint("officewall", DontHitPetersWallConstraint(whiteboardY+15))
	mp.SetOptions(opt)

	return mp, err
}

func followPoints(ctx context.Context, r robot.Robot, points []spatial.Pose, moveFrameName string) error {
	resources, err := getInputEnabled(ctx, r)
	if err != nil {
		return err
	}
	fs, err := r.FrameSystem(ctx, "fs", "")
	if err != nil {
		return err
	}
	armFrame := fs.GetFrame(r.ArmNames()[0])

	markerOffFrame, err := frame.NewStaticFrame("marker_offset", spatial.NewPoseFromOrientation(r3.Vector{}, &spatial.OrientationVectorDegrees{OY: 1, OZ: 1}))
	if err != nil {
		return err
	}
	markerFrame, err := frame.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0, 0, 160}))
	if err != nil {
		return err
	}

	eraserOffFrame, err := frame.NewStaticFrame("eraser_offset", spatial.NewPoseFromOrientation(r3.Vector{}, &spatial.OrientationVectorDegrees{OY: -1, OZ: 1}))
	if err != nil {
		return err
	}
	eraserFrame, err := frame.NewStaticFrame("eraser", spatial.NewPoseFromPoint(r3.Vector{0, 0, 160}))
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

	fss.SetPlannerGen(mpFuncBasic)
	goal := spatial.NewPoseFromProtobuf(&pb.Pose{
		X:  480,
		Y:  whiteboardY + 80,
		Z:  600,
		OY: -1,
	})

	seedMap, err := getCurrentInputs(ctx, resources)
	if err != nil {
		return err
	}

	curPos, err := fs.TransformFrame(seedMap, moveFrame, fs.World())
	if err != nil {
		return err
	}

	steps, err := fss.SolvePose(ctx, seedMap, goal, moveFrame, fs.World())
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
	waypoints := make(chan map[string][]frame.Input, 9999)

	validOV := &spatial.OrientationVector{OX: 0, OY: -1, OZ: 0}

	goToGoal := func(seedMap map[string][]frame.Input, goal spatial.Pose) map[string][]frame.Input {

		curPos, _ = fs.TransformFrame(seedMap, moveFrame, fs.World())

		validFunc, gradFunc := motionplan.NewLineConstraintAndGradient(curPos.Point(), goal.Point(), validOV, pathO, pathD)
		destGrad := motionplan.NewPoseFlexOVMetric(goal, destO)

		// update constraints
		mpFunc := func(f frame.Frame, ncpu int, logger golog.Logger) (motionplan.MotionPlanner, error) {
			// just in case frame changed
			mp, err := motionplan.NewCBiRRTMotionPlanner(f, 4, logger)
			opt := motionplan.NewDefaultPlannerOptions()
			opt.AddConstraint("officewall", DontHitPetersWallConstraint(whiteboardY))

			opt.SetPathDist(gradFunc)
			opt.SetMetric(destGrad)
			opt.AddConstraint("whiteboard", validFunc)

			mp.SetOptions(opt)

			return mp, err
		}
		fss.SetPlannerGen(mpFunc)

		waysteps, err := fss.SolvePose(ctx, seedMap, goal, moveFrame, fs.World())
		if err != nil {
			return map[string][]frame.Input{}
		}
		for _, waystep := range waysteps {
			waypoints <- waystep
		}
		return waysteps[len(waysteps)-1]
	}

	finish := func(seedMap map[string][]frame.Input) map[string][]frame.Input {

		goal := spatial.NewPoseFromProtobuf(&pb.Pose{X: 260, Y: whiteboardY + 150, Z: 520, OY: -1})

		curPos, _ = fs.TransformFrame(seedMap, moveFrame, fs.World())

		// update constraints
		mpFunc := func(f frame.Frame, ncpu int, logger golog.Logger) (motionplan.MotionPlanner, error) {
			// just in case frame changed
			mp, err := motionplan.NewCBiRRTMotionPlanner(f, 4, logger)
			opt := motionplan.NewDefaultPlannerOptions()
			opt.AddConstraint("officewall", DontHitPetersWallConstraint(whiteboardY))

			mp.SetOptions(opt)

			return mp, err
		}
		fss.SetPlannerGen(mpFunc)

		waysteps, err := fss.SolvePose(ctx, seedMap, goal, moveFrame, fs.World())
		if err != nil {
			return map[string][]frame.Input{}
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

func getInputEnabled(ctx context.Context, r robot.Robot) (map[string]frame.InputEnabled, error) {
	fs, err := r.FrameSystem(ctx, "fs", "")
	if err != nil {
		return nil, err
	}
	input := frame.StartPositions(fs)
	resources := map[string]frame.InputEnabled{}

	for k := range input {
		if strings.HasSuffix(k, "_offset") {
			continue
		}

		all := robot.AllResourcesByName(r, k)
		if len(all) != 1 {
			return nil, fmt.Errorf("got %d resources instead of 1 for (%s)", len(all), k)
		}

		ii, ok := all[0].(frame.InputEnabled)
		if !ok {
			return nil, fmt.Errorf("%v(%T) is not InputEnabled", k, all[0])
		}

		resources[k] = ii

	}
	return resources, nil
}

func goToInputs(ctx context.Context, res map[string]frame.InputEnabled, dest map[string][]frame.Input) error {
	for name, inputFrame := range res {
		err := inputFrame.GoToInputs(ctx, dest[name])
		if err != nil {
			return err
		}
	}
	return nil
}

func getCurrentInputs(ctx context.Context, res map[string]frame.InputEnabled) (map[string][]frame.Input, error) {
	posMap := map[string][]frame.Input{}
	for name, inputFrame := range res {
		pos, err := inputFrame.CurrentInputs(ctx)
		if err != nil {
			return nil, err
		}
		posMap[name] = pos
	}
	return posMap, nil
}

// Write out the word "VIAM"
var viamPoints = []spatial.Pose{
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 480, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 440, Y: whiteboardY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 400, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 400, Y: whiteboardY + 10, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 380, Y: whiteboardY + 10, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 380, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 380, Y: whiteboardY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 380, Y: whiteboardY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 360, Y: whiteboardY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 360, Y: whiteboardY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 320, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 280, Y: whiteboardY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 280, Y: whiteboardY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 340, Y: whiteboardY + 10, Z: 550, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 340, Y: whiteboardY, Z: 550, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 300, Y: whiteboardY, Z: 550, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 300, Y: whiteboardY + 10, Z: 550, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 260, Y: whiteboardY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 260, Y: whiteboardY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 230, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 200, Y: whiteboardY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 170, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 140, Y: whiteboardY, Z: 500, OY: -1}),
}

// ViamLogo writes out the word "VIAM" in a stylized font
var viamLogo = []spatial.Pose{
	// V
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 480, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 440, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 400, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 420, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 440, Y: whiteboardY, Z: 560, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 460, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 480, Y: whiteboardY, Z: 600, OY: -1}),

	spatial.NewPoseFromProtobuf(&pb.Pose{X: 480, Y: whiteboardY + 10, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 390, Y: whiteboardY + 10, Z: 600, OY: -1}),

	// I
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 390, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 390, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 370, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 370, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 390, Y: whiteboardY, Z: 600, OY: -1}),

	spatial.NewPoseFromProtobuf(&pb.Pose{X: 390, Y: whiteboardY + 10, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 360, Y: whiteboardY + 10, Z: 520, OY: -1}),

	// A
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 360, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 320, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 280, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 300, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 320, Y: whiteboardY, Z: 560, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 340, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 360, Y: whiteboardY, Z: 520, OY: -1}),

	spatial.NewPoseFromProtobuf(&pb.Pose{X: 360, Y: whiteboardY + 10, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 270, Y: whiteboardY + 10, Z: 520, OY: -1}),

	// M
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 260, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 260, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 240, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 220, Y: whiteboardY, Z: 560, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 200, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 180, Y: whiteboardY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 180, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 195, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 195, Y: whiteboardY, Z: 560, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 220, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 245, Y: whiteboardY, Z: 560, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 245, Y: whiteboardY, Z: 520, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 260, Y: whiteboardY, Z: 520, OY: -1}),
}

// Erase where VIAM was written
var eraserPoints = []spatial.Pose{
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 480, Y: whiteboardY + 1.5, Z: 595, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 120, Y: whiteboardY + 1.5, Z: 595, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 120, Y: whiteboardY + 1.5, Z: 555, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 480, Y: whiteboardY + 1.5, Z: 555, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 480, Y: whiteboardY + 1.5, Z: 515, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 120, Y: whiteboardY + 1.5, Z: 515, OY: -1}),
}

// DontHitPetersWallConstraint defines some obstacles that nothing should intersect with
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
