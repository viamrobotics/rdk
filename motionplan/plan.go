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

type Plan struct {
	trajectory
	path Path

	// nodes corresponding to inputs can be cached with the Plan for easy conversion back into a form usable by RRT
	// depending on how the trajectory is constructed these may be nil and should be computed before usage
	nodes []node
}

func newPlan(solution []node, sf *solverFrame, relative bool) (*Plan, error) {
	if len(solution) < 2 {
		return nil, errors.New("cannot construct a Plan using fewer than two nodes")
	}
	traj := sf.nodesToTrajectory(solution)
	path, err := newRelativePath(solution, sf)
	if err != nil {
		return nil, err
	}
	if relative {
		path, err = newAbsolutePathFromRelative(path)
		if err != nil {
			return nil, err
		}
	}
	return &Plan{
		trajectory: traj,
		path:       path,
		nodes:      solution,
	}, nil
}

// TODO: Is AsPath a good name?
func (plan *Plan) AsPath() Path {
	return plan.path
}

// RemainingPlan returns a new Plan equal to the given plan from the waypointIndex onwards.
func (plan *Plan) RemainingPlan(waypointIndex int) (*Plan, error) {
	// TODO: I don't think a deep copy should be necessary here but maybe?
	if _, err := plan.GetInput(waypointIndex); err != nil {
		return nil, err
	}
	return &Plan{
		path:       plan.path[waypointIndex:],
		trajectory: plan.trajectory[waypointIndex:],
		nodes:      plan.nodes[waypointIndex:],
	}, nil
}

// TODO: this should probably be a method on Path
func (plan *Plan) Offset(offset spatialmath.Pose) *Plan {
	newPath := make([]PathStep, 0, len(plan.path))
	for _, step := range plan.path {
		newStep := make(PathStep, len(step))
		for frame, pose := range step {
			newStep[frame] = referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.Compose(offset, pose.Pose()))
		}
		newPath = append(newPath, newStep)
	}
	return &Plan{
		path:       newPath,
		trajectory: plan.trajectory,
		nodes:      plan.nodes,
	}
}

// TODO: could make Plan an interface and type this specifically as a geoPlan but this might be overkill
func (plan *Plan) ToGeoPlan(geoOrigin *spatialmath.GeoPose) (*Plan, error) {
	newPath := make([]PathStep, 0, len(plan.path))
	for _, step := range plan.path {
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
	return &Plan{
		trajectory: plan.trajectory,
		path:       newPath,
		nodes:      plan.nodes,
	}, nil
}

type trajectory []InputStep

type InputStep map[string][]referenceframe.Input

func (traj trajectory) Length() int {
	return len(traj)
}

func (traj trajectory) GetInput(index int) (InputStep, error) {
	if index < 0 || index > len(traj) {
		return nil, fmt.Errorf("could not access trajectory index %d, must be between 0 and %d", index, len(traj))
	}
	return traj[index], nil
}

// GetFrameInputs is a helper function which will extract the waypoints of a single frame from the map output of a trajectory.
func (traj trajectory) GetFrameInputs(frameName string) ([][]referenceframe.Input, error) {
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
func (traj trajectory) String() string {
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
func (traj trajectory) EvaluateCost(distFunc ik.SegmentMetric) (totalCost float64) {
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
