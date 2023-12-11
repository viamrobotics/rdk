//go:build !no_cgo

package tpspace

import (
	"context"
	"errors"
	//~ "math"
	"sync"
	//~ "fmt"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
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

	mu          sync.RWMutex
	trajCache   map[float64][]*TrajNode
	defaultSeed []referenceframe.Input
}

// NewPTGIK creates a new ptgIK, which creates a frame using the provided PTG, and wraps it providing functions to fill the PTG
// interface, allowing inverse kinematics queries to be run against it.
func NewPTGIK(simPTG PTG, logger logging.Logger, refDistFar, refDistRestricted float64, randSeed, trajCount int) (PTGSolver, error) {
	refDist := refDistFar
	if refDist <= 0 {
		return nil, errors.New("refDist must be greater than zero")
	}
	ptgFrame := newPTGIKFrame(simPTG, trajCount, refDistFar, refDistRestricted)

	nlopt, err := ik.CreateNloptIKSolver(ptgFrame, logger, 1, false)
	if err != nil {
		return nil, err
	}

	inputs := []referenceframe.Input{}
	//~ for i := 0; i < trajCount; i++ {
		//~ inputs = append(inputs,
			//~ referenceframe.Input{float64(i)*(math.Pi/float64(trajCount))*0.9 + 0.01},
			//~ referenceframe.Input{float64(i+1) * refDist / 10},
		//~ )
	//~ }
	ptgDof := ptgFrame.DoF()
	for i := 0; i < len(ptgDof); i++ {
		boundRange := ptgDof[i].Max - ptgDof[i].Min
		inputs = append(inputs,
			referenceframe.Input{ptgDof[i].Min + 0.3*boundRange},
		)
	}

	ptg := &ptgIK{
		PTG:             simPTG,
		refDist:         refDist,
		ptgFrame:        ptgFrame,
		fastGradDescent: nlopt,
		trajCache:       map[float64][]*TrajNode{},
		defaultSeed:     inputs,
	}

	// create an ends-only grid sim for quick end-of-trajectory calculations
	gridSim, err := NewPTGGridSim(simPTG, 0, 500, true)
	if err != nil {
		return nil, err
	}
	ptg.gridSim = gridSim

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

	if seed == nil {
		seed = ptg.defaultSeed
	}

	// Spawn the IK solver to generate a solution
	err := ptg.fastGradDescent.Solve(ctx, internalSolutionGen, seed, solveMetric, nloptSeed)
	// We should have zero or one solutions

	select {
	case solved = <-internalSolutionGen:
	default:
	}
	if err != nil || solved == nil || solved.Configuration[1].Value < defaultZeroDist {
		//~ fmt.Println(1, err, solved, solved.Configuration)
		// nlopt did not return a valid solution or otherwise errored. Fall back fully to the grid check.
		return ptg.gridSim.Solve(ctx, solutionChan, seed, solveMetric, nloptSeed)
	}

	//~ if !solved.Exact {
		//~ // nlopt returned something but was unable to complete the solve. See if the grid check produces something better.
		//~ err = ptg.gridSim.Solve(ctx, internalSolutionGen, seed, solveMetric, nloptSeed)
		//~ if err == nil {
			//~ var gridSolved *ik.Solution
			//~ select {
			//~ case gridSolved = <-internalSolutionGen:
			//~ default:
			//~ }
			//~ // Check if the grid has a better solution
			//~ if gridSolved != nil {
				//~ if gridSolved.Score < solved.Score {
					//~ // ~ fmt.Println("grid2!")
					//~ solved = gridSolved
				//~ }
			//~ }
		//~ }
	//~ }

	solutionChan <- solved
	return nil
}

func (ptg *ptgIK) MaxDistance() float64 {
	return ptg.refDist
}

func (ptg *ptgIK) Trajectory(alpha, dist float64) ([]*TrajNode, error) {
	traj := []*TrajNode{}
	ptg.mu.RLock()
	precomp := ptg.trajCache[alpha]
	ptg.mu.RUnlock()
	if precomp != nil && precomp[len(precomp)-1].Dist >= dist {
		exact := false
		for _, wp := range precomp {
			if wp.Dist <= dist {
				if wp.Dist == dist {
					exact = true
				}
				traj = append(traj, wp)
			} else {
				break
			}
		}
		if !exact {
			time := 0.
			if len(traj) > 0 {
				time = traj[len(traj)-1].Time
			}
			lastNode, err := computePTGNode(ptg, alpha, dist, time)
			if err != nil {
				return nil, err
			}
			traj = append(traj, lastNode)
		}
	} else {
		var err error
		traj, err = ComputePTG(ptg, alpha, dist, defaultResolutionSeconds)
		if err != nil {
			return nil, err
		}
		ptg.mu.Lock()
		// Caching here provides a ~33% speedup to a solve call
		ptg.trajCache[alpha] = traj
		ptg.mu.Unlock()
	}

	return traj, nil
}

func (ptg *ptgIK) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	return ptg.ptgFrame.Transform(inputs)
}
