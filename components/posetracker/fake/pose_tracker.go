// Package fake implements a fake pose tracker.
package fake

import (
	"context"

	"go.viam.com/rdk/components/posetracker"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

var model = resource.DefaultModelFamily.WithModel("fake")

func init() {
	resource.RegisterComponent(
		posetracker.API,
		model,
		resource.Registration[posetracker.PoseTracker, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (posetracker.PoseTracker, error) {
				return &PoseTracker{Named: conf.ResourceName().AsNamed()}, nil
			},
		},
	)
}

// PoseTracker is a fake pose tracker that always returns empty poses.
type PoseTracker struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
}

// Poses returns an empty pose map.
func (p *PoseTracker) Poses(ctx context.Context, bodyNames []string, extra map[string]interface{}) (referenceframe.FrameSystemPoses, error) {
	return referenceframe.FrameSystemPoses{}, nil
}
