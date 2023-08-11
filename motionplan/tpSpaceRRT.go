//go:build !windows

package motionplan

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/utils"

	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	defaultGoalCheck = 5  // Check if the goal is reachable every this many iterations
	defaultAutoBB    = 1. // Automatic bounding box on driveable area as a multiple of start-goal distance
	// Note: while fully holonomic planners can use the limits of the frame as implicit boundaries, with non-holonomic motion
	// this is not the case, and the total workspace available to the planned frame is not directly related to the motion available
	// from a single set of inputs.

	// whether to add intermediate waypoints.
	defaultAddInt = true
	// Add a subnode every this many mm along a valid trajectory. Large values run faster, small gives better paths
	// Meaningless if the above is false.
	defaultAddNodeEvery = 250.

	// When getting the closest node to a pose, only look for nodes at least this far away.
	defaultDuplicateNodeBuffer = 50.

	// Don't add new RRT tree nodes if there is an existing node within this distance.
	defaultIdenticalNodeDistance = 5.

	// Default distance in mm to get within for tp-space trajectories.
	defaultTPSpaceGoalDist = 10.
	
	defaultMaxReseeds = 10
)

type tpspaceOptions struct {
	*rrtOptions
	goalCheck int // Check if goal is reachable every this many iters

	// TODO: base this on frame limits?
	autoBB float64 // Automatic bounding box on driveable area as a multiple of start-goal distance

	addIntermediate bool // whether to add intermediate waypoints.
	// Add a subnode every this many mm along a valid trajectory. Large values run faster, small gives better paths
	// Meaningless if the above is false.
	addNodeEvery float64

	// Don't add new RRT tree nodes if there is an existing node within this distance
	dupeNodeBuffer float64
	
	// If the squared norm between two poses is less than this, consider them equal
	poseSolveDist float64
	
	maxReseeds int

	// Cached functions for calculating TP-space distances for each PTG
	distOptions map[tpspace.PTG]*plannerOptions
	invertDistOptions map[tpspace.PTG]*plannerOptions
}

// candidate is putative node which could be added to an RRT tree. It includes a distance score, the new node and its future parent.
type candidate struct {
	dist       float64
	treeNode   node
	newNode    node
	err        error
	lastInTraj bool
}

// tpSpaceRRTMotionPlanner.
type tpSpaceRRTMotionPlanner struct {
	*planner
	algOpts *tpspaceOptions
	tpFrame tpspace.PTGProvider
}

// newTPSpaceMotionPlanner creates a newTPSpaceMotionPlanner object with a user specified random seed.
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

	// either the passed in frame,
	tpFrame, ok := mp.frame.(tpspace.PTGProvider)
	if !ok {
		return nil, fmt.Errorf("frame %v must be a PTGProvider", mp.frame)
	}

	tpPlanner := &tpSpaceRRTMotionPlanner{
		planner: mp,
		tpFrame: tpFrame,
	}
	tpPlanner.setupTPSpaceOptions()
	return tpPlanner, nil
}

// TODO: seed is not immediately useful for TP-space.
func (mp *tpSpaceRRTMotionPlanner) plan(ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
) ([]node, error) {
	solutionChan := make(chan *rrtPlanReturn, 1)

	seedPos := spatialmath.NewZeroPose()

	startNode := &basicNode{q: make([]referenceframe.Input, len(mp.frame.DoF())), cost: 0, pose: seedPos, corner: false}
	goalNode := &basicNode{q: nil, cost: 0, pose: goal, corner: false}

	utils.PanicCapturingGo(func() {
		mp.planRunner(ctx, seed, &rrtParallelPlannerShared{
			&rrtMaps{
				startMap: map[node]node{startNode: nil},
				goalMap:  map[node]node{goalNode: nil},
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
			return plan.steps, plan.err()
		}
		return nil, errors.New("nil tp-space plan returned, unable to complete plan")
	}
}

// planRunner will execute the plan. Plan() will call planRunner in a separate thread and wait for results.
// Separating this allows other things to call planRunner in parallel allowing the thread-agnostic Plan to be accessible.
func (mp *tpSpaceRRTMotionPlanner) planRunner(
	ctx context.Context,
	_ []referenceframe.Input, // TODO: this may be needed for smoothing
	rrt *rrtParallelPlannerShared,
) {
	defer close(rrt.solutionChan)

	// get start and goal poses
	var startPose spatialmath.Pose
	var goalPose spatialmath.Pose
	for k, v := range rrt.maps.startMap {
		if v == nil {
			if k.Pose() != nil {
				startPose = k.Pose()
			} else {
				rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("node %v must provide a Pose", k)}
				return
			}
			break
		}
	}
	for k, v := range rrt.maps.goalMap {
		if v == nil {
			if k.Pose() != nil {
				goalPose = k.Pose()
			} else {
				rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("node %v must provide a Pose", k)}
				return
			}
			break
		}
	}

	dist := math.Sqrt(mp.planOpts.DistanceFunc(&Segment{StartPosition: startPose, EndPosition: goalPose}))
	fmt.Println("dist", dist)
	fmt.Println(startPose.Point(), goalPose.Point())
	midPt := goalPose.Point().Sub(startPose.Point()) // used for initial seed

	var randPos spatialmath.Pose
	//~ for iter := 0; iter < mp.algOpts.PlanIter; iter++ {
	for iter := 0; iter < 125; iter++ {
		fmt.Println("iter", iter)
		if ctx.Err() != nil {
			mp.logger.Debugf("TP Space RRT timed out after %d iterations", iter)
			rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("TP Space RRT timeout %w", ctx.Err()), maps: rrt.maps}
			return
		}
		// Get random cartesian configuration
		rDist := dist * (mp.algOpts.autoBB + float64(iter)/10.)
		randPosX := float64(mp.randseed.Intn(int(rDist)))
		randPosY := float64(mp.randseed.Intn(int(rDist)))
		randPosTheta := math.Pi * (mp.randseed.Float64() - 0.5)
		randPos = spatialmath.NewPose(
			r3.Vector{midPt.X + (randPosX - rDist/2.), midPt.Y + (randPosY - rDist/2.), 0},
			&spatialmath.OrientationVector{OZ: 1, Theta: randPosTheta},
		)
		randPosNode := &basicNode{q: nil, cost: 0, pose: randPos, corner: false}
		_ = randPosNode

		//~ goalNode := &basicNode{pose: goalPose}

		seedMapNode, err := mp.attemptExtension(ctx, nil, randPosNode, rrt.maps.startMap, false)
		if err != nil {
			rrt.solutionChan <- &rrtPlanReturn{planerr: err, maps: rrt.maps}
			return
		}
		//~ seedMapNode := &basicNode{pose: spatialmath.NewZeroPose()}
		goalMapNode, err := mp.attemptExtension(ctx, nil, randPosNode, rrt.maps.goalMap, true)
		if err != nil {
			rrt.solutionChan <- &rrtPlanReturn{planerr: err, maps: rrt.maps}
			return
		}
		
		if seedMapNode != nil && goalMapNode != nil {
		
			seedMapNode, err = mp.attemptExtension(ctx, nil, goalMapNode, rrt.maps.startMap, false)
			if err != nil {
				rrt.solutionChan <- &rrtPlanReturn{planerr: err, maps: rrt.maps}
				return
			}
			if seedMapNode == nil {
				continue
			}
			goalMapNode, err := mp.attemptExtension(ctx, nil, seedMapNode, rrt.maps.goalMap, true)
			if err != nil {
				rrt.solutionChan <- &rrtPlanReturn{planerr: err, maps: rrt.maps}
				return
			}
			if goalMapNode == nil {
				continue
			}
			fmt.Println("start tree", seedMapNode.Pose().Point(), seedMapNode.Pose().Orientation().OrientationVectorDegrees())
			fmt.Println("goal tree", goalMapNode.Pose().Point(), goalMapNode.Pose().Orientation().OrientationVectorDegrees())
			reachedDelta := mp.planOpts.DistanceFunc(&Segment{StartPosition: seedMapNode.Pose(), EndPosition: goalMapNode.Pose()})
			// If we tried the goal and have a close-enough XY location, check if the node is good enough to be a final goal
			if reachedDelta <= mp.algOpts.poseSolveDist {
				// If we've reached the goal, break out
				path := extractPath(rrt.maps.startMap, rrt.maps.goalMap, &nodePair{a: seedMapNode, b: goalMapNode})
				//~ path = path[1:] // The 
				rrt.solutionChan <- &rrtPlanReturn{steps: path, maps: rrt.maps}
				return
			}
		}
	}
	rrt.solutionChan <- &rrtPlanReturn{maps: rrt.maps, planerr: errors.New("tpspace RRT unable to create valid path")}
}

// getExtensionCandidate will return either nil, or the best node on a PTG to reach the desired random node and its RRT tree parent.
func (mp *tpSpaceRRTMotionPlanner) getExtensionCandidate(
	ctx context.Context,
	randPosNode node,
	ptgNum int,
	curPtg tpspace.PTG,
	rrt rrtMap,
	nearest node,
	planOpts *plannerOptions, // Need to pass this in explicitly for smoothing
	invert bool,
) (*candidate, error) {
	nm := &neighborManager{nCPU: mp.planOpts.NumThreads}
	nm.parallelNeighbors = 10

	var successNode node
	// Get the distance function that will find the nearest RRT map node in TP-space of *this* PTG
	ptgDistOpt := mp.algOpts.distOptions[curPtg]
	if invert {
		ptgDistOpt = mp.algOpts.invertDistOptions[curPtg]
	}

	if nearest == nil {
		// Get nearest neighbor to rand config in tree using this PTG
		nearest = nm.nearestNeighbor(ctx, ptgDistOpt, randPosNode, rrt)
		if nearest == nil {
			return nil, nil
		}
	}
	//~ fmt.Println("nearest", nearest.Pose().Point(), nearest.Pose().Orientation().OrientationVectorDegrees())

	// Get cartesian distance from NN to rand
	relPose := spatialmath.Compose(spatialmath.PoseInverse(nearest.Pose()), randPosNode.Pose())
	// Convert cartesian distance to tp-space using inverse curPtg, yielding TP-space coordinates goalK and goalD
	bestNode, err := curPtg.CToTP(ctx, relPose)
	if err != nil || bestNode == nil {
		return nil, err
	}
	bestDist := mp.planOpts.DistanceFunc(&Segment{StartPosition: relPose, EndPosition: bestNode.Pose})
	goalAlpha := bestNode.Alpha
	goalD := bestNode.Dist
	//~ fmt.Println("alpha dist", goalAlpha, goalD)
	// Check collisions along this traj and get the longest distance viable
	trajK, err := curPtg.Trajectory(goalAlpha, goalD)
	if err != nil {
		return nil, err
	}
	finalTrajNode := trajK[len(trajK) - 1]

	arcStartPose := nearest.Pose()
	if invert {
		arcStartPose = spatialmath.Compose(arcStartPose, spatialmath.PoseInverse(finalTrajNode.Pose))
	}
	//~ fmt.Println("arc tform", finalTrajNode.Pose.Point())
	//~ fmt.Println("arc tform invert", spatialmath.PoseInverse(finalTrajNode.Pose).Point())
	//~ fmt.Println("arcstart", arcStartPose.Point())

	sinceLastCollideCheck := 0.
	lastDist := 0.
	var nodePose spatialmath.Pose
	// Check each point along the trajectory to confirm constraints are met
	for i := 0; i < len(trajK); i++ {
		trajPt := trajK[i]
		if invert {
			// Start at known-good map point and extend
			// For the goal tree this means iterating backwards
			trajPt = trajK[(len(trajK)-1)-i]
		}

		sinceLastCollideCheck += math.Abs(trajPt.Dist - lastDist)
		trajState := &State{Position: spatialmath.Compose(arcStartPose, trajPt.Pose), Frame: mp.frame}
		//~ fmt.Println("trajstate", trajState.Position.Point())
		nodePose = trajState.Position
		if sinceLastCollideCheck > planOpts.Resolution {
			ok, _ := planOpts.CheckStateConstraints(trajState)
			if !ok {
				return nil, nil
			}
			sinceLastCollideCheck = 0.
		}

		lastDist = trajPt.Dist
	}
	
	isLastNode := finalTrajNode.Dist == curPtg.RefDistance()
	
	// add the last node in trajectory
	successNode = &basicNode{
		q:      referenceframe.FloatsToInputs([]float64{float64(ptgNum), goalAlpha, finalTrajNode.Dist}),
		cost:   nearest.Cost() + finalTrajNode.Dist,
		pose:   nodePose,
		corner: false,
	}

	cand := &candidate{dist: bestDist, treeNode: nearest, newNode: successNode, lastInTraj: isLastNode}

	// check if this  successNode is too close to nodes already in the tree, and if so, do not add.
	// Get nearest neighbor to new node that's already in the tree
	//~ nearest = nm.nearestNeighbor(ctx, planOpts, successNode, rrt)
	//~ if nearest != nil {
		//~ dist := planOpts.DistanceFunc(&Segment{StartPosition: successNode.Pose(), EndPosition: nearest.Pose()})
		//~ // Ensure successNode is sufficiently far from the nearest node already existing in the tree
		//~ // If too close, don't add a new node
		//~ if dist < defaultIdenticalNodeDistance {
			//~ cand = nil
		//~ }
	//~ }
	return cand, nil
}

// attemptExtension will attempt to extend the rrt map towards the goal node, and will return the candidate added to the map that is
// closest to that goal.
func (mp *tpSpaceRRTMotionPlanner) attemptExtension(
	ctx context.Context,
	seedNode,
	goalNode node,
	rrt rrtMap,
	invert bool,
) (node, error) {
	var reseedCandidate *candidate
	for i := 0; i < mp.algOpts.maxReseeds; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		candidates := []*candidate{}
		for ptgNum, curPtg := range mp.tpFrame.PTGs() {
			// Find the best traj point for each traj family, and store for later comparison
			// TODO: run in parallel
			cand, err := mp.getExtensionCandidate(ctx, goalNode, ptgNum, curPtg, rrt, seedNode, mp.planOpts, invert)
			if err != nil {
				return nil, err
			}
			if cand != nil {
				if cand.err == nil {
					candidates = append(candidates, cand)
				}
			}
		}
		var err error
		reseedCandidate, err = mp.extendMap(ctx, candidates, rrt, invert)
		if err != nil {
			return nil, err
		}
		if reseedCandidate == nil {
			return nil, nil
		}
		dist := mp.planOpts.DistanceFunc(&Segment{StartPosition: reseedCandidate.newNode.Pose(), EndPosition: goalNode.Pose()})
		if dist < mp.planOpts.GoalThreshold || !reseedCandidate.lastInTraj {
			// Reached the goal position, or otherwise failed to fully extend to the end of a trajectory
			return reseedCandidate.newNode, nil
		}
		if dist < reseedCandidate.newNode.Q()[2].Value && i < mp.algOpts.maxReseeds - 2{
			i = mp.algOpts.maxReseeds - 2
		}
		seedNode = reseedCandidate.newNode
	}
	return reseedCandidate.newNode, nil
}

// extendMap grows the rrt map to the best candidate node, returning the added candidate.
func (mp *tpSpaceRRTMotionPlanner) extendMap(
	ctx context.Context,
	candidates []*candidate,
	rrt rrtMap,
	invert bool,
) (*candidate, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	var addedNode node
	// If we found any valid nodes that we can extend to, find the very best one and add that to the tree
	bestDist := math.Inf(1)
	var bestCand *candidate
	for _, cand := range candidates {
		if cand.dist < bestDist {
			bestCand = cand
			bestDist = cand.dist
		}
	}
	treeNode := bestCand.treeNode // The node already in the tree to which we are parenting
	newNode := bestCand.newNode // The node we are adding because it was the best extending PTG
	//~ fmt.Println("treenode", treeNode.Pose().Point())
	//~ fmt.Println("newnode", newNode.Pose().Point())

	ptgNum := int(newNode.Q()[0].Value)
	randAlpha := newNode.Q()[1].Value
	randDist := newNode.Q()[2].Value

	fmt.Println("traj", randAlpha, randDist)
	trajK, err := mp.tpFrame.PTGs()[ptgNum].Trajectory(randAlpha, randDist)
	if err != nil {
		return nil, err
	}

	arcStartPose := treeNode.Pose()
	if invert {
		arcStartPose = spatialmath.Compose(arcStartPose, spatialmath.PoseInverse(trajK[len(trajK) - 1].Pose))
	}
	lastDist := 0.
	sinceLastNode := 0.

	var trajState *State
	if mp.algOpts.addIntermediate {
		for i := 0; i < len(trajK); i++ {
			trajPt := trajK[i]
			if invert {
				trajPt = trajK[(len(trajK)-1)-i]
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			trajState = &State{Position: spatialmath.Compose(arcStartPose, trajPt.Pose)}
			//~ if !invert {
				//~ fmt.Printf("FINALPATH,%f,%f\n", trajState.Position.Point().X, trajState.Position.Point().Y)
			//~ }else{
				//~ fmt.Printf("FINALPATH2,%f,%f\n", trajState.Position.Point().X, trajState.Position.Point().Y)
			//~ }
			sinceLastNode += (trajPt.Dist - lastDist)

			// Optionally add sub-nodes along the way. Will make the final path a bit better
			if sinceLastNode > mp.algOpts.addNodeEvery {
				// add the last node in trajectory
				addedNode = &basicNode{
					q:      referenceframe.FloatsToInputs([]float64{float64(ptgNum), randAlpha, trajPt.Dist}),
					cost:   treeNode.Cost() + trajPt.Dist,
					pose:   trajState.Position,
					corner: false,
				}
				rrt[addedNode] = treeNode
				sinceLastNode = 0.
			}
			lastDist = trajPt.Dist
		}
		//~ fmt.Printf("WP,%f,%f\n", trajState.Position.Point().X, trajState.Position.Point().Y)
	}
	rrt[newNode] = treeNode
	return bestCand, nil
}

func (mp *tpSpaceRRTMotionPlanner) setupTPSpaceOptions() {
	tpOpt := &tpspaceOptions{
		rrtOptions: newRRTOptions(),
		goalCheck:  defaultGoalCheck,
		autoBB:     defaultAutoBB,

		addIntermediate: defaultAddInt,
		addNodeEvery:    defaultAddNodeEvery,

		dupeNodeBuffer: defaultDuplicateNodeBuffer,
		poseSolveDist: defaultIdenticalNodeDistance,
		
		maxReseeds: defaultMaxReseeds,

		distOptions: map[tpspace.PTG]*plannerOptions{},
		invertDistOptions: map[tpspace.PTG]*plannerOptions{},
	}

	for _, curPtg := range mp.tpFrame.PTGs() {
		tpOpt.distOptions[curPtg] = mp.make2DTPSpaceDistanceOptions(curPtg, tpOpt.dupeNodeBuffer, false)
		tpOpt.invertDistOptions[curPtg] = mp.make2DTPSpaceDistanceOptions(curPtg, tpOpt.dupeNodeBuffer, true)
	}

	mp.algOpts = tpOpt
}

// make2DTPSpaceDistanceOptions will create a plannerOptions object with a custom DistanceFunc constructed such that
// distances can be computed in TP space using the given PTG.
func (mp *tpSpaceRRTMotionPlanner) make2DTPSpaceDistanceOptions(ptg tpspace.PTG, min float64, invert bool) *plannerOptions {
	opts := newBasicPlannerOptions()

	segMetric := func(seg *Segment) float64 {
		if seg.StartPosition == nil || seg.EndPosition == nil {
			return math.Inf(1)
		}
		//~ fmt.Println("calc dist", seg.StartPosition.Point(), seg.EndPosition.Point())
		var relPose spatialmath.Pose
		if invert {
			relPose = spatialmath.Compose(spatialmath.PoseInverse(seg.EndPosition), seg.StartPosition)
		} else {
			relPose = spatialmath.Compose(spatialmath.PoseInverse(seg.StartPosition), seg.EndPosition)
		}
		closeNode, err := ptg.CToTP(context.Background(), relPose)
		if err != nil || closeNode == nil {
			//~ fmt.Println("inf")
			return math.Inf(1)
		}
		dist := SquaredNormSegmentMetric(&Segment{StartPosition: closeNode.Pose, EndPosition: relPose})
		//~ fmt.Println("dist", dist)
		// Note that we want to return distances in TP space here
		//~ return closeNode.Dist
		return dist
	}
	opts.DistanceFunc = segMetric
	return opts
}

func (mp *tpSpaceRRTMotionPlanner) smoothPath(ctx context.Context, path []node) []node {
	mp.logger.Info("smoothing not yet supported for tp-space")
	return path
}
