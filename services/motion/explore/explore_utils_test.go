package explore

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	"go.viam.com/rdk/components/base"
	baseFake "go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/components/camera"
	cameraFake "go.viam.com/rdk/components/camera/fake"
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

var (
	testBaseName         = resource.NewName(base.API, "test_base")
	testCameraName       = resource.NewName(camera.API, "test_camera")
	testFrameServiceName = resource.NewName(framesystem.API, "test_fs")
	defaultKBOptsExtra   = map[string]interface{}{
		"angular_degs_per_sec":          .25,
		"linear_m_per_sec":              .1,
		"obstacle_polling_frequency_hz": 1,
	}
)

func createNewExploreMotionService(ctx context.Context, logger golog.Logger, fakeBase base.Base, cam camera.Camera,
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
	if cam != nil {
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

func createMockVisionService(obstacle obstacleMetadata) vSvc.Service {
	var noObstacle obstacleMetadata
	mockVisionService := &inject.VisionService{}
	mockVisionService.GetObjectPointCloudsFunc = func(ctx context.Context, cameraName string, extra map[string]interface{},
	) ([]*vision.Object, error) {
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

func createCameraLink(camName, baseFrame string) (*referenceframe.LinkInFrame, error) {
	camPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0})
	camSphere, err := spatialmath.NewSphere(camPose, 5, "cam-sphere")
	if err != nil {
		return nil, err
	}

	camLink := referenceframe.NewLinkInFrame(
		baseFrame, // referenceframe.World,
		spatialmath.NewZeroPose(),
		camName,
		camSphere,
	)
	return camLink, nil
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
