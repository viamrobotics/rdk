//go:build !windows && !no_cgo

package motionplan

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
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
	defaultAutoBB = 0.7 // Automatic bounding box on driveable area as a multiple of start-goal distance
	// Note: while fully holonomic planners can use the limits of the frame as implicit boundaries, with non-holonomic motion
	// this is not the case, and the total workspace available to the planned frame is not directly related to the motion available
	// from a single set of inputs.

	// How much the bounding box of random points to sample increases in size with each algorithm iteration.
	autoBBscale = 0.1

	// whether to add intermediate waypoints.
	defaultAddInt = true
	// Add a subnode every this many mm along a valid trajectory. Large values run faster, small gives better paths
	// Meaningless if the above is false.
	defaultAddNodeEvery = 400.

	// Don't add new RRT tree nodes if there is an existing node within this distance.
	// Consider nodes on trees to be connected if they are within this distance.
	defaultIdenticalNodeDistance = 80.

	// When extending the RRT tree towards some point, do not extend more than this many times in a single RRT invocation.
	defaultMaxReseeds = 20

	// Make an attempt to solve the tree every this many iterations
	// For a unidirectional solve, this means attempting to reach the goal rather than a random point
	// For a bidirectional solve, this means trying to connect the two trees directly.
	defaultAttemptSolveEvery = 15

	// When attempting a solve per the above, make no more than this many tries. Preserves performance with large trees.
	defaultMaxConnectAttempts = 20

	defaultBidirectional = true

	// default motion planning collision resolution is every 2mm.
	// For bases we increase this to 60mm, a bit more than 2 inches.
	defaultPTGCollisionResolution = 60

	// When checking a PTG for validity and finding a collision, using the last good configuration will result in a highly restricted
	// node that is directly facing a wall. To prevent this, we walk back along the trajectory by this percentage of the traj length
	// so that the node we add has more freedom of movement to extend in the future.
	defaultCollisionWalkbackPct = 0.8

	// When evaluating the partial node to add to a tree after defaultCollisionWalkbackPct is applied, ensure the trajectory is still at
	// least this long.
	defaultMinTrajectoryLength = 350

	// Print very fine-grained debug info. Useful for observing the inner RRT tree structure directly.
	pathdebug = false
)

// Using the standard SquaredNormMetric, we run into issues where far apart distances will underflow gradient calculations.
// This metric, used only for gradient descent, computes the gradient using centimeters rather than millimeters allowing for smaller
// values that do not underflow.
var defaultGoalMetricConstructor = ik.NewPosWeightSquaredNormMetric

// This should only be used when bidirectional mode is `false`.
var defaultPosOnlyGoalMetricConstructor = ik.NewPositionOnlyMetric

type tpspaceOptions struct {
	// TODO: base this on frame limits?
	autoBB float64 // Automatic bounding box on driveable area as a multiple of start-goal distance

	addIntermediate bool // whether to add intermediate waypoints.
	// Add a subnode every this many mm along a valid trajectory. Large values run faster, small gives better paths
	// Meaningless if the above is false.
	addNodeEvery float64

	// If the squared norm between two poses is less than this, consider them equal
	poseSolveDist float64

	// Don't add new RRT tree nodes if there is an existing node within this distance.
	identicalNodeDistance float64

	// Make an attempt to solve the tree every this many iterations
	// For a unidirectional solve, this means attempting to reach the goal rather than a random point
	// For a bidirectional solve, this means trying to connect the two trees directly
	attemptSolveEvery int

	// Whether to attempt to solve with both trees simultaneously or just one
	bidirectional bool

	goalMetricConstructor func(spatialmath.Pose) ik.StateMetric

	// Cached functions for calculating TP-space distances for each PTG
	distOptions       map[tpspace.PTG]*plannerOptions
	invertDistOptions map[tpspace.PTG]*plannerOptions
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
	if opt.profile == PositionOnlyMotionProfile && opt.PositionSeeds <= 0 {
		tpPlanner.algOpts.bidirectional = false
		tpPlanner.algOpts.goalMetricConstructor = defaultPosOnlyGoalMetricConstructor
	}

	return tpPlanner, nil
}

// TODO: seed is not immediately useful for TP-space.
func (mp *tpSpaceRRTMotionPlanner) plan(ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
) ([]node, error) {
	mp.planOpts.SetGoal(goal)
	solutionChan := make(chan *rrtPlanReturn, 1)

	seedPos := spatialmath.NewZeroPose()

	startNode := &basicNode{q: make([]referenceframe.Input, len(mp.frame.DoF())), cost: 0, pose: seedPos, corner: false}
	goalNode := &basicNode{q: make([]referenceframe.Input, len(mp.frame.DoF())), cost: 0, pose: goal, corner: false}

	var planRunners sync.WaitGroup

	planRunners.Add(1)
	utils.PanicCapturingGo(func() {
		defer planRunners.Done()
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
		planRunners.Wait()
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
				rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("node %v must provide a Pose", k)}
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
				rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("node %v must provide a Pose", k)}
				return
			}
		}
	}
	mp.goalNodes = append(mp.goalNodes, goalNode)
	mp.logger.CDebugf(ctx, "Starting TPspace solving with startMap len %d and goalMap len %d", len(rrt.maps.startMap), len(rrt.maps.goalMap))

	publishFinishedPath := func(path []node) {
		// If we've reached the goal, extract the path from the RRT trees and return
		correctedPath, err := rectifyTPspacePath(path, mp.frame, spatialmath.NewZeroPose())
		if err != nil {
			rrt.solutionChan <- &rrtPlanReturn{planerr: err, maps: rrt.maps}
			return
		}
		rrt.solutionChan <- &rrtPlanReturn{steps: correctedPath, maps: rrt.maps}

		// Print debug info if requested
		if pathdebug {
			allPtgs := mp.tpFrame.PTGSolvers()
			lastPose := spatialmath.NewZeroPose()
			for _, mynode := range correctedPath {
				trajPts, err := allPtgs[int(mynode.Q()[0].Value)].Trajectory(mynode.Q()[1].Value, mynode.Q()[2].Value)
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
	}

	m1chan := make(chan *nodeAndError, 1)
	m2chan := make(chan *nodeAndError, 1)
	defer close(m1chan)
	defer close(m2chan)

	// The midpoint should not be the 50% interpolation of start/goal poses, but should be the 50% interpolated point with the orientation
	// pointing at the goal from the start
	midPt := startPose.Point().Add(goalPose.Point()).Mul(0.5)
	midOrient := &spatialmath.OrientationVector{OZ: 1, Theta: math.Atan2(-midPt.X, midPt.Y)}

	midptNode := &basicNode{pose: spatialmath.NewPose(midPt, midOrient), cost: midPt.Sub(startPose.Point()).Norm()}
	var randPosNode node = midptNode

	for iter := 0; iter < mp.planOpts.PlanIter; iter++ {
		mp.logger.CDebugf(ctx, "TP Space RRT iteration %d", iter)
		if ctx.Err() != nil {
			mp.logger.CDebugf(ctx, "TP Space RRT timed out after %d iterations", iter)
			rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("TP Space RRT timeout %w", ctx.Err()), maps: rrt.maps}
			return
		}

		seedReached := &nodeAndError{}
		goalReached := &nodeAndError{}
		rseed := mp.randseed.Int31()
		utils.PanicCapturingGo(func() {
			m1chan <- mp.attemptExtension(ctx, randPosNode, rrt.maps.startMap, false, rseed)
		})
		if mp.algOpts.bidirectional {
			rseed2 := mp.randseed.Int31()
			utils.PanicCapturingGo(func() {
				m2chan <- mp.attemptExtension(ctx, randPosNode, rrt.maps.goalMap, true, rseed2)
			})
			goalReached = <-m2chan
		}
		seedReached = <-m1chan

		err := multierr.Combine(seedReached.error, goalReached.error)
		if err != nil {
			rrt.solutionChan <- &rrtPlanReturn{planerr: err, maps: rrt.maps}
			return
		}
		if seedReached.node != nil && goalReached.node != nil {
			reachedDelta := mp.planOpts.DistanceFunc(&ik.Segment{StartPosition: seedReached.node.Pose(), EndPosition: goalReached.node.Pose()})
			if reachedDelta > mp.planOpts.GoalThreshold {
				// If both maps extended, but did not reach the same point, then attempt to extend them towards each other
				seedReached = mp.attemptExtension(ctx, goalReached.node, rrt.maps.startMap, false, mp.randseed.Int31())
				if seedReached.error != nil {
					rrt.solutionChan <- &rrtPlanReturn{planerr: seedReached.error, maps: rrt.maps}
					return
				}
				if seedReached.node != nil {
					reachedDelta = mp.planOpts.DistanceFunc(&ik.Segment{
						StartPosition: seedReached.node.Pose(),
						EndPosition:   goalReached.node.Pose(),
					})
					if reachedDelta > mp.planOpts.GoalThreshold {
						goalReached = mp.attemptExtension(ctx, seedReached.node, rrt.maps.goalMap, true, mp.randseed.Int31())
						if goalReached.error != nil {
							rrt.solutionChan <- &rrtPlanReturn{planerr: goalReached.error, maps: rrt.maps}
							return
						}
					}
					if goalReached.node != nil {
						reachedDelta = mp.planOpts.DistanceFunc(&ik.Segment{StartPosition: seedReached.node.Pose(), EndPosition: goalReached.node.Pose()})
					}
				}
			}
			if reachedDelta <= mp.planOpts.GoalThreshold {
				// If we've reached the goal, extract the path from the RRT trees and return
				path := extractPath(rrt.maps.startMap, rrt.maps.goalMap, &nodePair{a: seedReached.node, b: goalReached.node}, false)
				publishFinishedPath(path)
				return
			}
		}
		if iter%mp.algOpts.attemptSolveEvery == 0 {
			// Attempt a solve; we iterate through our goal tree and attempt to find any connection to the seed tree
			paths := [][]node{}

			// Exhaustively searching the tree gets expensive quickly, so we cap the number of connect attempts we make each time we call
			// this.
			attempts := 0                                                                     // Track the number of connection attempts we have made
			pctCheck := 100 * float64(defaultMaxConnectAttempts) / float64(len(mp.goalNodes)) // Target checking this proportion of nodes.

			for _, goalMapNode := range mp.goalNodes {
				if ctx.Err() != nil {
					rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("TP Space RRT timeout %w", ctx.Err()), maps: rrt.maps}
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

				seedReached := mp.attemptExtension(ctx, goalMapNode, rrt.maps.startMap, false, mp.randseed.Int31())
				if seedReached.error != nil {
					rrt.solutionChan <- &rrtPlanReturn{planerr: seedReached.error, maps: rrt.maps}
					return
				}
				if seedReached.node == nil {
					continue
				}
				var reachedDelta float64
				if mp.algOpts.bidirectional {
					reachedDelta = mp.planOpts.DistanceFunc(&ik.Segment{StartPosition: seedReached.node.Pose(), EndPosition: goalMapNode.Pose()})
				} else {
					reachedDelta = mp.planOpts.goalMetric(&ik.State{Position: seedReached.node.Pose()})
				}
				if reachedDelta <= mp.planOpts.GoalThreshold {
					// If we've reached the goal, extract the path from the RRT trees and return
					path := extractPath(rrt.maps.startMap, rrt.maps.goalMap, &nodePair{a: seedReached.node, b: goalMapNode}, false)
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
			rrt.solutionChan <- &rrtPlanReturn{planerr: err, maps: rrt.maps}
			return
		}
	}
	rrt.solutionChan <- &rrtPlanReturn{maps: rrt.maps, planerr: errors.New("tpspace RRT unable to create valid path")}
}

// getExtensionCandidate will return either nil, or the best node on a valid PTG to reach the desired random node and its RRT tree parent.
func (mp *tpSpaceRRTMotionPlanner) getExtensionCandidate(
	ctx context.Context,
	randPosNode node,
	ptgNum int,
	curPtg tpspace.PTGSolver,
	rrt rrtMap,
	nearest node,
	invert bool,
	rseed int,
) (*candidate, error) {
	nm := &neighborManager{nCPU: mp.planOpts.NumThreads / len(mp.tpFrame.PTGSolvers())}
	nm.parallelNeighbors = 10

	var successNode node
	// Get the distance function that will find the nearest RRT map node in TP-space of *this* PTG
	ptgDistOpt := mp.algOpts.distOptions[curPtg]
	if invert {
		ptgDistOpt = mp.algOpts.invertDistOptions[curPtg]
	}

	if nearest == nil {
		// Get nearest neighbor to rand config in tree using this PTG
		// TODO: running nearestNeighbor actually involves a ptg.Solve() call, duplicating work.
		nearest = nm.nearestNeighbor(ctx, ptgDistOpt, randPosNode, rrt)
		if nearest == nil {
			return nil, errNoNeighbors
		}
	}
	// TODO: We could potentially improve solving by first getting the rough distance to the randPosNode to any point in the rrt tree,
	// then dynamically expanding or contracting the limits of IK to be some fraction of that distance.

	// Get cartesian distance from NN to rand
	var targetFunc ik.StateMetric
	if invert {
		sqMet := mp.algOpts.goalMetricConstructor(randPosNode.Pose())
		targetFunc = func(pose *ik.State) float64 {
			return sqMet(&ik.State{Position: spatialmath.PoseBetweenInverse(pose.Position, nearest.Pose())})
		}
	} else {
		relPose := spatialmath.PoseBetween(nearest.Pose(), randPosNode.Pose())
		targetFunc = mp.algOpts.goalMetricConstructor(relPose)
	}
	solutionChan := make(chan *ik.Solution, 1)
	err := curPtg.Solve(context.Background(), solutionChan, nil, targetFunc, rseed)

	var bestNode *ik.Solution
	select {
	case bestNode = <-solutionChan:
	default:
	}
	if err != nil || bestNode == nil {
		return nil, err
	}
	arcStartPose := nearest.Pose()
	successNodes := []node{}
	arcPose := spatialmath.NewZeroPose() // This will be the relative pose that is the delta from one end of the combined traj to the other.
	// We may produce more than one consecutive arc. Reduce the one configuration to several 2dof arcs
	for i := 0; i < len(bestNode.Configuration); i += 2 {
		var subNode node
		if invert {
			nodeIdx := len(bestNode.Configuration) - (i + 2)
			subNode = newConfigurationNode(bestNode.Configuration[nodeIdx : nodeIdx+2])
		} else {
			subNode = newConfigurationNode(bestNode.Configuration[i : i+2])
		}

		subNodePose, err := curPtg.Transform(subNode.Q())
		if err != nil {
			return nil, err
		}
		if invert {
			arcStartPose = spatialmath.PoseBetweenInverse(subNodePose, arcStartPose)
		}

		// Check collisions along this traj and get the longest distance viable
		trajK, err := curPtg.Trajectory(subNode.Q()[0].Value, subNode.Q()[1].Value)
		if err != nil {
			return nil, err
		}
		goodNode := mp.checkTraj(trajK, invert, arcStartPose)
		if goodNode == nil {
			break
		}
		partialExtend := false

		if invert {
			arcPose = spatialmath.Compose(subNodePose, arcPose)
		} else {
			for i, val := range subNode.Q() {
				if goodNode.Q()[i] != val {
					partialExtend = true
				}
			}
			arcPose = spatialmath.Compose(arcPose, goodNode.Pose())
		}

		// add the last node in trajectory
		arcStartPose = spatialmath.Compose(arcStartPose, goodNode.Pose())
		successNode = &basicNode{
			q:      append([]referenceframe.Input{{float64(ptgNum)}}, goodNode.Q()...),
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
	// Get nearest neighbor to new node that's already in the tree
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
func (mp *tpSpaceRRTMotionPlanner) checkTraj(trajK []*tpspace.TrajNode, invert bool, arcStartPose spatialmath.Pose) node {
	sinceLastCollideCheck := 0.
	lastDist := 0.
	passed := []node{}
	// Check each point along the trajectory to confirm constraints are met
	// TODO: RSDK-5007 will allow this to use a Segment and be better integrated into our existing frameworks.
	for i := 0; i < len(trajK); i++ {
		trajPt := trajK[i]
		if invert {
			// Start at known-good map point and extend
			// For the goal tree this means iterating backwards
			trajPt = trajK[(len(trajK)-1)-i]
		}

		sinceLastCollideCheck += math.Abs(trajPt.Dist - lastDist)
		trajState := &ik.State{Position: spatialmath.Compose(arcStartPose, trajPt.Pose), Frame: mp.frame}
		if sinceLastCollideCheck > mp.planOpts.Resolution || i == 0 || i == len(trajK)-1 {
			// In addition to checking every `Resolution`, we also check both endpoints.
			ok, _ := mp.planOpts.CheckStateConstraints(trajState)
			if !ok {
				okDist := trajPt.Dist * defaultCollisionWalkbackPct
				if okDist > defaultMinTrajectoryLength && !invert {
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
			sinceLastCollideCheck = 0.
		}

		okNode := &basicNode{
			q:    []referenceframe.Input{{trajPt.Alpha}, {trajPt.Dist}},
			cost: trajPt.Dist,
			pose: trajPt.Pose,
		}
		passed = append(passed, okNode)
		lastDist = trajPt.Dist
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
	invert bool,
	rseed int32,
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
				cand, err := mp.getExtensionCandidate(ctx, goalNode, ptgNumPar, curPtgPar, rrt, seedNode, invert, int(rseed))
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
		reseedCandidate, err = mp.extendMap(ctx, candidates, rrt, invert)
		if err != nil && !errors.Is(err, errNoCandidates) {
			return &nodeAndError{nil, err}
		}
		if reseedCandidate == nil {
			return &nodeAndError{nil, nil}
		}
		endNode := reseedCandidate.newNodes[len(reseedCandidate.newNodes)-1]
		dist := mp.planOpts.DistanceFunc(&ik.Segment{StartPosition: endNode.Pose(), EndPosition: goalNode.Pose()})
		if dist < mp.algOpts.poseSolveDist || lastIteration {
			// Reached the goal position, or otherwise failed to fully extend to the end of a trajectory
			return &nodeAndError{endNode, nil}
		}
		if i == 0 {
			// TP-space distance is NOT the same thing as cartesian distance, but they track sufficiently well that this is valid to do.
			maxReseeds = int(math.Min(float64(defaultMaxReseeds), math.Ceil(math.Sqrt(dist)/endNode.Q()[2].Value)+2))
		}
		// If our most recent traj was not a full-length extension, try to extend one more time and then return our best node.
		// This helps prevent the planner from doing a 15-point turn to adjust orientation, which is very difficult to accurately execute.
		if math.Sqrt(dist) < endNode.Q()[2].Value/2 {
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
	invert bool,
) (*candidate, error) {
	if len(candidates) == 0 {
		return nil, errNoCandidates
	}
	var addedNode node
	// If we found any valid nodes that we can extend to, find the very best one and add that to the tree
	bestDist := math.Inf(1)
	var bestCand *candidate
	for _, cand := range candidates {
		if cand.dist < bestDist {
			bestCand = cand
			bestDist = cand.dist
		} else if cand.dist == bestDist {
			// Need a tiebreaker for determinism
			if cand.newNodes[0].Q()[0].Value < bestCand.newNodes[0].Q()[0].Value {
				bestCand = cand
				bestDist = cand.dist
			}
		}
	}
	treeNode := bestCand.treeNode // The node already in the tree to which we are parenting
	newNodes := bestCand.newNodes // The node we are adding because it was the best extending PTG

	for _, newNode := range newNodes {
		ptgNum := int(newNode.Q()[0].Value)
		randAlpha := newNode.Q()[1].Value
		randDist := newNode.Q()[2].Value

		trajK, err := mp.tpFrame.PTGSolvers()[ptgNum].Trajectory(randAlpha, randDist)
		if err != nil {
			return nil, err
		}
		arcStartPose := treeNode.Pose()
		if invert {
			arcStartPose = spatialmath.PoseBetweenInverse(trajK[len(trajK)-1].Pose, arcStartPose)
		}
		lastDist := 0.
		sinceLastNode := 0.

		var trajState *ik.State
		if mp.algOpts.addIntermediate {
			for i := 0; i < len(trajK); i++ {
				trajPt := trajK[i]
				if invert {
					trajPt = trajK[(len(trajK)-1)-i]
				}
				if i == 0 {
					lastDist = trajPt.Dist
				}
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				trajState = &ik.State{Position: spatialmath.Compose(arcStartPose, trajPt.Pose)}
				if pathdebug {
					if !invert {
						mp.logger.CDebugf(ctx, "$FWDTREE,%f,%f", trajState.Position.Point().X, trajState.Position.Point().Y)
					} else {
						mp.logger.CDebugf(ctx, "$REVTREE,%f,%f", trajState.Position.Point().X, trajState.Position.Point().Y)
					}
				}
				sinceLastNode += math.Abs(trajPt.Dist - lastDist)

				// Optionally add sub-nodes along the way. Will make the final path a bit better
				// Intermediate nodes currently disabled on the goal map because they do not invert nicely
				if sinceLastNode > mp.algOpts.addNodeEvery {
					if !invert {
						// add the last node in trajectory
						addedNode = &basicNode{
							q:      referenceframe.FloatsToInputs([]float64{float64(ptgNum), randAlpha, trajPt.Dist}),
							cost:   trajPt.Dist,
							pose:   trajState.Position,
							corner: false,
						}
					}
					if addedNode != nil {
						rrt[addedNode] = treeNode
						sinceLastNode = 0.
					}
				}
				lastDist = trajPt.Dist
			}
			if pathdebug {
				mp.logger.CDebugf(ctx, "$WPI,%f,%f", trajState.Position.Point().X, trajState.Position.Point().Y)
			}
		}
		if invert {
			mp.goalNodes = append(mp.goalNodes, newNode)
		}
		rrt[newNode] = treeNode
		treeNode = newNode
	}
	return bestCand, nil
}

func (mp *tpSpaceRRTMotionPlanner) setupTPSpaceOptions() {
	if mp.planOpts.Resolution == defaultResolution {
		mp.planOpts.Resolution = defaultPTGCollisionResolution
	}

	tpOpt := &tpspaceOptions{
		autoBB: defaultAutoBB,

		addIntermediate:   defaultAddInt,
		addNodeEvery:      defaultAddNodeEvery,
		attemptSolveEvery: defaultAttemptSolveEvery,

		poseSolveDist:         defaultIdenticalNodeDistance,
		identicalNodeDistance: defaultIdenticalNodeDistance,

		distOptions:       map[tpspace.PTG]*plannerOptions{},
		invertDistOptions: map[tpspace.PTG]*plannerOptions{},

		bidirectional:         defaultBidirectional,
		goalMetricConstructor: defaultGoalMetricConstructor,
	}

	for _, curPtg := range mp.tpFrame.PTGSolvers() {
		tpOpt.distOptions[curPtg] = mp.make2DTPSpaceDistanceOptions(curPtg, false)
		tpOpt.invertDistOptions[curPtg] = mp.make2DTPSpaceDistanceOptions(curPtg, true)
	}

	mp.algOpts = tpOpt
}

// make2DTPSpaceDistanceOptions will create a plannerOptions object with a custom DistanceFunc constructed such that
// distances can be computed in TP space using the given PTG.
func (mp *tpSpaceRRTMotionPlanner) make2DTPSpaceDistanceOptions(ptg tpspace.PTGSolver, invert bool) *plannerOptions {
	opts := newBasicPlannerOptions(mp.frame)
	//nolint: gosec
	randSeed := rand.New(rand.NewSource(mp.randseed.Int63()))

	segMetric := func(seg *ik.Segment) float64 {
		// When running NearestNeighbor:
		// StartPosition is the seed/query
		// EndPosition is the pose already in the RRT tree
		if seg.StartPosition == nil || seg.EndPosition == nil {
			return math.Inf(1)
		}
		var targetFunc ik.StateMetric
		if invert {
			sqMet := mp.algOpts.goalMetricConstructor(seg.StartPosition)
			targetFunc = func(pose *ik.State) float64 {
				return sqMet(&ik.State{Position: spatialmath.PoseBetweenInverse(pose.Position, seg.EndPosition)})
			}
		} else {
			relPose := spatialmath.PoseBetween(seg.EndPosition, seg.StartPosition)
			targetFunc = mp.algOpts.goalMetricConstructor(relPose)
		}
		solutionChan := make(chan *ik.Solution, 1)
		err := ptg.Solve(context.Background(), solutionChan, nil, targetFunc, randSeed.Int())
		var closeNode *ik.Solution
		select {
		case closeNode = <-solutionChan:
		default:
		}
		if err != nil || closeNode == nil {
			return math.Inf(1)
		}
		pose, err := ptg.Transform(closeNode.Configuration)
		if err != nil {
			return math.Inf(1)
		}
		return targetFunc(&ik.State{Position: pose})
	}
	opts.DistanceFunc = segMetric
	return opts
}

func (mp *tpSpaceRRTMotionPlanner) smoothPath(ctx context.Context, path []node) []node {
	toIter := int(math.Min(float64(len(path)*len(path))/2, float64(mp.planOpts.SmoothIter)))
	currCost := sumCosts(path)

	maxCost := math.Inf(-1)
	for _, wp := range path {
		if wp.Cost() > maxCost {
			maxCost = wp.Cost()
		}
	}
	smoothPlannerMP, err := newTPSpaceMotionPlanner(mp.frame, mp.randseed, mp.logger, mp.planOpts)
	if err != nil {
		return path
	}
	smoothPlanner := smoothPlannerMP.(*tpSpaceRRTMotionPlanner)
	smoothPlanner.algOpts.bidirectional = true
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
		firstEdge := mp.randseed.Intn(len(path) - 2)
		secondEdge := firstEdge + 1 + mp.randseed.Intn((len(path)-1)-firstEdge)

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
		lastPose := spatialmath.NewZeroPose()
		for _, mynode := range path {
			trajPts, err := allPtgs[int(mynode.Q()[0].Value)].Trajectory(mynode.Q()[1].Value, mynode.Q()[2].Value)
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
				if pt.Dist >= mynode.Q()[2].Value {
					lastPose = intPose
					break
				}
			}
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
	parentPose := spatialmath.NewZeroPose()

	for j := 0; j <= firstEdge; j++ {
		pathNode := path[j]
		startMap[pathNode] = parent
		for _, adj := range []float64{0.25, 0.5, 0.75} {
			fullQ := pathNode.Q()
			newQ := []referenceframe.Input{fullQ[0], fullQ[1], {fullQ[2].Value * adj}}
			trajK, err := smoother.tpFrame.PTGSolvers()[int(math.Round(newQ[0].Value))].Trajectory(newQ[1].Value, newQ[2].Value)
			if err != nil {
				continue
			}

			intNode := &basicNode{
				q:      newQ,
				cost:   pathNode.Cost() - (pathNode.Q()[2].Value * (1 - adj)),
				pose:   spatialmath.Compose(parentPose, trajK[len(trajK)-1].Pose),
				corner: false,
			}
			startMap[intNode] = parent
		}
		parent = pathNode
		parentPose = parent.Pose()
	}
	// TODO: everything below this point can become an invocation of `smoother.planRunner`
	reached := smoother.attemptExtension(ctx, path[secondEdge], startMap, false, mp.randseed.Int31()+mp.randseed.Int31())
	if reached.error != nil || reached.node == nil {
		return nil, errors.New("could not extend to smoothing destination")
	}

	reachedDelta := mp.planOpts.DistanceFunc(&ik.Segment{StartPosition: reached.Pose(), EndPosition: path[secondEdge].Pose()})
	// If we tried the goal and have a close-enough XY location, check if the node is good enough to be a final goal
	if reachedDelta > mp.algOpts.poseSolveDist {
		return nil, errors.New("could not precisely reach smoothing destination")
	}

	newInputSteps := extractPath(startMap, nil, &nodePair{a: reached.node, b: nil}, false)

	if secondEdge < len(path)-1 {
		newInputSteps = append(newInputSteps, path[secondEdge+1:]...)
	}
	return rectifyTPspacePath(newInputSteps, mp.frame, spatialmath.NewZeroPose())
}

func (mp *tpSpaceRRTMotionPlanner) sample(rSeed node, iter int) (node, error) {
	dist := rSeed.Cost()
	if dist == 0 {
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
