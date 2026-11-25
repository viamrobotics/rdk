package armplanning

import (
	"context"
	"fmt"
	"slices"

	"go.viam.com/rdk/referenceframe"
)

// SingleComponentEpsilon is the largest change (rad or mm, L-infinity over a frame's inputs)
// between two waypoints that still counts as the frame not moving.
const SingleComponentEpsilon = 1e-4

// maxSequentializeDepth bounds how many times a segment is halved while looking for
// single-component legs: at most 2^maxSequentializeDepth pieces per segment.
const maxSequentializeDepth = 6

// MovedFrames returns, sorted, the frames whose inputs differ between from and to by more than
// SingleComponentEpsilon.
func MovedFrames(from, to *referenceframe.LinearInputs) []string {
	var moved []string
	for name, toInputs := range to.Items() {
		fromInputs := from.Get(name)
		if len(toInputs) == 0 || len(fromInputs) != len(toInputs) {
			continue
		}
		if referenceframe.InputsLinfDistance(fromInputs, toInputs) > SingleComponentEpsilon {
			moved = append(moved, name)
		}
	}
	slices.Sort(moved)
	return moved
}

// sequentialize rewrites steps (which follow start) so that no waypoint moves more than one frame.
// Executors command one component at a time, so a waypoint that moves two would be driven as two
// consecutive single-component moves through a configuration nobody checked. Every leg emitted
// here has passed the same check as the path it replaces.
func (pm *planManager) sequentialize(
	ctx context.Context,
	start *referenceframe.LinearInputs,
	steps []*referenceframe.LinearInputs,
	goal referenceframe.FrameSystemPoses,
) ([]*referenceframe.LinearInputs, error) {
	needed := false
	prev := start
	for _, step := range steps {
		if len(MovedFrames(prev, step)) > 1 {
			needed = true
			break
		}
		prev = step
	}
	if !needed {
		return steps, nil
	}

	psc, err := NewPlanSegmentContext(ctx, pm.pc, start, goal)
	if err != nil {
		return nil, err
	}

	out := make([]*referenceframe.LinearInputs, 0, len(steps))
	prev = start
	for _, step := range steps {
		legs, err := sequentializeSegment(ctx, psc, prev, step, 0)
		if err != nil {
			return nil, err
		}
		out = append(out, legs...)
		prev = step
	}
	pm.logger.Debugf("sequentialized %d waypoints into %d single-component waypoints", len(steps), len(out))
	return out, nil
}

// sequentializeSegment returns waypoints that lead from `from` (exclusive) to `to` (inclusive),
// each moving a single frame.
func sequentializeSegment(
	ctx context.Context,
	psc *PlanSegmentContext,
	from, to *referenceframe.LinearInputs,
	depth int,
) ([]*referenceframe.LinearInputs, error) {
	moved := MovedFrames(from, to)
	if len(moved) <= 1 {
		return []*referenceframe.LinearInputs{to}, nil
	}

	for _, order := range frameOrders(moved) {
		if legs := checkedLegs(ctx, psc, from, to, order); legs != nil {
			return legs, nil
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if depth >= maxSequentializeDepth {
		return nil, fmt.Errorf("the path found needs %v to move at the same time: no order of moving them one at a time is "+
			"collision free, even in %d pieces", moved, 1<<maxSequentializeDepth)
	}

	// The midpoint lies on the path that was already checked, so it is a valid place to restart from.
	mid := referenceframe.NewLinearInputs()
	for name, fromInputs := range from.Items() {
		toInputs := to.Get(name)
		if len(fromInputs) == 0 || len(toInputs) != len(fromInputs) {
			mid.Put(name, fromInputs)
			continue
		}
		interp, err := psc.pc.fs.Frame(name).Interpolate(fromInputs, toInputs, 0.5)
		if err != nil {
			return nil, err
		}
		mid.Put(name, interp)
	}

	first, err := sequentializeSegment(ctx, psc, from, mid, depth+1)
	if err != nil {
		return nil, err
	}
	second, err := sequentializeSegment(ctx, psc, mid, to, depth+1)
	if err != nil {
		return nil, err
	}
	return append(first, second...), nil
}

// checkedLegs moves the frames in order from their value in `from` to their value in `to`, one per
// leg, and returns the leg ends, or nil if any leg fails the path check. The last leg ends exactly
// at `to`.
func checkedLegs(
	ctx context.Context,
	psc *PlanSegmentContext,
	from, to *referenceframe.LinearInputs,
	order []string,
) []*referenceframe.LinearInputs {
	legs := make([]*referenceframe.LinearInputs, 0, len(order))
	state := from
	for i, name := range order {
		next := to
		if i < len(order)-1 {
			next = state.Copy()
			next.Put(name, to.Get(name))
		}
		if err := psc.CheckPath(ctx, state, next, true, nil); err != nil {
			return nil
		}
		legs = append(legs, next)
		state = next
	}
	return legs
}

// frameOrders lists the orders to try, in a fixed sequence so the same request always yields the
// same plan. Every permutation for up to three frames; beyond that, forward and reverse only.
func frameOrders(sorted []string) [][]string {
	if len(sorted) > 3 {
		reversed := slices.Clone(sorted)
		slices.Reverse(reversed)
		return [][]string{sorted, reversed}
	}
	var orders [][]string
	var permute func(prefix, rest []string)
	permute = func(prefix, rest []string) {
		if len(rest) == 0 {
			orders = append(orders, slices.Clone(prefix))
			return
		}
		for i := range rest {
			remaining := slices.Delete(slices.Clone(rest), i, i+1)
			permute(append(prefix, rest[i]), remaining)
		}
	}
	permute(nil, sorted)
	return orders
}
