package motion

import (
	"context"
	"fmt"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

// Localizer is an interface which both slam and movementsensor can satisfy when wrapped respectively.
type Localizer interface {
	GlobalPosition(context.Context) (spatialmath.Pose, error)
}

// NewLocalizer constructs either a SLAMLocalizer or MovementSensorLocalizer from the given resource
func NewLocalizer(ctx context.Context, res resource.Resource) (Localizer, error) {
	switch res := res.(type) {
	case slam.Service:
		return &SLAMLocalizer{Service: res}, nil
	case movementsensor.MovementSensor:
		return &MovementSensorLocalizer{MovementSensor: res}, nil
	default:
		return nil, fmt.Errorf("cannot localize on resource of type %T", res)
	}
}

// SLAMLocalizer is a struct which only wraps an existing slam service.
type SLAMLocalizer struct {
	slam.Service
}

// GlobalPosition returns slam's current position.
func (s SLAMLocalizer) GlobalPosition(ctx context.Context) (spatialmath.Pose, error) {
	pose, _, err := s.GetPosition(ctx)
	return pose, err
}

// MovementSensorLocalizer is a struct which only wraps an existing movementsensor.
type MovementSensorLocalizer struct {
	movementsensor.MovementSensor
}

// GlobalPosition returns a movementsensor's current position.
func (m MovementSensorLocalizer) GlobalPosition(ctx context.Context) (spatialmath.Pose, error) {
	gp, _, err := m.Position(ctx, nil)
	if err != nil {
		return nil, err
	}
	return spatialmath.GeoPointToPose(gp), nil
}
