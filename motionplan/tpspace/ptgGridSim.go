package tpspace

import (
	"context"
	"fmt"
	"math"

	"go.viam.com/rdk/spatialmath"
)

const (
	defaultMaxTime       = 15.
	defaultDiffT         = 0.005
	defaultAlphaCnt uint = 121

	defaultSearchRadius = 10.

	defaultMaxHeadingChange = 1.95 * math.Pi
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
	searchRad    float64 // Distance around a query point to search for precompute in the cached grid
}

// NewPTGGridSim creates a new PTG by simulating a PrecomputePTG for some distance, then cacheing the results in a grid for fast lookup.
func NewPTGGridSim(simPTG PrecomputePTG, arcs uint, simDist float64, endsOnly bool) (PTG, error) {
	if arcs == 0 {
		arcs = defaultAlphaCnt
	}

	ptg := &ptgGridSim{
		refDist:   simDist,
		alphaCnt:  arcs,
		maxTime:   defaultMaxTime,
		diffT:     defaultDiffT,
		searchRad: defaultSearchRadius,
		endsOnly:  endsOnly,

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

func (ptg *ptgGridSim) CToTP(ctx context.Context, pose spatialmath.Pose) (*TrajNode, error) {
	
	point := pose.Point()
	x := point.X
	y := point.Y
	
	nearbyNodes := []*TrajNode{}
	// Try to find a closest point to the paths:
	bestDist := math.Inf(1)
	var bestNode *TrajNode

	if !ptg.endsOnly {
		// First, try to do a quick grid-based lookup
		// TODO: an octree should be faster
		for tx := int(math.Round(x - ptg.searchRad)); tx < int(math.Round(x+ptg.searchRad)); tx++ {
			if ptg.trajNodeGrid[tx] != nil {
				for ty := int(math.Round(y - ptg.searchRad)); ty < int(math.Round(y+ptg.searchRad)); ty++ {
					nearbyNodes = append(nearbyNodes, ptg.trajNodeGrid[tx][ty]...)
				}
			}
		}

		if len(nearbyNodes) > 0 {
			for _, nearbyNode := range nearbyNodes {
				distToPoint := math.Pow(nearbyNode.ptX-x, 2) + math.Pow(nearbyNode.ptY-y, 2)
				if distToPoint < bestDist {
					bestDist = distToPoint

					bestNode = nearbyNode
				}
			}
			return bestNode, nil
		}

		for k := 0; k < int(ptg.alphaCnt); k++ {
			nMax := len(ptg.precomputeTraj[k]) - 1
			for n := 0; n <= nMax; n++ {
				distToPoint := math.Pow(ptg.precomputeTraj[k][n].ptX-x, 2) + math.Pow(ptg.precomputeTraj[k][n].ptY-y, 2)
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

		distToPoint := math.Pow(ptg.precomputeTraj[k][n].Dist, 2) +
			math.Pow(ptg.precomputeTraj[k][n].ptX-x, 2) + math.Pow(ptg.precomputeTraj[k][n].ptY-y, 2)

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
	k := alpha2index(alpha, ptg.alphaCnt)
	if int(k) >= len(ptg.precomputeTraj) {
		return nil, fmt.Errorf("requested trajectory of index %d but this grid sim only has %d available", k, len(ptg.precomputeTraj))
	}
	fullTraj := ptg.precomputeTraj[k]
	if fullTraj[len(fullTraj) - 1].Dist < dist {
		return nil, fmt.Errorf("requested traj to dist %f but only simulated trajectories to distance %f", dist, fullTraj[len(fullTraj) - 1].Dist)
	}
	var traj []*TrajNode
	for _, trajNode := range fullTraj {
		// Walk the trajectory until we pass the specified distance
		if trajNode.Dist > dist {
			break
		}
		traj = append(traj, trajNode)
	}
	return traj, nil
}

func (ptg *ptgGridSim) simulateTrajectories(simPTG PrecomputePTG) ([][]*TrajNode, error) {
	// C-space path structure
	allTraj := make([][]*TrajNode, 0, ptg.alphaCnt)

	for k := uint(0); k < ptg.alphaCnt; k++ {
		alpha := index2alpha(k, ptg.alphaCnt)
		
		alphaTraj, err := ComputePTG(alpha, simPTG, ptg.refDist, ptg.diffT)
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
