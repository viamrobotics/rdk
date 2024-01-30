//go:build !no_cgo

package tpspace

import (
	"context"
	"errors"
	"math"
	"sync"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	defaultZeroDist = 1e-3 // Sometimes nlopt will minimize trajectories to zero. Ensure min traj dist is at least this
)

type ptgIK struct {
	PTG
	refDist         float64
	ptgFrame        referenceframe.Frame
	fastGradDescent *ik.NloptIK

	gridSim PTGSolver

	mu sync.RWMutex
	// trajCache speeds up queries by saving previously computed trajectories and not re-computing them from scratch.
	// The first key is the resolution of the trajectory, the second is the alpha value.
	trajCache   map[float64]map[float64][]*TrajNode
	defaultSeed []referenceframe.Input
}

// NewPTGIK creates a new ptgIK, which creates a frame using the provided PTG, and wraps it providing functions to fill the PTG
// interface, allowing inverse kinematics queries to be run against it.
func NewPTGIK(simPTG PTG, logger logging.Logger, refDistLong, refDistShort float64, randSeed, trajCount int) (PTGSolver, error) {
	if refDistLong <= 0 {
		return nil, errors.New("refDistLong must be greater than zero to create a ptgIK")
	}

	limits := []referenceframe.Limit{}
	for i := 0; i < trajCount; i++ {
		dist := refDistShort
		if i == 0 {
			// We only want to increase the length of the first leg of the PTG. Since gradient descent does not currently optimize
			// for reducing path length, having more than one long leg will result in very inefficient paths.
			dist = refDistLong
		}
		limits = append(limits,
			referenceframe.Limit{Min: -math.Pi, Max: math.Pi},
			referenceframe.Limit{Min: defaultMinPTGlen, Max: dist},
		)
	}

	ptgFrame := newPTGIKFrame(simPTG, limits)

	nlopt, err := ik.CreateNloptIKSolver(ptgFrame, logger, 1, false, false)
	if err != nil {
		return nil, err
	}

	ptg := &ptgIK{
		PTG:             simPTG,
		refDist:         refDistLong,
		ptgFrame:        ptgFrame,
		fastGradDescent: nlopt,
		trajCache:       map[float64]map[float64][]*TrajNode{},
	}
	ptg.defaultSeed = PTGIKSeed(ptg)

	// create an ends-only grid sim for quick end-of-trajectory calculations
	gridSim, err := NewPTGGridSim(simPTG, 0, refDistShort, true)
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
	seedOutput := true

	if solved != nil {
		// If nlopt failed to gradient descend, it will return the seed. If the seed is what was returned, we want to use our precomputed
		// grid check instead.
		for i, v := range solved.Configuration {
			if v.Value != seed[i].Value {
				seedOutput = false
				break
			}
		}
	}
	if err != nil || solved == nil || ptg.arcDist(solved.Configuration) < defaultZeroDist || seedOutput {
		// nlopt did not return a valid solution or otherwise errored. Fall back fully to the grid check.
		return ptg.gridSim.Solve(ctx, solutionChan, seed, solveMetric, nloptSeed)
	}

	solutionChan <- solved
	return nil
}

func (ptg *ptgIK) MaxDistance() float64 {
	return ptg.refDist
}

func (ptg *ptgIK) Trajectory(alpha, dist, resolution float64) ([]*TrajNode, error) {
	var precomp, traj []*TrajNode
	ptg.mu.RLock()
	thisRes := ptg.trajCache[resolution]
	if thisRes != nil {
		precomp = thisRes[alpha]
	}
	ptg.mu.RUnlock()
	if precomp != nil && precomp[len(precomp)-1].Dist >= dist && dist > 0 {
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
			lastNode, err := computePTGNode(ptg, alpha, dist)
			if err != nil {
				return nil, err
			}
			traj = append(traj, lastNode)
		}
	} else {
		var err error
		traj, err = ComputePTG(ptg, alpha, dist, resolution)
		if err != nil {
			return nil, err
		}
		if dist > 0 {
			ptg.mu.Lock()
			// Caching here provides a ~33% speedup to a solve call
			if ptg.trajCache[resolution] == nil {
				ptg.trajCache[resolution] = map[float64][]*TrajNode{}
			}
			ptg.trajCache[resolution][alpha] = traj
			ptg.mu.Unlock()
		}
	}

	return traj, nil
}

func (ptg *ptgIK) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	return ptg.ptgFrame.Transform(inputs)
}

// DoF returns the DoF of the associated referenceframe.
func (ptg *ptgIK) DoF() []referenceframe.Limit {
	return ptg.ptgFrame.DoF()
}

func (ptg *ptgIK) arcDist(inputs []referenceframe.Input) float64 {
	dist := 0.
	for i := 1; i < len(inputs); i += 2 {
		dist += (inputs[i].Value - defaultMinPTGlen)
	}
	return dist
}
