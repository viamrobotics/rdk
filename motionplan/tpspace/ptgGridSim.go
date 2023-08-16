package tpspace

import (
	"context"
	"math"

	"go.viam.com/rdk/spatialmath"
)

const (
	defaultMaxTime       = 15.
	defaultDiffT         = 0.005
	defaultAlphaCnt uint = 91
)

// ptgGridSim will take a PrecomputePTG, and simulate out a number of trajectories through some requested time/distance for speed of lookup
// later. It will store the trajectories in a grid data structure allowing relatively fast lookups.
type ptgGridSim struct {
	refDist  float64
	alphaCnt uint

	maxTime float64 // secs of robot execution to simulate
	diffT   float64 // discretize trajectory simulation to this time granularity

	simPTG PrecomputePTG

	precomputeTraj [][]*TrajNode

	// If true, then CToTP calls will *only* check the furthest end of each precomputed trajectory.
	// This is useful when used in conjunction with IK
	endsOnly bool

	// Discretized x[y][]node maps for rapid NN lookups
	trajNodeGrid map[int]map[int][]*TrajNode
}

// NewPTGGridSim creates a new PTG by simulating a PrecomputePTG for some distance, then cacheing the results in a grid for fast lookup.
func NewPTGGridSim(simPTG PrecomputePTG, arcs uint, simDist float64, endsOnly bool) (PTG, error) {
	if arcs == 0 {
		arcs = defaultAlphaCnt
	}

	ptg := &ptgGridSim{
		refDist:  simDist,
		alphaCnt: arcs,
		maxTime:  defaultMaxTime,
		diffT:    defaultDiffT,
		endsOnly: endsOnly,

		trajNodeGrid: map[int]map[int][]*TrajNode{},
	}
	ptg.simPTG = simPTG

	precomp, err := ptg.simulateTrajectories(ptg.simPTG)
	if err != nil {
		return nil, err
	}
	ptg.precomputeTraj = precomp

	return ptg, nil
}

func (ptg *ptgGridSim) CToTP(ctx context.Context, distFunc func(spatialmath.Pose) float64) (*TrajNode, error) {
	// Try to find a closest point to the paths:
	bestDist := math.Inf(1)
	var bestNode *TrajNode

	if !ptg.endsOnly {
		for k := 0; k < int(ptg.alphaCnt); k++ {
			nMax := len(ptg.precomputeTraj[k]) - 1
			for n := 0; n <= nMax; n++ {
				distToPoint := distFunc(ptg.precomputeTraj[k][n].Pose)
				if distToPoint < bestDist {
					bestDist = distToPoint

					bestNode = ptg.precomputeTraj[k][n]
				}
			}
		}

		if bestNode != nil {
			return bestNode, nil
		}
	}

	// Given a point (x,y), compute the "k_closest" whose extrapolation
	//  is closest to the point, and the associated "d_closest" distance,
	//  which can be normalized by "1/refDistance" to get TP-Space distances.
	for k := 0; k < int(ptg.alphaCnt); k++ {
		n := len(ptg.precomputeTraj[k]) - 1
		distToPoint := distFunc(ptg.precomputeTraj[k][n].Pose)

		if distToPoint < bestDist {
			bestDist = distToPoint
			bestNode = ptg.precomputeTraj[k][n]
		}
	}

	return bestNode, nil
}

func (ptg *ptgGridSim) RefDistance() float64 {
	return ptg.refDist
}

func (ptg *ptgGridSim) Trajectory(alpha, dist float64) ([]*TrajNode, error) {
	return ComputePTG(ptg.simPTG, alpha, dist, defaultDiffT)
}

func (ptg *ptgGridSim) simulateTrajectories(simPTG PrecomputePTG) ([][]*TrajNode, error) {
	// C-space path structure
	allTraj := make([][]*TrajNode, 0, ptg.alphaCnt)

	for k := uint(0); k < ptg.alphaCnt; k++ {
		alpha := index2alpha(k, ptg.alphaCnt)

		alphaTraj, err := ComputePTG(simPTG, alpha, ptg.refDist, ptg.diffT)
		if err != nil {
			return nil, err
		}

		if !ptg.endsOnly {
			for _, tNode := range alphaTraj {
				gridX := int(math.Round(tNode.ptX))
				gridY := int(math.Round(tNode.ptY))
				// Discretize into a grid for faster lookups later
				if _, ok := ptg.trajNodeGrid[gridX]; !ok {
					ptg.trajNodeGrid[gridX] = map[int][]*TrajNode{}
				}
				ptg.trajNodeGrid[gridX][gridY] = append(ptg.trajNodeGrid[gridX][gridY], tNode)
			}
		}

		allTraj = append(allTraj, alphaTraj)
	}

	return allTraj, nil
}
