package motion

import (
	"context"
	"math"

	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

// SLAMOrientationAdjustment is needed because a SLAM map pose has orientation of OZ=1, Theta=0 when the rover is intended to be pointing
// at the +X axis of the SLAM map.
// However, for a rover's relative planning frame, driving forwards increments +Y. Thus we must adjust where the rover thinks it is.
var SLAMOrientationAdjustment = spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: -90})

// Localizer is an interface which both slam and movementsensor can satisfy when wrapped respectively.
type Localizer interface {
	CurrentPosition(context.Context) (*referenceframe.PoseInFrame, error)
}

// slamLocalizer is a struct which only wraps an existing slam service.
type slamLocalizer struct {
	slam.Service
	relative bool
}

// NewSLAMLocalizer creates a new Localizer that relies on a slam service to report Pose.
func NewSLAMLocalizer(slam slam.Service, relative bool) Localizer {
	return &slamLocalizer{Service: slam, relative: relative}
}

// CurrentPosition returns slam's current position.
func (s *slamLocalizer) CurrentPosition(ctx context.Context) (*referenceframe.PoseInFrame, error) {
	pose, _, err := s.Position(ctx)
	if err != nil {
		return nil, err
	}
	if s.relative {
		pose = spatialmath.Compose(pose, SLAMOrientationAdjustment)
	}

	// Slam poses are returned such that theta=0 points along the +X axis
	// We must rotate 90 degrees to match the base convention of y = forwards
	return referenceframe.NewPoseInFrame(referenceframe.World, pose), err
}

// movementSensorLocalizer is a struct which only wraps an existing movementsensor.
type movementSensorLocalizer struct {
	movementsensor.MovementSensor
	origin      *geo.Point
	calibration spatialmath.Pose
}

// NewMovementSensorLocalizer creates a Localizer from a MovementSensor.
// An origin point must be specified and the localizer will return Poses relative to this point.
// A calibration pose can also be specified, which will adjust the location after it is calculated relative to the origin.
func NewMovementSensorLocalizer(ms movementsensor.MovementSensor, origin *geo.Point, calibration spatialmath.Pose) Localizer {
	return &movementSensorLocalizer{MovementSensor: ms, origin: origin, calibration: calibration}
}

// CurrentPosition returns a movementsensor's current position.
func (m *movementSensorLocalizer) CurrentPosition(ctx context.Context) (*referenceframe.PoseInFrame, error) {
	gp, _, err := m.Position(ctx, nil)
	if err != nil {
		return nil, err
	}
	var o spatialmath.Orientation
	properties, err := m.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}
	switch {
	case properties.CompassHeadingSupported:
		headingLeft, err := m.CompassHeading(ctx, nil)
		if err != nil {
			return nil, err
		}
		// CompassHeading is a left-handed value. Convert to be right-handed. Use math.Mod to ensure that 0 reports 0 rather than 360.
		heading := math.Mod(math.Abs(headingLeft-360), 360)
		o = &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: heading}
	case properties.OrientationSupported:
		o, err = m.Orientation(ctx, nil)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("could not get orientation from Localizer")
	}

	pose := spatialmath.NewPose(spatialmath.GeoPointToPose(gp, m.origin).Point(), o)
	return referenceframe.NewPoseInFrame(m.Name().Name, spatialmath.Compose(pose, m.calibration)), nil
}
