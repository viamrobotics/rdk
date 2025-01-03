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
	tpFrame referenceframe.Frame
	solvers []tpspace.PTGSolver

	// This tracks the nodes added to the goal tree in an ordered fashion. Nodes will always be added to this slice in the
	// same order, yielding deterministic results when the goal tree is iterated over.
	goalNodes []node
}

// newTPSpaceMotionPlanner creates a newTPSpaceMotionPlanner object with a user specified random seed.
func newTPSpaceMotionPlanner(
	fs referenceframe.FrameSystem,
	seed *rand.Rand,
	logger logging.Logger,
	opt *plannerOptions,
) (motionPlanner, error) {
	if opt == nil {
		return nil, errNoPlannerOptions
	}

	mp, err := newPlanner(fs, seed, logger, opt)
	if err != nil {
		return nil, err
	}
	tpPlanner := &tpSpaceRRTMotionPlanner{
		planner: mp,
	}
	// TODO: Only one motion chain allowed if tpspace for now. Eventually this may not be a restriction.
	if len(opt.motionChains) != 1 {
		return nil, fmt.Errorf("exactly one motion chain permitted for tpspace, but planner option had %d", len(opt.motionChains))
	}
	for _, frame := range opt.motionChains[0].frames {
		if tpFrame, ok := frame.(tpspace.PTGProvider); ok {
			tpPlanner.tpFrame = frame
			tpPlanner.solvers = tpFrame.PTGSolvers()
		}
	}

	tpPlanner.setupTPSpaceOptions()

	return tpPlanner, nil
}

func (mp *tpSpaceRRTMotionPlanner) plan(ctx context.Context, seed, goal *PlanState) ([]node, error) {
	zeroInputs := referenceframe.FrameSystemInputs{}
	zeroInputs[mp.tpFrame.Name()] = make([]referenceframe.Input, len(mp.tpFrame.DoF()))
	solutionChan := make(chan *rrtSolution, 1)

	maps := &rrtMaps{startMap: map[node]node{}, goalMap: map[node]node{}}

	startNode := &basicNode{
		q:     zeroInputs,
		poses: referenceframe.FrameSystemPoses{mp.tpFrame.Name(): referenceframe.NewZeroPoseInFrame(referenceframe.World)},
	}
	if seed != nil {
		startNode = &basicNode{q: zeroInputs, poses: seed.poses}
	}
	maps.startMap = map[node]node{startNode: nil}
	goalNode := &basicNode{q: zeroInputs, poses: goal.poses}
	maps.goalMap = map[node]node{flipNodePoses(goalNode): nil}

	var planRunners sync.WaitGroup

	planRunners.Add(1)
	utils.PanicCapturingGo(func() {
		defer planRunners.Done()
		mp.rrtBackgroundRunner(ctx, &rrtParallelPlannerShared{maps, nil, solutionChan})
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

func (mp *tpSpaceRRTMotionPlanner) tpFramePoseToFrameSystemPoses(pose spatialmath.Pose) referenceframe.FrameSystemPoses {
	return referenceframe.FrameSystemPoses{mp.tpFrame.Name(): referenceframe.NewPoseInFrame(referenceframe.World, pose)}
}

func (mp *tpSpaceRRTMotionPlanner) tpFramePose(step referenceframe.FrameSystemPoses) spatialmath.Pose {
	return step[mp.tpFrame.Name()].Pose()
}

// planRunner will execute the plan. Plan() will call planRunner in a separate thread and wait for results.
// Separating this allows other things to call planRunner in parallel allowing the thread-agnostic Plan to be accessible.
func (mp *tpSpaceRRTMotionPlanner) rrtBackgroundRunner(
	ctx context.Context,
	rrt *rrtParallelPlannerShared,
) {
	defer close(rrt.solutionChan)
	// get start and goal poses
	var startPoses referenceframe.FrameSystemPoses
	var goalPose spatialmath.Pose
	var goalNode node

	goalScore := math.Inf(1)
	for k, v := range rrt.maps.startMap {
		if v == nil {
			if k.Poses() != nil {
				startPoses = k.Poses()
			} else {
				rrt.solutionChan <- &rrtSolution{err: fmt.Errorf("start node %v must provide poses", k)}
				return
			}
			break
		}
	}
	startPoseIF, ok := startPoses[mp.tpFrame.Name()]
	if !ok {
		rrt.solutionChan <- &rrtSolution{err: fmt.Errorf("start node did not provide pose for tpspace frame %s", mp.tpFrame.Name())}
		return
	}
	startPose := startPoseIF.Pose()
	for k, v := range rrt.maps.goalMap {
		if v == nil {
			// There may be more than one node in the tree which satisfies the goal, i.e. its parent is nil.
			// However for the purposes of this we can just take the first one we see.
			if k.Poses() != nil {
				dist := mp.planOpts.poseDistanceFunc(
					&ik.Segment{
						StartPosition: startPose,
						EndPosition:   k.Poses()[mp.tpFrame.Name()].Pose(),
					})
				if dist < goalScore {
					// Update to use the closest goal to the start.
					// This is necessary in order to solve deterministically.
					goalPose = k.Poses()[mp.tpFrame.Name()].Pose()
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
		correctedPath, err := rectifyTPspacePath(path, mp.tpFrame, startPose)
		if err != nil {
			rrt.solutionChan <- &rrtSolution{err: err, maps: rrt.maps}
			return
		}

		// Print debug info if requested
		if pathdebug {
			allPtgs := mp.solvers
			lastPose := startPose
			for _, mynode := range correctedPath {
				trajPts, err := allPtgs[int(mynode.Q()[mp.tpFrame.Name()][0].Value)].Trajectory(
					mynode.Q()[mp.tpFrame.Name()][1].Value,
					mynode.Q()[mp.tpFrame.Name()][2].Value,
					mynode.Q()[mp.tpFrame.Name()][3].Value,
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

	midptNode := &basicNode{
		poses: mp.tpFramePoseToFrameSystemPoses(spatialmath.NewPose(midPt, midOrient)),
		cost:  midPt.Sub(startPose.Point()).Norm(),
	}
	var randPosNode node = midptNode

	for iter := 0; iter < mp.planOpts.PlanIter; iter++ {
		if pathdebug {
			randPose := mp.tpFramePose(randPosNode.Poses())
			mp.logger.Debugf("$RRTGOAL,%f,%f", randPose.Point().X, randPose.Point().Y)
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
			m2chan <- mp.attemptExtension(ctx, flipNodePoses(randPosNode), rrt.maps.goalMap, true)
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
			reachedDelta = mp.planOpts.poseDistanceFunc(&ik.Segment{
				StartPosition: mp.tpFramePose(seedReached.node.Poses()),
				EndPosition:   mp.tpFramePose(flipNodePoses(goalReached.node).Poses()),
			})
			if reachedDelta > mp.planOpts.GoalThreshold {
				// If both maps extended, but did not reach the same point, then attempt to extend them towards each other
				seedReached = mp.attemptExtension(ctx, flipNodePoses(goalReached.node), rrt.maps.startMap, false)
				if seedReached.error != nil {
					rrt.solutionChan <- &rrtSolution{err: seedReached.error, maps: rrt.maps}
					return
				}
				if seedReached.node != nil {
					reachedDelta = mp.planOpts.poseDistanceFunc(&ik.Segment{
						StartPosition: mp.tpFramePose(seedReached.node.Poses()),
						EndPosition:   mp.tpFramePose(flipNodePoses(goalReached.node).Poses()),
					})
					if reachedDelta > mp.planOpts.GoalThreshold {
						goalReached = mp.attemptExtension(ctx, flipNodePoses(seedReached.node), rrt.maps.goalMap, true)
						if goalReached.error != nil {
							rrt.solutionChan <- &rrtSolution{err: goalReached.error, maps: rrt.maps}
							return
						}
					}
					if goalReached.node != nil {
						reachedDelta = mp.planOpts.poseDistanceFunc(&ik.Segment{
							StartPosition: mp.tpFramePose(seedReached.node.Poses()),
							EndPosition:   mp.tpFramePose(flipNodePoses(goalReached.node).Poses()),
						})
					}
				}
			}
			if reachedDelta <= mp.planOpts.GoalThreshold {
				// If we've reached the goal, extract the path from the RRT trees and return
				path := extractTPspacePath(
					mp.tpFrame.Name(),
					rrt.maps.startMap,
					rrt.maps.goalMap,
					&nodePair{a: seedReached.node, b: goalReached.node},
				)
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

				seedReached := mp.attemptExtension(ctx, flipNodePoses(goalMapNode), rrt.maps.startMap, false)
				if seedReached.error != nil {
					rrt.solutionChan <- &rrtSolution{err: seedReached.error, maps: rrt.maps}
					return
				}
				if seedReached.node == nil {
					continue
				}
				reachedDelta = mp.planOpts.poseDistanceFunc(&ik.Segment{
					StartPosition: mp.tpFramePose(seedReached.node.Poses()),
					EndPosition:   mp.tpFramePose(flipNodePoses(goalMapNode).Poses()),
				})
				if reachedDelta <= mp.planOpts.GoalThreshold {
					// If we've reached the goal, extract the path from the RRT trees and return
					path := extractTPspacePath(
						mp.tpFrame.Name(),
						rrt.maps.startMap,
						rrt.maps.goalMap,
						&nodePair{a: seedReached.node, b: goalMapNode},
					)
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

	nm := &neighborManager{nCPU: mp.planOpts.NumThreads / len(mp.solvers)}
	nm.parallelNeighbors = 10

	var successNode node
	var solution *ik.Solution
	var err error

	if nearest == nil {
		// Get nearest neighbor to rand config in tree using this PTG
		nearest = nm.nearestNeighbor(ctx, ptgDistOpt, randPosNode, rrt)
		if nearest == nil {
			return nil, errNoNeighbors
		}

		rawVal, ok := distMap.Load(mp.tpFramePose(nearest.Poses()))
		if !ok {
			mp.logger.Error("nearest neighbor failed to find nearest pose in distMap")
			return nil, errNoNeighbors
		}

		solution, ok = rawVal.(*ik.Solution)
		if !ok {
			mp.logger.Error("nearest neighbor ik.Solution type conversion failed")
			return nil, errNoNeighbors
		}
	} else {
		solution, err = mp.ptgSolution(curPtg, mp.tpFramePose(nearest.Poses()), mp.tpFramePose(randPosNode.Poses()))
		if err != nil || solution == nil {
			return nil, err
		}
	}

	// Get cartesian distance from NN to rand
	arcStartPose := mp.tpFramePose(nearest.Poses())
	successNodes := []node{}
	arcPose := spatialmath.NewZeroPose() // This will be the relative pose that is the delta from one end of the combined traj to the other.

	// We may produce more than one consecutive arc. Reduce the one configuration to several 2dof arcs
	for i := 0; i < len(solution.Configuration); i += 2 {
		subConfig := referenceframe.FrameSystemInputs{
			mp.tpFrame.Name(): referenceframe.FloatsToInputs(solution.Configuration[i : i+2]),
		}
		subNode := newConfigurationNode(subConfig)

		// Check collisions along this traj and get the longest distance viable
		trajK, err := curPtg.Trajectory(
			subNode.Q()[mp.tpFrame.Name()][0].Value,
			0,
			subNode.Q()[mp.tpFrame.Name()][1].Value,
			mp.planOpts.Resolution,
		)
		if err != nil {
			return nil, err
		}

		goodNode := mp.checkTraj(trajK, arcStartPose)
		if goodNode == nil {
			break
		}

		partialExtend := false
		for i, val := range subNode.Q()[mp.tpFrame.Name()] {
			if goodNode.Q()[mp.tpFrame.Name()][i] != val {
				partialExtend = true
			}
		}

		arcPose = spatialmath.Compose(arcPose, mp.tpFramePose(goodNode.Poses()))

		// add the last node in trajectory
		arcStartPose = spatialmath.Compose(arcStartPose, mp.tpFramePose(goodNode.Poses()))

		successNode = &basicNode{
			q: referenceframe.FrameSystemInputs{
				mp.tpFrame.Name(): {{float64(ptgNum)}, goodNode.Q()[mp.tpFrame.Name()][0], {0}, goodNode.Q()[mp.tpFrame.Name()][1]},
			},
			cost:   goodNode.Cost(),
			poses:  mp.tpFramePoseToFrameSystemPoses(arcStartPose),
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

	targetFunc := defaultGoalMetricConstructor(spatialmath.PoseBetween(arcStartPose, mp.tpFramePose(randPosNode.Poses())))
	bestDist := targetFunc(&ik.State{Position: arcPose})

	cand := &candidate{dist: bestDist, treeNode: nearest, newNodes: successNodes}
	// check if this  successNode is too close to nodes already in the tree, and if so, do not add.
	// Get nearest neighbor to new node that's already in the tree. Note that this uses cartesian distance (planOpts.poseDistanceFunc)
	// rather than the TP-space distance functions in algOpts.
	nearest = nm.nearestNeighbor(ctx, mp.planOpts, successNode, rrt)
	if nearest != nil {
		dist := mp.planOpts.poseDistanceFunc(&ik.Segment{
			StartPosition: mp.tpFramePose(successNode.Poses()),
			EndPosition:   mp.tpFramePose(nearest.Poses()),
		})
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

		trajState := &ik.State{Position: spatialmath.Compose(arcStartPose, trajPt.Pose), Frame: mp.tpFrame}

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
			q: referenceframe.FrameSystemInputs{
				mp.tpFrame.Name(): {{trajPt.Alpha}, {trajPt.Dist}},
			},
			cost:   trajPt.Dist,
			poses:  mp.tpFramePoseToFrameSystemPoses(trajPt.Pose),
			corner: false,
		}
		passed = append(passed, okNode)
	}

	lastTrajPt := trajK[len(trajK)-1]
	return &basicNode{
		q: referenceframe.FrameSystemInputs{
			mp.tpFrame.Name(): {{lastTrajPt.Alpha}, {lastTrajPt.Dist}},
		},
		cost:   lastTrajPt.Dist,
		poses:  passed[len(passed)-1].Poses(),
		corner: false,
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
	candChan := make(chan *candidate, len(mp.solvers))
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

		for ptgNum, curPtg := range mp.solvers {
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

		for i := 0; i < len(mp.solvers); i++ {
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
			distTravelledByCandidate += math.Abs(newNode.Q()[mp.tpFrame.Name()][3].Value - newNode.Q()[mp.tpFrame.Name()][2].Value)
		}
		distToGoal := mp.tpFramePose(endNode.Poses()).Point().Distance(mp.tpFramePose(goalNode.Poses()).Point())
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
		ptgNum := int(newNode.Q()[mp.tpFrame.Name()][0].Value)
		randAlpha := newNode.Q()[mp.tpFrame.Name()][1].Value
		randDist := newNode.Q()[mp.tpFrame.Name()][3].Value - newNode.Q()[mp.tpFrame.Name()][2].Value

		trajK, err := mp.solvers[ptgNum].Trajectory(randAlpha, 0, randDist, mp.planOpts.Resolution)
		if err != nil {
			return nil, err
		}
		arcStartPose := mp.tpFramePose(treeNode.Poses())
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
					q: referenceframe.FrameSystemInputs{
						mp.tpFrame.Name(): referenceframe.FloatsToInputs([]float64{float64(ptgNum), randAlpha, 0, trajPt.Dist}),
					},
					cost:   trajPt.Dist,
					poses:  mp.tpFramePoseToFrameSystemPoses(trajState.Position),
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
	}

	mp.algOpts = tpOpt
}

func (mp *tpSpaceRRTMotionPlanner) ptgSolution(ptg tpspace.PTGSolver,
	nearestPose, randPosNodePose spatialmath.Pose,
) (*ik.Solution, error) {
	relPose := spatialmath.PoseBetween(nearestPose, randPosNodePose)
	targetFunc := defaultGoalMetricConstructor(relPose)
	seedDist := relPose.Point().Norm()
	seed := tpspace.PTGIKSeed(ptg)
	dof := ptg.DoF()
	if seedDist < dof[1].Max {
		seed[1].Value = seedDist
	}
	if relPose.Point().X < 0 {
		seed[0].Value *= -1
	}

	solution, err := ptg.Solve(context.Background(), seed, targetFunc)

	return solution, err
}

// make2DTPSpaceDistanceOptions will create a plannerOptions object with a custom DistanceFunc constructed such that
// distances can be computed in TP space using the given PTG.
// Also returns a pointer to a sync.Map of nearest poses -> ik.Solution so the (expensive to compute) solution can be reused.
func (mp *tpSpaceRRTMotionPlanner) make2DTPSpaceDistanceOptions(ptg tpspace.PTGSolver) (*plannerOptions, *sync.Map) {
	m := sync.Map{}
	opts := newBasicPlannerOptions()
	segMetric := func(seg *ik.Segment) float64 {
		// When running NearestNeighbor:
		// StartPosition is the seed/query
		// EndPosition is the pose already in the RRT tree
		if seg.StartPosition == nil || seg.EndPosition == nil {
			return math.Inf(1)
		}
		solution, err := mp.ptgSolution(ptg, seg.EndPosition, seg.StartPosition)

		if err != nil || solution == nil {
			return math.Inf(1)
		}

		m.Store(seg.EndPosition, solution)

		return solution.Score
	}
	opts.poseDistanceFunc = segMetric
	opts.nodeDistanceFunc = func(node1, node2 node) float64 {
		return segMetric(&ik.Segment{
			StartPosition: mp.tpFramePose(node1.Poses()),
			EndPosition:   mp.tpFramePose(node2.Poses()),
		})
	}
	opts.Resolution = defaultPTGCollisionResolution
	return opts, &m
}

// smoothPath takes in a path and attempts to smooth it by randomly sampling edges in the path and seeing
// if they can be connected.
func (mp *tpSpaceRRTMotionPlanner) smoothPath(ctx context.Context, path []node) []node {
	toIter := int(math.Min(float64(len(path)*len(path))/2, float64(mp.planOpts.SmoothIter)))
	currCost := sumCosts(path)
	smoothPlannerMP, err := newTPSpaceMotionPlanner(mp.fs, mp.randseed, mp.logger, mp.planOpts)
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
		allPtgs := mp.solvers
		lastPose := mp.tpFramePose(path[0].Poses())
		for _, mynode := range path {
			trajPts, err := allPtgs[int(mynode.Q()[mp.tpFrame.Name()][0].Value)].Trajectory(
				mynode.Q()[mp.tpFrame.Name()][1].Value,
				mynode.Q()[mp.tpFrame.Name()][2].Value,
				mynode.Q()[mp.tpFrame.Name()][3].Value,
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
	parentPose := mp.tpFramePose(path[0].Poses())

	for j := 0; j <= firstEdge; j++ {
		pathNode := path[j]
		startMap[pathNode] = parent
		for adjNum := defaultSmoothChunkCount - 1; adjNum > 0; adjNum-- {
			fullQ := pathNode.Q()[mp.tpFrame.Name()]
			adj := (fullQ[3].Value - fullQ[2].Value) * (float64(adjNum) / float64(defaultSmoothChunkCount))
			newQ := referenceframe.FrameSystemInputs{
				mp.tpFrame.Name(): {fullQ[0], fullQ[1], fullQ[2], {fullQ[3].Value - adj}},
			}
			trajK, err := smoother.solvers[int(math.Round(newQ[mp.tpFrame.Name()][0].Value))].Trajectory(
				newQ[mp.tpFrame.Name()][1].Value,
				newQ[mp.tpFrame.Name()][2].Value,
				newQ[mp.tpFrame.Name()][3].Value,
				mp.planOpts.Resolution,
			)
			if err != nil {
				continue
			}

			intNode := &basicNode{
				q:      newQ,
				cost:   pathNode.Cost() - math.Abs(adj),
				poses:  mp.tpFramePoseToFrameSystemPoses(spatialmath.Compose(parentPose, trajK[len(trajK)-1].Pose)),
				corner: false,
			}
			startMap[intNode] = parent
		}
		parent = pathNode
		parentPose = mp.tpFramePose(parent.Poses())
	}
	// TODO: everything below this point can become an invocation of `smoother.planRunner`
	reached := smoother.attemptExtension(ctx, path[secondEdge], startMap, false)
	if reached.error != nil || reached.node == nil {
		return nil, errors.New("could not extend to smoothing destination")
	}

	reachedDelta := mp.planOpts.poseDistanceFunc(&ik.Segment{
		StartPosition: mp.tpFramePose(reached.node.Poses()),
		EndPosition:   mp.tpFramePose(path[secondEdge].Poses()),
	})

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
	return rectifyTPspacePath(newInputSteps, mp.tpFrame, mp.tpFramePose(path[0].Poses()))
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
		r3.Vector{
			mp.tpFramePose(rSeed.Poses()).Point().X + (randPosX - rDist/2.),
			mp.tpFramePose(rSeed.Poses()).Point().Y + (randPosY - rDist/2.),
			0,
		},
		&spatialmath.OrientationVector{OZ: 1, Theta: randPosTheta},
	)
	return &basicNode{poses: mp.tpFramePoseToFrameSystemPoses(randPos)}, nil
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
		wpPose, err := frame.Transform(wp.Q()[frame.Name()])
		if err != nil {
			return nil, err
		}
		runningPose = spatialmath.Compose(runningPose, wpPose)

		thisNode := &basicNode{
			q:      wp.Q(),
			cost:   wp.Cost(),
			poses:  referenceframe.FrameSystemPoses{frame.Name(): referenceframe.NewPoseInFrame(referenceframe.World, runningPose)},
			corner: wp.Corner(),
		}
		correctedPath = append(correctedPath, thisNode)
	}
	return correctedPath, nil
}

func extractTPspacePath(fName string, startMap, goalMap map[node]node, pair *nodePair) []node {
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
		if startMap[startReached] == nil {
			path = append(path,
				&basicNode{
					q: referenceframe.FrameSystemInputs{
						fName: {{0}, {0}, {0}, {0}},
					},
					cost:   startReached.Cost(),
					poses:  startReached.Poses(),
					corner: startReached.Corner(),
				})
		} else {
			path = append(path, startReached)
		}
		startReached = startMap[startReached]
	}

	// reverse the slice
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	// extract the path to the goal
	for goalReached != nil {
		goalPiF := goalReached.Poses()[fName]
		var goalReachedReversed node
		if goalMap[goalReached] == nil {
			// Add the final node
			goalReachedReversed = &basicNode{
				q: referenceframe.FrameSystemInputs{
					fName: {{0}, {0}, {0}, {0}},
				},
				cost: goalReached.Cost(),
				poses: referenceframe.FrameSystemPoses{fName: referenceframe.NewPoseInFrame(
					goalPiF.Parent(),
					spatialmath.Compose(goalPiF.Pose(), flipPose),
				)},
				corner: goalReached.Corner(),
			}
		} else {
			goalReachedReversed = &basicNode{
				q: referenceframe.FrameSystemInputs{
					fName: {
						goalReached.Q()[fName][0],
						goalReached.Q()[fName][1],
						goalReached.Q()[fName][3],
						goalReached.Q()[fName][2],
					},
				},
				cost: goalReached.Cost(),
				poses: referenceframe.FrameSystemPoses{fName: referenceframe.NewPoseInFrame(
					goalPiF.Parent(),
					spatialmath.Compose(goalPiF.Pose(), flipPose),
				)},
				corner: goalReached.Corner(),
			}
		}
		path = append(path, goalReachedReversed)
		goalReached = goalMap[goalReached]
	}
	return path
}

// Returns a new node whose orientation is flipped 180 degrees from the provided node.
// It does NOT flip the configurations/inputs.
func flipNodePoses(n node) node {
	flippedPoses := referenceframe.FrameSystemPoses{}
	for f, pif := range n.Poses() {
		flippedPoses[f] = referenceframe.NewPoseInFrame(pif.Parent(), spatialmath.Compose(pif.Pose(), flipPose))
	}

	return &basicNode{
		q:      n.Q(),
		cost:   n.Cost(),
		poses:  flippedPoses,
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
