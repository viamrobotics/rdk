package motionplan

import (
	"context"
	"math"
	"sync"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	defaultDiffT = 0.01 // seconds. Return trajectories updating velocities at this resolution.
	nloptSeed    = 42   // This should be fine to kepe constant

	defaultZeroDist = 1e-3 // Sometimes nlopt will minimize trajectories to zero. Ensure min traj dist is at least this
)

// ptgGridSim will take a PrecomputePTG, and simulate out a number of trajectories through some requested time/distance for speed of lookup
// later. It will store the trajectories in a grid data structure allowing relatively fast lookups.
type ptgIK struct {
	refDist         float64
	simPTG          tpspace.PrecomputePTG
	ptgFrame        referenceframe.Frame
	fastGradDescent *NloptIK

	gridSim tpspace.PTG

	mu        sync.RWMutex
	trajCache map[float64][]*tpspace.TrajNode
}

// NewPTGIK creates a new PTG by simulating a PrecomputePTG for some distance, then cacheing the results in a grid for fast lookup.
func NewPTGIK(simPTG tpspace.PrecomputePTG, logger golog.Logger, refDist float64, randSeed int) (tpspace.PTG, error) {
	ptgFrame, err := tpspace.NewPTGIKFrame(simPTG, refDist)
	if err != nil {
		return nil, err
	}

	nlopt, err := CreateNloptIKSolver(ptgFrame, logger, 1, defaultEpsilon*defaultEpsilon, true)
	if err != nil {
		return nil, err
	}

	// create an ends-only grid sim for quick end-of-trajectory calculations
	gridSim, err := tpspace.NewPTGGridSim(simPTG, 0, refDist, true)
	if err != nil {
		return nil, err
	}

	ptg := &ptgIK{
		refDist:         refDist,
		simPTG:          simPTG,
		ptgFrame:        ptgFrame,
		fastGradDescent: nlopt,
		gridSim:         gridSim,
		trajCache:       map[float64][]*tpspace.TrajNode{},
	}

	return ptg, nil
}

func (ptg *ptgIK) CToTP(ctx context.Context, distFunc func(spatialmath.Pose) float64) (*tpspace.TrajNode, error) {
	solutionGen := make(chan *IKSolution, 1)
	seedInput := []referenceframe.Input{{math.Pi / 2}, {ptg.refDist / 2}} // random value to seed the IK solver
	goalMetric := func(state *State) float64 {
		return distFunc(state.Position)
	}
	// Spawn the IK solver to generate a solution
	err := ptg.fastGradDescent.Solve(ctx, solutionGen, seedInput, goalMetric, nloptSeed)
	// We should have zero or one solutions
	var solved *IKSolution
	select {
	case solved = <-solutionGen:
	default:
	}
	close(solutionGen)
	if err != nil || solved == nil || solved.Configuration[1].Value < defaultZeroDist {
		return ptg.gridSim.CToTP(ctx, distFunc)
	}

	if solved.Partial {
		gridNode, err := ptg.gridSim.CToTP(ctx, distFunc)
		if err == nil {
			// Check if the grid has a better solution
			if distFunc(gridNode.Pose) < solved.Score {
				return gridNode, nil
			}
		}
	}

	traj, err := ptg.Trajectory(solved.Configuration[0].Value, solved.Configuration[1].Value)
	if err != nil {
		return nil, err
	}
	return traj[len(traj)-1], nil
}

func (ptg *ptgIK) RefDistance() float64 {
	return ptg.refDist
}

func (ptg *ptgIK) Trajectory(alpha, dist float64) ([]*tpspace.TrajNode, error) {
	ptg.mu.RLock()
	precomp := ptg.trajCache[alpha]
	ptg.mu.RUnlock()
	if precomp != nil {
		if precomp[len(precomp)-1].Dist >= dist {
			// Cacheing here provides a ~33% speedup to a solve call
			subTraj := []*tpspace.TrajNode{}
			for _, wp := range precomp {
				if wp.Dist < dist {
					subTraj = append(subTraj, wp)
				} else {
					break
				}
			}
			time := 0.
			if len(subTraj) > 0 {
				time = subTraj[len(subTraj)-1].Time
			}
			lastNode, err := tpspace.ComputePTGNode(ptg.simPTG, alpha, dist, time)
			if err != nil {
				return nil, err
			}
			subTraj = append(subTraj, lastNode)
			return subTraj, nil
		}
	}
	traj, err := tpspace.ComputePTG(ptg.simPTG, alpha, dist, defaultDiffT)
	if err != nil {
		return nil, err
	}
	ptg.mu.Lock()
	ptg.trajCache[alpha] = traj
	ptg.mu.Unlock()
	return traj, nil
}
