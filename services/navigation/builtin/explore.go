package builtin

import (
	"context"
	"math/rand"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/utils"
)

const (
	// Name of path geometry in which we look for obstacles
	immediateBasePathGeometryName = "immediateBasePath"
	// Distance in front of base to look for obstacle
	pathLengthMM float64 = 100
)

func createImmediatePathGeometry(ctx context.Context, svc *builtIn) (spatialmath.Geometry, error) {
	baseProperties, err := svc.base.Properties(ctx, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get properties from the base")
	}

	return spatialmath.NewBox(spatialmath.NewPoseFromPoint(
		r3.Vector{X: 0, Y: pathLengthMM / 2, Z: 0}),
		r3.Vector{
			X: baseProperties.WidthMeters,
			Y: pathLengthMM,
			Z: 3, //?????????
		},
		immediateBasePathGeometryName)
}

//nolint:unparam
func (svc *builtIn) startExploreMode(ctx context.Context) {
	svc.logger.Debug("startExploreMode called")

	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()

		baseImmediatePathGeometry, err := createImmediatePathGeometry(ctx, svc)
		if err != nil {
			svc.logger.Errorf("unable to build geometric region to look for obstacles: %v", err)
		}

		detectObstacle := func(ctx context.Context) (bool, error) {
			objs, err := svc.vision[0].GetObjectPointClouds(ctx, camera, nil)
			if err != nil {
				return false, err
			}

			var isObstacle bool
			for _, obj := range objs {
				isObstacle, err = baseImmediatePathGeometry.CollidesWith(obj.Geometry)
				if err != nil {
					return false, err
				}
			}
			return isObstacle, nil

		}

		for {
			if ctx.Err() != nil {
				// Stop motor
				if err := svc.base.Stop(ctx, nil); err != nil {
					svc.logger.Error("issue stopping base when exiting explore mode")
				}
				return
			}

			isObstacle, err := detectObstacle(ctx)
			if err != nil {
				svc.logger.Error("failed to determine in obstacle is in front of base")
			}

			if isObstacle {
				isMoving, err := svc.base.IsMoving(ctx)
				if err != nil {
					svc.logger.Warn("issue checking if base is moving")
				}
				if isMoving {
					if err = svc.base.Stop(ctx, nil); err != nil {
						svc.logger.Error("issue stopping base when obstacle is detected")
					}
				}

				randomAngle := float64(rand.Intn(360) - 180)
				if err := svc.base.Spin(ctx, randomAngle, svc.motionCfg.AngularDegsPerSec, nil); err != nil {
					svc.logger.Error("issue spinning base when obstacle is detected")
				}
			} else {
				isMoving, err := svc.base.IsMoving(ctx)
				if err != nil {
					svc.logger.Warn("issue checking if base is moving")
				}
				if !isMoving {
					if err := svc.base.SetVelocity(ctx, r3.Vector{X: 0, Y: svc.motionCfg.LinearMPerSec * 1000}, r3.Vector{}, nil); err != nil {
						svc.logger.Error("issue setting velocity base when no obstacle is detected")
					}
				}
			}
		}
	})
}
