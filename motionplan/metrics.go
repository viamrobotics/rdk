package motionplan

import (
	"math"

	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// GoalCloud can express leeway in individual dimensions with respect to a goal pose. Combined,
// these leeways describe a cloud where arriving at any destination within that cloud are considered
// equivalent.
//
// All of the leeways affect the algorithm independently. None of the leeways will be scaled based
// on how close a candidate pose is to the goal pose. Consider a case where a gripper wants to pick
// up a cup. There may be some freedom with respect to the exact orientation and theta of the
// gripper. But be cautious that there can be a scenario where a legal, but severe orientation
// leeway might work with a smaller difference in the candidate's theta (the cup stays mostly facing
// up), but not necessarily a larger one (the cup starts to tip and spill).
type GoalCloud struct {
	// ReferenceFrame represents which reference frame the following leeways will be applied
	// to. Consider a case where an arm with a hand is pushing a block up a hill. It would be
	// important for the arm to follow the slope of the incline (the hand stays close to the
	// incline). But it might be okay if the hand, at any given waypoint, moves a little bit more up
	// or down the hill:
	//
	//     +------+
	//     |     /|
	//     |    / |
	//     | H◆/  |
	//     | /    |
	//     |/     |
	// X > +------+
	//     ^
	//     Z
	//
	// If we solve IK for a pose that's a bit "to the left" or down the hill, it's important that we
	// also lower the hand closer to the incline. If we try to calculate this in the world reference
	// frame, this means that the leeway for Z is in terms of a specific IK solution's leeway for X.
	//
	// To solve this, we allow the user to specify a reference frame that the leeways will be
	// applied to. If the box's orientation vector is perpendicular to incline, the user can specify
	// that, in the box's reference frame, the leeway of Z' is 0. While the leeway for X' (in the
	// box reference frame) can be a wider range.
	ReferenceFrame string

	// The following X, Y and Z are translational leeways. They are all in units of millimeters, the
	// same as a goal pose. The value represents a leeway in the range of [-Value, +Value].

	// The X leeway where any X (with respect to the reference frame's orientation).
	X float64 `json:"x"`
	// The Y leeway where any X (with respect to the reference frame's orientation).
	Y float64 `json:"y"`
	// The Z leeway where any X (with respect to the reference frame's orientation).
	Z float64 `json:"z"`

	// The following orientation leeway values represents a leeway in the range of [-Value,
	// +Value]. The orientation values are unitless, but one must keep in mind they are applied to
	// an orientation vector that has been normalized to a unit sphere. For example, an OX leeway of
	// `1` would accept any OX for a candidate pose.

	// OX represents the leeway as described above.
	OX float64 `json:"ox"`
	// OY represents the leeway as described above.
	OY float64 `json:"oy"`
	// OZ represents the leeway as described above.
	OZ float64 `json:"oz"`

	// Theta represents the [-Theta, +Theta] in an objects rotation around its
	// orientation axis in the unit of degrees.
	Theta float64 `json:"theta"`
}

const orientationDistanceScaling = 10.

// SegmentFSMetricType is a string enum indicating which algorithm to use for distance in
// configuration space.
type SegmentFSMetricType string

const (
	// FSConfigurationDistanceMetric indicates calculating distance by summing the absolute differences of the inputs.
	FSConfigurationDistanceMetric SegmentFSMetricType = "fs_config"
	// FSConfigurationL2DistanceMetric indicates calculating distance by summing the L2 norm differences of the inputs.
	FSConfigurationL2DistanceMetric SegmentFSMetricType = "fs_config_l2"
)

// GoalMetricType is a string enum indicating the type of goal metric to use.
type GoalMetricType string

const (
	// PositionOnly indicates the use of point-wise distance.
	PositionOnly GoalMetricType = "position_only"
	// SquaredNorm indicates the use of the norm between two poses.
	SquaredNorm GoalMetricType = "squared_norm"
)

// SegmentFS is a referenceframe.FrameSystem-specific contains all the information a constraint needs to determine validity for a movement.
// It contains the starting inputs, the ending inputs, and the framesystem it refers to.
type SegmentFS struct {
	StartConfiguration *referenceframe.LinearInputs
	EndConfiguration   *referenceframe.LinearInputs
	FS                 *referenceframe.FrameSystem
}

// StateFS contains all the information a constraint needs to determine validity for a particular state or configuration of an entire
// framesystem. It contains inputs, the corresponding poses, and the frame it refers to.
// Pose field may be empty, and may be filled in by a constraint that needs it.
type StateFS struct {
	Configuration *referenceframe.LinearInputs
	FS            *referenceframe.FrameSystem

	geometries map[string]*referenceframe.GeometriesInFrame
	poses      referenceframe.FrameSystemPoses
}

// Geometries get Geometries and cache
func (s *StateFS) Geometries() (map[string]*referenceframe.GeometriesInFrame, error) {
	if s.geometries == nil {
		g, err := referenceframe.FrameSystemGeometriesLinearInputs(s.FS, s.Configuration)
		if err != nil {
			return nil, err
		}
		s.geometries = g
	}
	return s.geometries, nil
}

// Poses get poses and cache
func (s *StateFS) Poses() (referenceframe.FrameSystemPoses, error) {
	if s.poses == nil {
		p, err := s.Configuration.ComputePoses(s.FS)
		if err != nil {
			return nil, err
		}
		s.poses = p
	}
	return s.poses, nil
}

// StateFSMetric are functions which, given a StateFS, produces some score. Lower is better.
// This is used for gradient descent to converge upon a goal pose, for example.
type StateFSMetric func(*StateFS) float64

// SegmentFSMetric are functions which produce some score given an SegmentFS. Lower is better.
// This is used to sort produced IK solutions by goodness, for example.
type SegmentFSMetric func(*SegmentFS) float64

// OrientDist returns the arclength between two orientations in degrees.
func OrientDist(o1, o2 spatial.Orientation) float64 {
	return math.Abs(utils.RadToDeg(spatial.QuatToR4AA(spatial.OrientationBetween(o1, o2).Quaternion()).Theta))
}

// WeightedSquaredNormDistance is a distance function between two poses to be used for gradient descent.
func WeightedSquaredNormDistance(start, end spatial.Pose) float64 {
	return WeightedSquaredNormDistanceWithOptions(start, end, .1, orientationDistanceScaling)
}

// WeightedSquaredNormDistanceWithOptions is a distance function between two poses to be used for gradient descent.
func WeightedSquaredNormDistanceWithOptions(start, end spatial.Pose, cartesianScale, orientScale float64) float64 {
	// Increase weight for orientation since it's a small number
	orientDelta := 0.0
	if orientScale > 0 {
		orientDelta = spatial.QuatToR3AA(spatial.OrientationBetween(
			start.Orientation(),
			end.Orientation(),
		).Quaternion()).Mul(orientScale).Norm2()
	}

	ptDelta := 0.0
	if cartesianScale > 0 {
		ptDelta = end.Point().Mul(cartesianScale).Sub(start.Point().Mul(cartesianScale)).Norm2()
	}

	return ptDelta + orientDelta
}

// TODO(RSDK-2557): Writing a PenetrationDepthMetric will allow cbirrt to path along the sides of obstacles rather than terminating
// the RRT tree when an obstacle is hit

// FSConfigurationDistance is a fs metric which will sum the abs differences in each input from start to end.
func FSConfigurationDistance(segment *SegmentFS) float64 {
	score := 0.
	for frame, cfg := range segment.StartConfiguration.Items() {
		if endCfg := segment.EndConfiguration.Get(frame); endCfg != nil && len(cfg) == len(endCfg) {
			for i, val := range cfg {
				score += math.Abs(val - endCfg[i])
			}
		}
	}
	return score
}

// FSConfigurationL2Distance is a fs metric which will sum the L2 norm differences in each input from start to end.
func FSConfigurationL2Distance(segment *SegmentFS) float64 {
	score := 0.
	for frame, cfg := range segment.StartConfiguration.Items() {
		if endCfg := segment.EndConfiguration.Get(frame); endCfg != nil && len(cfg) == len(endCfg) {
			score += referenceframe.InputsL2Distance(cfg, endCfg)
		}
	}
	return score
}

// GetConfigurationDistanceFunc returns a function that measures the degree of "closeness"
// between the two states of a segment according to an algorithm determined by `distType`.
func GetConfigurationDistanceFunc(distType SegmentFSMetricType) SegmentFSMetric {
	switch distType {
	case FSConfigurationDistanceMetric:
		return FSConfigurationDistance
	case FSConfigurationL2DistanceMetric:
		return FSConfigurationL2Distance
	default:
		return FSConfigurationL2Distance
	}
}

// NewSquaredNormMetric creates a metric function that calculates squared norm distance to a goal pose
func NewSquaredNormMetric(goalPose spatial.Pose) func(spatial.Pose) float64 {
	return func(currentPose spatial.Pose) float64 {
		return WeightedSquaredNormDistance(currentPose, goalPose)
	}
}

// NewScaledSquaredNormMetric creates a metric function with scaled orientation weight
func NewScaledSquaredNormMetric(goalPose spatial.Pose, orientationScale float64) func(spatial.Pose) float64 {
	return func(currentPose spatial.Pose) float64 {
		return WeightedSquaredNormDistanceWithOptions(currentPose, goalPose, 0.1, orientationScale)
	}
}
