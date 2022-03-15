package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/action"
	"go.viam.com/rdk/motionplan"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot"
	spatial "go.viam.com/rdk/spatialmath"
	webserver "go.viam.com/rdk/web/server"
	"go.viam.com/utils"
)

//go:embed draw.json

var (
	logger = golog.NewDevelopmentLogger("minimultiaxis")
)

func init() {
	action.RegisterAction("draw", func(ctx context.Context, r robot.Robot) {
		err := draw(ctx, r, gcodePoints, "zaxis")
		if err != nil {
			logger.Errorf("error drawing: %s", err)
		}
	})
}

func draw(ctx context.Context, r robot.Robot, points []spatial.Pose, moveFrameName string) error {
	resources, err := getInputEnabled(ctx, r)
	if err != nil {
		return err
	}

	fs, err := r.FrameSystem(ctx, "fs", "")
	if err != nil {
		return err
	}

	// zaxis is the gantry frame
	moveFrame := fs.GetFrame(moveFrameName)
	if moveFrame == nil {
		return err
	}

	sfs := motionplan.NewSolvableFrameSystem(fs, logger)

	seedMap, err := getCurrentInputs(ctx, resources)
	if err != nil {
		return err
	}

	curPos, err := fs.TransformFrame(seedMap, moveFrameName, referenceframe.World)
	if err != nil {
		return err

	}

	// goal is inital home.
	goal := spatial.NewPoseFromProtobuf(&commonpb.Pose{
		X: 45,
		Y: 45,
		Z: 2,
	})
	opt := motionplan.NewDefaultPlannerOptions()
	steps, err := sfs.SolvePoseWithOptions(ctx, seedMap, goal, moveFrameName, referenceframe.World, opt)
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

		validFunc, gradFunc := motionplan.NewLineConstraint(curPos.Point(), goal.Point(), validOV, pathO, pathD)
		destGrad := motionplan.NewPoseFlexOVMetric(goal, destO)

		// update constraints
		opt := motionplan.NewDefaultPlannerOptions()

		opt.SetPathDist(gradFunc)
		opt.SetMetric(destGrad)
		opt.AddConstraint("whiteboard", validFunc)

		waysteps, err := sfs.SolvePoseWithOptions(ctx, seedMap, goal, moveFrame.Name(), fs.World().Name(), opt)
		if err != nil {
			return map[string][]referenceframe.Input{}
		}
		for _, waystep := range waysteps {
			waypoints <- waystep
		}
		return waysteps[len(waysteps)-1]
	}

	finish := func(seedMap map[string][]referenceframe.Input) map[string][]referenceframe.Input {
		goal := spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 10, Y: 0, Z: 10})

		curPos, _ = fs.TransformFrame(seedMap, moveFrameName, referenceframe.World)

		// update constraints
		opt := motionplan.NewDefaultPlannerOptions()

		waysteps, err := sfs.SolvePoseWithOptions(ctx, seedMap, goal, moveFrameName, fs.World().Name(), opt)
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

	return nil
}

/*
func convertGcodeToPose(ctx context.Context, dest []map[string]) ([]map[string], error){
	// figure this out
	return []map[string], nil
}
*/

func main() {
	utils.ContextualMain(webserver.RunServer, logger)
}

func getInputEnabled(ctx context.Context, r robot.Robot) (map[string]referenceframe.InputEnabled, error) {
	fs, err := r.FrameSystem(ctx, "fs", "")
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

// Change to gcodepoints
var gcodePoints = []spatial.Pose{
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 480, Y: 0, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 440, Y: 0, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&commonpb.Pose{X: 400, Y: 0, Z: 600, OY: -1}),
}
