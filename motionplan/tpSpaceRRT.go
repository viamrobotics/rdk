//go:build !windows

package motionplan

import (
	"fmt"
	"context"
	"math"

	"go.viam.com/utils"
	"github.com/golang/geo/r3"
	//~ rutils "go.viam.com/rdk/utils"

	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

var(
	defaultTurnRad = 100. // in mm, an approximate constant for estimating arc distances?
	defaultAutoBB = 5. // Automatic bounding box on driveable area as a multiple of start-goal distance
)

// tpspaceRRTMotionPlanner 
type tpspaceRRTMotionPlanner struct {
	*planner
	goalCheck int // Check if goal is reachable every this many iters
	autoBB    float64
	ptgs      []tpspace.PTG
}

func (mp *tpspaceRRTMotionPlanner) plan(ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
) ([][]referenceframe.Input, error) {
	solutionChan := make(chan *rrtPlanReturn, 1)
	
	seedPos, err := mp.frame.Transform(seed)
	if err != nil {
		return nil, err
	}
	zeroInput := make([]referenceframe.Input, len(seed))
	
	startNode := &configurationNode{q: zeroInput, endConfig: seedPos}
	goalNode := &configurationNode{q: zeroInput, endConfig: goal}
	
	utils.PanicCapturingGo(func() {
		mp.rrtBackgroundRunner(ctx, seed, &rrtParallelPlannerShared{
			&rrtMaps{
				startMap: map[node]node{startNode:nil},
				goalMap: map[node]node{goalNode:nil},
			},
			nil,
			solutionChan,
		})
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case plan := <-solutionChan:
		return plan.toInputs(), plan.err()
	}
}

// rrtBackgroundRunner will execute the plan. Plan() will call rrtBackgroundRunner in a separate thread and wait for results.
// Separating this allows other things to call rrtBackgroundRunner in parallel allowing the thread-agnostic Plan to be accessible.
func (mp *tpspaceRRTMotionPlanner) rrtBackgroundRunner(
	ctx context.Context,
	seed []referenceframe.Input,
	rrt *rrtParallelPlannerShared,
) {
	defer close(rrt.solutionChan)
	
	nm := &neighborManager{nCPU: mp.planOpts.NumThreads}
	nmContext, cancel := context.WithCancel(ctx)
	defer cancel()
	
	// get start and goal poses
	var startPose spatialmath.Pose
	var goalPose spatialmath.Pose
	for k, _ := range rrt.maps.startMap {
		if cNode, ok := k.(*configurationNode); ok {
			startPose = cNode.endConfig
		} else {
			rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("node %v must be a configurationNode", k)}
			return
		}
	}
	for k, _ := range rrt.maps.goalMap {
		if cNode, ok := k.(*configurationNode); ok {
			goalPose = cNode.endConfig
		} else {
			rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("node %v must be a configurationNode", k)}
			return
		}
	}
	
	dist := mp.planOpts.DistanceFunc(&Segment{StartPosition: startPose, EndPosition: goalPose}) * mp.autoBB
	// 2d only for now
	
	m1chan := make(chan node, 1)
	defer close(m1chan)

	success := false
	successNode := nil
	iter := 1
	// While not at goal:
	for !success {
		// Get random cartesian configuration
		// This needs to be guaranteed collision-free?
		randPos := goalPose
		tryGoal := true
		if iter % mp.goalCheck != 0 {
			tryGoal = false
			randPosX := float64(mp.randseed.Intn(int(dist)))
			randPosY := float64(mp.randseed.Intn(int(dist)))
			randPosTheta := math.Pi * (mp.randseed.Float64() - 0.5)
			randPos = spatialmath.NewPose(
				r3.Vector{goalPose.Point().X + (randPosX - dist/2.), goalPose.Point().Y + (randPosY - dist/2.), 0},
				&spatialmath.OrientationVector{OZ: 1, Theta: randPosTheta},
			)
		}
		iter++
		randPosNode := &configurationNode{endConfig: randPos}
		
		// For each PTG
		// TODO: run in parallel
		for ptgNum, curPtg := range mp.ptgs {
			
			// Make the distance function that will find the nearest RRT map node in TP-space of *this* PTG
			// TODO: cache this
			ptgDistOpt := make2DTPSpaceDistanceOptions(curPtg)
			
			// Get nearest neighbor to rand config in tree using this PTG
			utils.PanicCapturingGo(func() {
				nm.nearestNeighbor(nmContext, ptgDistOpt, randPosNode, rrt.maps.startMap, m1chan)
			})
			nearest1 := <-m1chan
			cNode, ok := nearest1.(*configurationNode)
			if !ok {
				rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("node %v must be a configurationNode", nearest1)}
				return
			}
			
			// Get cartesian diff from NN to rand
			relPosePt := spatialmath.PoseBetween(cNode.endConfig, randPos).Point()
			
			// Convert cartesian diff to tp-space using inverse curPtg: a-rand, d-rand
			kRand, dRand, err := curPtg.WorldSpaceToTP(relPosePt.X, relPosePt.Y)
			if err != nil {
				rrt.solutionChan <- &rrtPlanReturn{planerr: err}
				return
			}
			
			// Check collisions along this traj and get the longest distance viable
			kTraj := curPtg.Trajectory(kRand)
			pass := true
			
			var lastNode *tpspace.TrajNode
			
			for _, trajPt := range kTraj {
				if trajPt.Dist > dRand {
					// After we've passed dRand, no need to keep checking, just add to RRT tree
					successNode = &configurationNode{
						endConfig: lastNode.Pose,
						q: referenceframe.FloatsToInputs([]float64{float64(ptgNum), float64(kRand), lastNode.Dist}),
					}
					rrt.maps.startMap[successNode] = nearest1
					break
				}
				trajState := &State{Position: trajPt.Pose}
				ok, _ := mp.planOpts.CheckStateConstraints(trajState)
				if !ok {
					pass = false
					break
				}
				lastNode = trajPt
			}
			// Peter's note: I bet we'll get better paths if we add many small steps rather than just the one node
			
			// We successfully connected to the goal
			if pass && tryGoal {
				success = true
				break
			}
		}
	}
	
	// Rebuild the path from the goal node to the start
	
	
}

func make2DTPSpaceDistanceOptions(ptg tpspace.PTG) *plannerOptions {
	opts := newBasicPlannerOptions()
	
	segMet := func(seg *Segment) float64 {
		if seg.StartPosition == nil || seg.EndPosition == nil {
			return math.Inf(1)
		}
		relPosePt := spatialmath.PoseBetween(seg.StartPosition, seg.EndPosition).Point()
		_, d, err := ptg.WorldSpaceToTP(relPosePt.X, relPosePt.Y)
		if err != nil {
			return math.Inf(1)
		}
		return d
	}
	opts.DistanceFunc = segMet
	return opts
}
