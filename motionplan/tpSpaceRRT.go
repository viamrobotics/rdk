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
	defaultAddInt = false
	// Add a subnode every this many mm along a valid trajectory. Large values run faster, small gives better paths
	// Meaningless if the above is false.
	defaultAddNodeEvery = 50.

	// When getting the closest node to a pose, only look for nodes at least this far away.
	defaultDuplicateNodeBuffer = 50.

	// Don't add new RRT tree nodes if there is an existing node within this distance.
	defaultIdenticalNodeDistance = 5.

	// Default distance in mm to get within for tp-space trajectories.
	defaultTPSpaceGoalDist = 10.
)

type tpspaceOptions struct {
	goalCheck int // Check if goal is reachable every this many iters

	// TODO: base this on frame limits?
	autoBB float64 // Automatic bounding box on driveable area as a multiple of start-goal distance

	addIntermediate bool // whether to add intermediate waypoints.
	// Add a subnode every this many mm along a valid trajectory. Large values run faster, small gives better paths
	// Meaningless if the above is false.
	addNodeEvery float64

	// Don't add new RRT tree nodes if there is an existing node within this distance
	dupeNodeBuffer float64

	// Cached functions for calculating TP-space distances for each PTG
	distOptions map[tpspace.PTG]*plannerOptions
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
	midPt := goalPose.Point().Sub(startPose.Point())

	var randPos spatialmath.Pose
	for iter := 0; iter < mp.planOpts.PlanIter; iter++ {
		if ctx.Err() != nil {
			mp.logger.Debugf("TP Space RRT timed out after %d iterations", iter)
			rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("TP Space RRT timeout %w", ctx.Err()), maps: rrt.maps}
			return
		}
		// Get random cartesian configuration
		tryGoal := true
		// Check if we can reach the goal every N iters
		if iter%mp.algOpts.goalCheck != 0 {
			rDist := dist * (mp.algOpts.autoBB + float64(iter)/10.)
			tryGoal = false
			randPosX := float64(mp.randseed.Intn(int(rDist)))
			randPosY := float64(mp.randseed.Intn(int(rDist)))
			randPosTheta := math.Pi * (mp.randseed.Float64() - 0.5)
			randPos = spatialmath.NewPose(
				r3.Vector{midPt.X + (randPosX - rDist/2.), midPt.Y + (randPosY - rDist/2.), 0},
				&spatialmath.OrientationVector{OZ: 1, Theta: randPosTheta},
			)
		} else {
			randPos = goalPose
		}
		randPosNode := &basicNode{q: nil, cost: 0, pose: randPos, corner: false}

		successNode := mp.attemptExtension(ctx, nil, randPosNode, rrt)
		// If we tried the goal and have a close-enough XY location, check if the node is good enough to be a final goal
		if tryGoal && successNode != nil {
			if mp.planOpts.GoalThreshold > mp.planOpts.goalMetric(&State{Position: successNode.Pose(), Frame: mp.frame}) {
				// If we've reached the goal, break out
				path := extractPath(rrt.maps.startMap, rrt.maps.goalMap, &nodePair{a: successNode})
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
	rrt *rrtParallelPlannerShared,
	nearest node,
	planOpts *plannerOptions, // Need to pass this in explicitly for smoothing
) *candidate {
	nm := &neighborManager{nCPU: mp.planOpts.NumThreads}
	nm.parallelNeighbors = 10

	var successNode node
	// Get the distance function that will find the nearest RRT map node in TP-space of *this* PTG
	ptgDistOpt := mp.algOpts.distOptions[curPtg]

	if nearest == nil {
		// Get nearest neighbor to rand config in tree using this PTG
		nearest = nm.nearestNeighbor(ctx, ptgDistOpt, randPosNode, rrt.maps.startMap)
		if nearest == nil {
			return nil
		}
	}

	// Get cartesian distance from NN to rand
	relPose := spatialmath.Compose(spatialmath.PoseInverse(nearest.Pose()), randPosNode.Pose())
	relPosePt := relPose.Point()

	// Convert cartesian distance to tp-space using inverse curPtg, yielding TP-space coordinates goalK and goalD
	nodes := curPtg.CToTP(relPosePt.X, relPosePt.Y)
	bestNode, bestDist := mp.closestNode(relPose, nodes, mp.algOpts.dupeNodeBuffer)
	if bestNode == nil {
		return nil
	}
	goalK := bestNode.K
	goalD := bestNode.Dist
	// Check collisions along this traj and get the longest distance viable
	trajK := curPtg.Trajectory(goalK)
	var lastNode *tpspace.TrajNode
	var lastPose spatialmath.Pose

	sinceLastCollideCheck := 0.
	lastDist := 0.

	isLastNode := true
	// Check each point along the trajectory to confirm constraints are met
	for _, trajPt := range trajK {
		if trajPt.Dist > goalD {
			// After we've passed randD, no need to keep checking, just add to RRT tree
			isLastNode = false
			break
		}
		sinceLastCollideCheck += (trajPt.Dist - lastDist)
		trajState := &State{Position: spatialmath.Compose(nearest.Pose(), trajPt.Pose), Frame: mp.frame}
		if sinceLastCollideCheck > planOpts.Resolution {
			ok, _ := planOpts.CheckStateConstraints(trajState)
			if !ok {
				return nil
			}
			sinceLastCollideCheck = 0.
		}

		lastPose = trajState.Position
		lastNode = trajPt
		lastDist = lastNode.Dist
	}
	// add the last node in trajectory
	successNode = &basicNode{
		q:      referenceframe.FloatsToInputs([]float64{float64(ptgNum), float64(goalK), lastNode.Dist}),
		cost:   nearest.Cost() + lastNode.Dist,
		pose:   lastPose,
		corner: false,
	}

	cand := &candidate{dist: bestDist, treeNode: nearest, newNode: successNode, lastInTraj: isLastNode}

	// check if this  successNode is too close to nodes already in the tree, and if so, do not add.
	// Get nearest neighbor to new node that's already in the tree
	nearest = nm.nearestNeighbor(ctx, planOpts, successNode, rrt.maps.startMap)
	if nearest != nil {
		dist := planOpts.DistanceFunc(&Segment{StartPosition: successNode.Pose(), EndPosition: nearest.Pose()})
		// Ensure successNode is sufficiently far from the nearest node already existing in the tree
		// If too close, don't add a new node
		if dist < defaultIdenticalNodeDistance {
			cand = nil
		}
	}
	return cand
}

// attemptExtension will attempt to extend the rrt map towards the goal node, and will return the candidate added to the map that is
// closest to that goal.
func (mp *tpSpaceRRTMotionPlanner) attemptExtension(
	ctx context.Context,
	seedNode,
	goalNode node,
	rrt *rrtParallelPlannerShared,
) node {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		candidates := []*candidate{}
		for ptgNum, curPtg := range mp.tpFrame.PTGs() {
			// Find the best traj point for each traj family, and store for later comparison
			// TODO: run in parallel
			cand := mp.getExtensionCandidate(ctx, goalNode, ptgNum, curPtg, rrt, seedNode, mp.planOpts)
			if cand != nil {
				if cand.err == nil {
					candidates = append(candidates, cand)
				}
			}
		}
		reseedCandidate := mp.extendMap(ctx, candidates, rrt)
		if reseedCandidate == nil {
			return nil
		}
		dist := mp.planOpts.DistanceFunc(&Segment{StartPosition: reseedCandidate.newNode.Pose(), EndPosition: goalNode.Pose()})
		if dist < mp.planOpts.GoalThreshold || !reseedCandidate.lastInTraj {
			// Reached the goal position, or otherwise failed to fully extend to the end of a trajectory
			return reseedCandidate.newNode
		}
		seedNode = reseedCandidate.newNode
	}
}

// extendMap grows the rrt map to the best candidate node if it is valid to do so, returning the added candidate.
func (mp *tpSpaceRRTMotionPlanner) extendMap(
	ctx context.Context,
	candidates []*candidate,
	rrt *rrtParallelPlannerShared,
) *candidate {
	if len(candidates) == 0 {
		return nil
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
	treeNode := bestCand.treeNode
	newNode := bestCand.newNode

	ptgNum := int(newNode.Q()[0].Value)
	randK := uint(newNode.Q()[1].Value)

	trajK := mp.tpFrame.PTGs()[ptgNum].Trajectory(randK)

	lastDist := 0.
	sinceLastNode := 0.

	for _, trajPt := range trajK {
		if ctx.Err() != nil {
			return nil
		}
		if trajPt.Dist > newNode.Q()[2].Value {
			// After we've passed goalD, no need to keep checking, just add to RRT tree
			break
		}
		trajState := &State{Position: spatialmath.Compose(treeNode.Pose(), trajPt.Pose)}
		sinceLastNode += (trajPt.Dist - lastDist)

		// Optionally add sub-nodes along the way. Will make the final path a bit better
		if mp.algOpts.addIntermediate && sinceLastNode > mp.algOpts.addNodeEvery {
			// add the last node in trajectory
			addedNode = &basicNode{
				q:      referenceframe.FloatsToInputs([]float64{float64(ptgNum), float64(randK), trajPt.Dist}),
				cost:   treeNode.Cost() + trajPt.Dist,
				pose:   trajState.Position,
				corner: false,
			}
			rrt.maps.startMap[addedNode] = treeNode
			sinceLastNode = 0.
		}
		lastDist = trajPt.Dist
	}
	rrt.maps.startMap[newNode] = treeNode
	return bestCand
}

func (mp *tpSpaceRRTMotionPlanner) setupTPSpaceOptions() {
	tpOpt := &tpspaceOptions{
		goalCheck: defaultGoalCheck,
		autoBB:    defaultAutoBB,

		addIntermediate: defaultAddInt,
		addNodeEvery:    defaultAddNodeEvery,

		dupeNodeBuffer: defaultDuplicateNodeBuffer,

		distOptions: map[tpspace.PTG]*plannerOptions{},
	}

	for _, curPtg := range mp.tpFrame.PTGs() {
		tpOpt.distOptions[curPtg] = mp.make2DTPSpaceDistanceOptions(curPtg, tpOpt.dupeNodeBuffer)
	}

	mp.algOpts = tpOpt
}

// closestNode will look through a set of nodes and return the one that is closest to a given pose
// A minimum distance may be passed beyond which nodes are disregarded.
func (mp *tpSpaceRRTMotionPlanner) closestNode(pose spatialmath.Pose, nodes []*tpspace.TrajNode, min float64) (*tpspace.TrajNode, float64) {
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

// make2DTPSpaceDistanceOptions will create a plannerOptions object with a custom DistanceFunc constructed such that
// distances can be computed in TP space using the given PTG.
func (mp *tpSpaceRRTMotionPlanner) make2DTPSpaceDistanceOptions(ptg tpspace.PTG, min float64) *plannerOptions {
	opts := newBasicPlannerOptions(mp.frame)

	segMet := func(seg *Segment) float64 {
		if seg.StartPosition == nil || seg.EndPosition == nil {
			return math.Inf(1)
		}
		relPose := spatialmath.Compose(spatialmath.PoseInverse(seg.StartPosition), seg.EndPosition)
		relPosePt := relPose.Point()
		nodes := ptg.CToTP(relPosePt.X, relPosePt.Y)
		closeNode, _ := mp.closestNode(relPose, nodes, min)
		if closeNode == nil {
			return math.Inf(1)
		}
		return closeNode.Dist
	}
	opts.DistanceFunc = segMet
	return opts
}

func (mp *tpSpaceRRTMotionPlanner) smoothPath(ctx context.Context, path []node) []node {
	mp.logger.Info("smoothing not yet supported for tp-space")
	return path
}
