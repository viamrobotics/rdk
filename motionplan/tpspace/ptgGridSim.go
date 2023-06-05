package tpspace

import(
	//~ "fmt"
	"math"
	
	//~ rutils "go.viam.com/rdk/utils"
)

var (
	defaultMaxTime = 5.
	defaultMaxDist = 10000. // 10 meters
	defaultDiffT = 0.0005
	defaultMinDist = 15.
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
	maxMps float64
	maxDps float64
	turnRad float64 // mm
	k      float64 // k = +1 for forwards, -1 for backwards
	alphaCnt uint
	
	simPTG PrecomputePTG
	
	precomputeTraj [][]*TrajNode
}

func NewPTGGridSim(maxMps, maxDps float64, simPTG PrecomputePTG) (PTG, error) {
	
	simConf := &trajSimConf{
		maxTime: defaultMaxTime,
		maxDist: defaultMaxDist,
		diffT: defaultDiffT,
		minDist: defaultMinDist,
		alphaCnt: defaultAlphaCnt,
		turnRad: defaultTurnRad,
	}
	
	ptg := &ptgGridSim{
		maxMps: maxMps,
		maxDps: maxDps,
		refDist: defaultMaxDist,
		alphaCnt: defaultAlphaCnt,
	}
	ptg.simPTG = simPTG
	
	precomp, err := simulateTrajectories(ptg.simPTG, simConf)
	if err != nil {
		return nil, err
	}
	ptg.precomputeTraj = precomp

	return ptg, nil
}

func (ptg *ptgGridSim) WorldSpaceToTP(x, y float64) (uint, float64, error) {
	
	// Try to find a closest point to the paths:
	// ----------------------------------------------
	selectedDist := math.Inf(1)
	var selectedD float64
	selectedK := -1

	for k := 0; k < int(ptg.alphaCnt); k++ {
		
		nMax := len(ptg.precomputeTraj[k]) - 1
		for n := 0; n <= nMax; n++ {
			distToPoint := math.Pow(ptg.precomputeTraj[k][n].Pose.Point().X - x, 2) + math.Pow(ptg.precomputeTraj[k][n].Pose.Point().Y - y, 2)
			if distToPoint < selectedDist {
				selectedDist = distToPoint
				selectedK = k;
				// if n == nMax, we're at the distal point of the trajectory so just take the cartesian distance
				// assuming that's more than the arc distance
				if n == nMax {
					selectedD = math.Max(ptg.precomputeTraj[k][n].Dist, math.Sqrt(distToPoint))
				} else {
					selectedD = ptg.precomputeTraj[k][n].Dist
				}
			}
		}
	}

	if selectedK != -1 {
		//~ return uint(selectedK), selectedD / ptg.refDist, nil
		return uint(selectedK), selectedD, nil
	}

	
	// ------------------------------------------------------------------------------------
	// Given a point (x,y), compute the "k_closest" whose extrapolation
	//  is closest to the point, and the associated "d_closest" distance,
	//  which can be normalized by "1/refDistance" to get TP-Space distances.
	// ------------------------------------------------------------------------------------

	for k := 0; k < int(ptg.alphaCnt); k++ {
		
		n := len(ptg.precomputeTraj[k]) - 1
		
		// TODO: 
		distToPoint := math.Pow(ptg.precomputeTraj[k][n].Dist, 2) +
			math.Pow(ptg.precomputeTraj[k][n].Pose.Point().X - x, 2) + math.Pow(ptg.precomputeTraj[k][n].Pose.Point().Y - y, 2)

		if distToPoint < selectedDist{
			selectedDist = distToPoint;
			selectedK = k;
			selectedD = distToPoint;
		}
	}

	selectedD = math.Sqrt(selectedD)

	//~ return uint(selectedK), selectedD / ptg.refDist, nil
	return uint(selectedK), selectedD, nil
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

func simulateTrajectories(simPtg PrecomputePTG, conf *trajSimConf) ([][]*TrajNode, error) {
	xMin := 1000.0
	xMax := -1000.0
	yMin := 1000.0
	yMax := -1000.0
	
	// C-space path structure
	allTraj := make([][]*TrajNode, 0, conf.alphaCnt)
	
	for k := 0; k < int(conf.alphaCnt); k++ {
		//~ fmt.Println("k", k)
		alpha := index2alpha(uint(k), conf.alphaCnt)
		
		// Initialize trajectory with an all-zero node
		alphaTraj := []*TrajNode{&TrajNode{}}
		
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
		
		for t < conf.maxTime && dist < conf.maxDist && accumulatedHeadingChange < defaultMaxHeadingChange {
			v, w, err = simPtg.ptgDiffDriveSteer(alpha, t, x, y, phi)
			if err != nil {
				return nil, err
			}
			lastVs[1] = lastVs[0]
			lastWs[1] = lastWs[0]
			lastVs[0] = v
			lastWs[0] = w
			
			// finite difference eq
			x += math.Cos(phi) * v * conf.diffT
			y += math.Sin(phi) * v * conf.diffT
			phi += w * conf.diffT
			accumulatedHeadingChange += w * conf.diffT
			
			vInTPSpace := math.Sqrt(v*v + math.Pow(w * defaultTurnRad, 2))
			
			dist += vInTPSpace * conf.diffT
			t += conf.diffT
			
			wpDist1 := math.Sqrt(math.Pow(wpX - x, 2) + math.Pow(wpY - y, 2))
			wpDist2 := math.Abs(wpPhi - phi)
			wpDist := math.Max(wpDist1, wpDist2)
			
			if wpDist < conf.minDist {
				// Update velocities of last node because reasons
				alphaTraj[len(alphaTraj)-1].W = w
				alphaTraj[len(alphaTraj)-1].V = v
				
				alphaTraj = append(alphaTraj, &TrajNode{xyphiToPose(x, y, phi), t, dist, v, w})
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
		alphaTraj = append(alphaTraj, &TrajNode{xyphiToPose(x, y, phi), t, dist, v, w})
		
		allTraj = append(allTraj, alphaTraj)
	}
	
	// Make the grid I guess?
	// Actually this is unnecessary for the initial time being I think?
	
	return allTraj, nil
}
