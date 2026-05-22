package armplanning

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
)

// jointMoveEpsilon is the threshold below which a joint is treated as not
// moving for the joint-at-a-time experiment. IK returns float values that may
// differ from the start by ~1e-12 even on "untouched" joints, so an exact
// equality check would generate (and collision-check) pointless single-joint
// steps that don't change anything physically.
const jointMoveEpsilon = 0.0001

// jointAtATimeBudget caps wall-clock time spent on the joint-at-a-time
// experiment. Scenes where the DFS exhausts ~64 states with checkPath in the
// 1–5ms range can otherwise dominate planning time before cBiRRT even starts.
const jointAtATimeBudget = 100 * time.Millisecond

// tryJointAtATime attempts to reach any goal configuration in goalMap from
// `start` by moving one linearized input at a time, in some order, with each
// single-joint move checked for collisions. Returns the resulting trajectory
// (including the start config) if any ordering works; nil + false otherwise.
//
// Joints that don't physically move are excluded from the search. The remaining
// joints are explored in order of descending |goal - start| — large moves are
// more likely to collide, so trying them first lets the DFS fail-fast and
// gives the memoization more to prune.
//
// The first-level choices run in parallel: one goroutine per possible
// first-joint move. All goroutines share the same failed-state sync.Map, so
// convergent paths get pruned cross-goroutine. The first goroutine to succeed
// cancels the rest via the goal-local subcontext.
//
// This is an experiment: on scenes where a permutation of axis-aligned moves
// avoids obstacles, it sidesteps the much more expensive cBiRRT search.
func tryJointAtATime(
	ctx context.Context,
	psc *planSegmentContext,
	start *referenceframe.LinearInputs,
	goalMap rrtMap,
	logger logging.Logger,
) ([]*referenceframe.LinearInputs, bool) {
	startVals := start.GetLinearizedInputs()

	for goalNode := range goalMap {
		goalVals := goalNode.inputs.GetLinearizedInputs()
		if len(goalVals) != len(startVals) {
			continue
		}

		// Only enumerate joints that actually need to move. Use a tolerance:
		// IK output for an "untouched" joint frequently differs from start by
		// ~1e-12, and we don't want to emit (or test) a step that doesn't
		// physically move the joint.
		var joints []int
		for i := range startVals {
			if math.Abs(startVals[i]-goalVals[i]) > jointMoveEpsilon {
				joints = append(joints, i)
			}
		}
		if len(joints) == 0 {
			return []*referenceframe.LinearInputs{start}, true
		}

		// Order joints by descending move magnitude. Bigger moves collide more
		// often, so taking them first lets us fail-fast deeper in the tree.
		sort.Slice(joints, func(i, j int) bool {
			return math.Abs(startVals[joints[i]]-goalVals[joints[i]]) >
				math.Abs(startVals[joints[j]]-goalVals[joints[j]])
		})

		if path, ok := tryJointAtATimeParallel(ctx, psc, start, startVals, goalVals, joints, logger); ok {
			return path, true
		}
	}

	return nil, false
}

// tryJointAtATimeParallel runs one DFS per possible first-joint choice
// concurrently. The shared failed-state map prunes any state already
// determined unreachable to success, so two goroutines that converge on the
// same intermediate config share work. First success wins; we cancel the
// peer goroutines and return.
func tryJointAtATimeParallel(
	ctx context.Context,
	psc *planSegmentContext,
	start *referenceframe.LinearInputs,
	startVals, goalVals []float64,
	joints []int,
	logger logging.Logger,
) ([]*referenceframe.LinearInputs, bool) {
	type rootResult struct {
		path []*referenceframe.LinearInputs
		ok   bool
	}

	failed := &sync.Map{}
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	resultCh := make(chan rootResult, len(joints))

	for _, firstJ := range joints {
		rest := make([]int, 0, len(joints)-1)
		for _, k := range joints {
			if k != firstJ {
				rest = append(rest, k)
			}
		}

		go func(firstJ int, rest []int) {
			// Make the first single-joint move from start.
			current := append([]float64{}, startVals...)
			current[firstJ] = goalVals[firstJ]
			endConfig, err := psc.pc.lis.FloatsToInputs(current)
			if err != nil {
				resultCh <- rootResult{nil, false}
				return
			}
			if err := psc.checkPath(subCtx, start, endConfig, false, nil); err != nil {
				resultCh <- rootResult{nil, false}
				return
			}

			// DFS the remaining joints, each goroutine with its own path slice.
			path := []*referenceframe.LinearInputs{start, endConfig}
			if tryJointAtATimeDFS(subCtx, psc, current, goalVals, rest, &path, failed) {
				resultCh <- rootResult{path, true}
			} else {
				resultCh <- rootResult{nil, false}
			}
		}(firstJ, rest)
	}

	for k := 0; k < len(joints); k++ {
		select {
		case r := <-resultCh:
			if r.ok {
				logger.Debugf("joint-at-a-time succeeded in %d moves", len(r.path)-1)
				return r.path, true
			}
		case <-ctx.Done():
			return nil, false
		}
	}
	return nil, false
}

func tryJointAtATimeDFS(
	ctx context.Context,
	psc *planSegmentContext,
	current, goal []float64,
	remaining []int,
	path *[]*referenceframe.LinearInputs,
	failed *sync.Map,
) bool {
	if ctx.Err() != nil {
		return false
	}
	if len(remaining) == 0 {
		return true
	}
	mask := remainingMask(remaining)
	if _, ok := failed.Load(mask); ok {
		return false
	}
	startConfig := (*path)[len(*path)-1]
	for idx, j := range remaining {
		next := append([]float64{}, current...)
		next[j] = goal[j]

		endConfig, err := psc.pc.lis.FloatsToInputs(next)
		if err != nil {
			continue
		}
		if err := psc.checkPath(ctx, startConfig, endConfig, false, nil); err != nil {
			continue
		}

		*path = append(*path, endConfig)
		rest := make([]int, 0, len(remaining)-1)
		rest = append(rest, remaining[:idx]...)
		rest = append(rest, remaining[idx+1:]...)
		if tryJointAtATimeDFS(ctx, psc, next, goal, rest, path, failed) {
			return true
		}
		*path = (*path)[:len(*path)-1]
	}
	failed.Store(mask, struct{}{})
	return false
}

// remainingMask returns a bitmask with one bit set per index in `remaining`.
// Joint indices above 63 would overflow uint64; arms have well under that.
func remainingMask(remaining []int) uint64 {
	var m uint64
	for _, j := range remaining {
		m |= uint64(1) << uint(j)
	}
	return m
}
