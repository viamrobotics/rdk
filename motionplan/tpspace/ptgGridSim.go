package tpspace

import (
	"context"
	"math"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

const (
	defaultMaxTime       = 15.
	defaultDiffT         = 0.01
	defaultAlphaCnt uint = 91
)

// ptgGridSim will take a PTG, and simulate out a number of trajectories through some requested time/distance for speed of lookup
// later. It will store the trajectories in a grid data structure allowing relatively fast lookups.
type ptgGridSim struct {
	PTG
	refDist  float64
	alphaCnt uint

	diffT   float64 // discretize trajectory simulation to this time granularity

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
		diffT:    defaultDiffT,
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

func (ptg *ptgGridSim) Trajectory(alpha, dist float64) ([]*TrajNode, error) {
	return ComputePTG(ptg, alpha, dist, defaultDiffT)
}

func (ptg *ptgGridSim) simulateTrajectories() ([][]*TrajNode, error) {
	// C-space path structure
	allTraj := make([][]*TrajNode, 0, ptg.alphaCnt)

	for k := uint(0); k < ptg.alphaCnt; k++ {
		alpha := index2alpha(k, ptg.alphaCnt)

		alphaTraj, err := ComputePTG(ptg, alpha, ptg.refDist, ptg.diffT)
		if err != nil {
			return nil, err
		}
		allTraj = append(allTraj, alphaTraj)
	}

	return allTraj, nil
}
