package motionplan

import (
	"errors"
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	pb "go.viam.com/api/service/motion/v1"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

type Plan interface {
	Trajectory() Trajectory
	Path() Path
}

// RemainingPlan returns a new Plan equal to the given plan from the waypointIndex onwards.
func RemainingPlan(p Plan, waypointIndex int) (Plan, error) {
	plan, ok := p.(*rrtPlan)
	if !ok {
		return nil, errBadPlanImpl
	}
	if waypointIndex < 0 || waypointIndex > len(plan.traj) {
		return nil, fmt.Errorf("could not access trajectory index %d, must be between 0 and %d", waypointIndex, len(plan.traj))
	}
	return &rrtPlan{
		path:  plan.Path()[waypointIndex:],
		traj:  plan.Trajectory()[waypointIndex:],
		nodes: plan.nodes[waypointIndex:],
	}, nil
}

// TODO: this should probably be a method on Path
func OffsetPlan(p Plan, offset spatialmath.Pose) (Plan, error) {
	plan, ok := p.(*rrtPlan)
	if !ok {
		return nil, errBadPlanImpl
	}
	newPath := make([]PathStep, 0, len(plan.Path()))
	for _, step := range plan.Path() {
		newStep := make(PathStep, len(step))
		for frame, pose := range step {
			newStep[frame] = referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.Compose(offset, pose.Pose()))
		}
		newPath = append(newPath, newStep)
	}
	return &rrtPlan{
		path:  newPath,
		traj:  plan.traj,
		nodes: plan.nodes,
	}, nil
}

type geoPlan struct {
	rrtPlan
}

func NewGeoPlan(p Plan, geoOrigin *spatialmath.GeoPose) (*geoPlan, error) {
	plan, ok := p.(*rrtPlan)
	if !ok {
		return nil, errBadPlanImpl
	}
	newPath := make([]PathStep, 0, len(plan.Path()))
	for _, step := range plan.Path() {
		newStep := make(PathStep)
		for frame, pif := range step {
			pose := pif.Pose()
			geoPose := spatialmath.PoseToGeoPose(geoOrigin, pose)
			heading := math.Mod(math.Abs(geoPose.Heading()-360), 360)
			o := &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: heading}
			smuggledGeoPose := spatialmath.NewPose(r3.Vector{X: geoPose.Location().Lng(), Y: geoPose.Location().Lat()}, o)
			newStep[frame] = referenceframe.NewPoseInFrame(pif.Parent(), smuggledGeoPose)
		}
		newPath = append(newPath, newStep)
	}
	return &geoPlan{rrtPlan{
		traj:  plan.traj,
		path:  newPath,
		nodes: plan.nodes,
	}}, nil
}

type Trajectory []map[string][]referenceframe.Input

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
func (traj Trajectory) EvaluateCost(distFunc ik.SegmentMetric) (totalCost float64) {
	last := map[string][]referenceframe.Input{}
	for _, step := range traj {
		for frame, inputs := range step {
			if len(inputs) > 0 {
				if lastInputs, ok := last[frame]; ok {
					cost := distFunc(&ik.Segment{StartConfiguration: lastInputs, EndConfiguration: inputs})
					totalCost += cost
				}
				last[frame] = inputs
			}
		}
	}
	return totalCost
}

type Path []PathStep

func newRelativePath(solution []node, sf *solverFrame) (Path, error) {
	path := make(Path, 0, len(solution))
	for _, step := range solution {
		stepMap := sf.sliceToMap(step.Q())
		step := make(map[string]*referenceframe.PoseInFrame)
		for frame := range stepMap {
			tf, err := sf.fss.Transform(stepMap, referenceframe.NewPoseInFrame(frame, spatialmath.NewZeroPose()), referenceframe.World)
			if err != nil {
				return nil, err
			}
			pose, ok := tf.(*referenceframe.PoseInFrame)
			if !ok {
				return nil, errors.New("pose not transformable")
			}
			step[frame] = pose
		}
		path = append(path, step)
	}
	return path, nil
}

func newAbsolutePathFromRelative(path Path) (Path, error) {
	if len(path) < 2 {
		return nil, errors.New("need to have at least 2 elements in path")
	}
	newPath := make([]PathStep, 0, len(path))
	newPath = append(newPath, path[0])
	for i, step := range path[1:] {
		newStep := make(PathStep, len(step))
		for frame, pose := range step {
			newStep[frame] = referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.Compose(newPath[i][frame].Pose(), pose.Pose()))
		}
		newPath = append(newPath, newStep)
	}
	return newPath, nil
}

func (path Path) GetFramePoses(frameName string) ([]spatialmath.Pose, error) {
	poses := []spatialmath.Pose{}
	for _, step := range path {
		pose, ok := step[frameName]
		if !ok {
			return nil, fmt.Errorf("frame named %s not found in path", frameName)
		}
		poses = append(poses, pose.Pose())
	}
	return poses, nil
}

// TODO: If the frame system ever uses resource names instead of strings this should be adjusted too
type PathStep map[string]*referenceframe.PoseInFrame

func (ps PathStep) ToProto() *pb.PlanStep {
	step := make(map[string]*pb.ComponentState)
	for name, pose := range ps {
		pbPose := spatialmath.PoseToProtobuf(pose.Pose())
		step[name] = &pb.ComponentState{Pose: pbPose}
	}
	return &pb.PlanStep{Step: step}
}

// pathStepFromProto converts a *pb.PlanStep to a PlanStep.
func PathStepFromProto(ps *pb.PlanStep) (PathStep, error) {
	if ps == nil {
		return PathStep{}, errors.New("received nil *pb.PlanStep")
	}

	step := make(PathStep, len(ps.Step))
	for k, v := range ps.Step {
		step[k] = referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPoseFromProtobuf(v.Pose))
	}
	return step, nil
}
