package orderbench

import (
	"fmt"
	"sort"
	"strings"
)

// DefaultRegressionThreshold is the per-plan slowdown ratio at which a plan is listed in the
// report. This is a reporting threshold, not a gate: replaying the same build twice puts one or two
// of 98 plans past it, so failing a build on it would be pure flake.
const DefaultRegressionThreshold = 1.25

// DefaultGateThreshold is the per-plan slowdown that actually fails a build. It sits well clear of
// run-to-run noise, and comfortably below the multiples that real regressions have produced -- the
// #6378 blow-up moved seven plans by 6.1x.
const DefaultGateThreshold = 2.0

// DefaultTotalThreshold is the slowdown ratio for the order as a whole, which does gate. It can be
// far tighter than the per-plan number because summing 98 plans averages the jitter away: repeat
// runs of one build land within about 2% of each other.
const DefaultTotalThreshold = 1.10

// minReportedMS keeps ratios of trivially short plans out of the regression list. A plan that goes
// from 3ms to 9ms is a 3x that means nothing.
const minReportedMS = 50.0

// minGatedDeltaMS is the wall time a plan must actually add before its ratio can fail a build.
// Without it, a change that puts a flat per-plan cost on scene preprocessing reads as a fleet of
// 7x regressions on the cheapest plans in the order -- true as ratios, and nearly irrelevant to how
// long the order takes.
const minGatedDeltaMS = 250.0

// maxTableRows caps how many plans each table lists. A pull request comment that runs to a hundred
// rows does not get read.
const maxTableRows = 12

// PlanStat is one plan's result reduced across repeat passes.
type PlanStat struct {
	Index         int     `json:"index"`
	Name          string  `json:"name"`
	Step          string  `json:"step"`
	Motion        string  `json:"motion"`
	Passes        int     `json:"passes"`
	PlanMS        float64 `json:"plan_ms"`
	TrajectoryLen int     `json:"trajectory_len"`
	TotalL2       float64 `json:"total_l2"`
	Failures      int     `json:"failures"`
	// RoadmapSolved and CBiRRTSolved are summed across passes; they explain where a latency change
	// came from when the totals move.
	RoadmapSolved int    `json:"roadmap_solved"`
	CBiRRTSolved  int    `json:"cbirrt_solved"`
	FirstErr      string `json:"first_err,omitempty"`
}

// Aggregate summarises one side of a comparison.
type Aggregate struct {
	Plans    int     `json:"plans"`
	TotalMS  float64 `json:"total_ms"`
	Failures int     `json:"failures"`
	TotalL2  float64 `json:"total_l2"`
}

// PlanDelta pairs one plan's result on each side.
type PlanDelta struct {
	Name   string    `json:"name"`
	Index  int       `json:"index"`
	Step   string    `json:"step"`
	Motion string    `json:"motion"`
	Base   *PlanStat `json:"base,omitempty"`
	Head   *PlanStat `json:"head,omitempty"`
	// Ratio is head/base plan time. Zero when either side is missing or the base is zero.
	Ratio float64 `json:"ratio"`
	// DeltaMS is head minus base wall time. Ratios rank what changed most; this ranks what costs
	// most, and the two orderings routinely disagree.
	DeltaMS float64 `json:"delta_ms"`
	// TrajectoryRatio is head/base trajectory length: a planner that buys speed with a worse path
	// should not read as a win.
	TrajectoryRatio float64 `json:"trajectory_ratio"`
	NewFailure      bool    `json:"new_failure"`
	FixedFailure    bool    `json:"fixed_failure"`
}

// Report is a full base-vs-head comparison.
type Report struct {
	BaseLabel string      `json:"base_label"`
	HeadLabel string      `json:"head_label"`
	Base      Aggregate   `json:"base"`
	Head      Aggregate   `json:"head"`
	Deltas    []PlanDelta `json:"deltas"`

	// PlanThreshold is the ratio above which a plan is listed; GateThreshold is the ratio above
	// which it fails the build. Keeping them separate is what lets the report stay informative
	// without the gate becoming flaky.
	PlanThreshold  float64 `json:"plan_threshold"`
	GateThreshold  float64 `json:"gate_threshold"`
	TotalThreshold float64 `json:"total_threshold"`
}

// TotalRatio is head/base wall time for the whole order.
func (r *Report) TotalRatio() float64 {
	if r.Base.TotalMS == 0 {
		return 0
	}
	return r.Head.TotalMS / r.Base.TotalMS
}

// Summarize reduces raw records to one PlanStat per plan, taking the median across repeat passes.
// A median, not a mean: one interfering CI job should not move the answer.
func Summarize(records []Record) map[string]*PlanStat {
	type acc struct {
		stat    *PlanStat
		planMS  []float64
		trajLen []float64
		l2      []float64
	}
	byName := map[string]*acc{}

	for _, rec := range records {
		a, ok := byName[rec.Name]
		if !ok {
			a = &acc{stat: &PlanStat{
				Index:  rec.Index,
				Name:   rec.Name,
				Step:   rec.Step,
				Motion: rec.Motion,
			}}
			byName[rec.Name] = a
		}
		a.stat.Passes++
		a.stat.RoadmapSolved += rec.GoalsRoadmapSolved
		a.stat.CBiRRTSolved += rec.GoalsCBIRRTSolved
		a.planMS = append(a.planMS, rec.PlanMS)
		if rec.Failed() {
			a.stat.Failures++
			if a.stat.FirstErr == "" {
				a.stat.FirstErr = rec.Err
			}
			continue
		}
		a.trajLen = append(a.trajLen, float64(rec.TrajectoryLen))
		a.l2 = append(a.l2, rec.TotalL2)
	}

	out := make(map[string]*PlanStat, len(byName))
	for name, a := range byName {
		a.stat.PlanMS = median(a.planMS)
		a.stat.TrajectoryLen = int(median(a.trajLen))
		a.stat.TotalL2 = median(a.l2)
		out[name] = a.stat
	}
	return out
}

func aggregate(stats map[string]*PlanStat) Aggregate {
	var agg Aggregate
	for _, s := range stats {
		agg.Plans++
		agg.TotalMS += s.PlanMS
		agg.TotalL2 += s.TotalL2
		if s.Failures > 0 {
			agg.Failures++
		}
	}
	return agg
}

// Compare builds a report from two sets of raw records. Records are joined on plan name, which is
// derived from the manifest and so is stable across revisions.
func Compare(base, head []Record, baseLabel, headLabel string) *Report {
	baseStats := Summarize(base)
	headStats := Summarize(head)

	report := &Report{
		BaseLabel:      baseLabel,
		HeadLabel:      headLabel,
		Base:           aggregate(baseStats),
		Head:           aggregate(headStats),
		PlanThreshold:  DefaultRegressionThreshold,
		GateThreshold:  DefaultGateThreshold,
		TotalThreshold: DefaultTotalThreshold,
	}

	names := map[string]bool{}
	for name := range baseStats {
		names[name] = true
	}
	for name := range headStats {
		names[name] = true
	}

	for name := range names {
		b, h := baseStats[name], headStats[name]
		delta := PlanDelta{Name: name, Base: b, Head: h}
		switch {
		case h != nil:
			delta.Index, delta.Step, delta.Motion = h.Index, h.Step, h.Motion
		case b != nil:
			delta.Index, delta.Step, delta.Motion = b.Index, b.Step, b.Motion
		}
		if b != nil && h != nil {
			delta.DeltaMS = h.PlanMS - b.PlanMS
			if b.PlanMS > 0 {
				delta.Ratio = h.PlanMS / b.PlanMS
			}
			if b.TrajectoryLen > 0 && h.TrajectoryLen > 0 {
				delta.TrajectoryRatio = float64(h.TrajectoryLen) / float64(b.TrajectoryLen)
			}
			delta.NewFailure = b.Failures == 0 && h.Failures > 0
			delta.FixedFailure = b.Failures > 0 && h.Failures == 0
		}
		report.Deltas = append(report.Deltas, delta)
	}

	sort.Slice(report.Deltas, func(i, j int) bool { return report.Deltas[i].Index < report.Deltas[j].Index })
	return report
}

// Regressions returns the plans that got materially slower, ordered by the wall time they add.
// Plans too short to time reliably are excluded regardless of their ratio.
func (r *Report) Regressions() []PlanDelta {
	out := r.slowerThan(r.PlanThreshold)
	sort.Slice(out, func(i, j int) bool { return out[i].DeltaMS > out[j].DeltaMS })
	return out
}

// GateFailures returns the plans slow enough to fail a build, worst first. A plan qualifies only if
// it both exceeds the gate ratio and adds real wall time, so a flat per-plan cost spread over the
// cheapest plans in the order cannot by itself fail a build -- it will show up in the order total
// instead, which is where it belongs.
func (r *Report) GateFailures() []PlanDelta {
	var out []PlanDelta
	for _, d := range r.slowerThan(r.GateThreshold) {
		if d.DeltaMS >= minGatedDeltaMS {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeltaMS > out[j].DeltaMS })
	return out
}

func (r *Report) slowerThan(threshold float64) []PlanDelta {
	var out []PlanDelta
	for _, d := range r.Deltas {
		if d.Base == nil || d.Head == nil {
			continue
		}
		if d.Base.PlanMS < minReportedMS && d.Head.PlanMS < minReportedMS {
			continue
		}
		if d.Ratio > threshold {
			out = append(out, d)
		}
	}
	return out
}

// Improvements returns the plans that got materially faster, ordered by the wall time they save.
func (r *Report) Improvements() []PlanDelta {
	var out []PlanDelta
	for _, d := range r.Deltas {
		if d.Base == nil || d.Head == nil || d.Ratio == 0 {
			continue
		}
		if d.Base.PlanMS < minReportedMS && d.Head.PlanMS < minReportedMS {
			continue
		}
		if d.Ratio < 1/r.PlanThreshold {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeltaMS < out[j].DeltaMS })
	return out
}

// QualityRegressions returns plans whose trajectory got materially longer. A planner that buys
// latency with a worse path has not improved, and the latency table alone would call it a win.
func (r *Report) QualityRegressions() []PlanDelta {
	var out []PlanDelta
	for _, d := range r.Deltas {
		if d.TrajectoryRatio > trajectoryRegressionRatio {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TrajectoryRatio > out[j].TrajectoryRatio })
	return out
}

// trajectoryRegressionRatio is how much longer a trajectory has to get before it is called out.
// The planner is stochastic, so short paths wander a little between runs.
const trajectoryRegressionRatio = 1.15

// NewFailures returns plans the head revision failed and the base revision solved.
func (r *Report) NewFailures() []PlanDelta {
	var out []PlanDelta
	for _, d := range r.Deltas {
		if d.NewFailure {
			out = append(out, d)
		}
	}
	return out
}

// Verdict reports whether the head revision regressed against the gate. A whole-order slowdown and
// a newly failing plan both count; so does a single plan blowing up, because that is exactly the
// shape past regressions took -- a handful of plans carrying a total that the mean hides.
func (r *Report) Verdict() error {
	var problems []string
	if ratio := r.TotalRatio(); ratio > r.TotalThreshold {
		problems = append(problems, fmt.Sprintf("order total %.2fx slower (threshold %.2fx)", ratio, r.TotalThreshold))
	}
	if failures := r.NewFailures(); len(failures) > 0 {
		names := make([]string, 0, len(failures))
		for _, d := range failures {
			names = append(names, d.Name)
		}
		problems = append(problems, fmt.Sprintf("%d newly failing plan(s): %s", len(failures), strings.Join(names, ", ")))
	}
	if gated := r.GateFailures(); len(gated) > 0 {
		worst := gated[0]
		problems = append(problems, fmt.Sprintf("%d plan(s) over %.2fx, worst %s at %.2fx",
			len(gated), r.GateThreshold, worst.Name, worst.Ratio))
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("motion order benchmark regressed: %s", strings.Join(problems, "; "))
}

// Markdown renders the report for a pull request comment.
func (r *Report) Markdown() string {
	var b strings.Builder

	b.WriteString("### Motion order benchmark\n\n")
	fmt.Fprintf(&b, "`%s` (base) vs `%s` (head), %d plans replayed in recorded order.\n\n", r.BaseLabel, r.HeadLabel, r.Head.Plans)

	fmt.Fprintf(&b, "| | base | head | ratio |\n|---|---:|---:|---:|\n")
	fmt.Fprintf(&b, "| order total | %s | %s | **%s** |\n",
		formatMS(r.Base.TotalMS), formatMS(r.Head.TotalMS), formatRatio(r.TotalRatio()))
	fmt.Fprintf(&b, "| plans solved | %d | %d | |\n", r.Base.Plans-r.Base.Failures, r.Head.Plans-r.Head.Failures)
	fmt.Fprintf(&b, "| plans failed | %d | %d | |\n", r.Base.Failures, r.Head.Failures)
	if r.Base.TotalL2 > 0 {
		fmt.Fprintf(&b, "| total joint travel (L2) | %.1f | %.1f | %s |\n",
			r.Base.TotalL2, r.Head.TotalL2, formatRatio(r.Head.TotalL2/r.Base.TotalL2))
	}
	b.WriteString("\n")

	if failures := r.NewFailures(); len(failures) > 0 {
		fmt.Fprintf(&b, "#### ⛔ Newly failing plans (%d)\n\n", len(failures))
		for _, d := range failures {
			fmt.Fprintf(&b, "- `%s` — %s\n", d.Name, firstLine(d.Head.FirstErr))
		}
		b.WriteString("\n")
	}

	regressions := r.Regressions()
	if len(regressions) > 0 {
		fmt.Fprintf(&b, "#### 🔻 Slower than %.2fx (%d), by time added\n\n", r.PlanThreshold, len(regressions))
		writeDeltaTable(&b, regressions)
	}

	improvements := r.Improvements()
	if len(improvements) > 0 {
		fmt.Fprintf(&b, "#### 🔺 Faster than %.2fx (%d), by time saved\n\n", r.PlanThreshold, len(improvements))
		writeDeltaTable(&b, improvements)
	}

	if len(regressions) == 0 && len(improvements) == 0 {
		fmt.Fprintf(&b, "No individual plan moved by more than %.2fx.\n\n", r.PlanThreshold)
	}

	if quality := r.QualityRegressions(); len(quality) > 0 {
		fmt.Fprintf(&b, "#### 📏 Longer trajectories (%d)\n\n", len(quality))
		b.WriteString("| plan | base | head | ratio | plan time |\n|---|---:|---:|---:|---:|\n")
		shown := quality
		if len(shown) > maxTableRows {
			shown = shown[:maxTableRows]
		}
		for _, d := range shown {
			fmt.Fprintf(&b, "| `%s` | %d | %d | **%s** | %s |\n",
				d.Name, d.Base.TrajectoryLen, d.Head.TrajectoryLen, formatRatio(d.TrajectoryRatio), formatRatio(d.Ratio))
		}
		if len(quality) > len(shown) {
			fmt.Fprintf(&b, "\n…and %d more.\n", len(quality)-len(shown))
		}
		b.WriteString("\n")
	}

	if err := r.Verdict(); err != nil {
		fmt.Fprintf(&b, "\n**%s**\n", err.Error())
	}
	return b.String()
}

func writeDeltaTable(b *strings.Builder, deltas []PlanDelta) {
	b.WriteString("| plan | base | head | Δ | ratio | traj len | roadmap/cBiRRT |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|\n")

	shown := deltas
	var hidden []PlanDelta
	if len(deltas) > maxTableRows {
		shown, hidden = deltas[:maxTableRows], deltas[maxTableRows:]
	}

	for _, d := range shown {
		trajCell := "—"
		if d.TrajectoryRatio > 0 {
			trajCell = fmt.Sprintf("%d → %d (%s)", d.Base.TrajectoryLen, d.Head.TrajectoryLen, formatRatio(d.TrajectoryRatio))
		}
		fmt.Fprintf(b, "| `%s` | %s | %s | %s | **%s** | %s | %d/%d → %d/%d |\n",
			d.Name, formatMS(d.Base.PlanMS), formatMS(d.Head.PlanMS), formatDelta(d.DeltaMS), formatRatio(d.Ratio), trajCell,
			d.Base.RoadmapSolved, d.Base.CBiRRTSolved, d.Head.RoadmapSolved, d.Head.CBiRRTSolved)
	}

	if len(hidden) > 0 {
		var tail float64
		for _, d := range hidden {
			tail += d.DeltaMS
		}
		fmt.Fprintf(b, "\n…and %d more, %s in total.\n", len(hidden), formatDelta(tail))
	}
	b.WriteString("\n")
}

func formatMS(ms float64) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.2fs", ms/1000)
	}
	return fmt.Sprintf("%.0fms", ms)
}

// formatDelta always carries a sign, so a table sorted by time added stays readable when it runs
// past zero into the plans that got faster.
func formatDelta(ms float64) string {
	sign := "+"
	if ms < 0 {
		sign = "−"
		ms = -ms
	}
	return sign + formatMS(ms)
}

func formatRatio(ratio float64) string {
	if ratio == 0 {
		return "—"
	}
	return fmt.Sprintf("%.2fx", ratio)
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}
