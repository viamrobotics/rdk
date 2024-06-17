package tpspace

import (
	"context"
	"fmt"
	"math"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

const (
	defaultAlphaCnt             uint    = 91  // When precomputing arcs, use this many different, equally-spaced alpha values
	defaultSimulationResolution float64 = 50. // When precomputing arcs, precompute nodes at this resolution
)

// ptgGridSim will take a PTG, and simulate out a number of trajectories through some requested time/distance for speed of lookup
// later. It will store the trajectories in a grid data structure allowing relatively fast lookups.
type ptgGridSim struct {
	PTG
	refDist  float64
	alphaCnt uint

	precomputeTraj [][]*TrajNode

	// If true, then CToTP calls will *only* check the furthest end of each precomputed trajectory.
	// This is useful when used in conjunction with IK
	endsOnly bool
}

// NewPTGGridSim creates a new PTG by simulating a PTG for some distance, then cacheing the results in a grid for fast lookup.
func NewPTGGridSim(simPTG PTG, arcs uint, simDist float64, endsOnly bool) (PTGSolver, error) {
	if arcs == 0 {
		arcs = defaultAlphaCnt
	}

	ptg := &ptgGridSim{
		refDist:  simDist,
		alphaCnt: arcs,
		endsOnly: endsOnly,
	}
	ptg.PTG = simPTG

	precomp, err := ptg.simulateTrajectories()
	if err != nil {
		return nil, err
	}
	ptg.precomputeTraj = precomp

	return ptg, nil
}

func (ptg *ptgGridSim) Solve(
	ctx context.Context,
	solutionChan chan<- *ik.Solution,
	seed []referenceframe.Input,
	solveMetric ik.StateMetric,
	rseed int,
) error {
	// Try to find a closest point to the paths:
	bestDist := math.Inf(1)
	var bestNode *TrajNode

	if !ptg.endsOnly {
		for k := 0; k < int(ptg.alphaCnt); k++ {
			nMax := len(ptg.precomputeTraj[k]) - 1
			for n := 0; n <= nMax; n++ {
				distToPoint := solveMetric(&ik.State{Position: ptg.precomputeTraj[k][n].Pose})
				if distToPoint < bestDist {
					bestDist = distToPoint

					bestNode = ptg.precomputeTraj[k][n]
				}
			}
		}

		if bestNode != nil {
			solutionChan <- &ik.Solution{
				Configuration: []referenceframe.Input{{bestNode.Alpha}, {bestNode.Dist}},
				Score:         bestDist,
				Exact:         false,
			}
			return nil
		}
	}

	// Given a point (x,y), compute the "k_closest" whose extrapolation
	//  is closest to the point, and the associated "d_closest" distance,
	//  which can be normalized by "1/refDistance" to get TP-Space distances.
	for k := 0; k < int(ptg.alphaCnt); k++ {
		n := len(ptg.precomputeTraj[k]) - 1
		distToPoint := solveMetric(&ik.State{Position: ptg.precomputeTraj[k][n].Pose})

		if distToPoint < bestDist {
			bestDist = distToPoint
			bestNode = ptg.precomputeTraj[k][n]
		}
	}

	solutionChan <- &ik.Solution{
		Configuration: []referenceframe.Input{{bestNode.Alpha}, {bestNode.Dist}},
		Score:         bestDist,
		Exact:         false,
	}
	return nil
}

func (ptg *ptgGridSim) MaxDistance() float64 {
	return ptg.refDist
}

func (ptg *ptgGridSim) Trajectory(alpha, start, end, resolution float64) ([]*TrajNode, error) {
	if end == 0 {
		return computePTG(ptg, alpha, end, resolution)
	}
	startPos := math.Abs(start)
	endPos := math.Abs(end)
	traj, err := computePTG(ptg, alpha, endPos, resolution)
	if err != nil {
		return nil, err
	}

	if startPos > 0 {
		firstNode, err := computePTGNode(ptg, alpha, startPos)
		if err != nil {
			return nil, err
		}
		first := -1
		for i, wp := range traj {
			if wp.Dist > startPos {
				first = i
				break
			}
		}
		if first == -1 {
			return nil, fmt.Errorf("failure in trajectory calculation, found no nodes with dist greater than %f", startPos)
		}
		return append([]*TrajNode{firstNode}, traj[first:len(traj)-1]...), nil
	}
	if end < start {
		return invertComputedPTG(traj), nil
	}
	return traj, nil
}

// DoF returns the DoF of the associated referenceframe.
func (ptg *ptgGridSim) DoF() []referenceframe.Limit {
	return []referenceframe.Limit{
		{Min: -1 * math.Pi, Max: math.Pi},
		{Min: 0, Max: ptg.refDist},
	}
}

func (ptg *ptgGridSim) simulateTrajectories() ([][]*TrajNode, error) {
	// C-space path structure
	allTraj := make([][]*TrajNode, 0, ptg.alphaCnt)

	for k := uint(0); k < ptg.alphaCnt; k++ {
		alpha := index2alpha(k, ptg.alphaCnt)
		alphaTraj, err := computePTG(ptg, alpha, ptg.refDist, defaultSimulationResolution)
		if err != nil {
			return nil, err
		}
		allTraj = append(allTraj, alphaTraj)
	}

	return allTraj, nil
}
