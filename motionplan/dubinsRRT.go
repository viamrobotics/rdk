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

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
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

type obstacle struct {
	Center []float64 `json:"center"`
	Dims   []float64 `json:"dims"`
}

// MobileRobotPlanConfig describes a motion planning problem for a 2D mobile robot.
type MobileRobotPlanConfig struct {
	Name           string  `json:"name"`
	GridConversion float64 `json:"grid-conversion"` // in mm
	Type           string  `json:"type"`
	// planning conditions
	Start []float64 `json:"start"`
	Goal  []float64 `json:"goal"`

	// robot params
	RobotDims []float64 `json:"robot-dims"`
	Radius    float64   `json:"radius"`
	PointSep  float64   `json:"point-sep"`

	// map definition
	Xlim      []float64  `json:"xlim"`
	YLim      []float64  `json:"ylim"`
	Obstacles []obstacle `json:"obstacles"`
}

// NewDubinsRRTMotionPlanner creates a DubinsRRTMotionPlanner object.
func NewDubinsRRTMotionPlanner(frame referenceframe.Frame, nCPU int, logger golog.Logger, d Dubins) (MotionPlanner, error) {
	mp := &DubinsRRTMotionPlanner{frame: frame, logger: logger, nCPU: nCPU, D: d}

	mp.iter = planIter
	mp.stepSize = stepSize

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
	goal *commonpb.Pose,
	seed []referenceframe.Input,
	opt *PlannerOptions,
) ([][]referenceframe.Input, error) {
	solutionChan := make(chan *planReturn, 1)
	if opt == nil {
		opt = NewDefaultPlannerOptions()
		seedPos, err := mp.frame.Transform(seed[:2])
		if err != nil {
			solutionChan <- &planReturn{err: err}
			return nil, err
		}
		opt = DefaultConstraint(seedPos, spatial.NewPoseFromProtobuf(goal), mp.Frame(), opt)
	}

	utils.PanicCapturingGo(func() {
		mp.planRunner(ctx, goal, seed, opt, nil, solutionChan, 0.1)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case plan := <-solutionChan:
		finalSteps := make([][]referenceframe.Input, 0, len(plan.steps))
		for _, step := range plan.steps {
			finalSteps = append(finalSteps, step.inputs)
		}
		return finalSteps, plan.err
	}
}

// planRunner will execute the plan. When Plan() is called, it will call planRunner in a separate thread and wait for the results.
// Separating this allows other things to call planRunner in parallel while also enabling the thread-agnostic Plan to be accessible.
func (mp *DubinsRRTMotionPlanner) planRunner(ctx context.Context,
	goal *commonpb.Pose,
	seed []referenceframe.Input,
	opt *PlannerOptions,
	endpointPreview chan *configuration,
	solutionChan chan *planReturn,
	goalRate float64,
) {
	defer close(solutionChan)
	inputSteps := []*configuration{}

	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	seedConfig := &configuration{seed}
	seedMap := make(map[*configuration]*configuration)
	childMap := make(map[*configuration][]*configuration)
	seedMap[seedConfig] = nil
	childMap[seedConfig] = make([]*configuration, 0)

	pathLenMap := make(map[*configuration]float64)
	pathLenMap[seedConfig] = 0

	goalInputs := make([]referenceframe.Input, 3)
	goalInputs[0] = referenceframe.Input{Value: goal.X}
	goalInputs[1] = referenceframe.Input{Value: goal.Y}
	goalInputs[2] = referenceframe.Input{Value: goal.Theta}
	goalConfig := &configuration{goalInputs}

	dm := &dubinOptionManager{nCPU: mp.nCPU, d: mp.D}

	for i := 0; i < mp.iter; i++ {
		select {
		case <-ctx.Done():
			solutionChan <- &planReturn{err: ctx.Err()}
			return
		default:
		}

		var target *configuration
		if (rand.Float64() > 1-goalRate) || i == 0 {
			target = goalConfig
		} else {
			inputDubins := referenceframe.RandomFrameInputs(mp.frame, mp.randseed)
			inputDubins = append(inputDubins, referenceframe.Input{Value: rand.Float64() * 2 * math.Pi})
			target = &configuration{inputDubins}
		}

		targetConnected := false
		options := dm.selectOptions(ctxWithCancel, target, seedMap, 10)
		for node, o := range options {
			if o.TotalLen == math.Inf(1) {
				break
			}

			if mp.checkPath(node, target, opt, dm, o) {
				seedMap[target] = node
				if o.TotalLen < 0 {
					continue
				}
				pathLenMap[target] = pathLenMap[node] + o.TotalLen
				childMap[node] = append(childMap[node], target)
				childMap[target] = make([]*configuration, 0)
				targetConnected = true
				break
			}
		}

		if targetConnected && target != goalConfig {
			// reroute near neighbors through new node if it shortens the path
			neighbors := findNearNeighbors(target, seedMap, 10)
			for _, n := range neighbors {
				start := configuration2slice(target)
				end := configuration2slice(n)

				bestOption := dm.d.AllOptions(start, end, true)[0]
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

			solutionChan <- &planReturn{steps: inputSteps}
			for _, step := range inputSteps {
				mp.logger.Debugf("%v\n", *step)
			}
			return
		}
	}

	solutionChan <- &planReturn{err: errors.New("could not solve path")}
}

func updateChildren(
	relinkedNode *configuration,
	pathLenMap map[*configuration]float64,
	childMap map[*configuration][]*configuration,
	diff float64,
) {
	pathLenMap[relinkedNode] -= diff
	for _, child := range childMap[relinkedNode] {
		updateChildren(child, pathLenMap, childMap, diff)
	}
}

func (mp *DubinsRRTMotionPlanner) checkPath(
	from, to *configuration,
	opt *PlannerOptions,
	dm *dubinOptionManager,
	o DubinOption,
) bool {
	start := configuration2slice(from)
	end := configuration2slice(to)
	path := dm.d.generatePoints(start, end, o.DubinsPath, o.Straight)

	mp.logger.Debugf("%v\n", path)

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
		}
		input2 := make([]referenceframe.Input, 2)
		input2[0], input2[1] = referenceframe.Input{Value: p2[0]}, referenceframe.Input{Value: p2[1]}
		pose2, err := mp.frame.Transform(input2)
		if err != nil {
			mp.logger.Error("Transform failed")
		}

		ci := &ConstraintInput{
			StartPos:   pose1,
			EndPos:     pose2,
			StartInput: input1,
			EndInput:   input2,
			Frame:      mp.frame,
		}

		if ok, _ := opt.CheckConstraintPath(ci, mp.Resolution()); !ok {
			pathOk = false
			break
		}

		p1 = p2
	}

	return pathOk
}

// Used for coordinating parallel computations of dubins options.
type dubinOptionManager struct {
	optKeys chan *configuration
	options chan *nodeToOption
	optLock sync.RWMutex
	sample  *configuration
	ready   bool
	nCPU    int
	d       Dubins
}

type nodeToOption struct {
	key   *configuration
	value DubinOption
}

type nodeToOptionList []nodeToOption

func (p nodeToOptionList) Len() int           { return len(p) }
func (p nodeToOptionList) Less(i, j int) bool { return p[i].value.TotalLen < p[j].value.TotalLen }
func (p nodeToOptionList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (dm *dubinOptionManager) selectOptions(
	ctx context.Context,
	sample *configuration,
	rrtMap map[*configuration]*configuration,
	nbOptions int,
) map[*configuration]DubinOption {
	if len(rrtMap) > 1000 {
		// If the map is large, calculate distances in parallel
		return dm.parallelselectOptions(ctx, sample, rrtMap, nbOptions)
	}

	// get all options from all nodes
	pl := make(nodeToOptionList, 0)
	for node := range rrtMap {
		start := configuration2slice(node)
		end := configuration2slice(sample)
		allOpts := dm.d.AllOptions(start, end, true)

		if len(allOpts) > 0 {
			pl = append(pl, nodeToOption{node, allOpts[0]})
		}
	}
	sort.Sort(pl)

	// Sort and choose best nbOptions options
	options := make(map[*configuration]DubinOption)
	topn := nbOptions
	if len(pl) < nbOptions {
		topn = len(pl)
	}

	for _, p := range pl[:topn] {
		options[p.key] = p.value
	}

	return options
}

func (dm *dubinOptionManager) parallelselectOptions(
	ctx context.Context,
	sample *configuration,
	rrtMap map[*configuration]*configuration,
	nbOptions int,
) map[*configuration]DubinOption {
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
			pl = append(pl, *opt)
		default:
		}
	}

	// Sort and choose best nbOptions options
	sort.Sort(pl)
	options := make(map[*configuration]DubinOption)
	for _, p := range pl[:nbOptions] {
		options[p.key] = p.value
	}

	return options
}

func (dm *dubinOptionManager) startOptWorkers(ctx context.Context) {
	dm.options = make(chan *nodeToOption, dm.nCPU)
	dm.optKeys = make(chan *configuration, dm.nCPU)
	for i := 0; i < dm.nCPU; i++ {
		utils.PanicCapturingGo(func() {
			dm.optWorker(ctx)
		})
	}
}

func (dm *dubinOptionManager) optWorker(ctx context.Context) {
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
				start := configuration2slice(node)
				end := configuration2slice(dm.sample)
				allOpts := dm.d.AllOptions(start, end, true)
				dm.optLock.RUnlock()

				if len(allOpts) > 0 {
					pl = append(pl, nodeToOption{node, allOpts[0]})
				}
			}
		default:
			dm.optLock.RLock()
			if dm.ready {
				dm.optLock.RUnlock()
				opt := pl[0]
				dm.options <- &opt
				if len(pl) == 1 {
					return
				}
				pl = pl[1:]
			}
			dm.optLock.RUnlock()
		}
	}
}

func mobile2DConfigDist(from, to *configuration) float64 {
	return math.Pow(from.inputs[0].Value-to.inputs[0].Value, 2) + math.Pow(from.inputs[1].Value-to.inputs[1].Value, 2)
}

func findNearNeighbors(sample *configuration, rrtMap map[*configuration]*configuration, nbNeighbors int) []*configuration {
	keys := make([]*configuration, 0, len(rrtMap))

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

func configuration2slice(c *configuration) []float64 {
	s := make([]float64, 0)
	for _, v := range c.inputs {
		s = append(s, v.Value)
	}
	return s
}
