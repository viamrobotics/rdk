package servo

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	benchDt     = 0.02
	benchGain   = 2.0
	benchWindow = 100 // 2 s at 50 Hz: testLimits caps a joint at 1 rad/s, so a 30° sweep needs over 0.5 s
)

// benchScale shrinks a model/path's relative XYZ where the arm's reach demands it (default 1); recorded in the
// results doc. The SO-101 reaches about 300 mm, so the policy path's ~170 mm min-jerk moves would be mostly
// out of reach.
var benchScale = map[string]float64{"so101/policy_aligned": 0.5}

type benchResult struct {
	Model, Path, Solver          string
	TickP50, TickP99, TickMax    time.Duration
	RMSmm, MaxMM                 float64
	LimitedTicks, BranchChanges  int
	ClampRMSRad, ClampMaxRad     float64 // ‖raw dq − applied step‖ on limited ticks
	RawStepMaxRad                float64 // largest raw solver dq component before limiting
	EvalsP50, EvalsP99, Failures int
	Overruns                     int // ticks whose solve exceeded the tick period (benchDt)

	// Rotation error of the arm against the held target, each tick before the step: rotVec(target, FK) for
	// TaskPose, the tool-axis angle for TaskPointing.
	RotRMSDeg, RotMaxDeg float64
	// Distance from FK(q + raw dq) to that tick's tickGoal pose, before Limits.Apply: how well the solver hits
	// the per-tick goal it was given, independent of controller lag. Rotation as for Rot*.
	GoalResidualRMSmm, GoalResidualMaxMM, GoalResidualRMSDeg, GoalResidualMaxDeg float64
	// The same after Limits.Apply (FK(q + applied step)): limiting's effect on hitting the goal.
	GoalAppliedRMSmm, GoalAppliedMaxMM, GoalAppliedRMSDeg, GoalAppliedMaxDeg float64
}

// rotErrDeg is the rotation error between pose a and target b: the full rotation for TaskPose, the tool-axis
// angle for TaskPointing.
func rotErrDeg(task Task, a, b spatialmath.Pose) float64 {
	aq, bq := a.Orientation().Quaternion(), b.Orientation().Quaternion()
	if task == TaskPointing {
		r, _, _ := pointingError(toolAxis(aq), toolAxis(bq))
		return r.Norm() * 180 / math.Pi
	}
	return rotVec(bq, aq).Norm() * 180 / math.Pi
}

// acc accumulates an RMS and a max.
type acc struct{ sq, max float64 }

func (a *acc) add(v float64)     { a.sq += v * v; a.max = math.Max(a.max, v) }
func (a *acc) rms(n int) float64 { return math.Sqrt(a.sq / float64(n)) }

// benchPath is a target sequence: anchor-relative XYZ (mm, orientation held) unless Abs is set.
type benchPath struct {
	Name string
	T    []float64
	XYZ  [][3]float64
	Abs  []spatialmath.Pose
	// Start overrides the anchor as the arm's initial joints (the first Abs target is FK of Start).
	Start []float64
}

func (p benchPath) len() int {
	if p.Abs != nil {
		return len(p.Abs)
	}
	return len(p.XYZ)
}

func loadPaths(t testing.TB) []benchPath {
	t.Helper()
	files, err := filepath.Glob("testdata/paths/*.json")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(files), test.ShouldEqual, 5)
	var out []benchPath
	for _, f := range files {
		b, err := os.ReadFile(f)
		test.That(t, err, test.ShouldBeNil)
		var p struct {
			Name  string       `json:"name"`
			T     []float64    `json:"t"`
			XYZmm [][3]float64 `json:"xyz_mm"`
		}
		test.That(t, json.Unmarshal(b, &p), test.ShouldBeNil)
		out = append(out, benchPath{Name: p.Name, T: p.T, XYZ: p.XYZmm})
	}
	return out
}

func benchAnchor(m referenceframe.Model, model string) []float64 {
	switch model {
	case "ur5e":
		return []float64{0, -1.2, 1.2, -1.5, -1.57, 0}
	case "xarm6":
		return []float64{0, -0.3, -0.6, 0, 0.9, 0}
	}
	q := make([]float64, len(m.DoF()))
	for i, l := range m.DoF() {
		q[i] = (l.Min + l.Max) / 2
	}
	return q
}

func benchTask(model string) Task {
	if model == "so101" {
		return TaskPointing
	}
	return TaskPose
}

func benchPose(t testing.TB, m referenceframe.Model, q []float64) spatialmath.Pose {
	t.Helper()
	p, err := m.Transform(q)
	test.That(t, err, test.ShouldBeNil)
	return p
}

// adversarialPaths builds the two per-model paths from its kinematics.
func adversarialPaths(t testing.TB, model string) []benchPath {
	t.Helper()
	m := loadModel(t, model)
	anchor := benchAnchor(m, model)
	home := benchPose(t, m, anchor)

	// singular_wrist: the wrist joint through zero to the opposite sign over 4 s.
	j := 4
	if model == "so101" {
		j = 3 // wrist flex
	}
	lim := m.DoF()[j]
	from := append([]float64(nil), anchor...)
	if model == "so101" {
		from[j] = 0.8 // the SO-101 midpoint has the flex at ~0 already, so start the sweep 0.8 rad to one side
	}
	end := math.Min(math.Max(-from[j], lim.Min), lim.Max)
	sw := benchPath{Name: "singular_wrist", Start: from}
	for i := 0; i <= 200; i++ {
		s := float64(i) / 200
		q := append([]float64(nil), from...)
		q[j] = from[j] + (end-from[j])*s
		sw.T = append(sw.T, 4*s)
		sw.Abs = append(sw.Abs, benchPose(t, m, q))
	}

	// past_limits: +x at 50 mm/s (1 mm per sample) until DLS can't reach within 1 mm, 50 mm further, hold 2 s.
	s, l, task := newDLSSolver(), testLimits(t, m), benchTask(model)
	q, last := append([]float64(nil), anchor...), make([]float64, len(anchor))
	at := func(x float64) spatialmath.Pose {
		return spatialmath.NewPose(home.Point().Add(r3.Vector{X: x}), home.Orientation())
	}
	reach := func(x float64) bool {
		tgt := at(x)
		for k := 0; k < 400; k++ {
			dq, _, err := s.Step(StepInput{Model: m, Q: q, Target: tgt, Gain: benchGain, Dt: benchDt, Task: task, Limits: l})
			test.That(t, err, test.ShouldBeNil)
			step, _ := l.Apply(q, dq, last, benchDt)
			for i := range q {
				q[i] += step[i]
			}
			last = step
			if benchPose(t, m, q).Point().Distance(tgt.Point()) < 0.01 {
				break
			}
		}
		return benchPose(t, m, q).Point().Distance(tgt.Point()) < 1
	}
	var xFail float64
	for x := 1.0; ; x++ {
		test.That(t, x, test.ShouldBeLessThan, 3000) // never failed: the anchor is wrong
		if !reach(x) {
			xFail = x
			break
		}
	}
	pl := benchPath{Name: "past_limits"}
	for i, x := 0, 0.0; x <= xFail+50; i, x = i+1, x+1 {
		pl.T = append(pl.T, float64(i)*benchDt)
		pl.Abs = append(pl.Abs, at(x))
	}
	end0 := pl.T[len(pl.T)-1]
	for i := 1; i <= 100; i++ {
		pl.T = append(pl.T, end0+float64(i)*benchDt)
		pl.Abs = append(pl.Abs, pl.Abs[len(pl.Abs)-1])
	}
	return []benchPath{sw, pl}
}

// countBranchChanges counts windows of benchWindow ticks where some joint changes by more than 30° (endpoint to
// endpoint) while the target moves less than 10 mm and 10°. Both solvers are local, so a branch change shows up
// as a sustained joint sweep under a nearly stationary target. Windows don't overlap (after a hit, resume past
// the window), so this is a lower bound on events: one flip longer than a window can count more than once.
func countBranchChanges(qs [][]float64, tgts []spatialmath.Pose) int {
	n := 0
	for i := 0; i+benchWindow < len(qs); i++ {
		var moved, mm, rad float64
		for j := range qs[i] {
			moved = math.Max(moved, math.Abs(qs[i+benchWindow][j]-qs[i][j]))
		}
		for k := i; k <= i+benchWindow; k++ {
			mm = math.Max(mm, tgts[k].Point().Distance(tgts[i].Point()))
			rad = math.Max(rad, rotVec(tgts[k].Orientation().Quaternion(), tgts[i].Orientation().Quaternion()).Norm())
		}
		if moved > 30*math.Pi/180 && mm < 10 && rad < 10*math.Pi/180 {
			n++
			i += benchWindow
		}
	}
	return n
}

func TestCountBranchChanges(t *testing.T) {
	// A joint sweeping deg degrees linearly over sweep ticks, then holding, for total ticks; the target
	// moves targetMM over the same ticks.
	count := func(deg float64, sweep, total int, targetMM float64) int {
		var qs [][]float64
		var tgts []spatialmath.Pose
		for i := 0; i < total; i++ {
			f := math.Min(float64(i)/float64(sweep), 1)
			qs = append(qs, []float64{f * deg * math.Pi / 180})
			tgts = append(tgts, spatialmath.NewPoseFromPoint(r3.Vector{X: f * targetMM}))
		}
		return countBranchChanges(qs, tgts)
	}
	test.That(t, count(60, 75, 150, 0), test.ShouldEqual, 1)   // 60° over 1.5 s
	test.That(t, count(60, 75, 150, 50), test.ShouldEqual, 0)  // the target moved too
	test.That(t, count(60, 150, 300, 0), test.ShouldEqual, 1)  // 3 s sweep: one window hit, the next sees 19°
	test.That(t, count(180, 300, 350, 0), test.ShouldEqual, 3) // a 6 s flip counts once per window
	test.That(t, count(60, 100, 101, 0), test.ShouldEqual, 1)  // sweep ends at the last sample
	test.That(t, count(60, 99, 100, 0), test.ShouldEqual, 0)   // one sample short of a window
}

func pctile[T int | time.Duration](v []T, p float64) T {
	if len(v) == 0 {
		var z T
		return z
	}
	s := append([]T(nil), v...)
	sort.Slice(s, func(a, b int) bool { return s[a] < s[b] })
	return s[int(float64(len(s)-1)*p)]
}

// runPath steps the solver through Limits.Apply against a perfect arm at 50 Hz, holding the latest target
// between path samples with zero feed-forward.
func runPath(t testing.TB, model string, path benchPath, s Solver) benchResult {
	t.Helper()
	m := loadModel(t, model)
	l, task := testLimits(t, m), benchTask(model)
	anchor := benchAnchor(m, model)
	home := benchPose(t, m, anchor)
	sc := 1.0
	if v, ok := benchScale[model+"/"+path.Name]; ok {
		sc = v
	}
	target := func(i int) spatialmath.Pose {
		if path.Abs != nil {
			return path.Abs[i]
		}
		d := path.XYZ[i]
		return spatialmath.NewPose(home.Point().Add(r3.Vector{X: d[0] * sc, Y: d[1] * sc, Z: d[2] * sc}), home.Orientation())
	}
	res := benchResult{Model: model, Path: path.Name, Solver: s.Name()}
	q, last := append([]float64(nil), anchor...), make([]float64, len(anchor))
	if path.Start != nil {
		q = append([]float64(nil), path.Start...)
	}
	var ticks []time.Duration
	var evals []int
	var sumSq, clampSq float64
	var rot, gRawMM, gRawDeg, gAppMM, gAppDeg acc
	var qs [][]float64
	var tgts []spatialmath.Pose
	idx, t0 := 0, path.T[0]
	n := int(math.Round((path.T[len(path.T)-1]-t0)/benchDt)) + 1
	for k := 0; k < n; k++ {
		now := t0 + float64(k)*benchDt + 1e-9
		for idx+1 < path.len() && path.T[idx+1] <= now {
			idx++
		}
		tgt := target(idx)
		qs, tgts = append(qs, append([]float64(nil), q...)), append(tgts, tgt)
		e := benchPose(t, m, q).Point().Distance(tgt.Point())
		sumSq += e * e
		res.MaxMM = math.Max(res.MaxMM, e)
		rot.add(rotErrDeg(task, benchPose(t, m, q), tgt))
		cp, cq, ferr := FK(m, q)
		test.That(t, ferr, test.ShouldBeNil)
		in := StepInput{Model: m, Q: q, Target: tgt, Gain: benchGain, Dt: benchDt, Task: task, Limits: l}
		goal, _ := tickGoal(in, cp, cq)

		start := time.Now()
		dq, info, err := s.Step(in)
		el := time.Since(start)
		ticks, evals = append(ticks, el), append(evals, info.Evals)
		if el > time.Duration(benchDt*float64(time.Second)) {
			res.Overruns++
		}
		if err != nil {
			res.Failures++
			dq = make([]float64, len(q))
		}
		for _, d := range dq {
			res.RawStepMaxRad = math.Max(res.RawStepMaxRad, math.Abs(d))
		}
		step, by := l.Apply(q, dq, last, benchDt)
		if by != 0 {
			res.LimitedTicks++
			var c float64
			for i := range dq {
				c += (dq[i] - step[i]) * (dq[i] - step[i])
			}
			clampSq += c
			res.ClampMaxRad = math.Max(res.ClampMaxRad, math.Sqrt(c))
		}
		resid := func(d []float64) (mm, deg float64) {
			qn := make([]float64, len(q))
			for i := range q {
				qn[i] = q[i] + d[i]
			}
			p := benchPose(t, m, qn)
			return p.Point().Distance(goal.Point()), rotErrDeg(task, p, goal)
		}
		mm, deg := resid(dq)
		gRawMM.add(mm)
		gRawDeg.add(deg)
		mm, deg = resid(step)
		gAppMM.add(mm)
		gAppDeg.add(deg)
		for i := range q {
			q[i] += step[i]
		}
		last = step
	}
	res.RotRMSDeg, res.RotMaxDeg = rot.rms(n), rot.max
	res.GoalResidualRMSmm, res.GoalResidualMaxMM = gRawMM.rms(n), gRawMM.max
	res.GoalResidualRMSDeg, res.GoalResidualMaxDeg = gRawDeg.rms(n), gRawDeg.max
	res.GoalAppliedRMSmm, res.GoalAppliedMaxMM = gAppMM.rms(n), gAppMM.max
	res.GoalAppliedRMSDeg, res.GoalAppliedMaxDeg = gAppDeg.rms(n), gAppDeg.max
	res.RMSmm = math.Sqrt(sumSq / float64(n))
	if res.LimitedTicks > 0 {
		res.ClampRMSRad = math.Sqrt(clampSq / float64(res.LimitedTicks))
	}
	res.TickP50, res.TickP99, res.TickMax = pctile(ticks, 0.5), pctile(ticks, 0.99), pctile(ticks, 1)
	res.EvalsP50, res.EvalsP99 = pctile(evals, 0.5), pctile(evals, 0.99)
	res.BranchChanges = countBranchChanges(qs, tgts)
	return res
}

func TestSolverBench(t *testing.T) {
	if testing.Short() || os.Getenv("SERVO_BENCH") == "" {
		t.Skip("benchmark: set SERVO_BENCH=1")
	}
	var results []benchResult
	for _, model := range []string{"ur5e", "xarm6", "so101"} {
		paths := append(loadPaths(t), adversarialPaths(t, model)...)
		solvers := solversUnderTest(t)
		var names []string
		for n := range solvers {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, path := range paths {
			for _, name := range names {
				runtime.GC() // keep harness allocations out of the timed ticks
				r := runPath(t, model, path, solvers[name])
				t.Logf("%-6s %-15s %-9s p99=%-9v rms=%6.2f max=%7.2f lim=%4d br=%d evalsP99=%d fail=%d over=%d",
					r.Model, r.Path, r.Solver, r.TickP99, r.RMSmm, r.MaxMM, r.LimitedTicks, r.BranchChanges, r.EvalsP99, r.Failures, r.Overruns)
				results = append(results, r)
				// Sanity only: catch regressions, not decide the comparison.
				test.That(t, r.RMSmm, test.ShouldBeLessThan, 50)
				test.That(t, r.TickP99, test.ShouldBeLessThan, 50*time.Millisecond)
			}
		}
	}
	if out := os.Getenv("SERVO_BENCH_OUT"); out != "" {
		b, err := json.MarshalIndent(results, "", "  ")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, os.WriteFile(out, b, 0o644), test.ShouldBeNil)
	}
}

func BenchmarkSolverStep(b *testing.B) {
	m := loadModel(b, "ur5e")
	l := testLimits(b, m)
	anchor := benchAnchor(m, "ur5e")
	home := benchPose(b, m, anchor)
	var circle benchPath
	for _, p := range loadPaths(b) {
		if p.Name == "circle_50" {
			circle = p
		}
	}
	if circle.XYZ == nil {
		b.Fatal("circle_50 path missing")
	}
	for name, s := range solversUnderTest(b) {
		b.Run(name, func(b *testing.B) {
			q, last := append([]float64(nil), anchor...), make([]float64, len(anchor))
			for i := 0; i < b.N; i++ {
				d := circle.XYZ[i%len(circle.XYZ)]
				tgt := spatialmath.NewPose(home.Point().Add(r3.Vector{X: d[0], Y: d[1], Z: d[2]}), home.Orientation())
				dq, _, err := s.Step(StepInput{Model: m, Q: q, Target: tgt, Gain: benchGain, Dt: benchDt, Task: TaskPose, Limits: l})
				if err != nil {
					b.Fatal(err)
				}
				step, _ := l.Apply(q, dq, last, benchDt)
				for j := range q {
					q[j] += step[j]
				}
				last = step
			}
		})
	}
}
