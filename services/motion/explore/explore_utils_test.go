package explore

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	baseFake "go.viam.com/rdk/components/base/fake"

	"go.viam.com/rdk/components/camera"
	cameraFake "go.viam.com/rdk/components/camera/fake"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	vSvc "go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision"
)

func createFakeBase(ctx context.Context, logger golog.Logger) (base.Base, error) {
	fakeBaseCfg := resource.Config{
		Name:  testBaseName.Name,
		API:   base.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{X: 300, Y: 200, Z: 100}},
	}
	return baseFake.NewBase(ctx, nil, fakeBaseCfg, logger)
}

func createFakeCamera(ctx context.Context, logger golog.Logger) (camera.Camera, error) {
	fakeCameraCfg := resource.Config{
		Name:  testCameraName.Name,
		API:   camera.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: 5}},
		ConvertedAttributes: &cameraFake.Config{
			Width:  1000,
			Height: 1000,
		},
	}
	return cameraFake.NewCamera(ctx, fakeCameraCfg, logger)
}

func createMockCamera(ctx context.Context, obstacles []obstacleMetadata, expectedError error) camera.Camera {
	mockCamera := &inject.Camera{}

	mockCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		pc := pointcloud.New()
		for _, obsData := range obstacles {
			if err := pc.Set(obsData.position, pointcloud.NewValueData(obsData.data)); err != nil {
				return nil, expectedError
			}
		}
		return pc, expectedError
	}
	return mockCamera
}

func createMockVisionService(ctx context.Context, obstacle obstacleMetadata, expectedError error) vSvc.Service {
	var noObstacle obstacleMetadata
	mockVisionService := &inject.VisionService{}
	mockVisionService.GetObjectPointCloudsFunc = func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*vision.Object, error) {
		if obstacle == noObstacle {
			return []*vision.Object{}, nil
		}

		obstaclePosition := spatialmath.NewPoseFromPoint(obstacle.position)
		box, err := spatialmath.NewBox(obstaclePosition, r3.Vector{X: 100, Y: 100, Z: 100}, "test-case-2")
		if err != nil {
			return nil, err
		}

		detection, err := vision.NewObjectWithLabel(pointcloud.New(), obstacle.label, box.ToProtobuf())
		if err != nil {
			return nil, err
		}

		return []*vision.Object{detection}, nil
	}
	return mockVisionService
}

func createFrameSystemService(
	ctx context.Context,
	deps resource.Dependencies,
	fsParts []*referenceframe.FrameSystemPart,
	logger golog.Logger,
) (framesystem.Service, error) {
	fsSvc, err := framesystem.New(ctx, deps, logger)
	if err != nil {
		return nil, err
	}
	conf := resource.Config{
		ConvertedAttributes: &framesystem.Config{Parts: fsParts},
	}
	if err := fsSvc.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	deps[fsSvc.Name()] = fsSvc

	return fsSvc, nil
}

func createBaseLink(t *testing.T, baseName string) *referenceframe.LinkInFrame {
	basePose := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0})
	baseSphere, err := spatialmath.NewSphere(basePose, 10, "base-box")
	test.That(t, err, test.ShouldBeNil)

	baseLink := referenceframe.NewLinkInFrame(
		referenceframe.World,
		spatialmath.NewZeroPose(),
		baseName,
		baseSphere,
	)
	return baseLink
}

func createCameraLink(t *testing.T, camName, baseFrame string) *referenceframe.LinkInFrame {
	camPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0})
	camSphere, err := spatialmath.NewSphere(camPose, 5, "cam-sphere")
	test.That(t, err, test.ShouldBeNil)

	camLink := referenceframe.NewLinkInFrame(
		baseFrame, // referenceframe.World,
		spatialmath.NewZeroPose(),
		camName,
		camSphere,
	)
	return camLink
}

type obstacleMetadata struct {
	position r3.Vector
	data     int
	label    string
}

func geometriesContainsPoint(geometries []spatialmath.Geometry, point r3.Vector) (bool, error) {
	var collisionDetected bool
	for _, geo := range geometries {
		pointGeoCfg := spatialmath.GeometryConfig{
			Type:              spatialmath.BoxType,
			TranslationOffset: point,
		}
		pointGeo, err := pointGeoCfg.ParseConfig()
		if err != nil {
			return false, err
		}
		collides, err := geo.CollidesWith(pointGeo)
		if err != nil {
			return false, err
		}
		if collides {
			collisionDetected = true
		}
	}
	return collisionDetected, nil
}
