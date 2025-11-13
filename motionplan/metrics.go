package motionplan

import (
	"fmt"
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

// ScoringMetric is a string enum indicating a choice of plan scoring algorithm.
type ScoringMetric string

const (
	// FSConfigScoringMetric indicates the use of FS configuration distance for scoring.
	FSConfigScoringMetric ScoringMetric = "fs_config"
	// FSConfigL2ScoringMetric indicates the use of the L2 norm in FS configuration space for scoring.
	FSConfigL2ScoringMetric ScoringMetric = "fs_config_l2"
	// PTGDistance indicates the use of distance in TP-space for scoring.
	PTGDistance ScoringMetric = "ptg_distance"
)

// GoalMetricType is a string enum indicating the type of goal metric to use.
type GoalMetricType string

const (
	// PositionOnly indicates the use of point-wise distance.
	PositionOnly GoalMetricType = "position_only"
	// SquaredNorm indicates the use of the norm between two poses.
	SquaredNorm GoalMetricType = "squared_norm"
)

// Segment is a referenceframe.Frame-specific contains all the information a constraint needs to determine validity for a movement.
// It contains the starting inputs, the ending inputs, corresponding poses, and the frame it refers to.
// Pose fields may be empty, and may be filled in by a constraint that needs them.
type Segment struct {
	StartPosition      spatial.Pose
	EndPosition        spatial.Pose
	StartConfiguration []referenceframe.Input
	EndConfiguration   []referenceframe.Input
	Frame              referenceframe.Frame
}

// SegmentFS is a referenceframe.FrameSystem-specific contains all the information a constraint needs to determine validity for a movement.
// It contains the starting inputs, the ending inputs, and the framesystem it refers to.
type SegmentFS struct {
	StartConfiguration *referenceframe.LinearInputs
	EndConfiguration   *referenceframe.LinearInputs
	FS                 *referenceframe.FrameSystem
}

func (s *Segment) String() string {
	startPosString := "nil"
	endPosString := "nil"
	if s.StartPosition != nil {
		startPosString = fmt.Sprint(s.StartPosition)
	}
	if s.EndPosition != nil {
		endPosString = fmt.Sprint(s.EndPosition)
	}
	return fmt.Sprintf(
		"Segment: \n\t StartPosition: %s,\n\t EndPosition: %s,\n\t StartConfiguration:%v,\n\t EndConfiguration:%v,\n\t Frame: %v",
		startPosString,
		endPosString,
		s.StartConfiguration,
		s.EndConfiguration,
		s.Frame,
	)
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

// SegmentMetric are functions which produce some score given an Segment. Lower is better.
// This is used to sort produced IK solutions by goodness, for example.
type SegmentMetric func(*Segment) float64

// SegmentFSMetric are functions which produce some score given an SegmentFS. Lower is better.
// This is used to sort produced IK solutions by goodness, for example.
type SegmentFSMetric func(*SegmentFS) float64

// NewZeroFSMetric always returns zero as the distance.
func NewZeroFSMetric() StateFSMetric {
	return func(from *StateFS) float64 { return 0 }
}

type combinableStateFSMetric struct {
	metrics []StateFSMetric
}

func (m *combinableStateFSMetric) combinedDist(input *StateFS) float64 {
	dist := 0.
	for _, metric := range m.metrics {
		dist += metric(input)
	}
	return dist
}

func CombineFSMetrics(metrics ...StateFSMetric) StateFSMetric {
	cm := &combinableStateFSMetric{metrics: metrics}
	return cm.combinedDist
}

// OrientDist returns the arclength between two orientations in degrees.
func OrientDist(o1, o2 spatial.Orientation) float64 {
	return math.Abs(utils.RadToDeg(spatial.QuatToR4AA(spatial.OrientationBetween(o1, o2).Quaternion()).Theta))
}

// OrientDistToRegion will return a function which will tell you how far the unit sphere component of an orientation
// vector is from a region defined by a point and an arclength around it. The theta value of OV is disregarded.
// This is useful, for example, in defining the set of acceptable angles of attack for writing on a whiteboard.
func OrientDistToRegion(goal spatial.Orientation, alpha float64) func(spatial.Orientation) float64 {
	ov1 := goal.OrientationVectorRadians()
	return func(o spatial.Orientation) float64 {
		ov2 := o.OrientationVectorRadians()
		acosInput := ov1.OX*ov2.OX + ov1.OY*ov2.OY + ov1.OZ*ov2.OZ

		// Account for floating point issues
		if acosInput > 1.0 {
			acosInput = 1.0
		}
		if acosInput < -1.0 {
			acosInput = -1.0
		}
		dist := math.Acos(acosInput)
		return math.Max(0, dist-alpha)
	}
}

// NewSquaredNormSegmentMetric returns a metric which will return the cartesian distance between the two positions.
// It allows the caller to choose the scaling level of orientation.
func NewSquaredNormSegmentMetric(orientationScaleFactor float64) SegmentMetric {
	return func(segment *Segment) float64 {
		delta := spatial.PoseDelta(segment.StartPosition, segment.EndPosition)
		// Increase weight for orientation since it's a small number
		return delta.Point().Norm2() + spatial.QuatToR3AA(delta.Orientation().Quaternion()).Mul(orientationScaleFactor).Norm2()
	}
}

// SquaredNormNoOrientSegmentMetric is a metric which will return the cartesian distance between the two positions.
func SquaredNormNoOrientSegmentMetric(segment *Segment) float64 {
	delta := spatial.PoseDelta(segment.StartPosition, segment.EndPosition)
	return delta.Point().Norm2()
}

// WeightedSquaredNormSegmentMetric is a distance function between two poses to be used for gradient descent.
// This changes the magnitude of the position delta used to be smaller and avoid numeric instability issues that happens with large floats.
// It also scales the orientation distance to give more weight to it.
func WeightedSquaredNormSegmentMetric(segment *Segment) float64 {
	return WeightedSquaredNormDistance(segment.StartPosition, segment.EndPosition)
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

func WeightedSquaredNormDistanceMany(
	start, end referenceframe.FrameSystemPoses,
	cartesianScale, orientScale float64) (float64, error) {

	score := 0.0
	
	for f, s := range start {
		e := end[f]
		if s.Parent() != e.Parent() {
			return 0, fmt.Errorf("WeightedSquaredNormDistanceFS start and end frame mismatch (%s) != (%s)", s.Parent(), e.Parent())
		}
		score += WeightedSquaredNormDistanceWithOptions(s.Pose(), e.Pose(), cartesianScale, orientScale)
	}
	
	return score, nil
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
