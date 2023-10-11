package builtin

import (
	"context"
	"math"
	"math/rand"

	"github.com/golang/geo/r3"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const defaultDistanceMM = 100 * 1000

//nolint:unparam
func (svc *builtIn) startExploreMode(ctx context.Context) {
	svc.logger.Debug("startExploreMode called")

	for {
		randAngle := (rand.Float64() - 0.5) * math.Pi

		newPose := frame.NewPoseInFrame(svc.base.Name().Name, spatialmath.NewPose(
			r3.Vector{
				X: defaultDistanceMM * math.Sin(randAngle),
				Y: defaultDistanceMM * math.Sin(randAngle),
				Z: 0,
			}, spatialmath.NewOrientationVector()))

		_, err := svc.motionService.Move(ctx, svc.base.Name(), newPose, nil, nil, nil) // worldState, constraints, extra)
		if err != nil {
			svc.logger.Debug("error occurred when moving")
		}

	}
}
