package builtin

import (
	"context"
	"math/rand"

	"github.com/golang/geo/r3"
	"go.viam.com/utils"

	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const defaultDistanceMM = 100 * 1000

func (svc *builtIn) startExploreMode(ctx context.Context) {
	svc.logger.Debug("startExploreMode called")

	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()

		// Send motionCfg parameters through extra until motionCfg can be added to Move()
		extra := map[string]interface{}{"motionCfg": svc.motionCfg}

		for {
			if ctx.Err() != nil {
				return
			}

			//nolint:gosec
			destination := frame.NewPoseInFrame(svc.base.Name().Name, spatialmath.NewPose(
				r3.Vector{
					X: defaultDistanceMM * (2*rand.Float64() - 1.0),
					Y: defaultDistanceMM * (2*rand.Float64() - 1.0),
					Z: 0.,
				}, spatialmath.NewOrientationVector()))

			_, err := svc.exploreMotionService.Move(ctx, svc.base.Name(), destination, nil, nil, extra)
			if err != nil {
				svc.logger.Debugf("error occurred when moving to point %v: %v", destination, err)
			}
		}
	})
}
