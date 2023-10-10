package builtin

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	_ "go.viam.com/rdk/components/movementsensor/fake"
	"go.viam.com/rdk/pointcloud"
	_ "go.viam.com/rdk/services/motion/builtin"
	"go.viam.com/rdk/services/navigation"
	visionSvc "go.viam.com/rdk/services/vision"
	_ "go.viam.com/rdk/services/vision/colordetector"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision"
)

func TestCreateImmediatePathGeometry(t *testing.T) {
	ctx := context.Background()

	svc, closeNavSvc := setupNavigationServiceFromConfig(t, "../data/nav_no_map_cfg.json")
	svcStruct := svc.(*builtIn)

	t.Run("base with no geometries", func(t *testing.T) {
		props, err := svcStruct.base.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)

		expectedGeometry, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0}),
			r3.Vector{X: props.WidthMeters, Y: pathLengthMM, Z: defaultBaseHeightMM},
			immediateBasePathGeometryName+"default_base",
		)
		test.That(t, err, test.ShouldBeNil)

		geometry, err := svcStruct.createImmediatePathBase(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, geometry, test.ShouldResemble, []spatialmath.Geometry{expectedGeometry})
	})

	t.Run("base with a single geometry", func(t *testing.T) {
		pose := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0})
		dimsMM := r3.Vector{X: 20, Y: 40, Z: 20}
		base := &inject.Base{}
		base.GeometriesFunc = func(ctx context.Context) ([]spatialmath.Geometry, error) {
			box, err := spatialmath.NewBox(
				pose,
				dimsMM,
				"injected_base",
			)
			return []spatialmath.Geometry{box}, err
		}
		// Update base
		svcStruct.base = base

		expectedGeometry, err := spatialmath.NewBox(
			pose,
			r3.Vector{X: dimsMM.X, Y: dimsMM.Y + pathLengthMM, Z: dimsMM.Z},
			immediateBasePathGeometryName+"default_base",
		)
		test.That(t, err, test.ShouldBeNil)

		geometry, err := svcStruct.createImmediatePathBase(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, geometry, test.ShouldResemble, []spatialmath.Geometry{expectedGeometry})
	})

	t.Run("base with multiple geometries", func(t *testing.T) {
		// Box data
		boxCenter := r3.Vector{X: 0, Y: 0, Z: 0}
		boxDimsMM := r3.Vector{X: 20, Y: 40, Z: 20}

		// Sphere data
		sphereCenter := r3.Vector{X: 5, Y: 0, Z: 0}
		sphereRadiusMM := 30.

		// Capsule data
		capsuleCenter := r3.Vector{X: -5, Y: 0, Z: 10}
		capsuleRadiusMM := 10.
		capsuleLengthMM := 10.

		base := &inject.Base{}
		base.GeometriesFunc = func(ctx context.Context) ([]spatialmath.Geometry, error) {
			var geometries []spatialmath.Geometry
			// Box
			box, err := spatialmath.NewBox(
				spatialmath.NewPoseFromPoint(boxCenter),
				boxDimsMM,
				"box",
			)
			test.That(t, err, test.ShouldBeNil)
			geometries = append(geometries, box)
			// Sphere
			sphere, err := spatialmath.NewSphere(
				spatialmath.NewPoseFromPoint(sphereCenter),
				sphereRadiusMM,
				"sphere",
			)
			if err != nil {
				return nil, err
			}
			geometries = append(geometries, sphere)
			// Capsule
			capsule, err := spatialmath.NewCapsule(
				spatialmath.NewPoseFromPoint(capsuleCenter),
				capsuleRadiusMM,
				capsuleLengthMM,
				"capsule",
			)
			if err != nil {
				return nil, err
			}
			geometries = append(geometries, capsule)
			return geometries, err
		}
		// Update base
		svcStruct.base = base

		expectedBoxGeometry, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(boxCenter.Add(r3.Vector{X: 0, Y: pathLengthMM / 2, Z: 0})),
			boxDimsMM,
			immediateBasePathGeometryName+"box",
		)
		test.That(t, err, test.ShouldBeNil)

		expectedSphereGeometry, err := spatialmath.NewCapsule(
			spatialmath.NewPoseFromPoint(sphereCenter.Add(r3.Vector{X: 0, Y: pathLengthMM / 2, Z: 0})),
			sphereRadiusMM,
			capsuleLengthMM,
			immediateBasePathGeometryName+"sphere",
		)
		test.That(t, err, test.ShouldBeNil)

		expectedCapsuleGeometry, err := spatialmath.NewCapsule(
			spatialmath.NewPoseFromPoint(capsuleCenter.Add(r3.Vector{X: 0, Y: pathLengthMM / 2, Z: 0})),
			capsuleRadiusMM,
			capsuleLengthMM+pathLengthMM,
			immediateBasePathGeometryName+"capsule",
		)
		test.That(t, err, test.ShouldBeNil)

		geometry, err := svcStruct.createImmediatePathBase(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, geometry, test.ShouldResemble,
			[]spatialmath.Geometry{
				expectedBoxGeometry,
				expectedSphereGeometry,
				expectedCapsuleGeometry,
			})
	})

	t.Run("base with bad geometry", func(t *testing.T) {
		injectedBase := &inject.Base{}
		injectedBase.GeometriesFunc = func(ctx context.Context) ([]spatialmath.Geometry, error) {
			return nil, errors.New("expected error")
		}
		// Update base
		svcStruct.base = injectedBase

		geometry, err := svcStruct.createImmediatePathBase(ctx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errBadGeometry)
		test.That(t, len(geometry), test.ShouldEqual, 0)
	})

	t.Run("base with bad properties", func(t *testing.T) {
		injectedBase := &inject.Base{}
		injectedBase.GeometriesFunc = func(ctx context.Context) ([]spatialmath.Geometry, error) {
			return []spatialmath.Geometry{}, nil
		}

		injectedBase.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (
			base.Properties, error,
		) {
			return base.Properties{}, errors.New("expected error")
		}
		// Update base
		svcStruct.base = injectedBase

		geometry, err := svcStruct.createImmediatePathBase(ctx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errBadProperties)
		test.That(t, len(geometry), test.ShouldEqual, 0)
	})

	// test oriented geometries

	closeNavSvc()
}

func TestExplore(t *testing.T) {
	ctx := context.Background()

	obstacleLocationInPath := r3.Vector{X: 0, Y: 0, Z: 0}
	obstacleLocationNotInPath := r3.Vector{X: 0, Y: 0, Z: 100}

	highObstacleProbability := 100
	lowObstacleProbability := 10

	svc, closeNavSvc := setupNavigationServiceFromConfig(t, "../data/nav_no_map_cfg.json")
	svcStruct := svc.(*builtIn)

	cases := []struct {
		description             string
		obstacleLocation        r3.Vector
		obstacleProbability     int
		getObjectPointCloudFunc func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*vision.Object, error)
	}{
		{
			description:         "with low probability obstacle in path",
			obstacleLocation:    obstacleLocationInPath,
			obstacleProbability: 100,
			getObjectPointCloudFunc: func(ctx context.Context, cameraName string, extra map[string]interface{}) (
				[]*vision.Object, error,
			) {
				cloud := pointcloud.New()
				err := cloud.Set(obstacleLocationInPath, pointcloud.NewValueData(lowObstacleProbability))
				test.That(t, err, test.ShouldBeNil)

				obj, err := vision.NewObject(cloud)
				test.That(t, err, test.ShouldBeNil)
				return []*vision.Object{obj}, nil
			},
		},
		{
			description:         "with high probability obstacle in path",
			obstacleLocation:    obstacleLocationInPath,
			obstacleProbability: lowObstacleProbability,
			getObjectPointCloudFunc: func(ctx context.Context, cameraName string, extra map[string]interface{}) (
				[]*vision.Object, error,
			) {
				cloud := pointcloud.New()
				err := cloud.Set(obstacleLocationInPath, pointcloud.NewValueData(highObstacleProbability))
				test.That(t, err, test.ShouldBeNil)

				obj, err := vision.NewObject(cloud)
				test.That(t, err, test.ShouldBeNil)
				return []*vision.Object{obj}, nil
			},
		},
		{
			description:         "with high probability obstacle not in path",
			obstacleLocation:    obstacleLocationNotInPath,
			obstacleProbability: highObstacleProbability,
			getObjectPointCloudFunc: func(ctx context.Context, cameraName string, extra map[string]interface{}) (
				[]*vision.Object, error,
			) {
				cloud := pointcloud.New()
				err := cloud.Set(obstacleLocationNotInPath, pointcloud.NewValueData(highObstacleProbability))
				test.That(t, err, test.ShouldBeNil)

				obj, err := vision.NewObject(cloud)
				test.That(t, err, test.ShouldBeNil)
				return []*vision.Object{obj}, nil
			},
		},
		{
			description: "with no obstacles",
			getObjectPointCloudFunc: func(ctx context.Context, cameraName string, extra map[string]interface{}) (
				[]*vision.Object, error,
			) {
				return []*vision.Object{}, nil
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			injectVisionService := &inject.VisionService{}
			injectVisionService.GetObjectPointCloudsFunc = tt.getObjectPointCloudFunc
			svcStruct.visionServices = []visionSvc.Service{injectVisionService}

			err := svcStruct.SetMode(ctx, navigation.ModeExplore, nil)
			test.That(t, err, test.ShouldBeNil)

			// log checker
		})
	}
	closeNavSvc()
}
