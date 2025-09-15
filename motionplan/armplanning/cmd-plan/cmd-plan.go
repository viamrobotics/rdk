// package main for testing armplanning
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	viz "github.com/viam-labs/motion-tools/client/client"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

func main() {
	err := realMain()
	if err != nil {
		panic(err)
	}
}

func realMain() error {
	ctx := context.Background()
	logger := logging.NewLogger("cmd-plan")

	pseudolinearLine := flag.Float64("pseudolinear-line", 0, "")
	pseudolinearOrientation := flag.Float64("pseudolinear-orientation", 0, "")
	seed := flag.Int("seed", -1, "")
	verbose := flag.Bool("v", false, "verbose")

	flag.Parse()
	if len(flag.Args()) == 0 {
		return fmt.Errorf("need a json file")
	}

	if *verbose {
		logger.SetLevel(logging.DEBUG)
	}

	logger.Infof("reading plan from %s", flag.Arg(0))

	content, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		return err
	}

	req := armplanning.PlanRequest{}

	err = json.Unmarshal(content, &req)
	if err != nil {
		return err
	}

	if *pseudolinearLine > 0 || *pseudolinearOrientation > 0 {
		req.Constraints.AddPseudolinearConstraint(motionplan.PseudolinearConstraint{*pseudolinearLine, *pseudolinearOrientation})
	}

	if *seed >= 0 {
		req.PlannerOptions.RandomSeed = *seed
	}

	start := time.Now()

	plan, err := armplanning.PlanMotion(ctx, logger, &req)
	if err != nil {
		return err
	}

	if len(plan.Path()) != len(plan.Trajectory()) {
		return fmt.Errorf("path and trajectory not the same %d vs %d", len(plan.Path()), len(plan.Trajectory()))
	}

	mylog := log.New(os.Stdout, "", 0)

	mylog.Printf("planning took %v", time.Since(start))

	relevantParts := []string{}
	for c := range plan.Path()[0] {
		if len(c) == 0 {
			continue
		}
		relevantParts = append(relevantParts, c)
	}
	sort.Strings(relevantParts)

	for idx, p := range plan.Path() {
		mylog.Printf("step %d", idx)

		t := plan.Trajectory()[idx]

		if len(p) != len(t) {
			return fmt.Errorf("p and t are different sizes %d vs %d", len(p), len(t))
		}

		for _, c := range relevantParts {
			pp := p[c]
			if len(t[c]) == 0 {
				continue
			}
			mylog.Printf("\t\t %s", c)
			mylog.Printf("\t\t\t %v", pp)
			mylog.Printf("\t\t\t %v", t[c])
			if idx > 0 {
				p := plan.Trajectory()[idx-1][c]
				mylog.Printf("\t\t\t\t distances l2: %0.4f Linf %0.4f cartesion: %0.2f",
					referenceframe.InputsL2Distance(p, t[c]),
					referenceframe.InputsLinfDistance(p, t[c]),
					pp.Pose().Point().Distance(plan.Path()[idx-1][c].Pose().Point()),
				)
			}
		}
	}

	renderFramePeriod := 50 * time.Millisecond
	if err := viz.RemoveAllSpatialObjects(); err != nil {
		mylog.Println("Couldn't visualize motion plan. Motion-tools server is probably not running. Skipping. Err:", err)
		return nil
	}

	for idx := range plan.Path() {
		// `DrawWorldState` just draws the obstacles. I think the FrameSystem/Path are necessary
		// because obstacles can be in terms of reference frames contained within the frame
		// system. Such as a camera attached to an arm.
		if err := viz.DrawWorldState(req.WorldState, req.FrameSystem, plan.Trajectory()[0]); err != nil {
			mylog.Println("Couldn't visualize motion plan. Motion-tools server is probably not running. Skipping. Err:", err)
			break
		}

		// `DrawFrameSystem` draws everything else we're interested in.
		if err := viz.DrawFrameSystem(req.FrameSystem, plan.Trajectory()[idx]); err != nil {
			mylog.Println("Couldn't visualize motion plan. Motion-tools server is probably not running. Skipping. Err:", err)
			break
		}

		var goalPoses []spatialmath.Pose
		for _, goalPlanState := range req.Goals {
			poses, err := goalPlanState.ComputePoses(req.FrameSystem)
			if err != nil {
				mylog.Println("Could not compute goal poses. Err:", err)
				continue
			}

			for _, poseValue := range poses {
				// Dan: This is my guess on how to assure the goal pose is in the world reference
				// frame.
				poseInWorldFrame := poseValue.Transform(
					referenceframe.NewPoseInFrame(
						req.FrameSystem.World().Name(),
						spatialmath.NewZeroPose())).(*referenceframe.PoseInFrame)
				goalPoses = append(goalPoses, poseInWorldFrame.Pose())
			}
		}

		// A matter of preference. The arrow head will point at the goal point. As opposed to the
		// tail starting at the goal point.
		arrowHeadAtPose := true
		if err := viz.DrawPoses(goalPoses, []string{"blue"}, arrowHeadAtPose); err != nil {
			mylog.Println("Couldn't visualize motion plan. Motion-tools server is probably not running. Skipping. Err:", err)
			break
		}

		if idx == 0 {
			mylog.Println("Rendering motion plan. Num steps:", len(plan.Path()),
				"Approx time:", time.Duration(len(plan.Path()))*renderFramePeriod)
		}

		time.Sleep(renderFramePeriod)
	}

	return nil
}
