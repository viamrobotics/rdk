// Package motionplan is an API for motioan planning without an implementation
package motionplan

import (
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// Path is a slice of FrameSystemPoses describing a series of Poses for a robot to travel to in the course of following a Plan.
// The pose of the referenceframe.FrameSystemPoses is the pose at the end of the corresponding set of inputs in the Trajectory.
type Path []referenceframe.FrameSystemPoses

// Trajectory is a slice of maps describing a series of Inputs for a robot to travel to in the course of following a Plan.
// Each item in this slice maps a Frame's name (found by calling frame.Name()) to the Inputs that Frame should be modified by.
type Trajectory []referenceframe.FrameSystemInputs

// TrajectoryFromLinearInputs converts a series of linear inputs into a Trajectory.
func TrajectoryFromLinearInputs(inps []*referenceframe.LinearInputs) Trajectory {
	ret := make(Trajectory, len(inps))
	for idx, inp := range inps {
		ret[idx] = inp.ToFrameSystemInputs()
	}

	return ret
}

// Plan is an interface that describes plans returned by this package.  There are two key components to a Plan:
// Its Trajectory contains information pertaining to the commands required to actuate the robot to realize the Plan.
// Its Path contains information describing the Pose of the robot as it travels the Plan.
type Plan interface {
	Path() Path
	Trajectory() Trajectory
}

// SimplePlan is a simple implementation of Plan.
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

// NewSimplePlanFromTrajectory instantiates a new Plan from a Trajectory, making the poses.
func NewSimplePlanFromTrajectory(
	trajAsInputs []*referenceframe.LinearInputs, fs *referenceframe.FrameSystem,
) (*SimplePlan, error) {
	path := Path{}
	for _, inputNode := range trajAsInputs {
		poseMap := make(map[string]*referenceframe.PoseInFrame)
		for frame := range inputNode.Keys() {
			tf, err := fs.Transform(inputNode, referenceframe.NewPoseInFrame(frame, spatialmath.NewZeroPose()), referenceframe.World)
			if err != nil {
				return nil, err
			}
			pose, ok := tf.(*referenceframe.PoseInFrame)
			if !ok {
				return nil, fmt.Errorf("pose not transformable")
			}
			poseMap[frame] = pose
		}
		path = append(path, poseMap)
	}

	return &SimplePlan{path: path, traj: TrajectoryFromLinearInputs(trajAsInputs)}, nil
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

// ExecutionState describes a plan and a particular state along it.

// Path returns the Path associated with the Plan.
func (plan *SimplePlan) Path() Path {
	return plan.path
}

// Trajectory returns the Trajectory associated with the Plan.
func (plan *SimplePlan) Trajectory() Trajectory {
	return plan.traj
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
func (traj Trajectory) EvaluateCost(distFunc SegmentFSMetric) float64 {
	var totalCost float64
	last := referenceframe.NewLinearInputs()
	for i, stepFSI := range traj {
		step := stepFSI.ToLinearInputs()
		if i != 0 {
			cost := distFunc(&SegmentFS{
				StartConfiguration: last,
				EndConfiguration:   step,
			})
			totalCost += cost
		}

		for k, v := range step.Items() {
			last.Put(k, v)
		}
	}
	return totalCost
}
