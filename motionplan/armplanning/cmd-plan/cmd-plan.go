// package main for testing armplanning
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime/pprof"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	viz "github.com/viam-labs/motion-tools/client/client"
	otelresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.viam.com/utils"
	"go.viam.com/utils/perf"
	"go.viam.com/utils/trace"

	"go.viam.com/rdk/cli"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/robot/framesystem"
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
	host := flag.String("host", "", "host to execute on")
	forceMotion := flag.Bool("force-move", false, "")
	waypointsFile := flag.String("output-waypoints", "", "json file to output waypoints")
	showPoses := flag.Bool("show-poses", false, "show shadows at each path position")
	tryManySeeds := flag.Int("try-many-seeds", 1, "try planning with more seeds and report L2 distances")

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

	// The default logger keeps `mp` at the default INFO level. But all loggers underneath only emit
	// WARN+ logs. Let's start with DEBUG everywhere and:
	logger.SetLevel(logging.DEBUG)
	if *verbose {
		// For verbose keep everything at DEBUG and only claw back `ik` logs to INFO.
		reg.Update([]logging.LoggerPatternConfig{
			{
				Pattern: "*.ik",
				Level:   "INFO",
			},
		}, logger)
	} else {
		// For regular cmd-plan runs, leave `mp` at DEBUG, and promote underneath loggers to emit
		// INFO+ logs.
		reg.Update([]logging.LoggerPatternConfig{
			{
				Pattern: "*.mp.*",
				Level:   "INFO",
			},
			{
				Pattern: "*.networking.*",
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

	metricsExporter := perf.NewDevelopmentExporterWithOptions(perf.DevelopmentExporterOptions{
		ReportingInterval: time.Second * 10,
		TracesDisabled:    true,
	})
	if err := metricsExporter.Start(); err != nil {
		return err
	}

	spansExporter := perf.NewOtelDevelopmentExporter()
	//nolint: errcheck
	trace.SetProvider(ctx, sdktrace.WithResource(otelresource.Empty()))
	trace.AddExporters(spansExporter)

	plan, meta, err := armplanning.PlanMotion(ctx, logger, req)
	if err := trace.Shutdown(ctx); err != nil {
		logger.Errorw("Got error while shutting down tracing", "err", err)
	}
	metricsExporter.Stop()
	if *interactive {
		if interactiveErr := doInteractive(req, plan, err, mylog, *showPoses); interactiveErr != nil {
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

	if *tryManySeeds > 1 {
		minDistance := 10000.0
		maxDistance := 0.0

		for i := 1; i < *tryManySeeds; i++ {
			req.PlannerOptions.RandomSeed = i
			seedPlan, _, err := armplanning.PlanMotion(ctx, logger, req)
			if err != nil {
				return fmt.Errorf("planning for seed %d failed %w", i, err)
			}

			seedTotalL2 := 0.0
			t := seedPlan.Trajectory()
			for idx := 1; idx < len(t); idx++ {
				for k := range t[idx] {
					myl2n := referenceframe.InputsL2Distance(t[idx-1][k], t[idx][k])
					seedTotalL2 += myl2n
				}
			}

			minDistance = min(minDistance, seedTotalL2)
			maxDistance = max(maxDistance, seedTotalL2)

			mylog.Printf("tryManySeeds seed %4d: traj_len=%d l2=%0.4f", i, len(t), seedTotalL2)
		}
		mylog.Printf("tryManySeeds result min: %0.2f max:%0.2f", minDistance, maxDistance)
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

				deltas := []float64{}
				for i, a := range t[c] {
					deltas = append(deltas, a-p[i])
				}

				mylog.Printf("\t\t\t\t deltas: %v", logging.FloatArrayFormat{"%0.5f", deltas})
			}
		}
	}

	if meta.PartialError != nil {
		mylog.Printf("partial results, error: %v", meta.PartialError)
	}

	mylog.Printf("planning took %v for %d goals => trajectory length: %d",
		time.Since(start).Truncate(time.Millisecond), len(req.Goals), len(plan.Trajectory()))
	mylog.Printf("totalCartesion: %0.4f\n", totalCartesion)
	mylog.Printf("totalL2: %0.4f\n", totalL2)

	// Print delta statistics if trajectory has more than 5 points
	if len(plan.Trajectory()) > 5 {
		stats := armplanning.TrajectoryDeltaStats(plan.Trajectory())
		mylog.Printf("\nDelta Statistics (trajectory length: %d):", len(plan.Trajectory()))
		for _, s := range stats {
			mylog.Printf("  %s:%d: avg=%0.5f stddev=%0.5f outside1=%d outside2=%d (n=%d)",
				s.Component, s.JointIdx, s.Mean, s.StdDev, s.Outside1, s.Outside2, s.Count)
		}
	}

	for i := 0; i < *loop; i++ {
		err = visualize(req, plan, mylog, *showPoses)
		if err != nil {
			mylog.Println("Couldn't visualize motion plan. Motion-tools server is probably not running. Skipping. Err:", err)
			break
		}
	}

	if *waypointsFile != "" {
		err := writeWaypointsToFile(ctx, plan, *waypointsFile)
		if err != nil {
			return err
		}
	}

	if *host != "" {
		err := executeOnArm(ctx, *host, plan, *forceMotion, logger)
		if err != nil {
			return err
		}
	}

	return nil
}

func visualize(req *armplanning.PlanRequest, plan motionplan.Plan, mylog *log.Logger, showPoses bool) error {
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

	if showPoses {
		// Helper to check if a frame or any ancestor has DOF (is moving)
		isMovingFrame := func(frameName string) bool {
			frame := req.FrameSystem.Frame(frameName)
			if frame == nil {
				return false
			}
			// Check if this frame has DOF
			if len(frame.DoF()) > 0 {
				return true
			}
			// Walk up the parent chain to see if any ancestor has DOF
			parent, err := req.FrameSystem.Parent(frame)
			for parent != nil && err == nil {
				if len(parent.DoF()) > 0 {
					return true
				}
				parent, err = req.FrameSystem.Parent(parent)
			}
			return false
		}

		// Draw shadows for path positions - moving components and their descendants
		// Alternate colors to distinguish different path positions
		shadowColors := []string{"blue", "red"}
		for idx := range plan.Path() {
			gifs, err := referenceframe.FrameSystemGeometries(req.FrameSystem, plan.Trajectory()[idx])
			if err != nil {
				return err
			}
			// Pick color for this path position (alternating)
			shadowColor := shadowColors[idx%len(shadowColors)]

			// Draw shadows only for moving frames and their descendants
			for frameName, gif := range gifs {
				// Skip if this frame and all ancestors are static
				if !isMovingFrame(frameName) {
					continue
				}

				// Create copies with unique labels to not interfere with animation
				shadowGeometries := make([]spatialmath.Geometry, len(gif.Geometries()))
				for i, geom := range gif.Geometries() {
					// Copy geometry without additional transformation (identity transform)
					shadowGeom := geom.Transform(spatialmath.NewZeroPose())
					shadowGeom.SetLabel(fmt.Sprintf("shadow_%d_%s_%d", idx, geom.Label(), i))
					shadowGeometries[i] = shadowGeom
				}
				// Use the original parent frame from gif
				shadowGIF := referenceframe.NewGeometriesInFrame(gif.Parent(), shadowGeometries)
				colors := make([]string, len(shadowGeometries))
				for i := range colors {
					colors[i] = shadowColor
				}
				if err := viz.DrawGeometries(shadowGIF, colors); err != nil {
					return err
				}
			}
		}
	}

	// Now animate through the path
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

func doInteractive(req *armplanning.PlanRequest, plan motionplan.Plan, planErr error, logger *log.Logger, showPoses bool) error {
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
				if err := visualize(req, plan, logger, showPoses); err != nil {
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

func getFullTrajectoryByComponent(plan motionplan.Plan) map[string][][]referenceframe.Input {
	byComponent := map[string][][]referenceframe.Input{}

	for _, s := range plan.Trajectory() {
		for cName, inputs := range s {
			if len(inputs) > 0 {
				byComponent[cName] = append(byComponent[cName], inputs)
			}
		}
	}

	return byComponent
}

func executeOnArm(ctx context.Context, host string, plan motionplan.Plan, force bool, logger logging.Logger) error {
	byComponent := getFullTrajectoryByComponent(plan)

	if len(byComponent) > 1 {
		return fmt.Errorf("executeOnArm only supports one component moving right now, not: %d", len(byComponent))
	}

	c, err := cli.ConfigFromCache(nil)
	if err != nil {
		return err
	}

	dopts, err := c.DialOptions()
	if err != nil {
		return err
	}

	theRobot, err := client.New(
		ctx,
		host,
		logger,
		client.WithDialOptions(dopts...),
	)
	if err != nil {
		return err
	}
	defer func() {
		err := theRobot.Close(ctx)
		if err != nil {
			logger.Errorf("cannot close robot: %v", err)
		}
	}()

	for cName, allInputs := range byComponent {
		r, err := robot.ResourceByName(theRobot, cName)
		if err != nil {
			return err
		}

		ie, ok := r.(framesystem.InputEnabled)
		if !ok {
			return fmt.Errorf("%s is not InputEnabled, is %T", cName, r)
		}

		cur, err := ie.CurrentInputs(ctx)
		if err != nil {
			return err
		}

		for j, v := range cur {
			delta := math.Abs(v - allInputs[0][j])
			if delta > .01 {
				err = fmt.Errorf("joint %d for resource %s too far start: %0.5f go: %0.5f delta: %0.5f",
					j, cName, v, allInputs[0][j], delta)
				if force {
					logger.Warnf("ignoring %v", err)
				} else {
					return err
				}
			}
		}

		logger.Infof("sending %d positions to %s", len(allInputs), cName)

		err = ie.GoToInputs(ctx, allInputs...)
		if err != nil {
			return err
		}
	}

	return nil
}

func writeWaypointsToFile(ctx context.Context, plan motionplan.Plan, fileName string) error {
	byComponent := getFullTrajectoryByComponent(plan)
	if len(byComponent) != 1 {
		return fmt.Errorf("to output waypointsFile need exactly one component moving, not %d", len(byComponent))
	}

	ff := &waypointsFileFormat{}
	for _, v := range byComponent {
		ff.Waypoints = v
	}

	file, err := os.OpenFile(filepath.Clean(fileName), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(file.Close)

	data, err := json.Marshal(ff)
	if err != nil {
		return err
	}

	_, err = file.Write(data)
	if err != nil {
		return err
	}
	return nil
}

type waypointsFileFormat struct {
	Waypoints [][]float64 `json:"waypoints_rad"`
}
