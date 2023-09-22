//go:build !no_cgo

package tpspace

import (
	"context"
	"errors"
	"sync"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

const (
	defaultResolutionSeconds = 0.01 // seconds. Return trajectories updating velocities at this resolution.

	defaultZeroDist = 1e-3 // Sometimes nlopt will minimize trajectories to zero. Ensure min traj dist is at least this
)

type ptgIK struct {
	PTG
	refDist         float64
	ptgFrame        referenceframe.Frame
	fastGradDescent *ik.NloptIK

	gridSim PTGSolver

	mu        sync.RWMutex
	trajCache map[float64][]*TrajNode
}

// NewPTGIK creates a new ptgIK, which creates a frame using the provided PTG, and wraps it providing functions to fill the PTG
// interface, allowing inverse kinematics queries to be run against it.
func NewPTGIK(simPTG PTG, logger golog.Logger, refDist float64, randSeed int) (PTGSolver, error) {
	if refDist <= 0 {
		return nil, errors.New("refDist must be greater than zero")
	}

	ptgFrame := newPTGIKFrame(simPTG, refDist)

	nlopt, err := ik.CreateNloptIKSolver(ptgFrame, logger, 1, false)
	if err != nil {
		return nil, err
	}

	// create an ends-only grid sim for quick end-of-trajectory calculations
	gridSim, err := NewPTGGridSim(simPTG, 0, refDist, true)
	if err != nil {
		return nil, err
	}

	ptg := &ptgIK{
		PTG:             simPTG,
		refDist:         refDist,
		ptgFrame:        ptgFrame,
		fastGradDescent: nlopt,
		gridSim:         gridSim,
		trajCache:       map[float64][]*TrajNode{},
	}

	return ptg, nil
}

func (ptg *ptgIK) Solve(
	ctx context.Context,
	solutionChan chan<- *ik.Solution,
	seed []referenceframe.Input,
	solveMetric ik.StateMetric,
	nloptSeed int,
) error {
	internalSolutionGen := make(chan *ik.Solution, 1)
	defer close(internalSolutionGen)
	var solved *ik.Solution

	// Spawn the IK solver to generate a solution
	err := ptg.fastGradDescent.Solve(ctx, internalSolutionGen, seed, solveMetric, nloptSeed)
	// We should have zero or one solutions

	select {
	case solved = <-internalSolutionGen:
	default:
	}
	if err != nil || solved == nil || solved.Configuration[1].Value < defaultZeroDist {
		// nlopt did not return a valid solution or otherwise errored. Fall back fully to the grid check.
		return ptg.gridSim.Solve(ctx, solutionChan, seed, solveMetric, nloptSeed)
	}

	if !solved.Exact {
		// nlopt returned something but was unable to complete the solve. See if the grid check produces something better.
		err = ptg.gridSim.Solve(ctx, internalSolutionGen, seed, solveMetric, nloptSeed)
		if err == nil {
			var gridSolved *ik.Solution
			select {
			case gridSolved = <-internalSolutionGen:
			default:
			}
			// Check if the grid has a better solution
			if gridSolved != nil {
				if gridSolved.Score < solved.Score {
					solved = gridSolved
				}
			}
		}
	}

	solutionChan <- solved
	return nil
}

func (ptg *ptgIK) MaxDistance() float64 {
	return ptg.refDist
}

func (ptg *ptgIK) Trajectory(inputs []referenceframe.Input) ([]*TrajNode, error) {
	alpha := inputs[0].Value
	dist := inputs[1].Value
	ptg.mu.RLock()
	precomp := ptg.trajCache[alpha]
	ptg.mu.RUnlock()
	if precomp != nil {
		if precomp[len(precomp)-1].Dist >= dist {
			// Caching here provides a ~33% speedup to a solve call
			subTraj := []*TrajNode{}
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
			lastNode, err := computePTGNode(ptg, alpha, dist, time)
			if err != nil {
				return nil, err
			}
			subTraj = append(subTraj, lastNode)
			return subTraj, nil
		}
	}
	traj, err := ComputePTG(ptg, alpha, dist, defaultResolutionSeconds)
	if err != nil {
		return nil, err
	}
	ptg.mu.Lock()
	ptg.trajCache[alpha] = traj
	ptg.mu.Unlock()
	return traj, nil
}
