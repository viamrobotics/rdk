package motionplan

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// DubinsRRTMotionPlanner an object able to solve for paths using Dubin's Car Model
// around obstacles to some goal for a given referenceframe.
// It uses the RRT* with vehicle dynamics algorithm, Khanal 2022
// https://arxiv.org/abs/2206.10533
type DubinsRRTMotionPlanner struct {
	frame    referenceframe.Frame
	logger   golog.Logger
	iter     int
	nCPU     int
	stepSize float64
	randseed *rand.Rand
	D        Dubins
}

// NewDubinsRRTMotionPlanner creates a DubinsRRTMotionPlanner object.
func NewDubinsRRTMotionPlanner(frame referenceframe.Frame, nCPU int, logger golog.Logger, d Dubins) (*DubinsRRTMotionPlanner, error) {
	mp := &DubinsRRTMotionPlanner{frame: frame, logger: logger, nCPU: nCPU, D: d}

	// TODO(rb): this should support PlannerOptions in the way the other planners do
	mp.iter = defaultPlanIter
	mp.stepSize = defaultResolution

	//nolint:gosec
	mp.randseed = rand.New(rand.NewSource(1))

	return mp, nil
}

// Frame will return the frame used for planning.
func (mp *DubinsRRTMotionPlanner) Frame() referenceframe.Frame {
	return mp.frame
}

// Resolution specifies how narrowly to check for constraints.
func (mp *DubinsRRTMotionPlanner) Resolution() float64 {
	return mp.stepSize
}

// Plan will take a context, a goal position, and an input start state and return a series of state waypoints which
// should be visited in order to arrive at the goal while satisfying all constraints.
func (mp *DubinsRRTMotionPlanner) Plan(ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
	planOpts *plannerOptions,
) ([][]referenceframe.Input, error) {
	solutionChan := make(chan *rrtPlanReturn, 1)
	if planOpts == nil {
		planOpts = newBasicPlannerOptions()
	}
	planOpts.SetGoalMetric(NewSquaredNormMetric(goal))

	utils.PanicCapturingGo(func() {
		mp.planRunner(ctx, goal, seed, planOpts, solutionChan, 0.1)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case plan := <-solutionChan:
		return plan.toInputs(), plan.err()
	}
}

// planRunner will execute the plan. When Plan() is called, it will call planRunner in a separate thread and wait for the results.
// Separating this allows other things to call planRunner in parallel while also enabling the thread-agnostic Plan to be accessible.
func (mp *DubinsRRTMotionPlanner) planRunner(
	ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
	planOpts *plannerOptions,
	solutionChan chan *rrtPlanReturn,
	goalRate float64,
) {
	defer close(solutionChan)
	inputSteps := []node{}

	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	seedConfig := &basicNode{q: seed}
	seedMap := make(map[node]node)
	childMap := make(map[node][]node)
	seedMap[seedConfig] = nil
	childMap[seedConfig] = make([]node, 0)

	pathLenMap := make(map[node]float64)
	pathLenMap[seedConfig] = 0

	goalInputs := make([]referenceframe.Input, 3)
	goalInputs[0] = referenceframe.Input{Value: goal.Point().X}
	goalInputs[1] = referenceframe.Input{Value: goal.Point().Y}
	goalInputs[2] = referenceframe.Input{Value: goal.Orientation().OrientationVectorDegrees().Theta}
	goalConfig := &basicNode{q: goalInputs}

	dm := &dubinPathAttrManager{nCPU: mp.nCPU, d: mp.D}

	for i := 0; i < mp.iter; i++ {
		select {
		case <-ctx.Done():
			solutionChan <- &rrtPlanReturn{planerr: ctx.Err()}
			return
		default:
		}

		var target node
		//nolint:gosec
		if (rand.Float64() > 1-goalRate) || i == 0 {
			target = goalConfig
		} else {
			inputDubins := referenceframe.RandomFrameInputs(mp.frame, mp.randseed)
			//nolint:gosec
			inputDubins = append(inputDubins, referenceframe.Input{Value: rand.Float64() * 2 * math.Pi})
			target = &basicNode{q: inputDubins}
		}

		targetConnected := false
		options := dm.selectOptions(ctxWithCancel, target, seedMap, 10)
		for n, o := range options {
			if o.TotalLen == math.Inf(1) {
				break
			}

			if mp.checkPath(n, target, planOpts, dm, o) {
				seedMap[target] = n
				if o.TotalLen < 0 {
					continue
				}
				pathLenMap[target] = pathLenMap[n] + o.TotalLen
				childMap[n] = append(childMap[n], target)
				childMap[target] = make([]node, 0)
				targetConnected = true
				break
			}
		}

		if targetConnected && target != goalConfig {
			// reroute near neighbors through new node if it shortens the path
			neighbors := findNearNeighbors(target, seedMap, 10)
			for _, n := range neighbors {
				start := nodeToSlice(target)
				end := nodeToSlice(n)

				bestOption := dm.d.AllPaths(start, end, true)[0]
				if bestOption.TotalLen < 0 {
					continue
				}

				if pathLenMap[target]+bestOption.TotalLen < pathLenMap[n] {
					// Remove n from it's parent's children
					parentChildList := childMap[seedMap[n]]
					for i, child := range parentChildList {
						if child == n {
							parentChildList[i] = parentChildList[len(parentChildList)-1]
							parentChildList[len(parentChildList)-1] = nil
							break
						}
					}
					childMap[seedMap[n]] = parentChildList[:len(parentChildList)-1]

					// Add n to target's children
					childMap[target] = append(childMap[target], n)

					// Set target as n's parent
					seedMap[n] = target

					// Update path lengths of n and its children
					diff := pathLenMap[n] - (pathLenMap[target] + bestOption.TotalLen)

					updateChildren(n, pathLenMap, childMap, diff)
				}
			}
		}

		if targetConnected && target == goalConfig {
			cancel()

			// extract the path to the seed
			seedReached := target
			for seedReached != nil {
				inputSteps = append(inputSteps, seedReached)
				seedReached = seedMap[seedReached]
			}

			// reverse the slice
			for i, j := 0, len(inputSteps)-1; i < j; i, j = i+1, j-1 {
				inputSteps[i], inputSteps[j] = inputSteps[j], inputSteps[i]
			}

			solutionChan <- &rrtPlanReturn{steps: inputSteps}
			for _, step := range inputSteps {
				mp.logger.Debugf("%v\n", step)
			}
			return
		}
	}

	solutionChan <- &rrtPlanReturn{planerr: errors.New("could not solve path")}
}

func updateChildren(
	relinkedNode node,
	pathLenMap map[node]float64,
	childMap map[node][]node,
	diff float64,
) {
	pathLenMap[relinkedNode] -= diff
	for _, child := range childMap[relinkedNode] {
		updateChildren(child, pathLenMap, childMap, diff)
	}
}

func (mp *DubinsRRTMotionPlanner) checkPath(
	from, to node,
	planOpts *plannerOptions,
	dm *dubinPathAttrManager,
	o DubinPathAttr,
) bool {
	start := nodeToSlice(from)
	end := nodeToSlice(to)
	path := dm.d.generatePoints(start, end, o.DubinsPath, o.Straight)

	pathOk := true
	var p2 []float64
	p1 := path[0]
	for _, p := range path[1:] {
		p2 = p

		input1 := make([]referenceframe.Input, 2)
		input1[0], input1[1] = referenceframe.Input{Value: p1[0]}, referenceframe.Input{Value: p1[1]}
		pose1, err := mp.frame.Transform(input1)
		if err != nil {
			mp.logger.Error("Transform failed")
			return false
		}
		input2 := make([]referenceframe.Input, 2)
		input2[0], input2[1] = referenceframe.Input{Value: p2[0]}, referenceframe.Input{Value: p2[1]}
		pose2, err := mp.frame.Transform(input2)
		if err != nil {
			mp.logger.Error("Transform failed")
			return false
		}

		ci := &Segment{
			StartPosition:      pose1,
			EndPosition:        pose2,
			StartConfiguration: input1,
			EndConfiguration:   input2,
			Frame:              mp.frame,
		}

		if ok, _ := planOpts.CheckSegmentAndStateValidity(ci, mp.Resolution()); !ok {
			pathOk = false
			break
		}

		p1 = p2
	}

	return pathOk
}

// Used for coordinating parallel computations of dubins path options.
type dubinPathAttrManager struct {
	optKeys chan node
	options chan *nodeToOptionList
	optLock sync.RWMutex
	sample  node
	ready   bool
	nCPU    int
	d       Dubins
}

type nodeToOption struct {
	key   node
	value DubinPathAttr
}

type nodeToOptionList []nodeToOption

func (p nodeToOptionList) Len() int           { return len(p) }
func (p nodeToOptionList) Less(i, j int) bool { return p[i].value.TotalLen < p[j].value.TotalLen }
func (p nodeToOptionList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (dm *dubinPathAttrManager) selectOptions(
	ctx context.Context,
	sample node,
	rrtMap map[node]node,
	nbOptions int,
) map[node]DubinPathAttr {
	if len(rrtMap) < 1 {
		// If the map is large, calculate distances in parallel
		return dm.parallelselectOptions(ctx, sample, rrtMap, nbOptions)
	}

	// get all options from all nodes
	pl := make(nodeToOptionList, 0)
	for node := range rrtMap {
		start := nodeToSlice(node)
		end := nodeToSlice(sample)
		bestOpt := dm.d.AllPaths(start, end, true)[0]

		if bestOpt.TotalLen != math.Inf(1) {
			pl = append(pl, nodeToOption{node, bestOpt})
		}
	}
	sort.Sort(pl)

	// Sort and choose best nbOptions options
	options := make(map[node]DubinPathAttr)
	topn := nbOptions
	if len(pl) < nbOptions {
		topn = len(pl)
	}

	for _, p := range pl[:topn] {
		options[p.key] = p.value
	}

	return options
}

func (dm *dubinPathAttrManager) parallelselectOptions(
	ctx context.Context,
	sample node,
	rrtMap map[node]node,
	nbOptions int,
) map[node]DubinPathAttr {
	dm.ready = false
	dm.startOptWorkers(ctx)
	defer close(dm.optKeys)
	defer close(dm.options)
	dm.optLock.Lock()
	dm.sample = sample
	dm.optLock.Unlock()

	for k := range rrtMap {
		dm.optKeys <- k
	}
	dm.optLock.Lock()
	dm.ready = true
	dm.optLock.Unlock()
	pl := make(nodeToOptionList, 0)
	returned := 0
	for returned < dm.nCPU {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		select {
		case opt := <-dm.options:
			returned++
			pl = append(pl, *opt...)
		default:
		}
	}

	// Sort and choose best nbOptions options
	sort.Sort(pl)
	options := make(map[node]DubinPathAttr)
	topn := nbOptions
	if len(pl) < nbOptions {
		topn = len(pl)
	}
	for _, p := range pl[:topn] {
		options[p.key] = p.value
	}

	return options
}

func (dm *dubinPathAttrManager) startOptWorkers(ctx context.Context) {
	dm.options = make(chan *nodeToOptionList, dm.nCPU)
	dm.optKeys = make(chan node, dm.nCPU)
	for i := 0; i < dm.nCPU; i++ {
		utils.PanicCapturingGo(func() {
			dm.optWorker(ctx)
		})
	}
}

func (dm *dubinPathAttrManager) optWorker(ctx context.Context) {
	pl := make(nodeToOptionList, 0)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case node := <-dm.optKeys:
			if node != nil {
				dm.optLock.RLock()
				start := nodeToSlice(node)
				end := nodeToSlice(dm.sample)
				bestOpt := dm.d.AllPaths(start, end, true)[0]
				dm.optLock.RUnlock()

				if bestOpt.TotalLen != math.Inf(1) {
					pl = append(pl, nodeToOption{node, bestOpt})
				}
			}
		default:
			dm.optLock.RLock()
			if dm.ready {
				dm.optLock.RUnlock()
				dm.options <- &pl
				return
			}
			dm.optLock.RUnlock()
		}
	}
}

func mobile2DConfigDist(from, to node) float64 {
	return math.Pow(from.Q()[0].Value-to.Q()[0].Value, 2) + math.Pow(from.Q()[1].Value-to.Q()[1].Value, 2)
}

// TODO: Update nearestNeighbor.go to take a custom distance function, so then everything can use the same function (rh pl rb).
func findNearNeighbors(sample node, rrtMap map[node]node, nbNeighbors int) []node {
	keys := make([]node, 0, len(rrtMap))

	for key := range rrtMap {
		if key == sample {
			continue
		}
		keys = append(keys, key)
	}

	sort.SliceStable(keys, func(i, j int) bool {
		return mobile2DConfigDist(keys[i], sample) < mobile2DConfigDist(keys[j], sample)
	})

	topn := nbNeighbors
	if len(keys) < nbNeighbors {
		topn = len(keys)
	}

	return keys[:topn]
}

func nodeToSlice(c node) []float64 {
	s := make([]float64, 0)
	for _, v := range c.Q() {
		s = append(s, v.Value)
	}
	return s
}
