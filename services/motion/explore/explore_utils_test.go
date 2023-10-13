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
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: 20}},
	}
	return baseFake.NewBase(ctx, nil, fakeBaseCfg, logger)
}

func createMockCamera(ctx context.Context, isObstacle bool, obstaclePose r3.Vector, obstacleProb int, expectedError error) camera.Camera {
	mockCamera := &inject.Camera{}

	mockCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		pc := pointcloud.New()
		err := pc.Set(obstaclePose, pointcloud.NewValueData(obstacleProb))
		if err != nil {
			return nil, expectedError
		}
		return pc, expectedError
	}
	return mockCamera
}

func createMockVisionService(ctx context.Context, cam camera.Camera, expectedError error) vSvc.Service {
	mockVisionService := &inject.VisionService{}

	mockVisionService.GetObjectPointCloudsFunc = func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*vision.Object, error) {
		pc, err := cam.NextPointCloud(ctx)
		if err != nil {
			return nil, expectedError
		}
		vObj, err := vision.NewObject(pc)
		if err != nil {
			return nil, expectedError
		}
		return []*vision.Object{vObj}, expectedError
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
	baseSphere, err := spatialmath.NewSphere(basePose, 10, "base-sphere")
	test.That(t, err, test.ShouldBeNil)
	baseLink := referenceframe.NewLinkInFrame(
		referenceframe.World,
		spatialmath.NewZeroPose(),
		baseName,
		baseSphere,
	)
	return baseLink
}

func createCameraLink(t *testing.T, camName string) *referenceframe.LinkInFrame {
	camPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0})
	camSphere, err := spatialmath.NewSphere(camPose, 10, "cam-sphere")
	test.That(t, err, test.ShouldBeNil)
	baseLink := referenceframe.NewLinkInFrame(
		referenceframe.World,
		spatialmath.NewZeroPose(),
		camName,
		camSphere,
	)
	return baseLink
}
