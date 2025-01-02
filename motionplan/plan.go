package motionplan

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// Plan is an interface that describes plans returned by this package.  There are two key components to a Plan:
// Its Trajectory contains information pertaining to the commands required to actuate the robot to realize the Plan.
// Its Path contains information describing the Pose of the robot as it travels the Plan.
type Plan interface {
	Trajectory() Trajectory
	Path() Path
}

// RemainingPlan returns a new Plan equal to the given plan from the waypointIndex onwards.
func RemainingPlan(plan Plan, waypointIndex int) (Plan, error) {
	if waypointIndex < 0 {
		return nil, errors.New("could not access plan with negative waypoint index")
	}
	traj := plan.Trajectory()
	if traj != nil && waypointIndex > len(traj) {
		return nil, fmt.Errorf("could not access trajectory index %d, must be less than %d", waypointIndex, len(plan.Trajectory()))
	}
	path := plan.Path()
	if path != nil && waypointIndex > len(path) {
		return nil, fmt.Errorf("could not access path index %d, must be less than %d", waypointIndex, len(plan.Path()))
	}
	simplePlan := NewSimplePlan(path[waypointIndex:], traj[waypointIndex:])
	if rrt, ok := plan.(*rrtPlan); ok {
		return &rrtPlan{SimplePlan: *simplePlan, nodes: rrt.nodes[waypointIndex:]}, nil
	}
	return simplePlan, nil
}

// OffsetPlan returns a new Plan that is equivalent to the given Plan if its Path was offset by the given Pose.
// Does not modify Trajectory.
func OffsetPlan(plan Plan, offset spatialmath.Pose) Plan {
	path := plan.Path()
	if path == nil {
		return NewSimplePlan(nil, plan.Trajectory())
	}
	newPath := make([]referenceframe.FrameSystemPoses, 0, len(path))
	for _, step := range path {
		newStep := make(referenceframe.FrameSystemPoses, len(step))
		for frame, pose := range step {
			newStep[frame] = referenceframe.NewPoseInFrame(pose.Parent(), spatialmath.Compose(offset, pose.Pose()))
		}
		newPath = append(newPath, newStep)
	}
	simplePlan := NewSimplePlan(newPath, plan.Trajectory())
	if rrt, ok := plan.(*rrtPlan); ok {
		return &rrtPlan{SimplePlan: *simplePlan, nodes: rrt.nodes}
	}
	return simplePlan
}

// Trajectory is a slice of maps describing a series of Inputs for a robot to travel to in the course of following a Plan.
// Each item in this slice maps a Frame's name (found by calling frame.Name()) to the Inputs that Frame should be modified by.
type Trajectory []referenceframe.FrameSystemInputs

// GetFrameInputs is a helper function which will extract the waypoints of a single frame from the map output of a trajectory.
func (traj Trajectory) GetFrameInputs(frameName string) ([][]referenceframe.Input, error) {
	solution := make([][]referenceframe.Input, 0, len(traj))
	for _, step := range traj {
		frameStep, ok := step[frameName]
		if !ok {
			return nil, fmt.Errorf("frame named %s not found in trajectory", frameName)
		}
		solution = append(solution, frameStep)
	}
	return solution, nil
}

// String returns a human-readable version of the trajectory, suitable for debugging.
func (traj Trajectory) String() string {
	var str string
	for _, step := range traj {
		str += "\n"
		for frame, input := range step {
			if len(input) > 0 {
				str += fmt.Sprintf("%s: %v\t", frame, input)
			}
		}
	}
	return str
}

// EvaluateCost calculates a cost to a trajectory as measured by the given distFunc Metric.
func (traj Trajectory) EvaluateCost(distFunc ik.SegmentFSMetric) float64 {
	var totalCost float64
	last := referenceframe.FrameSystemInputs{}
	for i, step := range traj {
		if i != 0 {
			cost := distFunc(&ik.SegmentFS{
				StartConfiguration: last,
				EndConfiguration:   step,
			})
			totalCost += cost
		}
		for k, v := range step {
			last[k] = v
		}
	}
	return totalCost
}

// Path is a slice of FrameSystemPoses describing a series of Poses for a robot to travel to in the course of following a Plan.
// The pose of the referenceframe.FrameSystemPoses is the pose at the end of the corresponding set of inputs in the Trajectory.
type Path []referenceframe.FrameSystemPoses

func newPath(solution []node, fs referenceframe.FrameSystem) (Path, error) {
	path := make(Path, 0, len(solution))
	for _, inputNode := range solution {
		poseMap := make(map[string]*referenceframe.PoseInFrame)
		for frame := range inputNode.Q() {
			tf, err := fs.Transform(inputNode.Q(), referenceframe.NewPoseInFrame(frame, spatialmath.NewZeroPose()), referenceframe.World)
			if err != nil {
				return nil, err
			}
			pose, ok := tf.(*referenceframe.PoseInFrame)
			if !ok {
				return nil, errors.New("pose not transformable")
			}
			poseMap[frame] = pose
		}
		path = append(path, poseMap)
	}
	return path, nil
}

func newPathFromRelativePath(path Path) (Path, error) {
	if len(path) < 2 {
		return nil, errors.New("need to have at least 2 elements in path")
	}
	newPath := make([]referenceframe.FrameSystemPoses, 0, len(path))
	newPath = append(newPath, path[0])
	for i, step := range path[1:] {
		newStep := make(referenceframe.FrameSystemPoses, len(step))
		for frame, pose := range step {
			lastPose := newPath[i][frame].Pose()
			newStep[frame] = referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.Compose(lastPose, pose.Pose()))
		}
		newPath = append(newPath, newStep)
	}
	return newPath, nil
}

// GetFramePoses returns a slice of poses a given frame should visit in the course of the Path.
func (path Path) GetFramePoses(frameName string) ([]spatialmath.Pose, error) {
	poses := []spatialmath.Pose{}
	for _, step := range path {
		poseInFrame, ok := step[frameName]
		if !ok {
			return nil, fmt.Errorf("frame named %s not found in path", frameName)
		}
		poses = append(poses, poseInFrame.Pose())
	}
	return poses, nil
}

func (path Path) String() string {
	var str string
	for _, step := range path {
		str += "\n"
		for frame, pose := range step {
			str += fmt.Sprintf("%s: %v %v\t", frame, pose.Pose().Point(), *pose.Pose().Orientation().OrientationVectorDegrees())
		}
	}
	return str
}

// FrameSystemPosesToProto converts a referenceframe.FrameSystemPoses to its representation in protobuf.
func FrameSystemPosesToProto(ps referenceframe.FrameSystemPoses) *pb.PlanStep {
	step := make(map[string]*pb.ComponentState)
	for name, pose := range ps {
		pbPose := spatialmath.PoseToProtobuf(pose.Pose())
		step[name] = &pb.ComponentState{Pose: pbPose}
	}
	return &pb.PlanStep{Step: step}
}

// FrameSystemPosesFromProto converts a *pb.PlanStep to a PlanStep.
func FrameSystemPosesFromProto(ps *pb.PlanStep) (referenceframe.FrameSystemPoses, error) {
	if ps == nil {
		return referenceframe.FrameSystemPoses{}, errors.New("received nil *pb.PlanStep")
	}

	step := make(referenceframe.FrameSystemPoses, len(ps.Step))
	for k, v := range ps.Step {
		step[k] = referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPoseFromProtobuf(v.Pose))
	}
	return step, nil
}

// NewGeoPlan returns a Plan containing a Path with GPS coordinates smuggled into the Pose struct. Each GPS point is created using:
// A Point with X as the longitude and Y as the latitude
// An orientation using the heading as the theta in an OrientationVector with Z=1.
func NewGeoPlan(plan Plan, pt *geo.Point) Plan {
	newPath := make([]referenceframe.FrameSystemPoses, 0, len(plan.Path()))
	for _, step := range plan.Path() {
		newStep := make(referenceframe.FrameSystemPoses)
		for frame, pif := range step {
			pose := pif.Pose()
			geoPose := spatialmath.PoseToGeoPose(spatialmath.NewGeoPose(pt, 0), pose)
			heading := math.Mod(math.Abs(geoPose.Heading()-360), 360)
			o := &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: heading}
			smuggledGeoPose := spatialmath.NewPose(r3.Vector{X: geoPose.Location().Lng(), Y: geoPose.Location().Lat()}, o)
			newStep[frame] = referenceframe.NewPoseInFrame(pif.Parent(), smuggledGeoPose)
		}
		newPath = append(newPath, newStep)
	}
	return NewSimplePlan(newPath, plan.Trajectory())
}

// SimplePlan is a struct containing a Path and a Trajectory, together these comprise a Plan.
type SimplePlan struct {
	path Path
	traj Trajectory
}

// NewSimplePlan instantiates a new Plan from a Path and Trajectory.
func NewSimplePlan(path Path, traj Trajectory) *SimplePlan {
	if path == nil {
		path = Path{}
	}
	if traj == nil {
		traj = Trajectory{}
	}
	return &SimplePlan{path: path, traj: traj}
}

// Path returns the Path associated with the Plan.
func (plan *SimplePlan) Path() Path {
	return plan.path
}

// Trajectory returns the Trajectory associated with the Plan.
func (plan *SimplePlan) Trajectory() Trajectory {
	return plan.traj
}

// ExecutionState describes a plan and a particular state along it.
type ExecutionState struct {
	plan  Plan
	index int

	// The current inputs of input-enabled elements described by the plan
	currentInputs referenceframe.FrameSystemInputs

	// The current PoseInFrames of input-enabled elements described by this plan.
	currentPose referenceframe.FrameSystemPoses
}

// NewExecutionState will construct an ExecutionState struct.
func NewExecutionState(
	plan Plan,
	index int,
	currentInputs referenceframe.FrameSystemInputs,
	currentPose referenceframe.FrameSystemPoses,
) (ExecutionState, error) {
	if plan == nil {
		return ExecutionState{}, errors.New("cannot create new ExecutionState with nil plan")
	}
	if currentInputs == nil {
		return ExecutionState{}, errors.New("cannot create new ExecutionState with nil currentInputs")
	}
	if currentPose == nil {
		return ExecutionState{}, errors.New("cannot create new ExecutionState with nil currentPose")
	}
	return ExecutionState{
		plan:          plan,
		index:         index,
		currentInputs: currentInputs,
		currentPose:   currentPose,
	}, nil
}

// Plan returns the plan associated with the execution state.
func (e *ExecutionState) Plan() Plan {
	return e.plan
}

// Index returns the currently-executing index of the execution state's Plan.
func (e *ExecutionState) Index() int {
	return e.index
}

// CurrentInputs returns the current inputs of the components associated with the ExecutionState.
func (e *ExecutionState) CurrentInputs() referenceframe.FrameSystemInputs {
	return e.currentInputs
}

// CurrentPoses returns the current poses in frame of the components associated with the ExecutionState.
func (e *ExecutionState) CurrentPoses() referenceframe.FrameSystemPoses {
	return e.currentPose
}

// CalculateFrameErrorState takes an ExecutionState and a Frame and calculates the error between the Frame's expected
// and actual positions.
func CalculateFrameErrorState(e ExecutionState, executionFrame, localizationFrame referenceframe.Frame) (spatialmath.Pose, error) {
	currentInputs, ok := e.CurrentInputs()[executionFrame.Name()]
	if !ok {
		return nil, newFrameNotFoundError(executionFrame.Name())
	}
	currentPose, ok := e.CurrentPoses()[localizationFrame.Name()]
	if !ok {
		return nil, newFrameNotFoundError(localizationFrame.Name())
	}
	currPoseInArc, err := executionFrame.Transform(currentInputs)
	if err != nil {
		return nil, err
	}
	path := e.Plan().Path()
	if path == nil {
		return nil, errors.New("cannot calculate error state on a nil Path")
	}
	if len(path) == 0 {
		return spatialmath.NewZeroPose(), nil
	}
	index := e.Index() - 1
	if index < 0 || index >= len(path) {
		return nil, fmt.Errorf("index %d out of bounds for Path of length %d", index, len(path))
	}
	pose, ok := path[index][executionFrame.Name()]
	if !ok {
		return nil, newFrameNotFoundError(executionFrame.Name())
	}
	if pose.Parent() != currentPose.Parent() {
		return nil, errors.New("cannot compose two PoseInFrames with different parents")
	}
	nominalPose := spatialmath.Compose(pose.Pose(), currPoseInArc)
	return spatialmath.PoseBetween(nominalPose, currentPose.Pose()), nil
}

// newFrameNotFoundError returns an error indicating that a given frame was not found in the given ExecutionState.
func newFrameNotFoundError(frameName string) error {
	return fmt.Errorf("could not find frame %s in ExecutionState", frameName)
}

// PlanState is a struct which holds both a referenceframe.FrameSystemPoses and a configuration.
// This is intended to be used as start or goal states for plans. Either field may be nil.
type PlanState struct {
	poses         referenceframe.FrameSystemPoses
	configuration referenceframe.FrameSystemInputs
}

// NewPlanState creates a PlanState from the given poses and configuration. Either or both may be nil.
func NewPlanState(poses referenceframe.FrameSystemPoses, configuration referenceframe.FrameSystemInputs) *PlanState {
	return &PlanState{poses: poses, configuration: configuration}
}

// Poses returns the poses of the PlanState.
func (p *PlanState) Poses() referenceframe.FrameSystemPoses {
	return p.poses
}

// Configuration returns the configuration of the PlanState.
func (p *PlanState) Configuration() referenceframe.FrameSystemInputs {
	return p.configuration
}

// ComputePoses returns the poses of a PlanState if they are populated, or computes them using the given FrameSystem if not.
func (p *PlanState) ComputePoses(fs referenceframe.FrameSystem) (referenceframe.FrameSystemPoses, error) {
	if p.poses != nil {
		return p.poses, nil
	}

	if len(p.configuration) == 0 {
		return nil, errors.New("cannot computes poses, neither poses nor configuration are populated")
	}

	return p.configuration.ComputePoses(fs)
}

// Serialize turns a PlanState into a map[string]interface suitable for being transmitted over proto.
func (p PlanState) Serialize() map[string]interface{} {
	m := map[string]interface{}{}
	poseMap := map[string]interface{}{}
	confMap := map[string]interface{}{}
	for fName, pif := range p.poses {
		pifProto := referenceframe.PoseInFrameToProtobuf(pif)
		poseMap[fName] = pifProto
	}
	for fName, conf := range p.configuration {
		confMap[fName] = referenceframe.InputsToFloats(conf)
	}
	m["poses"] = poseMap
	m["configuration"] = confMap
	return m
}

// DeserializePlanState turns a serialized PlanState back into a PlanState.
func DeserializePlanState(iface map[string]interface{}) (*PlanState, error) {
	ps := &PlanState{
		poses:         referenceframe.FrameSystemPoses{},
		configuration: referenceframe.FrameSystemInputs{},
	}
	if posesIface, ok := iface["poses"]; ok {
		if frameSystemPoseMap, ok := posesIface.(map[string]interface{}); ok {
			for fName, pifIface := range frameSystemPoseMap {
				pifJSON, err := json.Marshal(pifIface)
				if err != nil {
					return nil, err
				}
				pifPb := &commonpb.PoseInFrame{}
				err = json.Unmarshal(pifJSON, pifPb)
				if err != nil {
					return nil, err
				}
				pif := referenceframe.ProtobufToPoseInFrame(pifPb)
				ps.poses[fName] = pif
			}
		} else {
			return nil, errors.New("could not decode contents of poses")
		}
	}
	if confIface, ok := iface["configuration"]; ok {
		if confMap, ok := confIface.(map[string]interface{}); ok {
			for fName, inputsArrIface := range confMap {
				if inputsArr, ok := inputsArrIface.([]interface{}); ok {
					floats := make([]float64, 0, len(inputsArr))
					for _, inputIface := range inputsArr {
						if val, ok := inputIface.(float64); ok {
							floats = append(floats, val)
						} else {
							return nil, errors.New("configuration input array did not contain floats")
						}
					}
					ps.configuration[fName] = referenceframe.FloatsToInputs(floats)
				} else {
					return nil, errors.New("configuration did not contain array of inputs")
				}
			}
		} else {
			return nil, errors.New("could not decode contents of configuration")
		}
	}
	return ps, nil
}
