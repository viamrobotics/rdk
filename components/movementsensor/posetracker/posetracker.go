// Package posetracker implements a wrapper for a posetracker for the movement sensor interface
package posetracker

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterComponent(
		movementsensor.Subtype,
		"posetracker",
		registry.Component{Constructor: func(
			ctx context.Context,
			_ registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newPoseTracker(ctx, config, logger)
		}})
}

func newPoseTracker(
	ctx context.Context,
	config config.Component,
	logger golog.Logger,
) (interface{}, error) {
	return &posetracker{}, nil
}

// BodyToPoseInFrame is used in the pose tracker to find a body and get its pose in the world frame.
type BodyToPoseInFrame map[string]*referenceframe.PoseInFrame

type posetracker struct {
	sensor.Sensor
	generic.Generic
}
