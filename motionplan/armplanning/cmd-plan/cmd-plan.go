// package main for testing armplanning
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/pprof"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	viz "github.com/viam-labs/motion-tools/client/client"
	"go.viam.com/utils/perf"

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
	logger, reg := logging.NewLoggerWithRegistry("cmd-plan")

	pseudolinearLine := flag.Float64("pseudolinear-line", 0, "")
	pseudolinearOrientation := flag.Float64("pseudolinear-orientation", 0, "")
	seed := flag.Int("seed", -1, "")
	verbose := flag.Bool("v", false, "verbose")
	loop := flag.Int("loop", 1, "loop")
	cpu := flag.String("cpu", "", "cpu profiling")
	interactive := flag.Bool("i", false, "interactive")

	flag.Parse()

	if len(flag.Args()) == 0 {
		return fmt.Errorf("need a json file")
	}

	if *cpu != "" {
		logger.Infof("writing cpu data to [%s]", *cpu)
		f, err := os.Create(*cpu)
		if err != nil {
			return fmt.Errorf("couldn't create %s %w", *cpu, err)
		}

		err = pprof.StartCPUProfile(f)
		if err != nil {
			return fmt.Errorf("could not start CPU profile: %w", err)
		}
		defer func() {
			pprof.StopCPUProfile()
			err = f.Close()
			if err != nil {
				logger.Errorf("couldn't write profiling file: %v", err)
			}
		}()
	}

	_ = reg
	if *verbose {
		logger.SetLevel(logging.DEBUG)
		reg.Update([]logging.LoggerPatternConfig{
			{
				Pattern: "*.mp*",
				Level:   "DEBUG",
			},
		}, logger)
	} else {
		reg.Update([]logging.LoggerPatternConfig{
			{
				Pattern: "*.mp",
				Level:   "DEBUG",
			},
			{
				Pattern: "*.ik",
				Level:   "INFO",
			},
			{
				Pattern: "*.cbirrt",
				Level:   "INFO",
			},
		}, logger)
	}

	logger.Infof("reading plan from %s", flag.Arg(0))
	req, err := armplanning.ReadRequestFromFile(flag.Arg(0))
	if err != nil {
		return err
	}

	if *pseudolinearLine > 0 || *pseudolinearOrientation > 0 {
		req.Constraints.AddPseudolinearConstraint(motionplan.PseudolinearConstraint{*pseudolinearLine, *pseudolinearOrientation})
	}

	if *seed >= 0 {
		req.PlannerOptions.RandomSeed = *seed
	}

	err = armplanning.PrepSmartSeed(req.FrameSystem, logger)
	if err != nil {
		return err
	}

	logger.Infof("starting motion planning for %d goals", len(req.Goals))
	mylog := log.New(os.Stdout, "", 0)
	start := time.Now()

	exporter := perf.NewDevelopmentExporter()
	if err := exporter.Start(); err != nil {
		return err
	}

	plan, _, err := armplanning.PlanMotion(ctx, logger, req)
	exporter.Stop()
	if *interactive {
		if interactiveErr := doInteractive(req, plan, err, mylog); interactiveErr != nil {
			logger.Fatal("Interactive mode failed:", interactiveErr)
		}
		return nil
	}
	if err != nil {
		if plan != nil {
			mylog.Printf("error but partial result of length: %d", len(plan.Trajectory()))
		}
		return err
	}

	if len(plan.Path()) != len(plan.Trajectory()) {
		return fmt.Errorf("path and trajectory not the same %d vs %d", len(plan.Path()), len(plan.Trajectory()))
	}

	for *cpu != "" && time.Since(start) < (10*time.Second) {
		ss := time.Now()
		_, _, err := armplanning.PlanMotion(ctx, logger, req)
		if err != nil {
			return err
		}
		mylog.Printf("extra plan took %v", time.Since(ss))
	}

	relevantParts := []string{}
	for c := range plan.Path()[0] {
		if len(c) == 0 {
			continue
		}
		relevantParts = append(relevantParts, c)
	}
	sort.Strings(relevantParts)

	totalCartesion := 0.0
	totalL2 := 0.0

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
			mylog.Printf("\t\t\t joints: %v", logging.FloatArrayFormat{"%0.2f", t[c]})
			if idx > 0 {
				p := plan.Trajectory()[idx-1][c]

				myl2n := referenceframe.InputsL2Distance(p, t[c])
				totalL2 += myl2n
				cart := pp.Pose().Point().Distance(plan.Path()[idx-1][c].Pose().Point())
				totalCartesion += cart

				mylog.Printf("\t\t\t\t distances l2: %0.4f Linf %0.4f cartesion: %0.2f",
					myl2n,
					referenceframe.InputsLinfDistance(p, t[c]),
					cart)
			}
		}
	}

	mylog.Printf("planning took %v for %d goals => trajectory length: %d",
		time.Since(start).Truncate(time.Millisecond), len(req.Goals), len(plan.Trajectory()))
	mylog.Printf("totalCartesion: %0.4f\n", totalCartesion)
	mylog.Printf("totalL2: %0.4f\n", totalL2)

	for i := 0; i < *loop; i++ {
		err = visualize(req, plan, mylog)
		if err != nil {
			mylog.Println("Couldn't visualize motion plan. Motion-tools server is probably not running. Skipping. Err:", err)
			break
		}
	}

	return nil
}

func visualize(req *armplanning.PlanRequest, plan motionplan.Plan, mylog *log.Logger) error {
	renderFramePeriod := 5 * time.Millisecond
	if err := viz.RemoveAllSpatialObjects(); err != nil {
		return err
	}

	startInputs := req.StartState.Configuration()
	// `DrawWorldState` just draws the obstacles. I think the FrameSystem/Path are necessary
	// because obstacles can be in terms of reference frames contained within the frame
	// system. Such as a camera attached to an arm.
	if err := viz.DrawWorldState(req.WorldState, req.FrameSystem, startInputs); err != nil {
		return err
	}

	// `DrawFrameSystem` draws everything else we're interested in.
	if err := viz.DrawFrameSystem(req.FrameSystem, startInputs); err != nil {
		return err
	}

	if err := drawGoalPoses(req); err != nil {
		return err
	}

	for idx := range plan.Path() {
		if idx > 0 {
			midPoints, err := motionplan.InterpolateSegmentFS(
				&motionplan.SegmentFS{
					StartConfiguration: plan.Trajectory()[idx-1].ToLinearInputs(),
					EndConfiguration:   plan.Trajectory()[idx].ToLinearInputs(),
					FS:                 req.FrameSystem,
				}, 2)
			if err != nil {
				return err
			}

			for _, mp := range midPoints {
				if err := viz.DrawFrameSystem(req.FrameSystem, mp.ToFrameSystemInputs()); err != nil {
					return err
				}

				time.Sleep(renderFramePeriod)
			}
		}

		if err := viz.DrawFrameSystem(req.FrameSystem, plan.Trajectory()[idx]); err != nil {
			return err
		}

		if idx == 0 {
			mylog.Println("Rendering motion plan. Num steps:", len(plan.Path()))
		}

		time.Sleep(renderFramePeriod)
	}

	return nil
}

func drawGoalPoses(req *armplanning.PlanRequest) error {
	var goalPoses []spatialmath.Pose
	for _, goalPlanState := range req.Goals {
		poses, err := goalPlanState.ComputePoses(context.Background(), req.FrameSystem)
		if err != nil {
			return err
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
		return err
	}

	return nil
}

func doInteractive(req *armplanning.PlanRequest, plan motionplan.Plan, planErr error, logger *log.Logger) error {
	var ikErr *armplanning.IkConstraintError
	errors.As(planErr, &ikErr)
	if err := viz.RemoveAllSpatialObjects(); err != nil {
		return err
	}

	if err := viz.DrawWorldState(req.WorldState, req.FrameSystem, req.StartState.Configuration()); err != nil {
		return err
	}

	if err := viz.DrawFrameSystem(req.FrameSystem, req.StartState.Configuration()); err != nil {
		return err
	}

	// ikIterOrder is a hack for helping index into individual failures. Such that an interactive
	// user can deterministically reference errors.
	var ikIterOrder []string
	if ikErr != nil {
		for key := range ikErr.FailuresByType {
			ikIterOrder = append(ikIterOrder, key)
		}

		// We sort such that the failure index across runs of `cmd-plan` interactive mode will pull
		// out the same error. This does not have to be sorted in string order. That's just a
		// convenient stable comparison for now.
		slices.Sort(ikIterOrder)
	}

	stdinReader := bufio.NewReader(os.Stdin)
	render := true
	for {
		if render {
			if planErr == nil {
				if err := visualize(req, plan, logger); err != nil {
					return err
				}
			} else {
				if ikErr != nil {
					logger.Println("Plan error:", ikErr.OutputString(true))
				} else {
					logger.Println("Plan error:", planErr)
				}
			}
			render = false
		}

		//nolint
		fmt.Print("$ ") // `logger.Print` seems to add a newline.
		cmd, err := stdinReader.ReadString('\n')
		cmd = strings.TrimSpace(cmd)
		switch {
		case err != nil && errors.Is(err, io.EOF):
			logger.Println("\nExiting...")
			return nil
		case cmd == "quit":
			logger.Println("Exiting...")
			return nil
		case cmd == "h" || cmd == "help":
			logger.Println("r, render")
			logger.Println("-  Rerender the selected motion plan.")
			logger.Println()
			logger.Println("le, list errors")
			logger.Println("-  If there were no IK solutions that satisfied constraints,",
				"this will list all of the failures grouped by error string.")
			logger.Println()
			logger.Println("de, detailed errors")
			logger.Println("-  If there were no IK solutions that satisfied constraints,",
				"this will list the configuration for each failed solution.")
			logger.Println()
			logger.Println("re, render error <number>")
			logger.Println("-  Renders the configuration of a failed solution.")
			logger.Println()
			logger.Println("`quit` or Ctrl-d to exit")
		case cmd == "render" || cmd == "r":
			logger.Println("Rendering motion plan")
			render = true
		case cmd == "list errors" || cmd == "le":
			if ikErr == nil {
				logger.Println("The error was not an IK error. No further diagnostics.")
				logger.Println("  Err:", planErr)
				continue
			}

			logger.Println("Listing errors:")
			for _, errStr := range ikIterOrder {
				failedSolutions := ikErr.FailuresByType[errStr]
				logger.Printf("  Err: %q Count: %v", errStr, len(failedSolutions))
			}
		case cmd == "detailed errors" || cmd == "de":
			if ikErr == nil {
				logger.Println("The error was not an IK error. No further diagnostics.")
				logger.Println("  Err:", planErr)
				continue
			}

			logger.Println("Listing errors:")
			idxCounter := 1
			for _, errStr := range ikIterOrder {
				failedConfigurations := ikErr.FailuresByType[errStr]
				logger.Printf("  Err: %q Count: %v", errStr, len(failedConfigurations))
				for _, configuration := range failedConfigurations {
					logger.Printf("    %d Inputs: %v", idxCounter, configuration)
					idxCounter++
				}
			}
		case strings.HasPrefix(cmd, "render error ") || strings.HasPrefix(cmd, "re "):
			pieces := strings.Split(cmd, " ")
			errorNumberStr := pieces[len(pieces)-1]
			errorNumber, err := strconv.Atoi(errorNumberStr)
			if err != nil {
				logger.Printf("Failed to parse error number. Val: %v Err: %v", errorNumberStr, err)
				logger.Println("Usage: `re <error number>`")
			}

			idxCounter := 1
		searchLoop:
			for _, errStr := range ikIterOrder {
				failedConfigurations := ikErr.FailuresByType[errStr]
				for _, configuration := range failedConfigurations {
					if idxCounter != errorNumber {
						idxCounter++
						continue
					}

					logger.Println("Rendering failed solution")
					logger.Println("  Err:", errStr)
					logger.Println("  Inputs:", configuration)
					if err := viz.DrawFrameSystem(req.FrameSystem, configuration.ToFrameSystemInputs()); err != nil {
						return err
					}
					break searchLoop
				}
			}

		case len(cmd) == 0:
		default:
			logger.Println("Unknown command. Type `h` for help.")
		}
	}
}
