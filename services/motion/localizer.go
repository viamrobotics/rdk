package motion

import (
	"context"
	"math"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

// Localizer is an interface which both slam and movementsensor can satisfy when wrapped respectively.
type Localizer interface {
	CurrentPosition(context.Context) (*referenceframe.PoseInFrame, error)
}

// slamLocalizer is a struct which only wraps an existing slam service.
type slamLocalizer struct {
	slam.Service
}

func NewSLAMLocalizer(slam slam.Service) Localizer {
	return &slamLocalizer{Service: slam}
}

// CurrentPosition returns slam's current position.
func (s *slamLocalizer) CurrentPosition(ctx context.Context) (*referenceframe.PoseInFrame, error) {
	pose, _, err := s.GetPosition(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.NewPoseInFrame(referenceframe.World, pose), err
}

// movementSensorLocalizer is a struct which only wraps an existing movementsensor.
type movementSensorLocalizer struct {
	movementsensor.MovementSensor
	origin      *geo.Point
	calibration spatialmath.Pose
}

func NewMovementSensorLocalizer(ms movementsensor.MovementSensor, origin *geo.Point, calibration spatialmath.Pose) Localizer {
	return &movementSensorLocalizer{MovementSensor: ms, origin: origin, calibration: calibration}
}

// CurrentPosition returns a movementsensor's current position.
func (m *movementSensorLocalizer) CurrentPosition(ctx context.Context) (*referenceframe.PoseInFrame, error) {
	gp, _, err := m.Position(ctx, nil)
	if err != nil {
		return nil, err
	}
	heading, err := m.CompassHeading(ctx, nil)
	var o spatialmath.Orientation
	if err == nil {
		o = &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: heading}
	} else {
		o, err = m.Orientation(ctx, nil)
		if err != nil {
			return nil, err
		}
	}
	pose := spatialmath.NewPose(spatialmath.GeoPointToPose(gp, m.origin).Point(), o)
	correction := spatialmath.Compose(m.calibration, spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVector{OZ: 1, Theta: -math.Pi / 2}))
	return referenceframe.NewPoseInFrame(m.Name().Name, spatialmath.Compose(pose, correction)), nil
}
