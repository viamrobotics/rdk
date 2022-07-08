package motionplan

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/utils"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

// dubinsRRTMotionPlanner an object able to solve for paths using Dubin's Car Model
// around obstacles to some goal for a given referenceframe.
// It uses the RRT* with vehicle dynamics algorithm, Khanal 2022
// https://arxiv.org/abs/2206.10533
type dubinsRRTMotionPlanner struct {
	solDist         float64
	solver          InverseKinematics
	fastGradDescent *NloptIK
	frame           referenceframe.Frame
	logger          golog.Logger
	qstep           []float64
	iter            int
	nCPU            int
	stepSize        float64
	randseed        *rand.Rand
	d 				Dubins
}

// NewCBiRRTMotionPlanner creates a cBiRRTMotionPlanner object.
func NewDubinsRRTMotionPlanner(frame referenceframe.Frame, nCPU int, logger golog.Logger, d Dubins) (MotionPlanner, error) {
	ik, err := CreateCombinedIKSolver(frame, logger, nCPU)
	if err != nil {
		return nil, err
	}
	// nlopt should try only once
	nlopt, err := CreateNloptIKSolver(frame, logger, 1)
	if err != nil {
		return nil, err
	}
	mp := &dubinsRRTMotionPlanner{solver: ik, fastGradDescent: nlopt, frame: frame, logger: logger, solDist: jointSolveDist, nCPU: nCPU, d: d}

	mp.qstep = getFrameSteps(frame, frameStep)
	mp.iter = planIter
	mp.stepSize = stepSize

	//nolint:gosec
	mp.randseed = rand.New(rand.NewSource(1))

	return mp, nil
}

func (mp *dubinsRRTMotionPlanner) Frame() referenceframe.Frame {
	return mp.frame
}

func (mp *dubinsRRTMotionPlanner) Resolution() float64 {
	return mp.stepSize
}

func (mp *dubinsRRTMotionPlanner) Plan(ctx context.Context,
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
		goalPos := spatial.NewPoseFromProtobuf(fixOvIncrement(goal, spatial.PoseToProtobuf(seedPos)))
		opt = DefaultConstraint(seedPos, goalPos, mp.Frame(), opt)
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
func (mp *dubinsRRTMotionPlanner) planRunner(ctx context.Context,
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

	corners := map[*configuration]bool{}

	seedConfig := &configuration{seed}
	seedMap := make(map[*configuration]*configuration)
	seedMap[seedConfig] = nil

	pathLenMap := make(map[*configuration]float64)
	pathLenMap[seedConfig] = 0

	goalInputs := make([]referenceframe.Input, 3)
	goalInputs[0], goalInputs[1], goalInputs[2] = referenceframe.Input{Value: goal.X}, referenceframe.Input{Value: goal.Y}, referenceframe.Input{Value: goal.Theta}
	goalConfig := &configuration{goalInputs}

	dm := &dubinOptionManager{nCPU: mp.nCPU, d: mp.d}

	for i := 0; i < mp.iter; i++ {
		select {
		case <-ctx.Done():
			solutionChan <- &planReturn{err: ctx.Err()}
			return
		default:
		}

		var target *configuration
		if (rand.Float64() > 1 - goalRate) || i == 0 {
			target = goalConfig
		} else {
			input2D := referenceframe.RandomFrameInputs(mp.frame, mp.randseed)
			inputDubins := append(input2D, referenceframe.Input{Value: rand.Float64() * 2 * math.Pi})
			target = &configuration{inputDubins}
		}

		targetConnected := false
		options := dm.selectOptions(ctxWithCancel, target, seedMap, 10)
		for node, o := range options {
			if o.totalLen == math.Inf(1) {
				break
			}

			if mp.CheckPath(node, target, opt, dm, o) {
				seedMap[target] = node
				pathLenMap[target] = pathLenMap[node] + o.totalLen
				targetConnected = true
				break
			}
		}

		if targetConnected && target != goalConfig {
			corners[target] = true

			// reroute near neighbors through new node if it shortens the path
			neighbors := findNearNeighbors(target, seedMap, 10)
			for _, n := range neighbors {
				start := configuration2slice(target)
				end := configuration2slice(n)

				bestOption := dm.d.AllOptions(start, end, true)[0]
				if pathLenMap[target] + bestOption.totalLen < pathLenMap[n] {
					seedMap[n] = target
					pathLenMap[n] = pathLenMap[target] + bestOption.totalLen
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
			return
		}
	}

	solutionChan <- &planReturn{err: errors.New("could not solve path")}
}

func (mp *dubinsRRTMotionPlanner) CheckPath(
	from, to *configuration,
	opt *PlannerOptions,
	dm *dubinOptionManager,
	o DubinOption,
) bool {
	start := configuration2slice(from)
	end := configuration2slice(to)
	path := dm.d.GeneratePoints(start, end, o.dubinsPath, o.straight)

	pathOk := true
	p1, p2 := path[0], path[1]
	for _, p := range path[1:] {
		p2 = p

		pose1 := spatial.NewPoseFromPoint(r3.Vector{X: p1[0], Y: p1[1], Z: 0})
		pose2 := spatial.NewPoseFromPoint(r3.Vector{X: p2[0], Y: p2[1], Z: 0})
		input1 := make([]referenceframe.Input, 2)
		input1[0], input1[1] = referenceframe.Input{Value: p1[0]}, referenceframe.Input{Value: p1[1]}
		input2 := make([]referenceframe.Input, 2)
		input2[0], input2[1] = referenceframe.Input{Value: p2[0]}, referenceframe.Input{Value: p2[1]}

		ci := &ConstraintInput{
			StartPos: pose1,
			EndPos: pose2,
			StartInput: input1,
			EndInput: input2,
			Frame: mp.frame,
		}

		if ok, _ := opt.CheckConstraintPath(ci, mp.Resolution()); !ok {
			pathOk = false
			break
		}

		p1 = p2
	}

	return pathOk
}

// Used for coordinating parallel computations of dubins options
type dubinOptionManager struct {
	optKeys    	chan *configuration
	options 	chan *nodeToOption
	optLock    	sync.RWMutex
	sample   	*configuration
	ready     	bool
	nCPU      	int
	d	 	  	Dubins
}

type nodeToOption struct {
	key 		*configuration
	value 		DubinOption
}
  
type nodeToOptionList []nodeToOption

func (p nodeToOptionList) Len() int { return len(p) }
func (p nodeToOptionList) Less(i, j int) bool { return p[i].value.totalLen < p[j].value.totalLen }
func (p nodeToOptionList) Swap(i, j int){ p[i], p[j] = p[j], p[i] }

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
		allOpts := dm.d.AllOptions(start, end, false)

		for _, opt := range allOpts {
			pl = append(pl, nodeToOption{node, opt})
		}
	}
	sort.Sort(pl)

	// Sort and choose best nbOptions options
	var options map[*configuration]DubinOption
	for _, p := range pl[:nbOptions] {
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
	var options map[*configuration]DubinOption
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
		case k := <-dm.optKeys:
			if k != nil {
				dm.optLock.RLock()
				start := configuration2slice(k)
				end := configuration2slice(dm.sample)
				allOpts := dm.d.AllOptions(start, end, false)
				dm.optLock.RUnlock()

				for _, opt := range allOpts {
					pl = append(pl, nodeToOption{k, opt})
				}
			}
		default:
			dm.optLock.RLock()
			if dm.ready {
				dm.optLock.RUnlock()
				opt := pl[0]
				pl = pl[1:]
				dm.options <- &opt
				return
			}
			dm.optLock.RUnlock()
		}
	}
}


func mobile2DInputDist(from, to []referenceframe.Input) (float64) {
	return math.Pow(from[0].Value - to[0].Value, 2)
}

func mobile2DConfigDist(from, to *configuration) (float64) {
	return math.Pow(from.inputs[0].Value - to.inputs[0].Value, 2)
}

func findNearNeighbors(sample *configuration, rrtMap map[*configuration]*configuration, nbNeighbors int) []*configuration {
	keys := make([]*configuration, 0, len(rrtMap))
  
    for key := range rrtMap {
        keys = append(keys, key)
    }

	sort.SliceStable(keys, func(i, j int) bool{
        return mobile2DConfigDist(rrtMap[keys[i]], sample) < mobile2DConfigDist(rrtMap[keys[j]], sample)
    })

	topn := nbNeighbors
	if len(rrtMap) < nbNeighbors {
		topn = len(rrtMap)
	}

	return keys[:topn]
}

func configuration2slice(c *configuration) ([]float64) {
	s := make([]float64, 0)
	for _, v := range c.inputs {
		s = append(s, v.Value)
	}
	return s
}