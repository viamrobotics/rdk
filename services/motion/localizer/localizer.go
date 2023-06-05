// Package localizer introduces an interface which both slam and movementsensor can satisfy when wrapped respectively
package localizer

import (
	"context"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

// Localizer is an interface which both slam and movementsensor can satisfy when wrapped respectively.
type Localizer interface {
	GlobalPosition(context.Context) (spatialmath.Pose, error)
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
