// package main for testing armplanning.
//
// It takes one or more recorded plan request files and replays them in the order given:
//
//	cmd-plan plan.json                     # the original single-request behavior
//	cmd-plan plan1.json plan2.json         # replay both exactly as recorded
//	cmd-plan -chain plan1.json plan2.json  # plan2 starts where plan1's plan ended
//	cmd-plan -fs-override limits.json plan1.json plan2.json  # both against edited DoF limits
//
// Replaying as recorded asks whether each plan still solves on its own. Chaining asks whether the
// robot could actually have run them back to back, which is the question when a captured sequence
// worked step by step but drifted somewhere along the way.
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
	"runtime/debug"
	"runtime/pprof"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	viz "github.com/viam-labs/motion-tools/client/api"
	"github.com/viam-labs/motion-tools/draw"
	otelresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.viam.com/utils"
	"go.viam.com/utils/perf"
	"go.viam.com/utils/trace"

	"go.viam.com/rdk/cli"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/motionplan/armplanning/mpserver"
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

// runOptions carries the parsed flags that the per-request planning path needs, so each step
// doesn't have to thread a dozen parameters.
type runOptions struct {
	pseudolinearLine        float64
	pseudolinearOrientation float64
	seed                    int
	loop                    int
	cpu                     string
	interactive             bool
	host                    string
	forceMotion             bool
	showPoses               bool
	tryManySeeds            int
	noViz                   bool
	chain                   bool
}

// runner replays the sequence of plan requests named on the command line.
type runner struct {
	opts     *runOptions
	override *frameSystemOverride
	logger   logging.Logger
	// mpLogger is what PlanMotion logs through; -quiet swaps it for a logger with no appenders.
	mpLogger logging.Logger
	mylog    *log.Logger
	// start is when the run began, and bounds the -cpu re-plan loop.
	start time.Time
	// single is true when exactly one request was given, which keeps the output identical to what
	// this tool printed before it understood sequences.
	single bool
}

// step is one request in the sequence.
type step struct {
	idx      int
	total    int
	file     string
	req      *armplanning.PlanRequest
	origPlan motionplan.Plan
}

// stepResult is what one step produced, kept for the end-of-run summary and for chaining. A nil
// plan means the step did not produce one, which only happens when interactive mode consumed a
// planning failure.
type stepResult struct {
	file           string
	goals          int
	plan           motionplan.Plan
	meta           *armplanning.PlanMeta
	duration       time.Duration
	totalL2        float64
	totalCartesion float64
}

func realMain() error {
	// The planner allocates heavily during search (per-state geometry
	// materialization); the default GC target spends a quarter of planning
	// CPU on mark work. A higher target trades memory headroom for planning
	// speed. Overridable with the GOGC env var.
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(300)
	}

	ctx := context.Background()
	logger, reg := logging.NewLoggerWithRegistry("cmd-plan")

	runMotionPlanServer := flag.Bool("mp", false, "run the motion planning server. does not take a file as input.")
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
	quiet := flag.Bool("quiet", false, "quiet")
	noViz := flag.Bool("no-viz", false, "skip rendering the plan; useful for benchmarking success rate and planning speed")
	chain := flag.Bool("chain", false,
		"plan each request from the configuration the previous plan ended in, rather than from its own recorded start state")
	fsOverrideFile := flag.String("fs-override", "",
		"json file of frame system overrides to apply to every request, e.g. tighter DoF limits. See override.go for the schema")

	flag.Parse()

	if *runMotionPlanServer {
		return mpserver.RunServer()
	}

	files := flag.Args()
	if len(files) == 0 {
		return errors.New("need at least one json file")
	}

	// Requests are read one step at a time, because a single one carrying meshes is big enough
	// that holding a whole sequence in memory is worth avoiding. Stat them all up front so a typo
	// in the last path doesn't surface only after minutes spent planning the ones before it.
	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			return err
		}
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

		if len(files) > 1 {
			logger.Info("-cpu: skipping the repeated re-plan loop, the sequence itself is what gets profiled")
		}
	}

	// Persist learned roadmaps across runs: harvested corridors and scene
	// verdicts are what make replans of a hard scene sub-second, and replaying
	// captures cold defeats them otherwise. Respect an explicit setting;
	// export MOTION_ROADMAP_CACHE_DIR="" to disable.
	if _, ok := os.LookupEnv("MOTION_ROADMAP_CACHE_DIR"); !ok {
		if cacheDir, err := os.UserCacheDir(); err == nil {
			utils.UncheckedError(os.Setenv("MOTION_ROADMAP_CACHE_DIR", filepath.Join(cacheDir, "viam-motion-roadmap")))
		}
	}

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

	var override *frameSystemOverride
	if *fsOverrideFile != "" {
		var err error
		if override, err = readFrameSystemOverride(*fsOverrideFile); err != nil {
			return err
		}
	}

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

	mpLogger := logger
	if *quiet {
		// Suppress logs by using a logger that has no appenders to output to.
		mpLogger = logging.NewBlankLogger("mp")
	}

	r := &runner{
		opts: &runOptions{
			pseudolinearLine:        *pseudolinearLine,
			pseudolinearOrientation: *pseudolinearOrientation,
			seed:                    *seed,
			loop:                    *loop,
			cpu:                     *cpu,
			interactive:             *interactive,
			host:                    *host,
			forceMotion:             *forceMotion,
			showPoses:               *showPoses,
			tryManySeeds:            *tryManySeeds,
			noViz:                   *noViz,
			chain:                   *chain,
		},
		override: override,
		logger:   logger,
		mpLogger: mpLogger,
		mylog:    log.New(os.Stdout, "", 0),
		start:    time.Now(),
		single:   len(files) == 1,
	}

	if !r.single {
		mode := "as recorded"
		if r.opts.chain {
			mode = "chained, each plan starting where the previous one ended"
		}
		logger.Infof("replaying %d plan requests %s", len(files), mode)
	}

	var results []*stepResult

	// carried is the configuration the previous plan ended in. Only ever populated in chained mode.
	var carried referenceframe.FrameSystemInputs

	for i, file := range files {
		req, origPlan, err := r.loadRequest(file)
		if err != nil {
			return err
		}

		if carried != nil {
			if err := chainStartState(req, carried, logger); err != nil {
				return err
			}
		}

		res, err := r.planOne(ctx, &step{idx: i, total: len(files), file: file, req: req, origPlan: origPlan})
		if err != nil {
			return err
		}
		results = append(results, res)

		if res.plan == nil {
			// Interactive mode consumed a planning failure. There is no end state to chain from,
			// and replaying the rest of a sequence past a failure isn't what the user asked for.
			break
		}

		if r.opts.chain {
			if res.meta != nil && res.meta.PartialError != nil {
				logger.Warnf("plan %d/%d is partial (%v); the next request starts from where it stopped",
					i+1, len(files), res.meta.PartialError)
			}
			if carried = endConfiguration(res.plan); carried == nil {
				return fmt.Errorf("cannot chain from %s: its plan has an empty trajectory", file)
			}
		}
	}

	if err := trace.Shutdown(ctx); err != nil {
		logger.Errorw("Got error while shutting down tracing", "err", err)
	}
	metricsExporter.Stop()

	if !r.single {
		printSummary(r.mylog, results, time.Since(r.start))
	}

	if *waypointsFile != "" {
		plans := make([]motionplan.Plan, 0, len(results))
		for _, res := range results {
			if res.plan != nil {
				plans = append(plans, res.plan)
			}
		}
		if len(plans) == 0 {
			logger.Warnf("not writing %s, the run produced no plans", *waypointsFile)
		} else if err := writeWaypointsToFile(ctx, plans, *waypointsFile); err != nil {
			return err
		}
	}

	return nil
}

// loadRequest reads one request off disk and applies everything that is a property of the run
// rather than of the recorded request: the pseudolinear constraints, the seed and the frame system
// override.
func (r *runner) loadRequest(file string) (*armplanning.PlanRequest, motionplan.Plan, error) {
	r.logger.Infof("reading plan from %s", file)
	req, origPlan, err := armplanning.ReadRequestAndResponseFromFile(file)
	if err != nil {
		return nil, nil, err
	}

	if r.opts.pseudolinearLine > 0 || r.opts.pseudolinearOrientation > 0 {
		req.Constraints.AddPseudolinearConstraint(
			motionplan.PseudolinearConstraint{r.opts.pseudolinearLine, r.opts.pseudolinearOrientation})
	}

	if r.opts.seed >= 0 {
		req.PlannerOptions.RandomSeed = r.opts.seed
	}

	if r.override != nil {
		if err := r.override.apply(req.FrameSystem, r.logger); err != nil {
			return nil, nil, fmt.Errorf("applying %s to %s: %w", r.override.source, file, err)
		}
	}

	// Must run after the override, because the seed cache is built from the frame system's limits.
	if err := armplanning.PrepSmartSeed(req.FrameSystem, r.logger); err != nil {
		return nil, nil, err
	}

	return req, origPlan, nil
}

// planOne plans a single request, reports on it, and renders and executes it if asked to.
func (r *runner) planOne(ctx context.Context, s *step) (*stepResult, error) {
	if !r.single {
		r.mylog.Printf("\n===== plan %d/%d: %s =====", s.idx+1, s.total, s.file)
	}

	req := s.req
	r.logger.Infof("starting motion planning for %d goals", len(req.Goals))

	planStart := time.Now()
	plan, meta, planErr := armplanning.PlanMotion(ctx, r.mpLogger, req)
	res := &stepResult{
		file:     s.file,
		goals:    len(req.Goals),
		plan:     plan,
		meta:     meta,
		duration: time.Since(planStart),
	}

	if planErr == nil {
		var err error
		if res.totalCartesion, res.totalL2, err = planDistances(plan); err != nil {
			return nil, err
		}
	}

	if r.opts.interactive {
		if err := doInteractive(req, plan, planErr, r.mylog, r.opts.showPoses); err != nil {
			r.logger.Fatal("Interactive mode failed:", err)
		}
		if planErr != nil {
			// The user has already inspected the failure interactively, so exit cleanly rather
			// than surfacing the planning error a second time.
			res.plan = nil
		}
		return res, nil
	}

	if planErr != nil {
		if plan != nil {
			r.mylog.Printf("error but partial result of length: %d", len(plan.Trajectory()))
		}
		return nil, planErr
	}

	if len(plan.Path()) != len(plan.Trajectory()) {
		return nil, fmt.Errorf("path and trajectory not the same %d vs %d", len(plan.Path()), len(plan.Trajectory()))
	}

	// Keep re-planning under the profiler so a single request yields a sample big enough to read.
	// A sequence is its own workload and needs no padding.
	for r.opts.cpu != "" && r.single && time.Since(r.start) < (45*time.Second) {
		ss := time.Now()
		if _, _, err := armplanning.PlanMotion(ctx, r.mpLogger, req); err != nil {
			return nil, err
		}
		r.mylog.Printf("extra plan took %v", time.Since(ss))
	}

	if r.opts.tryManySeeds > 1 {
		if err := r.reportManySeeds(ctx, req); err != nil {
			return nil, err
		}
	}

	r.reportPlan(req, plan, meta, s.origPlan, res)

	for i := 0; !r.opts.noViz && i < r.opts.loop; i++ {
		if err := visualize(req, plan, r.mylog, r.opts.showPoses); err != nil {
			r.mylog.Println("Couldn't visualize motion plan. Motion-tools server is probably not running. Skipping. Err:", err)
			break
		}
	}

	if r.opts.host != "" {
		if err := executeOnArm(ctx, r.opts.host, plan, r.opts.forceMotion, r.logger); err != nil {
			return nil, err
		}
	}

	return res, nil
}

// reportManySeeds re-plans the request under a range of random seeds and reports the spread of
// joint space distances, to show how much of a plan's quality is luck.
func (r *runner) reportManySeeds(ctx context.Context, req *armplanning.PlanRequest) error {
	minDistance := 10000.0
	maxDistance := 0.0

	for i := 1; i < r.opts.tryManySeeds; i++ {
		req.PlannerOptions.RandomSeed = i
		seedPlan, _, err := armplanning.PlanMotion(ctx, r.logger, req)
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

		r.mylog.Printf("tryManySeeds seed %4d: traj_len=%d l2=%0.4f", i, len(t), seedTotalL2)
	}
	r.mylog.Printf("tryManySeeds result min: %0.2f max:%0.2f", minDistance, maxDistance)

	return nil
}

// planDistances is how far a plan travels in cartesian space and in joint space, summed over every
// moving frame. Kept separate from reportPlan so that modes which skip the report, interactive
// mode in particular, still have real numbers to put in the sequence summary.
func planDistances(plan motionplan.Plan) (totalCartesion, totalL2 float64, err error) {
	for idx, p := range plan.Path() {
		t := plan.Trajectory()[idx]
		if len(p) != len(t) {
			return 0, 0, fmt.Errorf("p and t are different sizes %d vs %d", len(p), len(t))
		}
		if idx == 0 {
			continue
		}

		// Keyed off the path rather than the trajectory so that a frame without a pose can never
		// be dereferenced, and skipping unnamed frames the way the step report does.
		for c, pose := range p {
			if c == "" || len(t[c]) == 0 {
				continue
			}
			totalL2 += referenceframe.InputsL2Distance(plan.Trajectory()[idx-1][c], t[c])
			totalCartesion += pose.Pose().Point().Distance(plan.Path()[idx-1][c].Pose().Point())
		}
	}

	return totalCartesion, totalL2, nil
}

// reportPlan prints the step-by-step breakdown of a plan.
func (r *runner) reportPlan(
	req *armplanning.PlanRequest,
	plan motionplan.Plan,
	meta *armplanning.PlanMeta,
	origPlan motionplan.Plan,
	res *stepResult,
) {
	mylog := r.mylog

	relevantParts := []string{}
	for c := range plan.Path()[0] {
		if len(c) == 0 {
			continue
		}
		relevantParts = append(relevantParts, c)
	}
	sort.Strings(relevantParts)

	if origPlan != nil {
		mylog.Printf("Original plan length: %v Current plan length: %v\n",
			len(origPlan.Trajectory()), len(plan.Trajectory()))
	}
	for idx, p := range plan.Path() {
		mylog.Printf("step %d", idx)

		t := plan.Trajectory()[idx]

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

				mylog.Printf("\t\t\t\t distances l2: %0.4f Linf %0.4f cartesion: %0.2f",
					referenceframe.InputsL2Distance(p, t[c]),
					referenceframe.InputsLinfDistance(p, t[c]),
					pp.Pose().Point().Distance(plan.Path()[idx-1][c].Pose().Point()))

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
		res.duration.Truncate(time.Millisecond), len(req.Goals), len(plan.Trajectory()))
	mylog.Printf("totalCartesion: %0.4f\n", res.totalCartesion)
	mylog.Printf("totalL2: %0.4f\n", res.totalL2)

	// Print delta statistics if trajectory has more than 5 points
	if len(plan.Trajectory()) > 5 {
		stats := armplanning.TrajectoryDeltaStats(plan.Trajectory())
		mylog.Printf("\nDelta Statistics (trajectory length: %d):", len(plan.Trajectory()))
		for _, s := range stats {
			mylog.Printf("  %s:%d: avg=%0.5f stddev=%0.5f outside1=%d outside2=%d (n=%d)",
				s.Component, s.JointIdx, s.Mean, s.StdDev, s.Outside1, s.Outside2, s.Count)
		}
	}
}

// printSummary reports one line per replayed request. Only interesting for a sequence, so callers
// skip it when a single request was given.
func printSummary(mylog *log.Logger, results []*stepResult, wallClock time.Duration) {
	mylog.Printf("\n===== sequence summary: %d plan(s) in %v =====", len(results), wallClock.Truncate(time.Millisecond))

	var planning time.Duration
	trajectorySteps := 0
	totalL2, totalCartesion := 0.0, 0.0

	for i, res := range results {
		if res.plan == nil {
			mylog.Printf("  %d %s: no plan", i+1, filepath.Base(res.file))
			continue
		}

		note := ""
		if res.meta != nil && res.meta.PartialError != nil {
			note = " PARTIAL"
		}
		mylog.Printf("  %d %s: %v for %d goals => trajectory length %d, totalL2 %0.4f totalCartesion %0.4f%s",
			i+1, filepath.Base(res.file), res.duration.Truncate(time.Millisecond), res.goals,
			len(res.plan.Trajectory()), res.totalL2, res.totalCartesion, note)

		planning += res.duration
		trajectorySteps += len(res.plan.Trajectory())
		totalL2 += res.totalL2
		totalCartesion += res.totalCartesion
	}

	mylog.Printf("  total: %v planning => trajectory length %d, totalL2 %0.4f totalCartesion %0.4f",
		planning.Truncate(time.Millisecond), trajectorySteps, totalL2, totalCartesion)
}

// endConfiguration is the configuration a plan leaves the robot in, or nil if the plan is empty.
func endConfiguration(plan motionplan.Plan) referenceframe.FrameSystemInputs {
	traj := plan.Trajectory()
	if len(traj) == 0 {
		return nil
	}
	return traj[len(traj)-1]
}

// chainStartState replaces req's recorded start state with the configuration the previous plan
// ended in, which is what -chain does.
//
// Only the configuration carries over. Each request keeps its own frame system, obstacles and
// goals, so a chained replay answers "could the robot have run these plans back to back" rather
// than re-planning the whole sequence as one motion. Frames the previous plan didn't cover keep
// whatever the request recorded for them, and every such divergence is logged: a chain that
// silently starts somewhere other than where the last one stopped is worse than no chain at all.
func chainStartState(req *armplanning.PlanRequest, prevEnd referenceframe.FrameSystemInputs, logger logging.Logger) error {
	merged := referenceframe.FrameSystemInputs{}
	for name, inputs := range req.StartState.Configuration() {
		merged[name] = slices.Clone(inputs)
	}

	for _, name := range sortedFrameNames(prevEnd) {
		inputs := prevEnd[name]
		if len(inputs) == 0 {
			// A frame that cannot move has nothing to carry over.
			continue
		}

		frame := req.FrameSystem.Frame(name)
		if frame == nil {
			logger.Warnf("chain: dropping the end configuration of frame %q, absent from the next request's frame system", name)
			continue
		}
		if len(frame.DoF()) != len(inputs) {
			return fmt.Errorf("chain: frame %q ended the previous plan with %d inputs but takes %d",
				name, len(inputs), len(frame.DoF()))
		}

		for i, limit := range frame.DoF() {
			if inputs[i] < limit.Min || inputs[i] > limit.Max {
				// Tightening a limit with -fs-override, or two requests captured against
				// different limits, can leave the robot parked outside the bounds it lands in.
				// The planner can still move it back inside, so say so and carry on.
				logger.Warnf("chain: frame %q joint %d carries over as %0.4f, outside its limit [%0.4f, %0.4f]",
					name, i, inputs[i], limit.Min, limit.Max)
			}
		}

		merged[name] = slices.Clone(inputs)
	}

	for _, name := range sortedFrameNames(merged) {
		if len(merged[name]) == 0 {
			continue
		}
		if _, carried := prevEnd[name]; !carried {
			logger.Warnf("chain: frame %q keeps the start configuration recorded in the request, "+
				"the previous plan did not cover it", name)
		}
	}

	req.StartState = armplanning.NewPlanState(nil, merged)

	return nil
}

// sortedFrameNames keeps the warnings emitted while walking a configuration in a stable order.
func sortedFrameNames(inputs referenceframe.FrameSystemInputs) []string {
	names := make([]string, 0, len(inputs))
	for name := range inputs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func visualize(req *armplanning.PlanRequest, plan motionplan.Plan, mylog *log.Logger, showPoses bool) error {
	renderFramePeriod := 5 * time.Millisecond
	if _, err := viz.RemoveAll(); err != nil {
		return err
	}

	startInputs := req.StartState.Configuration()
	// `DrawWorldState` just draws the obstacles. I think the FrameSystem/Path are necessary
	// because obstacles can be in terms of reference frames contained within the frame
	// system. Such as a camera attached to an arm.
	if _, err := viz.DrawWorldState(viz.DrawWorldStateOptions{
		WorldState:  req.GetWorldState(),
		FrameSystem: req.FrameSystem,
		Inputs:      startInputs,
	}); err != nil {
		return err
	}

	// `DrawFrameSystem` draws everything else we're interested in.
	if _, err := viz.DrawFrameSystem(viz.DrawFrameSystemOptions{
		FrameSystem: req.FrameSystem,
		Inputs:      startInputs,
	}); err != nil {
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
				colors := make([]draw.Color, len(shadowGeometries))
				for i := range colors {
					colors[i] = draw.ColorFromName(shadowColor)
				}
				if _, err := viz.DrawGeometriesInFrame(viz.DrawGeometriesInFrameOptions{
					Geometries: shadowGIF,
					Colors:     colors,
				}); err != nil {
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
				if _, err := viz.DrawFrameSystem(viz.DrawFrameSystemOptions{
					FrameSystem: req.FrameSystem,
					Inputs:      mp.ToFrameSystemInputs(),
				}); err != nil {
					return err
				}

				time.Sleep(renderFramePeriod)
			}
		}

		if _, err := viz.DrawFrameSystem(viz.DrawFrameSystemOptions{
			FrameSystem: req.FrameSystem,
			Inputs:      plan.Trajectory()[idx],
		}); err != nil {
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

	if _, err := viz.DrawPosesAsArrows(viz.DrawPosesAsArrowsOptions{
		Poses:  goalPoses,
		Colors: []draw.Color{draw.ColorFromName("blue")},
	}); err != nil {
		return err
	}

	return nil
}

func doInteractive(req *armplanning.PlanRequest, plan motionplan.Plan, planErr error, logger *log.Logger, showPoses bool) error {
	var ikErr *armplanning.IkConstraintError
	errors.As(planErr, &ikErr)
	if _, err := viz.RemoveAll(); err != nil {
		return err
	}

	if _, err := viz.DrawWorldState(viz.DrawWorldStateOptions{
		WorldState:  req.GetWorldState(),
		FrameSystem: req.FrameSystem,
		Inputs:      req.StartState.Configuration(),
	}); err != nil {
		return err
	}

	if _, err := viz.DrawFrameSystem(viz.DrawFrameSystemOptions{
		FrameSystem: req.FrameSystem,
		Inputs:      req.StartState.Configuration(),
	}); err != nil {
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

		//nolint: forbidigo
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
			logger.Println("sg, show goals")
			logger.Println("-  show the goals in viz tool")
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
		case cmd == "show goals" || cmd == "sg":
			for gi, goalPlanState := range req.Goals {
				poses, err := goalPlanState.ComputePoses(context.Background(), req.FrameSystem)
				if err != nil {
					return err
				}
				for pi, poseValue := range poses {
					poseInWorldFrame := poseValue.Transform(
						referenceframe.NewPoseInFrame(
							req.FrameSystem.World().Name(),
							spatialmath.NewZeroPose())).(*referenceframe.PoseInFrame)
					sphere, err := spatialmath.NewSphere(poseInWorldFrame.Pose(), 10, fmt.Sprintf("goal-%d-%v", gi, pi))
					if err != nil {
						return err
					}
					if _, err := viz.DrawGeometry(viz.DrawGeometryOptions{
						Geometry: sphere,
						Color:    draw.ColorFromName("blue"),
					}); err != nil {
						return err
					}
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
					if _, err := viz.DrawFrameSystem(viz.DrawFrameSystemOptions{
						FrameSystem: req.FrameSystem,
						Inputs:      configuration.ToFrameSystemInputs(),
					}); err != nil {
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

// writeWaypointsToFile writes the waypoints of every plan in the run, concatenated in the order
// they were planned. The seam between two plans is left exactly as planned, so a chained sequence
// repeats a position there while an as-is replay carries a jump wherever one plan's end doesn't
// meet the next one's start.
func writeWaypointsToFile(ctx context.Context, plans []motionplan.Plan, fileName string) error {
	ff := &waypointsFileFormat{}
	moving := ""

	for _, plan := range plans {
		byComponent := getFullTrajectoryByComponent(plan)
		if len(byComponent) != 1 {
			return fmt.Errorf("to output waypointsFile need exactly one component moving, not %d", len(byComponent))
		}

		for cName, v := range byComponent {
			if moving != "" && cName != moving {
				return fmt.Errorf("to output waypointsFile the same component must move in every plan, got both %s and %s",
					moving, cName)
			}
			moving = cName
			ff.Waypoints = append(ff.Waypoints, v...)
		}
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
