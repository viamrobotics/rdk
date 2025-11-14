package motionplan

import (
	"math"

	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

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
