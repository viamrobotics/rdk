package mpserver

import (
	"fmt"
	"maps"
	"math"
	"slices"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
)

// sequenceRejectionTolerance is the per-joint distance within which the planner drops a solution
// as a repeat of a rejected one.
const sequenceRejectionTolerance = 0.01

// sequenceBacktrack is, for one plan of a sequence that had solutions rejected, the configurations
// involved and what each costs. Every table cell is preformatted for display.
type sequenceBacktrack struct {
	// Columns titles the configurations in Frames' rows: the start, the final end, then each
	// rejected end.
	Columns []string                 `json:"columns"`
	Frames  []sequenceBacktrackFrame `json:"frames"`
	// Notes are plain-language observations about the tables.
	Notes []string `json:"notes,omitempty"`
}

type sequenceBacktrackFrame struct {
	Name string `json:"name"`
	// Joints has one row per joint holding a cell for each of the parent's Columns.
	Joints [][]sequenceCell `json:"joints"`
	// Linf holds, for each rejected end, its largest per-joint difference from the final end,
	// empty under the start and final columns. A trailing * marks a difference within the
	// rejection tolerance.
	Linf []string `json:"linf"`
	// Cost is the request's configuration distance metric, over all frames, from the start each
	// solution was planned from to the solution, empty under the start column. A trailing † marks a
	// rejected solution planned from a different start than the final one. It omits the planner's
	// small bias for joints far from mid-range.
	Cost []string `json:"cost"`
}

type sequenceCell struct {
	Value string `json:"value"`
	// HalfTurns is set on a rejected solution's joint that differs from the final end by a whole
	// number of half turns, which the planner treats as a different solution.
	HalfTurns bool `json:"half_turns,omitempty"`
	// Same is set on a rejected solution's joint that displays the same value as the final end's.
	Same bool `json:"same,omitempty"`
}

// buildSequenceBacktrack describes the rejected solutions of one plan, or returns nil when none were
// rejected. plan is nil for a plan that never succeeded, in which case there is no final end.
func buildSequenceBacktrack(
	request *armplanning.PlanRequest, plan motionplan.Plan, meta *armplanning.PlanMeta, sequenceSucceeded bool,
) *sequenceBacktrack {
	if meta == nil || len(meta.RejectedSolutions) == 0 {
		return nil
	}

	endLabel := "successful"
	if !sequenceSucceeded {
		endLabel = "last successful"
	}

	var finalStart, finalEnd referenceframe.FrameSystemInputs
	if plan != nil {
		if trajectory := plan.Trajectory(); len(trajectory) > 0 {
			finalStart, finalEnd = trajectory[0], trajectory[len(trajectory)-1]
		}
	}

	metric := motionplan.GetConfigurationDistanceFunc(motionplan.FSConfigurationL2DistanceMetric)
	if request.PlannerOptions != nil {
		metric = motionplan.GetConfigurationDistanceFunc(request.PlannerOptions.ConfigurationDistanceMetric)
	}

	cost := func(start, end referenceframe.FrameSystemInputs) string {
		if start == nil || end == nil {
			return "-"
		}

		return fmt.Sprintf("%.4f", metric(&motionplan.SegmentFS{
			StartConfiguration: start.ToLinearInputs(),
			EndConfiguration:   end.ToLinearInputs(),
		}))
	}

	backtrack := &sequenceBacktrack{Columns: []string{"start", endLabel}}
	columns := []referenceframe.FrameSystemInputs{finalStart, finalEnd}
	costs := []string{"", cost(finalStart, finalEnd)}
	frameNames := map[string]bool{}
	for rejectedIdx, rejected := range meta.RejectedSolutions {
		backtrack.Columns = append(backtrack.Columns, fmt.Sprintf("rejected %d", rejectedIdx+1))
		columns = append(columns, rejected.End)

		rejectedCost := cost(rejected.Start, rejected.End)
		if finalStart != nil && !sequenceStartsAgree(finalStart, rejected.Start) {
			rejectedCost += " †"
		}
		costs = append(costs, rejectedCost)

		for frameName, inputs := range rejected.End {
			if len(inputs) > 0 {
				frameNames[frameName] = true
			}
		}
	}

	if finalEnd != nil {
		for rejectedIdx, rejected := range meta.RejectedSolutions {
			if sequenceStartsAgree(finalEnd, rejected.End) {
				backtrack.Notes = append(backtrack.Notes, fmt.Sprintf(
					"The %s solution is the same as rejected %d. Re-planning after rejecting it found no different solution, "+
						"so the plan was not replaced.", endLabel, rejectedIdx+1,
				))
			}
		}
	}

	for _, frameName := range slices.Sorted(maps.Keys(frameNames)) {
		frame := sequenceBacktrackFrame{Name: frameName, Linf: []string{"", ""}, Cost: costs}
		for jointIdx := range meta.RejectedSolutions[0].End[frameName] {
			row := make([]sequenceCell, 0, len(columns))
			for columnIdx, column := range columns {
				if jointIdx >= len(column[frameName]) {
					row = append(row, sequenceCell{Value: "-"})
					continue
				}

				value := column[frameName][jointIdx]
				cell := sequenceCell{Value: fmt.Sprintf("%.4f", value)}
				if columnIdx >= 2 && finalEnd != nil && jointIdx < len(finalEnd[frameName]) {
					finalValue := finalEnd[frameName][jointIdx]
					cell.Same = cell.Value == fmt.Sprintf("%.4f", float64(finalValue))
					cell.HalfTurns = !cell.Same && differsByWholeHalfTurns(value, finalValue)
				}
				row = append(row, cell)
			}
			frame.Joints = append(frame.Joints, row)
		}

		for _, rejected := range meta.RejectedSolutions {
			if finalEnd == nil {
				frame.Linf = append(frame.Linf, "-")
				continue
			}

			distance := referenceframe.InputsLinfDistance(finalEnd[frameName], rejected.End[frameName])
			marker := ""
			if distance <= sequenceRejectionTolerance {
				marker = "*"
			}
			frame.Linf = append(frame.Linf, fmt.Sprintf("%.4f%s", distance, marker))
		}
		backtrack.Frames = append(backtrack.Frames, frame)
	}

	return backtrack
}

// differsByWholeHalfTurns reports whether two angles are one or more half turns apart, to within
// the rejection tolerance.
func differsByWholeHalfTurns(value, other referenceframe.Input) bool {
	difference := math.Abs(float64(value - other))
	halfTurns := math.Round(difference / math.Pi)

	return halfTurns >= 1 && math.Abs(difference-halfTurns*math.Pi) <= sequenceRejectionTolerance
}

// sequenceStartsAgree reports whether two configurations are within the rejection tolerance on
// every joint of every frame.
func sequenceStartsAgree(first, second referenceframe.FrameSystemInputs) bool {
	for frameName, inputs := range first {
		if referenceframe.InputsLinfDistance(inputs, second[frameName]) > sequenceRejectionTolerance {
			return false
		}
	}

	return true
}
