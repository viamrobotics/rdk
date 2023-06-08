//go:build !windows

package motionplan

import (
	"fmt"
	"context"
	"errors"
	"math"
	"math/rand"

	"go.viam.com/utils"
	"github.com/golang/geo/r3"
	"github.com/edaniels/golog"
	//~ rutils "go.viam.com/rdk/utils"

	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

var(
	defaultGoalCheck = 5
	defaultTurnRad = 100. // in mm, an approximate constant for estimating arc distances?
	defaultAutoBB = 1. // Automatic bounding box on driveable area as a multiple of start-goal distance
	
	// Add a subnode every this many mm along a valid trajectory. Large values run faster, small gives better paths
	defaultAddNodeEvery = 50.
)

// newCBiRRTMotionPlannerWithSeed creates a cBiRRTMotionPlanner object with a user specified random seed.
func newTPSpaceMotionPlanner(
	frame referenceframe.Frame,
	seed *rand.Rand,
	logger golog.Logger,
	opt *plannerOptions,
) (motionPlanner, error) {
	if opt == nil {
		return nil, errNoPlannerOptions
	}
	mp, err := newPlanner(frame, seed, logger, opt)
	if err != nil {
		return nil, err
	}

	return &tpspaceRRTMotionPlanner{
		planner:         mp,
		goalCheck: defaultGoalCheck,
		autoBB: defaultAutoBB,
		
		addNodeEvery: defaultAddNodeEvery,
		
		searchRadius: 100.,
		addIntermediate: true,
	}, nil
}

// tpspaceRRTMotionPlanner 
type tpspaceRRTMotionPlanner struct {
	*planner
	goalCheck int // Check if goal is reachable every this many iters
	
	// size of bounding box within which to randomly sample points
	// TODO: base this on frame limits?
	autoBB    float64
	addNodeEvery float64
	searchRadius float64
	
	addIntermediate bool
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
	
	startNode := &configurationNode{endConfig: seedPos}
	goalNode := &configurationNode{endConfig: goal}
	
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
		if plan != nil {
			return plan.toInputs(), plan.err()
		}
		return nil, errors.New("nil plan returned!")
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
	
	tpFrame, ok := mp.frame.(tpspace.PTGProvider)
	if !ok {
		rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("frame %v must be a PTGProvider", mp.frame)}
		return
	}
	
	nm := &neighborManager{nCPU: mp.planOpts.NumThreads}
	nm.parallelNeighbors = 10
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
	
	dist := math.Sqrt(mp.planOpts.DistanceFunc(&Segment{StartPosition: startPose, EndPosition: goalPose}))
	midPt := goalPose.Point().Sub(startPose.Point())
	fmt.Println("startPose", startPose)
	fmt.Println("goalPose", goalPose)
	fmt.Println("dist", dist)
	
	nnchan := make(chan node, 1)
	defer close(nnchan)

	success := false
	var successNode *configurationNode
	iter := 0
	// While not at goal:
	// TODO: context, timeout, etc
	for !success {
		select {
		case <-ctx.Done():
			mp.logger.Debugf("TP Space RRT timed out after %d iterations", i)
			rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("TP Space RRT timeout %w", ctx.Err()), maps: rrt.maps}
			return
		default:
		}
		// Get random cartesian configuration
		randPos := goalPose
		tryGoal := true
		// Check if we can reach the goal every N iters
		if iter % mp.goalCheck != 0 {
			rDist := dist * (mp.autoBB + float64(iter)/10.)
			tryGoal = false
			randPosX := float64(mp.randseed.Intn(int(rDist)))
			randPosY := float64(mp.randseed.Intn(int(rDist)))
			randPosTheta := math.Pi * (mp.randseed.Float64() - 0.5)
			randPos = spatialmath.NewPose(
				r3.Vector{midPt.X + (randPosX - rDist/2.), midPt.Y + (randPosY - rDist/2.), 0},
				&spatialmath.OrientationVector{OZ: 1, Theta: randPosTheta},
			)
		}
		iter++
		randPosNode := &configurationNode{endConfig: randPos}
		
		candidateNodes := map[float64][2]*configurationNode{}

		// Find the best traj point for each traj family, and store for later comparison
		// TODO: run in parallel
		for ptgNum, curPtg := range tpFrame.PTGs() {
			successNode = nil
			acceptableGoal := false
			// Make the distance function that will find the nearest RRT map node in TP-space of *this* PTG
			// TODO: cache this
			ptgDistOpt := mp.make2DTPSpaceDistanceOptions(curPtg, mp.searchRadius / 2)
			
			// Get nearest neighbor to rand config in tree using this PTG
			utils.PanicCapturingGo(func() {
				nm.nearestNeighbor(nmContext, ptgDistOpt, randPosNode, rrt.maps.startMap, nnchan)
			})
			nearest := <-nnchan
			if nearest == nil {
				continue
			}
			cNode, ok := nearest.(*configurationNode)
			if !ok {
				rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("node %v must be a configurationNode", nearest)}
				return
			}
			
			// Get cartesian diff from NN to rand
			relPose := spatialmath.Compose(spatialmath.PoseInverse(cNode.endConfig), randPos)
			relPosePt := relPose.Point()

			// Convert cartesian diff to tp-space using inverse curPtg: a-rand, d-rand
			nodes := curPtg.WorldSpaceToTP(relPosePt.X, relPosePt.Y, mp.searchRadius)
			bestNode, bestDist := mp.closestNode(relPose, nodes, mp.searchRadius / 2)
			if bestNode == nil {
				continue
			}
			kRand := bestNode.K
			dRand := bestNode.Dist
			// Check collisions along this traj and get the longest distance viable
			kTraj := curPtg.Trajectory(kRand)
			pass := true
			var lastNode *tpspace.TrajNode
			var lastPose spatialmath.Pose
			
			sinceLastCC := 0.
			sinceLastNode := 0.
			_ = sinceLastNode
			lastDist := 0.
			
			// Check each point along the trajectory to confirm constraints are met
			for _, trajPt := range kTraj {
				if trajPt.Dist > dRand {
					// After we've passed dRand, no need to keep checking, just add to RRT tree
					break
				}
				sinceLastCC += (trajPt.Dist - lastDist)
				trajState := &State{Position: spatialmath.Compose(cNode.endConfig, trajPt.Pose), Frame: mp.frame}
				if sinceLastCC > mp.planOpts.Resolution {
					ok, _ := mp.planOpts.CheckStateConstraints(trajState)
					if !ok {
						pass = false
						break
					}
				}
				
				lastPose = trajState.Position
				lastNode = trajPt
				lastDist = lastNode.Dist
			}
			if pass {
				// add the last node in trajectory
				successNode = &configurationNode{
					endConfig: lastPose,
					q: referenceframe.FloatsToInputs([]float64{float64(ptgNum), float64(kRand), lastNode.Dist}),
					cost: cNode.Cost() + lastNode.Dist,
				}
				
			}
			if tryGoal && bestDist < mp.planOpts.GoalThreshold {
				// If we tried the goal and have a close-enough XY location, check if the node is good enough to be a final goal
				acceptableGoal = mp.planOpts.GoalThreshold > mp.planOpts.goalMetric(&State{Position: lastPose, Frame: mp.frame})
			}
			
			if !acceptableGoal && successNode != nil {
				// check if this  successNode is too close to nodes already in the tree, and if so, do not add.
				
				// Get nearest neighbor to rand config in tree using this PTG
				utils.PanicCapturingGo(func() {
					nm.nearestNeighbor(nmContext, mp.planOpts, successNode, rrt.maps.startMap, nnchan)
				})
				nearest := <-nnchan
				if nearest == nil {
					continue
				}
				nearNode, _ := nearest.(*configurationNode)
				dist := mp.planOpts.DistanceFunc(&Segment{StartPosition: successNode.endConfig, EndPosition: nearNode.endConfig})
				if dist > 5. {
					candidateNodes[bestDist] = [2]*configurationNode{cNode, successNode}
				}
			}
			
			// We successfully connected to the goal
			if pass && acceptableGoal {
				success = true
				rrt.maps.startMap[successNode] = cNode
				candidateNodes = nil
				break
			}
		}
		
		// If we found any valid nodes that we can extend to, find the very best one and add that to the tree
		if len(candidateNodes) > 0 {
			bestDist := math.Inf(1)
			var bestNode [2]*configurationNode
			for k, v := range candidateNodes {
				if k < bestDist {
					bestNode = v
					bestDist = k
				}
			}
			
			ptgNum := int(bestNode[1].Q()[0].Value)
			kRand := uint(bestNode[1].Q()[1].Value)
			
			kTraj := tpFrame.PTGs()[ptgNum].Trajectory(kRand)
			
			lastDist := 0.
			sinceLastNode := 0.
			
			for _, trajPt := range kTraj {
				if trajPt.Dist > bestNode[1].Q()[2].Value {
					// After we've passed dRand, no need to keep checking, just add to RRT tree
					break
				}
				trajState := &State{Position: spatialmath.Compose(bestNode[0].endConfig, trajPt.Pose), Frame: mp.frame}
				sinceLastNode += (trajPt.Dist - lastDist)
				//~ intPose := trajState.Position
				//~ fmt.Printf("FINALPATH,%f,%f\n", intPose.Point().X, intPose.Point().Y)
				
				// Optionally add sub-nodes along the way. Will make the final path a bit better
				if mp.addIntermediate && sinceLastNode > mp.addNodeEvery {
					// add the last node in trajectory
					successNode = &configurationNode{
						endConfig: trajState.Position,
						q: referenceframe.FloatsToInputs([]float64{float64(ptgNum), float64(kRand), trajPt.Dist}),
						cost: bestNode[0].Cost() + trajPt.Dist,
					}
					rrt.maps.startMap[successNode] = bestNode[0]
					sinceLastNode = 0.
				}
				lastDist = trajPt.Dist
				
			}
			rrt.maps.startMap[bestNode[1]] = bestNode[0]
			fmt.Println("new final node", bestNode[1])
		}
	}
	
	// Rebuild the path from the goal node to the start
	path := extractPath(rrt.maps.startMap, rrt.maps.goalMap, &nodePair{a: successNode})
	
	fmt.Println("success! getting path", path)
	allPtgs := tpFrame.PTGs()
	lastPose := spatialmath.NewZeroPose()
	for _, mynode := range path {
		cnode, _ := mynode.(*configurationNode)
		fmt.Println("path cnode", cnode.endConfig.Point(), cnode.endConfig.Orientation().OrientationVectorDegrees())
		if len(mynode.Q()) > 0 {
			//~ fmt.Println("traj", int(mynode.Q()[0].Value))
			//~ fmt.Println("k", uint(mynode.Q()[1].Value))
			//~ fmt.Println("dist", mynode.Q()[2].Value)
			trajPts := allPtgs[int(mynode.Q()[0].Value)].Trajectory(uint(mynode.Q()[1].Value))
			//~ fmt.Println("trajpts", trajPts)
			for _, pt := range trajPts {
				//~ fmt.Println("pt", pt)
				intPose := spatialmath.Compose(lastPose, pt.Pose)
				fmt.Printf("FINALPATH,%f,%f\n", intPose.Point().X, intPose.Point().Y)
				if pt.Dist >= mynode.Q()[2].Value {
					break
				}
			}
		}
		lastPose = cnode.endConfig
	}
	rrt.solutionChan <- &rrtPlanReturn{steps: path, maps: rrt.maps}
	return
}

func (mp *tpspaceRRTMotionPlanner) closestNode(pose spatialmath.Pose, nodes []*tpspace.TrajNode, min float64) (*tpspace.TrajNode, float64) {
	var bestNode *tpspace.TrajNode
	bestDist := math.Inf(1)
	for _, tNode := range nodes {
		
		if tNode.Dist < min {
			continue
		}
		dist := mp.planOpts.DistanceFunc(&Segment{StartPosition: pose, EndPosition: tNode.Pose})
		if dist < bestDist {
			bestNode = tNode
			bestDist = dist
		}
	}
	return bestNode, bestDist
}

func (mp *tpspaceRRTMotionPlanner) make2DTPSpaceDistanceOptions(ptg tpspace.PTG, min float64) *plannerOptions {
	opts := newBasicPlannerOptions()
	
	segMet := func(seg *Segment) float64 {
		if seg.StartPosition == nil || seg.EndPosition == nil {
			return math.Inf(1)
		}
		relPose := spatialmath.Compose(spatialmath.PoseInverse(seg.StartPosition), seg.EndPosition)
		relPosePt := relPose.Point()
		nodes := ptg.WorldSpaceToTP(relPosePt.X, relPosePt.Y, mp.searchRadius)
		closeNode, _ := mp.closestNode(relPose, nodes, min)
		if closeNode == nil {
			return math.Inf(1)
		}
		return closeNode.Dist
	}
	opts.DistanceFunc = segMet
	return opts
}
