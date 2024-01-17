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
	Trajectory
	Path

	// nodes corresponding to inputs can be cached with the Plan for easy conversion back into a form usable by RRT
	// depending on how the Trajectory is constructed these may be nil and should be computed before usage
	nodes []node
}

func newPlan(solution []node, sf *solverFrame, relative bool) (*Plan, error) {
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
		Trajectory: traj,
		Path:       path,
		nodes:      solution,
	}, nil
}

// TODO: can probably think of a better name for this function
func (plan *Plan) Remaining(waypointIndex int) *Plan {
	// TODO: I don't think a deep copy should be necessary here but maybe?
	return &Plan{
		Path:       plan.Path[waypointIndex:],
		Trajectory: plan.Trajectory[waypointIndex:],
		nodes:      plan.nodes[waypointIndex:],
	}
}

func (plan *Plan) Offset(offset spatialmath.Pose) *Plan {
	newPath := make([]PathStep, 0, len(plan.Path))
	for _, step := range plan.Path {
		newStep := make(PathStep, len(step))
		for frame, pose := range step {
			newStep[frame] = referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.Compose(offset, pose.Pose()))
		}
		newPath = append(newPath, newStep)
	}
	return &Plan{
		Path:       newPath,
		Trajectory: plan.Trajectory,
		nodes:      plan.nodes,
	}
}

// TODO: could make Plan an interface and type this specifically as a geoPlan but this might be overkill
func (plan *Plan) ToGeoPlan(geoOrigin *spatialmath.GeoPose) (*Plan, error) {
	newPath := make([]PathStep, 0, len(plan.Path))
	for _, step := range plan.Path {
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
		Trajectory: plan.Trajectory,
		Path:       newPath,
		nodes:      plan.nodes,
	}, nil
}

type Trajectory []map[string][]referenceframe.Input

// GetFrameInputs is a helper function which will extract the waypoints of a single frame from the map output of a Trajectory.
func (traj Trajectory) GetFrameInputs(frameName string) ([][]referenceframe.Input, error) {
	solution := make([][]referenceframe.Input, 0, len(traj))
	for _, step := range traj {
		frameStep, ok := step[frameName]
		if !ok {
			return nil, fmt.Errorf("frame named %s not found in Trajectory", frameName)
		}
		solution = append(solution, frameStep)
	}
	return solution, nil
}

// String returns a human-readable version of the Trajectory, suitable for debugging.
func (traj Trajectory) String() string {
	var str string
	for _, step := range traj {
		str += "\n"
		for component, input := range step {
			if len(input) > 0 {
				str += fmt.Sprintf("%s: %v\t", component, input)
			}
		}
	}
	return str
}

// Evaluate assigns a numeric score to a plan that corresponds to the cumulative distance between input waypoints in the Trajectory.
func (traj Trajectory) Evaluate(distFunc ik.SegmentMetric) (totalCost float64) {
	if len(traj) < 2 {
		return math.Inf(1)
	}
	last := map[string][]referenceframe.Input{}
	for _, step := range traj {
		for component, inputs := range step {
			if len(inputs) > 0 {
				if lastInputs, ok := last[component]; ok {
					cost := distFunc(&ik.Segment{StartConfiguration: lastInputs, EndConfiguration: inputs})
					totalCost += cost
				}
				last[component] = inputs
			}
		}
	}
	return totalCost
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

	step := make(PathStep)
	for k, v := range ps.Step {
		step[k] = referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPoseFromProtobuf(v.Pose))
	}
	return step, nil
}

type Path []PathStep

func newRelativePath(solution []node, sf *solverFrame) (Path, error) {
	path := Path{}
	for _, step := range solution {
		stepMap := sf.sliceToMap(step.Q())
		step := make(map[string]*referenceframe.PoseInFrame)
		for frame, _ := range stepMap {
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
		return nil, errors.New("need to have at least 2 elements in Path")
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
			return nil, fmt.Errorf("frame named %s not found in Path", frameName)
		}
		poses = append(poses, pose.Pose())
	}
	return poses, nil
}
