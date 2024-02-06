package motion

import (
	"context"
	"math"

	"github.com/golang/geo/r3"
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

const poleEpsilon = 0.0001 // If the base is this close to vertical, we cannot plan.

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

	pose := spatialmath.NewPose(spatialmath.GeoPointToPoint(gp, m.origin), o)
	return referenceframe.NewPoseInFrame(m.Name().Name, spatialmath.Compose(pose, m.calibration)), nil
}

// TwoDLocalizer will check the orientation of the pose of a localizer, and ensure that it is normal to the XY plane.
// If it is not, it will be altered such that it is (accounting for e.g. an ourdoor base with one wheel on a rock). If the orientation is
// such that the base is pointed directly up or down (or is upside-down), an error is returned.
// The alteration to ensure normality to the plane is done by transforming the (0,1,0) vector by the provided orientation, and then
// using atan2 on the new x and y values to determine the vector of travel that would be followed.
func TwoDLocalizer(l Localizer) Localizer {
	return &yForwards2dLocalizer{l}
}

type yForwards2dLocalizer struct {
	Localizer
}

func (y *yForwards2dLocalizer) CurrentPosition(ctx context.Context) (*referenceframe.PoseInFrame, error) {
	currPos, err := y.Localizer.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	newPose, err := projectOrientationTo2dRotation(currPos.Pose())
	if err != nil {
		return nil, err
	}
	newPiF := referenceframe.NewPoseInFrame(currPos.Parent(), newPose)
	newPiF.SetName(currPos.Name())
	return newPiF, nil
}

func projectOrientationTo2dRotation(pose spatialmath.Pose) (spatialmath.Pose, error) {
	orient := pose.Orientation().OrientationVectorRadians() // OV is easy to check if we are in plane
	if orient.OZ < 0 {
		return nil, errors.New("base appears to be upside down, check your movement sensor")
	}
	if orient.OX == 0 && orient.OY == 0 {
		return pose, nil
	}

	// This is the vector in which the base would move if it had no changed orientation
	adjPt := spatialmath.NewPoseFromPoint(r3.Vector{0, 1, 0})
	// This is the vector the base would follow based on the reported orientation, were it unencumbered by things like gravity or the ground
	newAdjPt := spatialmath.Compose(spatialmath.NewPoseFromOrientation(orient), adjPt).Point()
	if 1-math.Abs(newAdjPt.Z) < poleEpsilon {
		if newAdjPt.Z > 0 {
			return nil, errors.New("base appears to be pointing straight up, check your movement sensor")
		}
		return nil, errors.New("base appears to be pointing straight down, check your movement sensor")
	}
	// This is the vector across the ground of the above hypothetical vector, projected onto the X-Y plane.
	theta := -math.Atan2(newAdjPt.Y, -newAdjPt.X)
	return spatialmath.NewPose(pose.Point(), &spatialmath.OrientationVector{OZ: 1, Theta: theta}), nil
}
