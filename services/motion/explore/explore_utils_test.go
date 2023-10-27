package explore

import (
	"context"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/components/base"
	baseFake "go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/components/camera"
	cameraFake "go.viam.com/rdk/components/camera/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	vSvc "go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision"
)

// createNewExploreMotionService creates a new motion service complete with base, camera (optional) and frame system.
// Note: The required vision service can/is added later as it will not affect the building of the frame system.
func createNewExploreMotionService(ctx context.Context, logger logging.Logger, fakeBase base.Base, cameras []camera.Camera,
) (motion.Service, error) {
	var fsParts []*referenceframe.FrameSystemPart
	deps := make(resource.Dependencies)

	// create base link
	baseLink, err := createBaseLink(testBaseName.Name)
	if err != nil {
		return nil, err
	}
	fsParts = append(fsParts, &referenceframe.FrameSystemPart{FrameConfig: baseLink})
	deps[testBaseName] = fakeBase

	// create camera link
	for _, cam := range cameras {
		cameraLink, err := createCameraLink(cam.Name().Name, fakeBase.Name().Name)
		if err != nil {
			return nil, err
		}
		fsParts = append(fsParts, &referenceframe.FrameSystemPart{FrameConfig: cameraLink})
		deps[cam.Name()] = cam
	}

	// create frame service
	fs, err := createFrameSystemService(ctx, deps, fsParts, logger)
	if err != nil {
		return nil, err
	}
	deps[testFrameServiceName] = fs

	// create explore motion service
	exploreMotionConf := resource.Config{ConvertedAttributes: &Config{}}
	return NewExplore(ctx, deps, exploreMotionConf, logger)
}

// createFakeBase instantiates a fake base.
func createFakeBase(ctx context.Context, logger logging.Logger) (base.Base, error) {
	fakeBaseCfg := resource.Config{
		Name:  testBaseName.Name,
		API:   base.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{X: 300, Y: 200, Z: 100}},
	}
	return baseFake.NewBase(ctx, nil, fakeBaseCfg, logger)
}

// createFakeCamera instantiates a fake camera.
func createFakeCamera(ctx context.Context, logger logging.Logger, name string) (camera.Camera, error) {
	fakeCameraCfg := resource.Config{
		Name:  name,
		API:   camera.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: 5}},
		ConvertedAttributes: &cameraFake.Config{
			Width:  1000,
			Height: 1000,
		},
	}
	return cameraFake.NewCamera(ctx, fakeCameraCfg, logger)
}

// createMockVisionService instantiates a mock vision service with a custom version of GetObjectPointCloud that returns
// vision objects from a given set of points.
func createMockVisionService(visionSvcNum string, obstacles []obstacleMetadata) vSvc.Service {
	mockVisionService := &inject.VisionService{}

	mockVisionService.GetObjectPointCloudsFunc = func(ctx context.Context, cameraName string, extra map[string]interface{},
	) ([]*vision.Object, error) {
		var vObjects []*vision.Object
		for _, obs := range obstacles {
			obstaclePosition := spatialmath.NewPoseFromPoint(obs.position)
			box, err := spatialmath.NewBox(obstaclePosition, r3.Vector{X: 100, Y: 100, Z: 100}, "test-case-2")
			if err != nil {
				return nil, err
			}

			detection, err := vision.NewObjectWithLabel(pointcloud.New(), obs.label+visionSvcNum, box.ToProtobuf())
			if err != nil {
				return nil, err
			}
			vObjects = append(vObjects, detection)
		}
		return vObjects, nil
	}
	return mockVisionService
}

// createFrameSystemService will create a basic frame service from the list of parts.
func createFrameSystemService(
	ctx context.Context,
	deps resource.Dependencies,
	fsParts []*referenceframe.FrameSystemPart,
	logger logging.Logger,
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

	return fsSvc, nil
}

// createBaseLink instantiates the base to world link for the frame system.
func createBaseLink(baseName string) (*referenceframe.LinkInFrame, error) {
	basePose := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0})
	baseSphere, err := spatialmath.NewSphere(basePose, 10, "base-box")
	if err != nil {
		return nil, err
	}

	baseLink := referenceframe.NewLinkInFrame(
		referenceframe.World,
		spatialmath.NewZeroPose(),
		baseName,
		baseSphere,
	)
	return baseLink, nil
}

// createCameraLink instantiates the camera to base link for the frame system.
func createCameraLink(camName, baseFrame string) (*referenceframe.LinkInFrame, error) {
	camPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0})
	camSphere, err := spatialmath.NewSphere(camPose, 5, "cam-sphere")
	if err != nil {
		return nil, err
	}

	camLink := referenceframe.NewLinkInFrame(
		baseFrame,
		spatialmath.NewZeroPose(),
		camName,
		camSphere,
	)
	return camLink, nil
}

// geometriesContainsPoint is a helper function to test if a point is in a given geometry.
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
