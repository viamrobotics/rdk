// Package main allows one to play around with a robotic arm.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/action"
	commonpb "go.viam.com/core/proto/api/common/v1"
	"go.viam.com/core/robot"
	webserver "go.viam.com/core/web/server"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/common/v1"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/core/motionplan"
	"go.viam.com/core/robots/xarm"
	vutils "go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"github.com/golang/geo/r3"
)

var wbY = -440.
var home7 = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0, 0})

var p1 = frame.JointPositionsFromRadians([]float64{3.75646398939225,-1.0162453766159272,1.2142890600914453,1.0521227724322786,-0.21337105357552288,-0.006502311329196852,-4.3822913510408945})
var p2 = frame.JointPositionsFromRadians([]float64{3.896845654143853,-0.8353398707254642,1.1306783805718412,0.8347159514038981,0.49562136809544177,-0.2260694386799326,-4.383397470889424})


var p3 = frame.InputsToJointPos(frame.InterpolateInputs(frame.JointPosToInputs(p1), frame.JointPosToInputs(p2), 0.5))
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

	//~ start, err := arm.CurrentJointPositions(ctx)
	//~ if err != nil {
		//~ return err
	//~ }
	
	err := arm.MoveToJointPositions(ctx, p1)
	if err != nil {
		return err
	}
	time.Sleep(5*time.Second)
	err = arm.MoveToJointPositions(ctx, p3)
	if err != nil {
		return err
	}
	time.Sleep(20*time.Second)
	err = arm.MoveToJointPositions(ctx, p2)
	if err != nil {
		return err
	}

	//~ for i := 0; i < 180; i += 10 {
		//~ start.Degrees[0] = float64(i)
		//~ err := arm.MoveToJointPositions(ctx, start)
		//~ if err != nil {
			//~ return err
		//~ }

		//~ if !utils.SelectContextOrWait(ctx, time.Second) {
			//~ return ctx.Err()
		//~ }
	//~ }

	return nil
}

func main() {
	TestWrite1()
	utils.ContextualMain(webserver.RunServer, logger)
	
}


func TestWrite1() {

	fs := frame.NewEmptySimpleFrameSystem("test")

	ctx := context.Background()
	m, err := frame.ParseJSONFile(vutils.ResolveFile("robots/xarm/xArm7_kinematics.json"), "")

	err = fs.AddFrame(m, fs.World())

	markerOffFrame, err := frame.NewStaticFrame("marker_offset", spatial.NewPoseFromOrientation(r3.Vector{}, &spatial.OrientationVectorDegrees{OY: 1, OZ: 1}))
	markerFrame, err := frame.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0, 0, 160}))
	err = fs.AddFrame(markerOffFrame, m)
	err = fs.AddFrame(markerFrame, markerOffFrame)

	eraserOffFrame, err := frame.NewStaticFrame("eraser_offset", spatial.NewPoseFromOrientation(r3.Vector{}, &spatial.OrientationVectorDegrees{OY: -1, OZ: 1}))
	eraserFrame, err := frame.NewStaticFrame("eraser", spatial.NewPoseFromPoint(r3.Vector{0, 0, 160}))
	err = fs.AddFrame(eraserOffFrame, m)
	err = fs.AddFrame(eraserFrame, eraserOffFrame)

	//~ markerFrame, err := frame.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0, 0, 105}))
	//~ err = fs.AddFrame(markerFrame, m)

	moveFrame := markerFrame

	// Have to be able to update the motion planner from here
	mpFunc := func(f frame.Frame, ncpu int, logger golog.Logger) (motionplan.MotionPlanner, error) {
		// just in case frame changed
		mp, err := motionplan.NewCBiRRTMotionPlanner(f, 4, logger)
		opt := motionplan.NewDefaultPlannerOptions()
		opt.AddConstraint("officewall", DontHitPetersWallConstraint)
		mp.SetOptions(opt)

		return mp, err
	}

	fss := motionplan.NewSolvableFrameSystem(fs, logger)

	fss.SetPlannerGen(mpFunc)

	arm, err := xarm.NewxArm(ctx, config.Component{Host: "10.0.0.98"}, logger, 7)
	// home
	//~ arm.MoveToJointPositions(ctx, frame.InputsToJointPos(home7))

	// draw pos start
	goal := spatial.NewPoseFromProtobuf(&pb.Pose{
		X:  480,
		Y:  wbY+80,
		Z:  600,
		OY: -1,
	})

	seedMap := map[string][]frame.Input{}

	jPos, err := arm.CurrentJointPositions(ctx)
	seedMap[m.Name()] = frame.JointPosToInputs(jPos)
	curPos, _ := fs.TransformFrame(seedMap, moveFrame, fs.World())

	fmt.Println("curpos", spatial.PoseToProtobuf(curPos))

	steps, err := fss.SolvePose(ctx, seedMap, goal, moveFrame, fs.World())

	fmt.Println("steps, err", steps, err)

	waypoints := make(chan []frame.Input, 9999)

	validOV := &spatial.OrientationVector{OX: 0, OY: -1, OZ: 0}

	goToGoal := func(seedMap map[string][]frame.Input, goal spatial.Pose) map[string][]frame.Input {

		curPos, _ = fs.TransformFrame(seedMap, moveFrame, fs.World())

		validFunc, gradFunc := motionplan.NewLineConstraintAndGradient(curPos.Point(), goal.Point(), validOV, 0.3, 0.05)
		destGrad := motionplan.NewPoseFlexOVMetric(goal, 0.2)

		// update constraints
		mpFunc = func(f frame.Frame, ncpu int, logger golog.Logger) (motionplan.MotionPlanner, error) {
			// just in case frame changed
			mp, err := motionplan.NewCBiRRTMotionPlanner(f, 4, logger)
			opt := motionplan.NewDefaultPlannerOptions()
			opt.AddConstraint("officewall", DontHitPetersWallConstraint)
			
			opt.SetPathDist(gradFunc)
			opt.SetMetric(destGrad)
			opt.AddConstraint("whiteboard", validFunc)

			mp.SetOptions(opt)

			return mp, err
		}
		fss.SetPlannerGen(mpFunc)

		waysteps, err := fss.SolvePose(ctx, seedMap, goal, moveFrame, fs.World())
		fmt.Println("waysteps", waysteps, err)
		for _, waystep := range waysteps {
			waypoints <- waystep[m.Name()]
		}
		return waysteps[len(waysteps)-1]
	}

	go func() {
		seed := steps[len(steps)-1]
		for _, goal = range viamPoints {
			seed = goToGoal(seed, goal)
		}
	}()

	for _, step := range steps {
		arm.MoveToJointPositions(ctx, frame.InputsToJointPos(step[m.Name()]))
	}
	for {
		select {
		case waypoint := <-waypoints:
			//~ fmt.Println(waypoint)
			arm.MoveToJointPositions(ctx, frame.InputsToJointPos(waypoint))
		default:
			time.Sleep(1000 * time.Millisecond)
		}
	}
}

// Write out the word "VIAM"
var viamPoints = []spatial.Pose{
	//~ spatial.NewPoseFromProtobuf(&pb.Pose{X: 440, Y: wbY, Z: 500, OY: -1}),
	//~ spatial.NewPoseFromProtobuf(&pb.Pose{X: 140, Y: wbY, Z: 550, OY: -1}),
	//~ spatial.NewPoseFromProtobuf(&pb.Pose{X: 140, Y: wbY+50, Z: 550, OY: -1}),
	
	
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 480, Y: wbY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 440, Y: wbY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 400, Y: wbY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 400, Y: wbY + 10, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 380, Y: wbY + 10, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 380, Y: wbY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 380, Y: wbY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 380, Y: wbY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 360, Y: wbY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 360, Y: wbY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 320, Y: wbY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 280, Y: wbY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 280, Y: wbY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 340, Y: wbY + 10, Z: 550, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 340, Y: wbY, Z: 550, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 300, Y: wbY, Z: 550, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 300, Y: wbY + 10, Z: 550, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 260, Y: wbY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 260, Y: wbY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 230, Y: wbY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 200, Y: wbY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 170, Y: wbY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 140, Y: wbY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 140, Y: wbY+150, Z: 500, OY: -1}),
}

// Write out the word "VIAM"
var eraserPoints = []spatial.Pose{
	//~ spatial.NewPoseFromProtobuf(&pb.Pose{X: 440, Y: wbY, Z: 500, OY: -1}),
	//~ spatial.NewPoseFromProtobuf(&pb.Pose{X: 140, Y: wbY, Z: 550, OY: -1}),
	//~ spatial.NewPoseFromProtobuf(&pb.Pose{X: 140, Y: wbY+50, Z: 550, OY: -1}),
	
	
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 480, Y: wbY, Z: 580, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 120, Y: wbY, Z: 580, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 120, Y: wbY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 480, Y: wbY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 480, Y: wbY+150, Z: 500, OY: -1}),
}

// DontHitPetersWallConstraint defines some obstacles that nothing should not intersect with
// TODO(pl): put this somewhere else, maybe in an example file or something
func DontHitPetersWallConstraint(ci *motionplan.ConstraintInput) (bool, float64) {

	checkPt := func(pose spatial.Pose) bool {
		pt := pose.Point()

		// wall in Peter's office
		if pt.Y < wbY-10 {
			//~ fmt.Println(1)
			return false
		}
		if pt.X < -600 {
			//~ fmt.Println(2)
			return false
		}
		// shelf in Peter's office
		if pt.Z < 5 && pt.Y < 260 && pt.X < 140 {
			//~ fmt.Println(3)
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
			//~ fmt.Println(4)
			return false, 0
		}
		if !checkPt(pos) {
			//~ fmt.Println(5)
			return false, 0
		}
	}
	if ci.EndPos != nil {
		if !checkPt(ci.EndPos) {
			//~ fmt.Println(6)
			return false, 0
		}
	} else if ci.EndInput != nil {
		pos, err := ci.Frame.Transform(ci.EndInput)
		if err != nil {
			//~ fmt.Println(7)
			return false, 0
		}
		if !checkPt(pos) {
			//~ fmt.Println(8)
			return false, 0
		}
	}
	//~ fmt.Println("ok")
	return true, 0
}
