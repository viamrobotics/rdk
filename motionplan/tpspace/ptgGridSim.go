package tpspace

import(
	//~ "fmt"
	"math"
	
	//~ rutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/spatialmath"
)

var (
	defaultMaxTime = 15.
	defaultMaxDist = 5000. // in mm
	defaultDiffT = 0.005
	//~ defaultDiffT = 0.0005
	defaultMinDist = 5.
	defaultAlphaCnt uint = 121
	defaultTurnRad = 100. // in mm, an approximate constant for estimating arc distances?
	
	defaultMaxHeadingChange = 1.95 * math.Pi
)

// This does something with circles
// Other ptgs will be based on this ptg somehow
type ptgGridSim struct {
	resolution float64 // mm
	refDist float64
	numPaths uint
	alphaCnt uint
	
	maxTime float64 // secs of robot execution to simulate
	diffT float64 // discretize trajectory simulation to this time granularity
	minDist float64 // Save traj points at this arc distance granularity
	turnRad float64 // robot turning radius
	
	simPTG PrecomputePTG
	
	precomputeTraj [][]*TrajNode
	
	// Discretized x[y][]node maps for rapid NN lookups
	trajNodeGrid map[int]map[int][]*TrajNode
}

func NewPTGGridSim(simPTG PrecomputePTG, arcs uint) (PTG, error) {
	
	if arcs == 0 {
		arcs = defaultAlphaCnt
	}
	
	ptg := &ptgGridSim{
		refDist: defaultMaxDist,
		alphaCnt: arcs,
		maxTime: defaultMaxTime,
		diffT: defaultDiffT,
		minDist: defaultMinDist,
		turnRad: defaultTurnRad,
		
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

func (ptg *ptgGridSim) WorldSpaceToTP(x, y, r float64) []*TrajNode {
	
	nearbyNodes := []*TrajNode{}
	
	// First, try to do a quick grid-based lookup
	// TODO: an octree should be faster
	for tx := int(math.Round(x-r)); tx < int(math.Round(x+r)); tx++ {
		if ptg.trajNodeGrid[tx] != nil {
			for ty := int(math.Round(y-r)); ty < int(math.Round(y+r)); ty++ {
				nearbyNodes = append(nearbyNodes, ptg.trajNodeGrid[tx][ty]...)
			}
		}
	}
	
	if len(nearbyNodes) > 0 {
		return nearbyNodes
	}
	
	// Try to find a closest point to the paths:
	bestDist := math.Inf(1)
	var bestNode *TrajNode

	for k := 0; k < int(ptg.alphaCnt); k++ {
		
		nMax := len(ptg.precomputeTraj[k]) - 1
		for n := 0; n <= nMax; n++ {
			distToPoint := math.Pow(ptg.precomputeTraj[k][n].ptX - x, 2) + math.Pow(ptg.precomputeTraj[k][n].ptY - y, 2)
			if distToPoint < bestDist {
				bestDist = distToPoint
				
				bestNode = ptg.precomputeTraj[k][n]
			}
		}
	}

	if bestNode != nil {
		
		return []*TrajNode{bestNode}
	}

	
	// Given a point (x,y), compute the "k_closest" whose extrapolation
	//  is closest to the point, and the associated "d_closest" distance,
	//  which can be normalized by "1/refDistance" to get TP-Space distances.
	for k := 0; k < int(ptg.alphaCnt); k++ {
		
		n := len(ptg.precomputeTraj[k]) - 1
		
		distToPoint := math.Pow(ptg.precomputeTraj[k][n].Dist, 2) +
			math.Pow(ptg.precomputeTraj[k][n].ptX - x, 2) + math.Pow(ptg.precomputeTraj[k][n].ptY - y, 2)

		if distToPoint < bestDist{
			bestDist = distToPoint;
			bestNode = ptg.precomputeTraj[k][n]
		}
	}

	return []*TrajNode{bestNode}
}

func (ptg *ptgGridSim) RefDistance() float64{
	return ptg.refDist
}

func (ptg *ptgGridSim) Trajectory(k uint) []*TrajNode {
	if int(k) >= len(ptg.precomputeTraj) {
		return nil
	}
	return ptg.precomputeTraj[k]
}

func (ptg *ptgGridSim) simulateTrajectories(simPtg PrecomputePTG) ([][]*TrajNode, error) {
	xMin := 500.0
	xMax := -500.0
	yMin := 500.0
	yMax := -500.0
	
	// C-space path structure
	allTraj := make([][]*TrajNode, 0, ptg.alphaCnt)
	
	for k := uint(0); k < ptg.alphaCnt; k++ {
		alpha := index2alpha(uint(k), ptg.alphaCnt)
		
		// Initialize trajectory with an all-zero node
		alphaTraj := []*TrajNode{&TrajNode{Pose: spatialmath.NewZeroPose()}}
		
		var err error
		var w float64
		var v float64
		var x float64
		var y float64
		var phi float64
		var t float64
		var dist float64
		
		// Last saved waypoints
		var wpX float64
		var wpY float64
		var wpPhi float64
		
		accumulatedHeadingChange := 0.
		
		lastVs := [2]float64{0,0}
		lastWs := [2]float64{0,0}
		
		// Step through each time point for this alpha
		for t < ptg.maxTime && dist < ptg.refDist && accumulatedHeadingChange < defaultMaxHeadingChange {
			v, w, err = simPtg.PtgDiffDriveSteer(alpha, t, x, y, phi)
			if err != nil {
				return nil, err
			}
			lastVs[1] = lastVs[0]
			lastWs[1] = lastWs[0]
			lastVs[0] = v
			lastWs[0] = w
			
			// finite difference eq
			x += math.Cos(phi) * v * ptg.diffT
			y += math.Sin(phi) * v * ptg.diffT
			phi += w * ptg.diffT
			accumulatedHeadingChange += w * ptg.diffT
			
			vInTPSpace := math.Sqrt(v*v + math.Pow(w * defaultTurnRad, 2))
			
			dist += vInTPSpace * ptg.diffT
			t += ptg.diffT
			
			wpDist1 := math.Sqrt(math.Pow(wpX - x, 2) + math.Pow(wpY - y, 2))
			wpDist2 := math.Abs(wpPhi - phi)
			wpDist := math.Max(wpDist1, wpDist2)
			
			if wpDist > ptg.minDist {
				// If our waypoint is farther along than our minimum, update
				
				// Update velocities of last node because reasons
				alphaTraj[len(alphaTraj)-1].W = w
				alphaTraj[len(alphaTraj)-1].V = v
				
				pose := xyphiToPose(x, y, phi)
				alphaTraj = append(alphaTraj, &TrajNode{pose, t, dist, k, v, w, pose.Point().X, pose.Point().Y})
				wpX = x
				wpY = y
				wpPhi = phi
			}
			
			// For the grid!
			xMin = math.Min(xMin, x)
			xMax = math.Max(xMax, x)
			yMin = math.Min(yMin, y)
			yMax = math.Max(yMax, y)
		}
		
		// Add final node
		alphaTraj[len(alphaTraj)-1].W = w
		alphaTraj[len(alphaTraj)-1].V = v
		pose := xyphiToPose(x, y, phi)
		tNode := &TrajNode{pose, t, dist, k,  v, w, pose.Point().X, pose.Point().Y}
		
		if _, ok := ptg.trajNodeGrid[int(math.Round(x))]; !ok {
			ptg.trajNodeGrid[int(math.Round(x))] = map[int][]*TrajNode{}
		}
		ptg.trajNodeGrid[int(math.Round(x))][int(math.Round(y))] = append(ptg.trajNodeGrid[int(math.Round(x))][int(math.Round(y))], tNode)
		
		alphaTraj = append(alphaTraj, tNode)
		
		allTraj = append(allTraj, alphaTraj)
	}
	
	// Make the grid I guess?
	// Actually this is unnecessary for the initial time being I think?
	
	return allTraj, nil
}

