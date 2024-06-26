//go:build !windows && !no_cgo

package motionplan

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/golang/geo/r3"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	// Automatic bounding box on driveable area as a multiple of start-goal distance.
	defaultAutoBB = 1.0
	// Note: while fully holonomic planners can use the limits of the frame as implicit boundaries, with non-holonomic motion
	// this is not the case, and the total workspace available to the planned frame is not directly related to the motion available
	// from a single set of inputs.

	// How much the bounding box of random points to sample increases in size with each algorithm iteration.
	autoBBscale = 0.1

	// Add a subnode every this many mm along a valid trajectory. Large values run faster, small gives better paths
	// Meaningless if the above is false.
	defaultAddNodeEvery = 1000.

	// Don't add new RRT tree nodes if there is an existing node within this distance. This is the distance determined by the DistanceFunc,
	// so is the sum of the square of the distance in mm, and the orientation distance accounting for scale adjustment.
	// Note that since the orientation adjustment is very large, this must be as well.
	defaultIdenticalNodeDistance = 4000.

	// When extending the RRT tree towards some point, do not extend more than this many times in a single RRT invocation.
	defaultMaxReseeds = 20

	// Make an attempt to solve the tree every this many iterations
	// For a unidirectional solve, this means attempting to reach the goal rather than a random point
	// For a bidirectional solve, this means trying to connect the two trees directly.
	defaultAttemptSolveEvery = 20

	// When attempting a solve per the above, make no more than this many tries. Preserves performance with large trees.
	defaultMaxConnectAttempts = 20

	// When checking a PTG for validity and finding a collision, using the last good configuration will result in a highly restricted
	// node that is directly facing a wall. To prevent this, we walk back along the trajectory by this percentage of the traj length
	// so that the node we add has more freedom of movement to extend in the future.
	defaultCollisionWalkbackPct = 0.8

	// When evaluating the partial node to add to a tree after defaultCollisionWalkbackPct is applied, ensure the trajectory is still at
	// least this long.
	defaultMinTrajectoryLength = 450

	// When smoothing, each piece of the path up to the first index will be broken into this many sub-nodes to form the new start tree.
	// A larger number gives more nucleation points for solving, but makes solving run slower.
	defaultSmoothChunkCount = 6

	// Print very fine-grained debug info. Useful for observing the inner RRT tree structure directly.
	pathdebug = false
)

// Using the standard SquaredNormMetric, we run into issues where far apart distances will underflow gradient calculations.
// This metric, used only for gradient descent, computes the gradient using centimeters rather than millimeters allowing for smaller
// values that do not underflow.
var defaultGoalMetricConstructor = ik.NewPosWeightSquaredNormMetric

// Used to flip goal nodes so they can solve forwards.
var flipPose = spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 180})

type tpspaceOptions struct {
	// TODO: base this on frame limits?
	autoBB float64 // Automatic bounding box on driveable area as a multiple of start-goal distance

	// Add a subnode every this many mm along a valid trajectory. Large values run faster, small gives better paths
	// Meaningless if the above is false.
	addNodeEvery float64

	// Don't add new RRT tree nodes if there is an existing node within this distance.
	identicalNodeDistance float64

	// Make an attempt to solve the tree every this many iterations
	// For a unidirectional solve, this means attempting to reach the goal rather than a random point
	// For a bidirectional solve, this means trying to connect the two trees directly
	attemptSolveEvery int

	goalMetricConstructor func(spatialmath.Pose) ik.StateMetric

	// Cached functions for calculating TP-space distances for each PTG
	distOptions map[tpspace.PTG]*plannerOptions
}

// candidate is putative node which could be added to an RRT tree. It includes a distance score, the new node and its future parent.
type candidate struct {
	dist     float64
	treeNode node
	newNodes []node
	err      error
}

type nodeAndError struct {
	node
	error
}

// tpSpaceRRTMotionPlanner.
type tpSpaceRRTMotionPlanner struct {
	*planner
	algOpts *tpspaceOptions
	tpFrame tpspace.PTGProvider

	// This tracks the nodes added to the goal tree in an ordered fashion. Nodes will always be added to this slice in the
	// same order, yielding deterministic results when the goal tree is iterated over.
	goalNodes []node
}

// newTPSpaceMotionPlanner creates a newTPSpaceMotionPlanner object with a user specified random seed.
func newTPSpaceMotionPlanner(
	frame referenceframe.Frame,
	seed *rand.Rand,
	logger logging.Logger,
	opt *plannerOptions,
) (motionPlanner, error) {
	if opt == nil {
		return nil, errNoPlannerOptions
	}

	mp, err := newPlanner(frame, seed, logger, opt)
	if err != nil {
		return nil, err
	}

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
func (mp *tpSpaceRRTMotionPlanner) plan(ctx context.Context, goal spatialmath.Pose, seed []referenceframe.Input) ([]node, error) {
	mp.planOpts.SetGoal(goal)
	solutionChan := make(chan *rrtSolution, 1)

	seedPos := mp.opt().StartPose

	startNode := &basicNode{q: make([]referenceframe.Input, len(mp.frame.DoF())), cost: 0, pose: seedPos, corner: false}
	maps := &rrtMaps{startMap: map[node]node{startNode: nil}}
	if mp.opt().PositionSeeds > 0 && mp.opt().profile == PositionOnlyMotionProfile {
		err := maps.fillPosOnlyGoal(goal, mp.opt().PositionSeeds, len(mp.frame.DoF()))
		if err != nil {
			return nil, err
		}
	} else {
		goalNode := &basicNode{
			q:      make([]referenceframe.Input, len(mp.frame.DoF())),
			cost:   0,
			pose:   spatialmath.Compose(goal, flipPose),
			corner: false,
		}
		maps.goalMap = map[node]node{goalNode: nil}
	}

	var planRunners sync.WaitGroup

	planRunners.Add(1)
	utils.PanicCapturingGo(func() {
		defer planRunners.Done()
		mp.rrtBackgroundRunner(ctx, seed, &rrtParallelPlannerShared{maps, nil, solutionChan})
	})
	select {
	case <-ctx.Done():
		planRunners.Wait()
		return nil, ctx.Err()
	case solution := <-solutionChan:
		if solution != nil {
			return solution.steps, solution.err
		}
		return nil, errors.New("nil tp-space plan returned, unable to complete plan")
	}
}

// planRunner will execute the plan. Plan() will call planRunner in a separate thread and wait for results.
// Separating this allows other things to call planRunner in parallel allowing the thread-agnostic Plan to be accessible.
func (mp *tpSpaceRRTMotionPlanner) rrtBackgroundRunner(
	ctx context.Context,
	_ []referenceframe.Input, // TODO: this may be needed for smoothing
	rrt *rrtParallelPlannerShared,
) {
	defer close(rrt.solutionChan)
	// get start and goal poses
	var startPose spatialmath.Pose
	var goalPose spatialmath.Pose
	var goalNode node

	goalScore := math.Inf(1)
	for k, v := range rrt.maps.startMap {
		if v == nil {
			if k.Pose() != nil {
				startPose = k.Pose()
			} else {
				rrt.solutionChan <- &rrtSolution{err: fmt.Errorf("node %v must provide a Pose", k)}
				return
			}
			break
		}
	}
	for k, v := range rrt.maps.goalMap {
		if v == nil {
			// There may be more than one node in the tree which satisfies the goal, i.e. its parent is nil.
			// However for the purposes of this we can just take the first one we see.
			if k.Pose() != nil {
				dist := mp.planOpts.DistanceFunc(&ik.Segment{StartPosition: startPose, EndPosition: k.Pose()})
				if dist < goalScore {
					// Update to use the closest goal to the start.
					// This is necessary in order to solve deterministically.
					goalPose = k.Pose()
					goalScore = dist
					goalNode = k
				}
			} else {
				rrt.solutionChan <- &rrtSolution{err: fmt.Errorf("node %v must provide a Pose", k)}
				return
			}
		}
	}
	mp.goalNodes = append(mp.goalNodes, goalNode)
	mp.logger.CDebugf(ctx, "Starting TPspace solving with startMap len %d and goalMap len %d", len(rrt.maps.startMap), len(rrt.maps.goalMap))

	publishFinishedPath := func(path []node) {
		// If we've reached the goal, extract the path from the RRT trees and return
		correctedPath, err := rectifyTPspacePath(path, mp.frame, startPose)
		if err != nil {
			rrt.solutionChan <- &rrtSolution{err: err, maps: rrt.maps}
			return
		}

		// Print debug info if requested
		if pathdebug {
			allPtgs := mp.tpFrame.PTGSolvers()
			lastPose := startPose
			for _, mynode := range correctedPath {
				trajPts, err := allPtgs[int(mynode.Q()[0].Value)].Trajectory(
					mynode.Q()[1].Value,
					mynode.Q()[2].Value,
					mynode.Q()[3].Value,
					mp.planOpts.Resolution,
				)
				if err != nil {
					// Unimportant; this is just for debug visualization
					break
				}
				for i, pt := range trajPts {
					intPose := spatialmath.Compose(lastPose, pt.Pose)
					if i == 0 {
						mp.logger.Debugf("$WP,%f,%f", intPose.Point().X, intPose.Point().Y)
					}
					mp.logger.Debugf("$FINALPATH,%f,%f", intPose.Point().X, intPose.Point().Y)
					if i == len(trajPts)-1 {
						lastPose = intPose
						break
					}
				}
			}
		}
		rrt.solutionChan <- &rrtSolution{steps: correctedPath, maps: rrt.maps}
	}

	m1chan := make(chan *nodeAndError, 1)
	m2chan := make(chan *nodeAndError, 1)
	defer close(m1chan)
	defer close(m2chan)

	// The midpoint should not be the 50% interpolation of start/goal poses, but should be the 50% interpolated point with the orientation
	// pointing at the goal from the start
	midPt := startPose.Point().Add(goalPose.Point()).Mul(0.5)
	midPtNormalized := midPt.Sub(startPose.Point())
	midOrient := &spatialmath.OrientationVector{OZ: 1, Theta: math.Atan2(-midPtNormalized.X, midPtNormalized.Y)}

	midptNode := &basicNode{pose: spatialmath.NewPose(midPt, midOrient), cost: midPt.Sub(startPose.Point()).Norm()}
	var randPosNode node = midptNode

	for iter := 0; iter < mp.planOpts.PlanIter; iter++ {
		if pathdebug {
			mp.logger.Debugf("$RRTGOAL,%f,%f", randPosNode.Pose().Point().X, randPosNode.Pose().Point().Y)
		}
		mp.logger.CDebugf(ctx, "TP Space RRT iteration %d", iter)
		if ctx.Err() != nil {
			mp.logger.CDebugf(ctx, "TP Space RRT timed out after %d iterations", iter)
			rrt.solutionChan <- &rrtSolution{err: fmt.Errorf("TP Space RRT timeout %w", ctx.Err()), maps: rrt.maps}
			return
		}
		utils.PanicCapturingGo(func() {
			m1chan <- mp.attemptExtension(ctx, randPosNode, rrt.maps.startMap, false)
		})
		utils.PanicCapturingGo(func() {
			m2chan <- mp.attemptExtension(ctx, flipNode(randPosNode), rrt.maps.goalMap, true)
		})
		seedReached := <-m1chan
		goalReached := <-m2chan

		err := multierr.Combine(seedReached.error, goalReached.error)
		if err != nil {
			rrt.solutionChan <- &rrtSolution{err: err, maps: rrt.maps}
			return
		}

		var reachedDelta float64
		if seedReached.node != nil && goalReached.node != nil {
			// Flip the orientation of the goal node for distance calculation and seed extension
			reachedDelta = mp.planOpts.DistanceFunc(&ik.Segment{
				StartPosition: seedReached.node.Pose(),
				EndPosition:   flipNode(goalReached.node).Pose(),
			})
			if reachedDelta > mp.planOpts.GoalThreshold {
				// If both maps extended, but did not reach the same point, then attempt to extend them towards each other
				seedReached = mp.attemptExtension(ctx, flipNode(goalReached.node), rrt.maps.startMap, false)
				if seedReached.error != nil {
					rrt.solutionChan <- &rrtSolution{err: seedReached.error, maps: rrt.maps}
					return
				}
				if seedReached.node != nil {
					reachedDelta = mp.planOpts.DistanceFunc(&ik.Segment{
						StartPosition: seedReached.node.Pose(),
						EndPosition:   flipNode(goalReached.node).Pose(),
					})
					if reachedDelta > mp.planOpts.GoalThreshold {
						goalReached = mp.attemptExtension(ctx, flipNode(seedReached.node), rrt.maps.goalMap, true)
						if goalReached.error != nil {
							rrt.solutionChan <- &rrtSolution{err: goalReached.error, maps: rrt.maps}
							return
						}
					}
					if goalReached.node != nil {
						reachedDelta = mp.planOpts.DistanceFunc(&ik.Segment{
							StartPosition: seedReached.node.Pose(),
							EndPosition:   flipNode(goalReached.node).Pose(),
						})
					}
				}
			}
			if reachedDelta <= mp.planOpts.GoalThreshold {
				// If we've reached the goal, extract the path from the RRT trees and return
				path := extractTPspacePath(rrt.maps.startMap, rrt.maps.goalMap, &nodePair{a: seedReached.node, b: goalReached.node})
				publishFinishedPath(path)
				return
			}
		}
		if iter%mp.algOpts.attemptSolveEvery == mp.algOpts.attemptSolveEvery-1 {
			// Attempt a solve; we iterate through our goal tree and attempt to find any connection to the seed tree
			paths := [][]node{}

			// Exhaustively searching the tree gets expensive quickly, so we cap the number of connect attempts we make each time we call
			// this.
			attempts := 0                                                                     // Track the number of connection attempts we have made
			pctCheck := 100 * float64(defaultMaxConnectAttempts) / float64(len(mp.goalNodes)) // Target checking this proportion of nodes.

			for _, goalMapNode := range mp.goalNodes {
				if ctx.Err() != nil {
					rrt.solutionChan <- &rrtSolution{err: fmt.Errorf("TP Space RRT timeout %w", ctx.Err()), maps: rrt.maps}
					return
				}

				// Exhaustively iterating the goal map gets *very* expensive, so we only iterate a given number of times
				if attempts > defaultMaxConnectAttempts {
					break
				}
				if pctCheck < 100. { // If we're not checking every node, see if this node is one of the ones we want to check.
					doCheck := 100 * mp.randseed.Float64()
					if doCheck < pctCheck {
						continue
					}
				}
				attempts++

				seedReached := mp.attemptExtension(ctx, flipNode(goalMapNode), rrt.maps.startMap, false)
				if seedReached.error != nil {
					rrt.solutionChan <- &rrtSolution{err: seedReached.error, maps: rrt.maps}
					return
				}
				if seedReached.node == nil {
					continue
				}
				reachedDelta = mp.planOpts.DistanceFunc(&ik.Segment{
					StartPosition: seedReached.node.Pose(),
					EndPosition:   flipNode(goalMapNode).Pose(),
				})
				if reachedDelta <= mp.planOpts.GoalThreshold {
					// If we've reached the goal, extract the path from the RRT trees and return
					path := extractTPspacePath(rrt.maps.startMap, rrt.maps.goalMap, &nodePair{a: seedReached.node, b: goalMapNode})
					paths = append(paths, path)
				}
			}
			mp.goalNodes = []node{goalNode}
			if len(paths) > 0 {
				var bestPath []node
				bestCost := math.Inf(1)
				for _, goodPath := range paths {
					currCost := sumCosts(goodPath)
					if currCost < bestCost {
						bestCost = currCost
						bestPath = goodPath
					}
				}
				publishFinishedPath(bestPath)
				return
			}
		}

		// Get random cartesian configuration
		randPosNode, err = mp.sample(midptNode, iter)
		if err != nil {
			rrt.solutionChan <- &rrtSolution{err: err, maps: rrt.maps}
			return
		}
	}
	rrt.solutionChan <- &rrtSolution{maps: rrt.maps, err: errors.New("tpspace RRT unable to create valid path")}
}

// getExtensionCandidate will return either nil, or the best node on a valid PTG to reach the desired random node and its RRT tree parent.
func (mp *tpSpaceRRTMotionPlanner) getExtensionCandidate(
	ctx context.Context,
	randPosNode node,
	ptgNum int,
	curPtg tpspace.PTGSolver,
	rrt rrtMap,
	nearest node,
) (*candidate, error) {
	// Get the distance function that will find the nearest RRT map node in TP-space of *this* PTG
	ptgDistOpt, distMap := mp.make2DTPSpaceDistanceOptions(curPtg)

	nm := &neighborManager{nCPU: mp.planOpts.NumThreads / len(mp.tpFrame.PTGSolvers())}
	nm.parallelNeighbors = 10

	var successNode node

	var solution *ik.Solution
	var targetFunc ik.StateMetric
	if nearest == nil {
		// Get nearest neighbor to rand config in tree using this PTG
		nearest = nm.nearestNeighbor(ctx, ptgDistOpt, randPosNode, rrt)
		if nearest == nil {
			return nil, errNoNeighbors
		}

		rawVal, ok := distMap.Load(nearest.Pose())
		if !ok {
			mp.logger.Error("nearest neighbor failed to find nearest pose in distMap")
			return nil, errNoNeighbors
		}

		solution, ok = rawVal.(*ik.Solution)
		if !ok {
			mp.logger.Error("nearest neighbor ik.Solution type conversion failed")
			return nil, errNoNeighbors
		}

		relPose := spatialmath.PoseBetween(nearest.Pose(), randPosNode.Pose())
		targetFunc = mp.algOpts.goalMetricConstructor(relPose)
	} else {
		ptgSolution, ptgTargetFunc, err := mp.ptgSolutionAndMetric(curPtg, nearest.Pose(), randPosNode.Pose())
		if err != nil || ptgSolution == nil {
			return nil, err
		}
		solution = ptgSolution
		targetFunc = ptgTargetFunc
	}
	// TODO: We could potentially improve solving by first getting the rough distance to the randPosNode to any point in the rrt tree,
	// then dynamically expanding or contracting the limits of IK to be some fraction of that distance.

	// Get cartesian distance from NN to rand
	arcStartPose := nearest.Pose()
	successNodes := []node{}
	arcPose := spatialmath.NewZeroPose() // This will be the relative pose that is the delta from one end of the combined traj to the other.
	// We may produce more than one consecutive arc. Reduce the one configuration to several 2dof arcs
	for i := 0; i < len(solution.Configuration); i += 2 {
		subNode := newConfigurationNode(solution.Configuration[i : i+2])

		// Check collisions along this traj and get the longest distance viable
		trajK, err := curPtg.Trajectory(subNode.Q()[0].Value, 0, subNode.Q()[1].Value, mp.planOpts.Resolution)
		if err != nil {
			return nil, err
		}
		goodNode := mp.checkTraj(trajK, arcStartPose)
		if goodNode == nil {
			break
		}
		partialExtend := false

		for i, val := range subNode.Q() {
			if goodNode.Q()[i] != val {
				partialExtend = true
			}
		}
		arcPose = spatialmath.Compose(arcPose, goodNode.Pose())

		// add the last node in trajectory
		arcStartPose = spatialmath.Compose(arcStartPose, goodNode.Pose())
		successNode = &basicNode{
			q:      []referenceframe.Input{{float64(ptgNum)}, goodNode.Q()[0], {0}, goodNode.Q()[1]},
			cost:   goodNode.Cost(),
			pose:   arcStartPose,
			corner: false,
		}
		successNodes = append(successNodes, successNode)
		if partialExtend {
			break
		}
	}

	if len(successNodes) == 0 {
		return nil, errInvalidCandidate
	}

	bestDist := targetFunc(&ik.State{Position: arcPose})

	cand := &candidate{dist: bestDist, treeNode: nearest, newNodes: successNodes}
	// check if this  successNode is too close to nodes already in the tree, and if so, do not add.
	// Get nearest neighbor to new node that's already in the tree. Note that this uses cartesian distance (planOpts.DistanceFunc) rather
	// than the TP-space distance functions in algOpts.
	nearest = nm.nearestNeighbor(ctx, mp.planOpts, successNode, rrt)
	if nearest != nil {
		dist := mp.planOpts.DistanceFunc(&ik.Segment{StartPosition: successNode.Pose(), EndPosition: nearest.Pose()})
		// Ensure successNode is sufficiently far from the nearest node already existing in the tree
		// If too close, don't add a new node
		if dist < mp.algOpts.identicalNodeDistance {
			cand = nil
		}
	}
	return cand, nil
}

// Check our constraints (mainly collision) and return a valid node to add, or nil if no nodes along the traj are valid.
func (mp *tpSpaceRRTMotionPlanner) checkTraj(trajK []*tpspace.TrajNode, arcStartPose spatialmath.Pose) node {
	passed := []node{}
	// Check each point along the trajectory to confirm constraints are met
	for i := 0; i < len(trajK); i++ {
		trajPt := trajK[i]

		trajState := &ik.State{Position: spatialmath.Compose(arcStartPose, trajPt.Pose), Frame: mp.frame}
		// In addition to checking every `Resolution`, we also check both endpoints.
		ok, _ := mp.planOpts.CheckStateConstraints(trajState)
		if !ok {
			okDist := trajPt.Dist * defaultCollisionWalkbackPct
			if okDist > defaultMinTrajectoryLength {
				// Check that okDist is larger than the minimum distance to move to add a partial trajectory.
				for i := len(passed) - 1; i > 0; i-- {
					if passed[i].Cost() < defaultMinTrajectoryLength {
						break
					}
					// Return the most recent node whose dist is less than okDist and larger than defaultMinTrajectoryLength
					if passed[i].Cost() < okDist {
						return passed[i]
					}
				}
			}
			return nil
		}

		okNode := &basicNode{
			q:    []referenceframe.Input{{trajPt.Alpha}, {trajPt.Dist}},
			cost: trajPt.Dist,
			pose: trajPt.Pose,
		}
		passed = append(passed, okNode)
	}
	return &basicNode{
		q:    []referenceframe.Input{{trajK[(len(trajK) - 1)].Alpha}, {trajK[(len(trajK) - 1)].Dist}},
		cost: trajK[(len(trajK) - 1)].Dist,
		pose: passed[len(passed)-1].Pose(),
	}
}

// attemptExtension will attempt to extend the rrt map towards the goal node, and will return the candidate added to the map that is
// closest to that goal.
func (mp *tpSpaceRRTMotionPlanner) attemptExtension(
	ctx context.Context,
	goalNode node,
	rrt rrtMap,
	isGoalTree bool,
) *nodeAndError {
	var reseedCandidate *candidate
	var seedNode node
	maxReseeds := 1 // Will be updated as necessary
	lastIteration := false
	candChan := make(chan *candidate, len(mp.tpFrame.PTGSolvers()))
	defer close(candChan)
	var activeSolvers sync.WaitGroup
	defer activeSolvers.Wait()

	for i := 0; i <= maxReseeds; i++ {
		select {
		case <-ctx.Done():
			return &nodeAndError{nil, ctx.Err()}
		default:
		}
		candidates := []*candidate{}

		for ptgNum, curPtg := range mp.tpFrame.PTGSolvers() {
			// Find the best traj point for each traj family, and store for later comparison
			ptgNumPar, curPtgPar := ptgNum, curPtg
			activeSolvers.Add(1)
			utils.PanicCapturingGo(func() {
				defer activeSolvers.Done()
				cand, err := mp.getExtensionCandidate(ctx, goalNode, ptgNumPar, curPtgPar, rrt, seedNode)
				if err != nil && !errors.Is(err, errNoNeighbors) && !errors.Is(err, errInvalidCandidate) {
					candChan <- nil
					return
				}
				if cand != nil {
					if cand.err == nil {
						candChan <- cand
						return
					}
				}
				candChan <- nil
			})
		}

		for i := 0; i < len(mp.tpFrame.PTGSolvers()); i++ {
			select {
			case <-ctx.Done():
				return &nodeAndError{nil, ctx.Err()}
			case cand := <-candChan:
				if cand != nil {
					candidates = append(candidates, cand)
				}
			}
		}
		var err error
		newReseedCandidate, err := mp.extendMap(ctx, candidates, rrt, isGoalTree)
		if err != nil && !errors.Is(err, errNoCandidates) {
			return &nodeAndError{nil, err}
		}
		if newReseedCandidate == nil {
			if reseedCandidate == nil {
				// We failed to extend at all
				return &nodeAndError{nil, nil}
			}
			break
		}
		reseedCandidate = newReseedCandidate
		endNode := reseedCandidate.newNodes[len(reseedCandidate.newNodes)-1]
		distTravelledByCandidate := 0.
		for _, newNode := range reseedCandidate.newNodes {
			distTravelledByCandidate += math.Abs(newNode.Q()[3].Value - newNode.Q()[2].Value)
		}
		distToGoal := endNode.Pose().Point().Distance(goalNode.Pose().Point())
		if distToGoal < mp.planOpts.GoalThreshold || lastIteration {
			// Reached the goal position, or otherwise failed to fully extend to the end of a trajectory
			return &nodeAndError{endNode, nil}
		}
		if i == 0 {
			// TP-space distance is NOT the same thing as cartesian distance, but they track sufficiently well that this is valid to do.
			maxReseeds = int(math.Min(float64(defaultMaxReseeds), math.Ceil(distToGoal/(distTravelledByCandidate/4))+2))
		}
		// If our most recent traj was not a full-length extension, try to extend one more time and then return our best node.
		// This helps prevent the planner from doing a 15-point turn to adjust orientation, which is very difficult to accurately execute.
		if distToGoal < distTravelledByCandidate/4 {
			lastIteration = true
		}

		seedNode = endNode
	}
	return &nodeAndError{reseedCandidate.newNodes[len(reseedCandidate.newNodes)-1], nil}
}

// extendMap grows the rrt map to the best candidate node, returning the added candidate.
func (mp *tpSpaceRRTMotionPlanner) extendMap(
	ctx context.Context,
	candidates []*candidate,
	rrt rrtMap,
	isGoalTree bool,
) (*candidate, error) {
	if len(candidates) == 0 {
		return nil, errNoCandidates
	}
	var addedNode *basicNode
	// If we found any valid nodes that we can extend to, find the very best one and add that to the tree
	// Dist measures how close our candidate gets us to a goal.
	bestDist := math.Inf(1)
	// Cost measures how long a candidate's path is.
	bestCost := math.Inf(1)
	var bestCand *candidate
	for _, cand := range candidates {
		if cand.dist <= bestDist || cand.dist < mp.planOpts.GoalThreshold {
			candCost := 0.
			for _, candNode := range cand.newNodes {
				candCost += candNode.Cost()
			}
			if bestDist > mp.planOpts.GoalThreshold || candCost < bestCost {
				// Update the new best candidate if one of the following is true:
				// 1. The former bestDist is greater than the goal threshold, thus this candidate gets us closer to the goal
				// 2. The cost of this candidate is lower than the cost of the current best candidate.
				// Note that if in this block, then we are already guaranteed to be either a dist improvement, or below goal threshold.
				bestCand = cand
				bestDist = cand.dist
				bestCost = candCost
			}
		}
	}
	treeNode := bestCand.treeNode // The node already in the tree to which we are parenting
	newNodes := bestCand.newNodes // The node we are adding because it was the best extending PTG
	for _, newNode := range newNodes {
		ptgNum := int(newNode.Q()[0].Value)
		randAlpha := newNode.Q()[1].Value
		randDist := newNode.Q()[3].Value - newNode.Q()[2].Value

		trajK, err := mp.tpFrame.PTGSolvers()[ptgNum].Trajectory(randAlpha, 0, randDist, mp.planOpts.Resolution)
		if err != nil {
			return nil, err
		}
		arcStartPose := treeNode.Pose()
		lastDist := 0.
		sinceLastNode := 0.

		var trajState *ik.State
		for i := 0; i < len(trajK); i++ {
			trajPt := trajK[i]
			if i == 0 {
				lastDist = trajPt.Dist
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			trajState = &ik.State{Position: spatialmath.Compose(arcStartPose, trajPt.Pose)}
			if pathdebug {
				if !isGoalTree {
					mp.logger.CDebugf(ctx, "$FWDTREE,%f,%f", trajState.Position.Point().X, trajState.Position.Point().Y)
				} else {
					mp.logger.CDebugf(ctx, "$REVTREE,%f,%f", trajState.Position.Point().X, trajState.Position.Point().Y)
				}
			}
			sinceLastNode += math.Abs(trajPt.Dist - lastDist)

			// Optionally add sub-nodes along the way. Will make the final path a bit better
			if sinceLastNode > mp.algOpts.addNodeEvery {
				// add the last node in trajectory
				addedNode = &basicNode{
					q:      referenceframe.FloatsToInputs([]float64{float64(ptgNum), randAlpha, 0, trajPt.Dist}),
					cost:   trajPt.Dist,
					pose:   trajState.Position,
					corner: false,
				}
				rrt[addedNode] = treeNode
				sinceLastNode = 0.
			}
			lastDist = trajPt.Dist
		}
		if pathdebug {
			mp.logger.CDebugf(ctx, "$WPI,%f,%f", trajState.Position.Point().X, trajState.Position.Point().Y)
		}
		if isGoalTree {
			mp.goalNodes = append(mp.goalNodes, newNode)
		}
		rrt[newNode] = treeNode
		treeNode = newNode
	}
	return bestCand, nil
}

func (mp *tpSpaceRRTMotionPlanner) setupTPSpaceOptions() {
	tpOpt := &tpspaceOptions{
		autoBB: defaultAutoBB,

		addNodeEvery:      defaultAddNodeEvery,
		attemptSolveEvery: defaultAttemptSolveEvery,

		identicalNodeDistance: defaultIdenticalNodeDistance,

		distOptions: map[tpspace.PTG]*plannerOptions{},

		goalMetricConstructor: defaultGoalMetricConstructor,
	}

	mp.algOpts = tpOpt
}

func (mp *tpSpaceRRTMotionPlanner) ptgSolutionAndMetric(ptg tpspace.PTGSolver,
	nearestPose, randPosNodePose spatialmath.Pose,
) (*ik.Solution, ik.StateMetric, error) {
	relPose := spatialmath.PoseBetween(nearestPose, randPosNodePose)
	targetFunc := mp.algOpts.goalMetricConstructor(relPose)
	seedDist := relPose.Point().Norm()
	seed := tpspace.PTGIKSeed(ptg)
	dof := ptg.DoF()
	if seedDist < dof[1].Max {
		seed[1].Value = seedDist
	}
	if relPose.Point().X < 0 {
		seed[0].Value *= -1
	}

	solutionChan := make(chan *ik.Solution, 1)
	err := ptg.Solve(context.Background(), solutionChan, seed, targetFunc, 0)

	var solution *ik.Solution
	select {
	case solution = <-solutionChan:
	default:
	}

	return solution, targetFunc, err
}

// make2DTPSpaceDistanceOptions will create a plannerOptions object with a custom DistanceFunc constructed such that
// distances can be computed in TP space using the given PTG.
// Also returns a pointer to a sync.Map of nearest poses -> ik.Solution so the (expensive to compute) solution can be reused.
func (mp *tpSpaceRRTMotionPlanner) make2DTPSpaceDistanceOptions(ptg tpspace.PTGSolver) (*plannerOptions, *sync.Map) {
	m := sync.Map{}
	opts := newBasicPlannerOptions(mp.frame)
	segMetric := func(seg *ik.Segment) float64 {
		// When running NearestNeighbor:
		// StartPosition is the seed/query
		// EndPosition is the pose already in the RRT tree
		if seg.StartPosition == nil || seg.EndPosition == nil {
			return math.Inf(1)
		}
		solution, _, err := mp.ptgSolutionAndMetric(ptg, seg.EndPosition, seg.StartPosition)

		if err != nil || solution == nil {
			return math.Inf(1)
		}

		m.Store(seg.EndPosition, solution)

		return solution.Score
	}
	opts.DistanceFunc = segMetric
	return opts, &m
}

// smoothPath takes in a path and attempts to smooth it by randomly sampling edges in the path and seeing
// if they can be connected.
func (mp *tpSpaceRRTMotionPlanner) smoothPath(ctx context.Context, path []node) []node {
	toIter := int(math.Min(float64(len(path)*len(path))/2, float64(mp.planOpts.SmoothIter)))
	currCost := sumCosts(path)
	smoothPlannerMP, err := newTPSpaceMotionPlanner(mp.frame, mp.randseed, mp.logger, mp.planOpts)
	if err != nil {
		return path
	}
	smoothPlanner := smoothPlannerMP.(*tpSpaceRRTMotionPlanner)
	smoothPlanner.algOpts.identicalNodeDistance = -1
	for i := 0; i < toIter; i++ {
		mp.logger.CDebugf(ctx, "TP Space smoothing iteration %d of %d", i, toIter)
		select {
		case <-ctx.Done():
			return path
		default:
		}
		// get start node of first edge. Cannot be either the last or second-to-last node.
		// Intn will return an int in the half-open interval half-open interval [0,n)
		firstEdge := mp.randseed.Intn(len(path))
		cdf := generateCDF(firstEdge, len(path))
		sample := mp.randseed.Float64()
		secondEdge := sort.Search(len(cdf), func(i int) bool {
			return cdf[i] >= sample
		})

		if secondEdge < firstEdge {
			secondEdge, firstEdge = firstEdge, secondEdge
		}

		newInputSteps, err := mp.attemptSmooth(ctx, path, firstEdge, secondEdge, smoothPlanner)

		if err != nil || newInputSteps == nil {
			continue
		}
		newCost := sumCosts(newInputSteps)
		if newCost >= currCost {
			// The smoothed path is longer than the original
			continue
		}

		path = newInputSteps
		currCost = newCost
	}

	if pathdebug {
		allPtgs := mp.tpFrame.PTGSolvers()
		lastPose := path[0].Pose()
		for _, mynode := range path {
			trajPts, err := allPtgs[int(mynode.Q()[0].Value)].Trajectory(
				mynode.Q()[1].Value,
				mynode.Q()[2].Value,
				mynode.Q()[3].Value,
				mp.planOpts.Resolution,
			)
			if err != nil {
				// Unimportant; this is just for debug visualization
				break
			}
			for i, pt := range trajPts {
				intPose := spatialmath.Compose(lastPose, pt.Pose)

				if i == 0 {
					mp.logger.Debugf("$SMOOTHWP,%f,%f", intPose.Point().X, intPose.Point().Y)
				}
				mp.logger.Debugf("$SMOOTHPATH,%f,%f", intPose.Point().X, intPose.Point().Y)
			}
			lastPose = spatialmath.Compose(lastPose, trajPts[len(trajPts)-1].Pose)
		}
	}

	return path
}

// attemptSmooth attempts to connect two given points in a path. The points must not be adjacent.
// Strategy is to subdivide the seed-side trajectories to give a greater probability of solving.
func (mp *tpSpaceRRTMotionPlanner) attemptSmooth(
	ctx context.Context,
	path []node,
	firstEdge, secondEdge int,
	smoother *tpSpaceRRTMotionPlanner,
) ([]node, error) {
	startMap := map[node]node{}
	var parent node
	parentPose := path[0].Pose()

	for j := 0; j <= firstEdge; j++ {
		pathNode := path[j]
		startMap[pathNode] = parent
		for adjNum := defaultSmoothChunkCount - 1; adjNum > 0; adjNum-- {
			fullQ := pathNode.Q()
			adj := (fullQ[3].Value - fullQ[2].Value) * (float64(adjNum) / float64(defaultSmoothChunkCount))
			newQ := []referenceframe.Input{fullQ[0], fullQ[1], fullQ[2], {fullQ[3].Value - adj}}
			trajK, err := smoother.tpFrame.PTGSolvers()[int(math.Round(newQ[0].Value))].Trajectory(
				newQ[1].Value,
				newQ[2].Value,
				newQ[3].Value,
				mp.planOpts.Resolution,
			)
			if err != nil {
				continue
			}

			intNode := &basicNode{
				q:      newQ,
				cost:   pathNode.Cost() - math.Abs(adj),
				pose:   spatialmath.Compose(parentPose, trajK[len(trajK)-1].Pose),
				corner: false,
			}
			startMap[intNode] = parent
		}
		parent = pathNode
		parentPose = parent.Pose()
	}
	// TODO: everything below this point can become an invocation of `smoother.planRunner`
	reached := smoother.attemptExtension(ctx, path[secondEdge], startMap, false)
	if reached.error != nil || reached.node == nil {
		return nil, errors.New("could not extend to smoothing destination")
	}

	reachedDelta := mp.planOpts.DistanceFunc(&ik.Segment{StartPosition: reached.Pose(), EndPosition: path[secondEdge].Pose()})
	// If we tried the goal and have a close-enough XY location, check if the node is good enough to be a final goal
	if reachedDelta > mp.planOpts.GoalThreshold {
		return nil, errors.New("could not precisely reach smoothing destination")
	}

	newInputSteps := extractPath(startMap, nil, &nodePair{a: reached.node, b: nil}, false)

	if secondEdge < len(path)-1 {
		newInputSteps = append(newInputSteps, path[secondEdge+1:]...)
	} else {
		// If secondEdge is the last node of the plan, then it's the node at the goal pose whose configuration should be 0, 0, 0.
		// newInputSteps will not contain this 0, 0, 0 node because it just extended to it. But path[secondEdge+1:] will not include it
		// either, it will reach past the end of the path.
		// Essentially, if we smoothed all the way to the goal node, then that smoothing process will have removed the path endpoint node,
		// so this step will replace it.
		newInputSteps = append(newInputSteps, path[len(path)-1])
	}
	return rectifyTPspacePath(newInputSteps, mp.frame, path[0].Pose())
}

func (mp *tpSpaceRRTMotionPlanner) sample(rSeed node, iter int) (node, error) {
	dist := rSeed.Cost()
	if dist < 1 {
		dist = 1.0
	}
	rDist := dist * (mp.algOpts.autoBB + float64(iter)*autoBBscale)
	randPosX := float64(mp.randseed.Intn(int(rDist)))
	randPosY := float64(mp.randseed.Intn(int(rDist)))
	randPosTheta := math.Pi * (mp.randseed.Float64() - 0.5)
	randPos := spatialmath.NewPose(
		r3.Vector{rSeed.Pose().Point().X + (randPosX - rDist/2.), rSeed.Pose().Point().Y + (randPosY - rDist/2.), 0},
		&spatialmath.OrientationVector{OZ: 1, Theta: randPosTheta},
	)
	return &basicNode{pose: randPos}, nil
}

// rectifyTPspacePath is needed because of how trees are currently stored. As trees grow from the start or goal, the Pose stored in the node
// is the distal pose away from the root of the tree, which in the case of the goal tree is in fact the 0-distance point of the traj.
// When this becomes a single path, poses should reflect the transformation at the end of each traj. Here we go through and recompute
// each pose in order to ensure correctness.
// TODO: if trees are stored as segments rather than nodes, then this becomes simpler/unnecessary. Related to RSDK-4139.
func rectifyTPspacePath(path []node, frame referenceframe.Frame, startPose spatialmath.Pose) ([]node, error) {
	correctedPath := []node{}
	runningPose := startPose
	for _, wp := range path {
		wpPose, err := frame.Transform(wp.Q())
		if err != nil {
			return nil, err
		}
		runningPose = spatialmath.Compose(runningPose, wpPose)

		thisNode := &basicNode{
			q:      wp.Q(),
			cost:   wp.Cost(),
			pose:   runningPose,
			corner: wp.Corner(),
		}
		correctedPath = append(correctedPath, thisNode)
	}
	return correctedPath, nil
}

func extractTPspacePath(startMap, goalMap map[node]node, pair *nodePair) []node {
	// need to figure out which of the two nodes is in the start map
	var startReached, goalReached node
	if _, ok := startMap[pair.a]; ok {
		startReached, goalReached = pair.a, pair.b
	} else {
		startReached, goalReached = pair.b, pair.a
	}

	// extract the path to the seed
	path := make([]node, 0)
	for startReached != nil {
		path = append(path, startReached)
		startReached = startMap[startReached]
	}

	// reverse the slice
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	// extract the path to the goal
	for goalReached != nil {
		goalReachedReversed := &basicNode{
			q: []referenceframe.Input{
				goalReached.Q()[0],
				goalReached.Q()[1],
				goalReached.Q()[3],
				goalReached.Q()[2],
			},
			cost:   goalReached.Cost(),
			pose:   spatialmath.Compose(goalReached.Pose(), flipPose),
			corner: goalReached.Corner(),
		}
		path = append(path, goalReachedReversed)
		goalReached = goalMap[goalReached]
	}
	return path
}

// Returns a new node whose orientation is flipped 180 degrees from the provided node.
func flipNode(n node) node {
	return &basicNode{
		q:      n.Q(),
		cost:   n.Cost(),
		pose:   spatialmath.Compose(n.Pose(), flipPose),
		corner: n.Corner(),
	}
}

// generateHeuristic returns a list of heuristics for each node in the path
// This is converted into a probability distribution through the softmax function.
func generateLookAheadHeuristic(firstEdge, pathLen int) []float64 {
	// The heuristic implemented here takes in the firstEdge and defines a lookAhead.
	// For nodes in each direction around firstEdge, the algorithm will increment by one
	// until it reaches firstEdge + lookAhead and firstEdge - lookAhead indices
	// From then on, it will decrement by one until it gets to the edge of the list
	// All values are passed into a softmax to convert the real-valued heuristics into a probability distribution

	// It is better to have a lookAhead that is shorter because biasing towards collapsing shorter paths is
	// more likely to yield success than finding one long one. Another observation to note is that sampling
	// edges that are not connectable is very costly because the algorithm cycles through all PTGs in hopes
	// of connecting them. Thus, finding short, connectable paths is best
	lookAhead := 3.0
	heuristics := make([]float64, pathLen)
	for i := 0; i < pathLen; i++ {
		// Creates ascending list until lookAhead and then descending list
		heuristics[i] = math.Pow(math.Max(1, lookAhead-math.Abs(lookAhead-math.Abs(float64(firstEdge-i)))), 2)
	}

	// firstEdge + adjacent edges should not be sampled, so set their heuristic to -Inf
	// This gets converted to zero probability by softmax
	heuristics[firstEdge] = math.Inf(-1)
	if firstEdge-1 >= 0 {
		heuristics[firstEdge-1] = math.Inf(-1)
	}
	if firstEdge+1 < pathLen {
		heuristics[firstEdge+1] = math.Inf(-1)
	}
	return heuristics
}

// softmax takes in a heuristic list and converts it into a proability distribution function
// This means the sum of the returned softmaxArr is equal to one.
func softmax(heuristics []float64) []float64 {
	sum := 0.0
	for _, heuristic := range heuristics {
		sum += math.Exp(heuristic)
	}

	softmaxArr := make([]float64, len(heuristics))
	// Account for case where sum equals zero. In this case, assign equal weight to all indices
	if sum == 0 {
		for i := 0; i < len(heuristics); i++ {
			softmaxArr[i] = 1.0 / float64(len(heuristics))
		}
	} else {
		for i, heuristic := range heuristics {
			softmaxArr[i] = math.Exp(heuristic) / sum
		}
	}
	return softmaxArr
}

// generateCDF returns a cumulative distribution function that can be used to
// sample the secondEdge for the smoothing algorithm.
func generateCDF(firstEdge, pathLen int) []float64 {
	heuristics := generateLookAheadHeuristic(firstEdge, pathLen)
	softmaxArr := softmax(heuristics)

	// sum all successive values in the softmaxArr to convert it from a PDF to a CDF
	for i := 1; i < len(softmaxArr); i++ {
		softmaxArr[i] += softmaxArr[i-1]
	}
	return softmaxArr
}
