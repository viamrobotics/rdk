package builtin

import (
	"context"
	"math/rand"
	"time"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/spatialmath"
)

const (
	// Name of path geometry in which we look for obstacles.
	immediateBasePathGeometryName = "immediateBasePath"
	// Distance in front of base to look for obstacle.
	pathLengthMM float64 = 100
	// Default height of the base to be used if the base geometry is not available from the frame system.
	defaultBaseHeightMM float64 = 10
	// The rate at which obstacle detection and its associated response is performed.
	obstacleDetectionLoopRate = time.Millisecond * 50

	errBadGeometry   = "issue getting geometries from base"
	errBadProperties = "unable to get properties from the base"
)

// createImmediatePathGeometry creates a geometry that indicates the immediate path in front of the obstacle in
// which to look for obstacles. This geometry is built using a hardcoded path length in front of the base and
// the base geometry defined in the frame system. If no geometry is provided for the base in the frame system,
// the base's width an a default height is used.
func (svc *builtIn) createImmediatePathBase(ctx context.Context) ([]spatialmath.Geometry, error) {
	var pathGeometries []spatialmath.Geometry

	// Create immediate path geometry from geometry
	createImmediatePathGeometry := func(baseGeometry spatialmath.Geometry) (spatialmath.Geometry, error) {
		pose := baseGeometry.Pose()

		// Box
		boxProto := baseGeometry.ToProtobuf().GetBox()
		if boxProto != nil {
			pathGeometry, err := spatialmath.NewBox(
				spatialmath.NewPose(pose.Point().Add(r3.Vector{X: 0, Y: pathLengthMM / 2, Z: 0}),
					pose.Orientation(),
				),
				r3.Vector{
					X: boxProto.DimsMm.X,
					Y: boxProto.DimsMm.Y,
					Z: boxProto.DimsMm.Z,
				},
				immediateBasePathGeometryName+baseGeometry.Label(),
			)
			if err != nil {
				return nil, errors.Wrapf(err, "issue creating immediate path box from box")
			}
			return pathGeometry, nil
		}
		// Sphere
		sphereProto := baseGeometry.ToProtobuf().GetSphere()
		if sphereProto != nil {
			pathGeometry, err := spatialmath.NewCapsule(
				spatialmath.NewPose(pose.Point().Add(r3.Vector{X: 0, Y: pathLengthMM / 2, Z: 0}),
					pose.Orientation(),
				),
				sphereProto.GetRadiusMm(),
				pathLengthMM,
				immediateBasePathGeometryName+baseGeometry.Label(),
			)
			if err != nil {
				return nil, errors.Wrapf(err, "issue creating immediate path capsule from sphere")
			}
			return pathGeometry, nil
		}
		// Capsule
		capsuleProto := baseGeometry.ToProtobuf().GetCapsule()
		if capsuleProto != nil {
			pathGeometry, err := spatialmath.NewCapsule(
				spatialmath.NewPose(pose.Point().Add(r3.Vector{X: 0, Y: pathLengthMM / 2, Z: 0}),
					pose.Orientation(),
				),
				capsuleProto.GetRadiusMm(),
				capsuleProto.LengthMm+pathLengthMM,
				immediateBasePathGeometryName+baseGeometry.Label(),
			)
			if err != nil {
				return nil, errors.Wrapf(err, "issue creating immediate path capsule from capsule")
			}
			return pathGeometry, nil
		}
		return nil, errors.New("invalid geometry, could not create immediate path")
	}

	// Define geometries and associated immediate paths if base geometries are available
	baseGeometries, err := svc.base.Geometries(ctx, nil)
	if err != nil {
		return nil, errors.Wrapf(err, errBadGeometry)
	}

	if len(baseGeometries) != 0 {
		for _, baseGeometry := range baseGeometries {
			pathGeometry, err := createImmediatePathGeometry(baseGeometry)
			if err != nil {
				return nil, err
			}
			pathGeometries = append(pathGeometries, pathGeometry)
		}
		return pathGeometries, nil
	}

	// Define default geometry and associated immediate path if no base geometry is available
	svc.logger.Warn("no geometry found associated with base, using default based on base width")

	baseProperties, err := svc.base.Properties(ctx, nil)
	if err != nil {
		return nil, errors.Wrapf(err, errBadProperties)
	}

	baseGeometry, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(
		r3.Vector{X: 0, Y: pathLengthMM / 2, Z: 0}),
		r3.Vector{X: baseProperties.WidthMeters, Y: pathLengthMM, Z: defaultBaseHeightMM},
		"default_base")
	if err != nil {
		return nil, errors.Wrapf(err, "issue creating default base as box")
	}

	pathGeometry, err := createImmediatePathGeometry(baseGeometry)
	if err != nil {
		return nil, err
	}

	return []spatialmath.Geometry{pathGeometry}, nil
}

// handleObstacle causes the specified base to stop and spin a random angle if an obstacle is observed.
func (svc *builtIn) handleObstacle(ctx context.Context) error {
	// If obstacle exists, stop the base and spin a random angle [-180, 180]
	isMoving, err := svc.base.IsMoving(ctx)
	if err != nil {
		return errors.Wrapf(err, "issue checking if base is moving")
	}
	if isMoving {
		if err = svc.base.Stop(ctx, nil); err != nil {
			return errors.Wrapf(err, "issue stopping base when obstacle is detected")
		}
	}

	//nolint:gosec
	randomAngle := float64(rand.Intn(360) - 180)
	if err := svc.base.Spin(ctx, randomAngle, svc.motionCfg.AngularDegsPerSec, nil); err != nil {
		return errors.Wrapf(err, "issue spinning base when obstacle is detected")
	}
	return nil
}

// handleNoObstacle causes the specified base to move forward when no obstacle is observed forward at the given
// velocity.
func (svc *builtIn) handleNoObstacle(ctx context.Context) error {
	// If obstacle does not exists, move forward at given velocity
	isMoving, err := svc.base.IsMoving(ctx)
	if err != nil {
		return errors.Errorf("issue checking if base is moving: %v", err)
	}
	if !isMoving {
		if err := svc.base.SetVelocity(ctx, r3.Vector{X: 0, Y: svc.motionCfg.LinearMPerSec * 1000}, r3.Vector{}, nil); err != nil {
			return errors.Errorf("issue setting velocity base when no obstacle is detected: %v", err)
		}
	}
	return nil
}

// startExploreMode begins a background process which implements a random walk algorithm. This random walk algorithm
// will move the base forward until it detects an obstacle. When an obstacle is detected, the base will stop and
// spin a random angle from [-180, 180] and check for an obstacle before moving forward again. Obstacle detection is
// done using GetObjectPointCloud in the associated the vision service a with the user defined camera.
func (svc *builtIn) startExploreMode(ctx context.Context) {
	svc.logger.Debug("startExploreMode called")

	// Create a geometric region representing the immediate path the base will be moving along. Obstacles will be
	// searched for in this region.
	baseImmediatePathGeometries, err := svc.createImmediatePathBase(ctx)
	if err != nil {
		svc.logger.Errorf("unable to build geometric region to look for obstacles: %v", err)
	}

	// Begin background process
	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()

		// Function used to detect if an obstacle is in the immediate path of the base based on given vision
		// services and cameras.
		detectObstacle := func(ctx context.Context) (bool, error) {
			// Iterate through vision services
			for _, visionService := range svc.visionServices {
				visionObjects, err := visionService.GetObjectPointClouds(ctx, "rplidar", nil) // camera, nil)
				if err != nil {
					return false, err
				}

				// Iterate through vision objects and return true if one is present in the baseImmediatePathGeometries
				for _, visionObject := range visionObjects {
					for _, baseImmediatePathGeometry := range baseImmediatePathGeometries {
						obs, err := baseImmediatePathGeometry.CollidesWith(visionObject.Geometry)
						if err != nil {
							return false, err
						}
						if obs {
							return true, nil
						}
					}
				}
			}
			return false, nil
		}

		timer := time.NewTicker(obstacleDetectionLoopRate)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				// Stop motor
				if err := svc.base.Stop(ctx, nil); err != nil {
					svc.logger.Error("issue stopping base when exiting explore mode")
				}
				return

			case <-timer.C:
				// Look for obstacle in immediate path
				isObstacle, err := detectObstacle(ctx)
				if err != nil {
					svc.logger.Error("failed to determine in obstacle is in front of base")
				}

				if isObstacle {
					err := svc.handleObstacle(ctx)
					svc.logger.Errorw("issue handling obstacle presence", "error", err)
				} else {
					err := svc.handleNoObstacle(ctx)
					svc.logger.Errorw("issue handling obstacle absence ", "error", err)
				}
			}
		}
	})
}
