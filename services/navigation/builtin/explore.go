package builtin

import (
	"context"
	"math"
	"math/rand"

	"github.com/golang/geo/r3"
	"go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const defaultDistanceMM = 50 * 1000

func (svc *builtIn) startExploreMode(ctx context.Context) {
	svc.logger.Debug("startExploreMode called")

	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()

		// Send motionCfg parameters through extra until motionCfg can be added to Move()
		extra := map[string]interface{}{"motionCfg": *svc.motionCfg}

		for {
			if ctx.Err() != nil {
				return
			}

			// Choose a new random point using a normal distribution centered on the position directly the robot
			randAngle := rand.NormFloat64() + math.Pi
			destination := referenceframe.NewPoseInFrame(svc.base.Name().Name, spatialmath.NewPose(
				r3.Vector{
					X: math.Sin(randAngle),
					Y: math.Cos(randAngle),
					Z: 0.,
				}.Normalize().Mul(defaultDistanceMM), spatialmath.NewOrientationVector()))

			_, err := svc.exploreMotionService.Move(ctx, svc.base.Name(), destination, nil, nil, extra)
			if err != nil {
				svc.logger.Debugf("error occurred when moving to point %v: %v", destination, err)
			}
		}
	})
}
