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
	rdkutils "go.viam.com/rdk/utils"
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
}

// NewSLAMLocalizer creates a new Localizer that relies on a slam service to report Pose.
func NewSLAMLocalizer(slam slam.Service) Localizer {
	return &slamLocalizer{Service: slam}
}

// CurrentPosition returns slam's current position.
func (s *slamLocalizer) CurrentPosition(ctx context.Context) (*referenceframe.PoseInFrame, error) {
	pose, _, err := s.Position(ctx)
	if err != nil {
		return nil, err
	}
	pose = spatialmath.Compose(pose, SLAMOrientationAdjustment)

	// Slam poses are returned such that theta=0 points along the +X axis
	// We must rotate 90 degrees to match the base convention of y = forwards
	pif := referenceframe.NewPoseInFrame(referenceframe.World, pose)
	if err := validatePose(pif.Pose()); err != nil {
		return nil, err
	}
	return pif, nil
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
func NewMovementSensorLocalizer(ms movementsensor.MovementSensor, origin *geo.Point, calibration spatialmath.Pose) (Localizer, error) {
	if err := validateGeopoint(origin); err != nil {
		return nil, err
	}

	if err := validatePose(calibration); err != nil {
		return nil, err
	}

	return &movementSensorLocalizer{MovementSensor: ms, origin: origin, calibration: calibration}, nil
}

// validateGeopoint validates that a geopoint can be used for
// motion planning.
func validateGeopoint(gp *geo.Point) error {
	if math.IsNaN(gp.Lat()) {
		return errors.New("lat can't be NaN")
	}

	if math.IsNaN(gp.Lng()) {
		return errors.New("lng can't be NaN")
	}

	return nil
}

// validatePose validates that a pose can be used for
// motion planning.
func validatePose(p spatialmath.Pose) error {
	if math.IsNaN(p.Point().X) {
		return errors.New("X can't be NaN")
	}

	if math.IsNaN(p.Point().Y) {
		return errors.New("Y can't be NaN")
	}

	if math.IsNaN(p.Point().Z) {
		return errors.New("Z can't be NaN")
	}
	if math.IsNaN(p.Orientation().Quaternion().Imag) {
		return errors.New("Imag can't be NaN")
	}
	if math.IsNaN(p.Orientation().Quaternion().Jmag) {
		return errors.New("Jmag can't be NaN")
	}

	if math.IsNaN(p.Orientation().Quaternion().Kmag) {
		return errors.New("Kmag can't be NaN")
	}

	if math.IsNaN(p.Orientation().Quaternion().Real) {
		return errors.New("Real can't be NaN")
	}

	return nil
}

// CurrentPosition returns a movementsensor's current position.
func (m *movementSensorLocalizer) CurrentPosition(ctx context.Context) (*referenceframe.PoseInFrame, error) {
	gp, _, err := m.Position(ctx, nil)
	if err != nil {
		return nil, err
	}

	if err := validateGeopoint(gp); err != nil {
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

		if math.IsNaN(headingLeft) {
			return nil, errors.New("heading can't be NaN")
		}
		// CompassHeading is a left-handed value. Convert to be right-handed. Use math.Mod to ensure that 0 reports 0 rather than 360.
		theta := rdkutils.SwapCompassHeadingHandedness(headingLeft)
		o = &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: theta}
	case properties.OrientationSupported:
		o, err = m.Orientation(ctx, nil)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("could not get orientation from Localizer")
	}

	pose := spatialmath.NewPose(spatialmath.GeoPointToPose(gp, m.origin).Point(), o)
	pif := referenceframe.NewPoseInFrame(m.Name().Name, spatialmath.Compose(pose, m.calibration))
	if err := validatePose(pif.Pose()); err != nil {
		return nil, err
	}

	return pif, nil
}
