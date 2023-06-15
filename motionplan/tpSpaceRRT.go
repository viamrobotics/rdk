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

var (
	defaultGoalCheck = 5  // Check if the goal is reachable every this many iterations
	defaultAutoBB    = 1. // Automatic bounding box on driveable area as a multiple of start-goal distance

	// whether to add intermediate waypoints.
	defaultAddInt = false
	// Add a subnode every this many mm along a valid trajectory. Large values run faster, small gives better paths
	// Meaningless if the above is false.
	defaultAddNodeEvery = 50.

	// When getting the closest node to a pose, only look for nodes at least this far away.
	defaultDuplicateNodeBuffer = 50.

	// Don't add new RRT tree nodes if there is an existing node within this distance.
	defaultIdenticalNodeDistance = 5.
)

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

	return &tpspaceRRTMotionPlanner{
		planner: mp,
		algOpts: newTpspaceOptions(),
	}, nil
}

// tpspaceRRTMotionPlanner.
type tpspaceRRTMotionPlanner struct {
	*planner
	algOpts *tpspaceOptions
}

func newTpspaceOptions() *tpspaceOptions {
	return &tpspaceOptions{
		rrtOptions: newRRTOptions(),
		goalCheck:  defaultGoalCheck,
		autoBB:     defaultAutoBB,

		addIntermediate: defaultAddInt,
		addNodeEvery:    defaultAddNodeEvery,

		dupeNodeBuffer: defaultDuplicateNodeBuffer,
	}
}

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
}

// candidate is putative node which could be added to an RRT tree. It includes a distance score, the new node and its future parent.
type candidate struct {
	dist     float64
	treeNode node
	newNode  node
	err      error
}

// TODO: seed is not immediately useful for TP-space.
func (mp *tpspaceRRTMotionPlanner) plan(ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
) ([][]referenceframe.Input, error) {
	solutionChan := make(chan *rrtPlanReturn, 1)

	seedPos := spatialmath.NewZeroPose()

	startNode := newConfigurationCostNode(nil, 0, seedPos)
	goalNode := newConfigurationCostNode(nil, 0, goal)

	utils.PanicCapturingGo(func() {
		mp.rrtBackgroundRunner(ctx, seed, &rrtParallelPlannerShared{
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
			return plan.toInputs(), plan.err()
		}
		return nil, errors.New("nil tp-space plan returned, unable to complete plan")
	}
}

// rrtBackgroundRunner will execute the plan. Plan() will call rrtBackgroundRunner in a separate thread and wait for results.
// Separating this allows other things to call rrtBackgroundRunner in parallel allowing the thread-agnostic Plan to be accessible.
func (mp *tpspaceRRTMotionPlanner) rrtBackgroundRunner(
	ctx context.Context,
	_ []referenceframe.Input,
	rrt *rrtParallelPlannerShared,
) {
	defer close(rrt.solutionChan)

	tpFrame, ok := mp.frame.(tpspace.PTGProvider)
	if !ok {
		rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("frame %v must be a PTGProvider", mp.frame)}
		return
	}

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

	earlySuccess := false
	var successNode node
	iter := 0
	// While not at goal:
	// TODO: context, timeout, etc
	for i := 0; i < mp.algOpts.PlanIter; i++ {
		if ctx.Err() != nil {
			mp.logger.Debugf("TP Space RRT timed out after %d iterations", iter)
			rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("TP Space RRT timeout %w", ctx.Err()), maps: rrt.maps}
			return
		}
		// Get random cartesian configuration
		var randPos spatialmath.Pose
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
		iter++
		randPosNode := newConfigurationCostNode(nil, 0, randPos)

		candidateNodes := map[float64][2]node{}

		// Find the best traj point for each traj family, and store for later comparison
		// TODO: run in parallel
		for ptgNum, curPtg := range tpFrame.PTGs() {
			cand := mp.getExtensionCandidate(ctx, randPosNode, ptgNum, curPtg, rrt)
			if cand != nil {
				if cand.err == nil {
					atGoal := false
					// If we tried the goal and have a close-enough XY location, check if the node is good enough to be a final goal
					if tryGoal && cand.dist < mp.planOpts.GoalThreshold {
						atGoal = mp.planOpts.GoalThreshold > mp.planOpts.goalMetric(&State{Position: cand.newNode.Pose(), Frame: mp.frame})
					}
					if atGoal {
						// If we've reached the goal, break out
						// TODO: do this better
						earlySuccess = true
						rrt.maps.startMap[cand.newNode] = cand.treeNode
						successNode = cand.newNode
						break
					}
					candidateNodes[cand.dist] = [2]node{cand.treeNode, cand.newNode}
				}
			}
		}
		if earlySuccess {
			break
		}
		mp.extendMap(ctx, candidateNodes, rrt, tpFrame)
	}

	// Rebuild the path from the goal node to the start
	path := extractPath(rrt.maps.startMap, rrt.maps.goalMap, &nodePair{a: successNode})

	rrt.solutionChan <- &rrtPlanReturn{steps: path, maps: rrt.maps}
}

// closestNode will look through a set of nodes and return the one that is closest to a given pose
// A minimum distance may be passed beyond which nodes are disregarded.
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

// make2DTPSpaceDistanceOptions will create a plannerOptions object with a custom DistanceFunc constructed such that
// distances can be computed in TP space using the given PTG.
func (mp *tpspaceRRTMotionPlanner) make2DTPSpaceDistanceOptions(ptg tpspace.PTG, min float64) *plannerOptions {
	opts := newBasicPlannerOptions()

	segMet := func(seg *Segment) float64 {
		if seg.StartPosition == nil || seg.EndPosition == nil {
			return math.Inf(1)
		}
		relPose := spatialmath.Compose(spatialmath.PoseInverse(seg.StartPosition), seg.EndPosition)
		relPosePt := relPose.Point()
		nodes := ptg.WorldSpaceToTP(relPosePt.X, relPosePt.Y)
		closeNode, _ := mp.closestNode(relPose, nodes, min)
		if closeNode == nil {
			return math.Inf(1)
		}
		return closeNode.Dist
	}
	opts.DistanceFunc = segMet
	return opts
}

// getExtensionCandidate will return either nil, or the best node on a PTG to reach the desired random node and its RRT tree parent.
func (mp *tpspaceRRTMotionPlanner) getExtensionCandidate(
	ctx context.Context,
	randPosNode node,
	ptgNum int,
	curPtg tpspace.PTG,
	rrt *rrtParallelPlannerShared,
) *candidate {
	nm := &neighborManager{nCPU: mp.planOpts.NumThreads}
	nm.parallelNeighbors = 10

	var successNode node
	acceptableGoal := false
	// Make the distance function that will find the nearest RRT map node in TP-space of *this* PTG
	// TODO: cache this
	ptgDistOpt := mp.make2DTPSpaceDistanceOptions(curPtg, mp.algOpts.dupeNodeBuffer)

	// Get nearest neighbor to rand config in tree using this PTG
	nearest := nm.nearestNeighbor(ctx, ptgDistOpt, randPosNode, rrt.maps.startMap)
	if nearest == nil {
		return nil
	}

	// Get cartesian distance from NN to rand
	relPose := spatialmath.Compose(spatialmath.PoseInverse(nearest.Pose()), randPosNode.Pose())
	relPosePt := relPose.Point()

	// Convert cartesian distance to tp-space using inverse curPtg, yielding TP-space coordinates goalK and goalD
	nodes := curPtg.WorldSpaceToTP(relPosePt.X, relPosePt.Y)
	bestNode, bestDist := mp.closestNode(relPose, nodes, mp.algOpts.dupeNodeBuffer)
	if bestNode == nil {
		return nil
	}
	goalK := bestNode.K
	goalD := bestNode.Dist
	// Check collisions along this traj and get the longest distance viable
	trajK := curPtg.Trajectory(goalK)
	pass := true
	var lastNode *tpspace.TrajNode
	var lastPose spatialmath.Pose

	sinceLastCollideCheck := 0.
	lastDist := 0.

	// Check each point along the trajectory to confirm constraints are met
	for _, trajPt := range trajK {
		if trajPt.Dist > goalD {
			// After we've passed randD, no need to keep checking, just add to RRT tree
			break
		}
		sinceLastCollideCheck += (trajPt.Dist - lastDist)
		trajState := &State{Position: spatialmath.Compose(nearest.Pose(), trajPt.Pose), Frame: mp.frame}
		if sinceLastCollideCheck > mp.planOpts.Resolution {
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
		successNode = newConfigurationCostNode(
			referenceframe.FloatsToInputs([]float64{float64(ptgNum), float64(goalK), lastNode.Dist}),
			nearest.Cost()+lastNode.Dist,
			lastPose,
		)
	} else {
		return nil
	}

	cand := &candidate{dist: bestDist, treeNode: nearest, newNode: successNode}

	if !acceptableGoal && successNode != nil {
		// check if this  successNode is too close to nodes already in the tree, and if so, do not add.

		// Get nearest neighbor to new node that's already in the tree
		nearest := nm.nearestNeighbor(ctx, mp.planOpts, successNode, rrt.maps.startMap)
		dist := math.Inf(1)
		if nearest != nil {
			dist = mp.planOpts.DistanceFunc(&Segment{StartPosition: successNode.Pose(), EndPosition: nearest.Pose()})
		}

		// Ensure successNode is sufficiently far from the nearest node already existing in the tree
		// If too close, don't add a new node
		if dist < defaultIdenticalNodeDistance {
			cand = nil
		}
	}
	return cand
}

// extendMap grows the rrt map to the best candidate node if it is valid to do so.
func (mp *tpspaceRRTMotionPlanner) extendMap(
	ctx context.Context,
	candidateNodes map[float64][2]node,
	rrt *rrtParallelPlannerShared,
	tpFrame tpspace.PTGProvider,
) {
	var addedNode node
	// If we found any valid nodes that we can extend to, find the very best one and add that to the tree
	if len(candidateNodes) > 0 {
		bestDist := math.Inf(1)
		var bestNode [2]node
		for k, v := range candidateNodes {
			if k < bestDist {
				bestNode = v
				bestDist = k
			}
		}

		ptgNum := int(bestNode[1].Q()[0].Value)
		randK := uint(bestNode[1].Q()[1].Value)

		trajK := tpFrame.PTGs()[ptgNum].Trajectory(randK)

		lastDist := 0.
		sinceLastNode := 0.

		for _, trajPt := range trajK {
			if ctx.Err() != nil {
				return
			}
			if trajPt.Dist > bestNode[1].Q()[2].Value {
				// After we've passed goalD, no need to keep checking, just add to RRT tree
				break
			}
			trajState := &State{Position: spatialmath.Compose(bestNode[0].Pose(), trajPt.Pose)}
			sinceLastNode += (trajPt.Dist - lastDist)

			// Optionally add sub-nodes along the way. Will make the final path a bit better
			if mp.algOpts.addIntermediate && sinceLastNode > mp.algOpts.addNodeEvery {
				// add the last node in trajectory
				addedNode = newConfigurationCostNode(
					referenceframe.FloatsToInputs([]float64{float64(ptgNum), float64(randK), trajPt.Dist}),
					bestNode[0].Cost()+trajPt.Dist,
					trajState.Position,
				)
				rrt.maps.startMap[addedNode] = bestNode[0]
				sinceLastNode = 0.
			}
			lastDist = trajPt.Dist
		}
		rrt.maps.startMap[bestNode[1]] = bestNode[0]
	}
}
