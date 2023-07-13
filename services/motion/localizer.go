package motion

import (
	"context"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
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
	origin *geo.Point
}

func NewMovementSensorLocalizer(ms movementsensor.MovementSensor, origin *geo.Point) Localizer {
	return &movementSensorLocalizer{MovementSensor: ms, origin: origin}
}

// CurrentPosition returns a movementsensor's current position.
func (m *movementSensorLocalizer) CurrentPosition(ctx context.Context) (*referenceframe.PoseInFrame, error) {
	gp, _, err := m.Position(ctx, nil)
	if err != nil {
		return nil, err
	}
	heading, err := m.Orientation(ctx, nil)
	if err != nil {
		return nil, err
	}
	pose := spatialmath.NewPose(spatialmath.GeoPointToPose(gp, m.origin).Point(), heading)
	offset := spatialmath.NewPoseFromOrientation(&spatialmath.EulerAngles{Yaw: utils.DegToRad(180 + 90)}) // +90 because want to align with east
	return referenceframe.NewPoseInFrame(m.Name().Name, spatialmath.Compose(pose, offset)), nil
}
