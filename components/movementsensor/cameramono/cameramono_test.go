package cameramono

import (
	"context"
	"image"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision/keypoints"
	"go.viam.com/rdk/vision/odometry"
)

var tCo = &cameramono{
	Unimplemented:           generic.Unimplemented{},
	activeBackgroundWorkers: sync.WaitGroup{},
	cancelCtx:               context.Background(),
	cancelFunc: func() {
	},
	result: result{
		trackedPos:    r3.Vector{X: 4, Y: 5, Z: 6},
		trackedOrient: &spatialmath.OrientationVector{Theta: 90, OX: 1, OY: 2, OZ: 3},
		angVel:        spatialmath.AngularVelocity{X: 10, Y: 20, Z: 30},
		linVel:        r3.Vector{X: 40, Y: 50, Z: 60},
	},
}

func TestInit(t *testing.T) {
	camName := "cam"
	conf := &AttrConfig{}
	_, err := conf.Validate("")
	test.That(t, err.Error(), test.ShouldContainSubstring, "single camera")
	conf.Camera = camName
	_, err = conf.Validate("")
	test.That(t, err.Error(), test.ShouldContainSubstring, "motion_estimation_config")

	conf.MotionConfig = &odometry.MotionEstimationConfig{
		KeyPointCfg: &keypoints.ORBConfig{
			Layers:          1,
			DownscaleFactor: 2,
			FastConf: &keypoints.FASTConfig{
				NMatchesCircle: 1,
				NMSWinSize:     1,
				Threshold:      1,
				Oriented:       true,
				Radius:         1,
			},
			BRIEFConf: &keypoints.BRIEFConfig{
				N:              1,
				Sampling:       1,
				UseOrientation: true,
				PatchSize:      1,
			},
		},
		MatchingCfg: &keypoints.MatchingConfig{
			DoCrossCheck: true,
			MaxDist:      1,
		},
		CamIntrinsics: &transform.PinholeCameraIntrinsics{
			Width:  300,
			Height: 240,
			Fx:     1,
			Fy:     1,
			Ppx:    1,
			Ppy:    1,
		},
		ScaleEstimatorCfg: &odometry.ScaleEstimatorConfig{
			ThresholdNormalAngle: 0.1,
			ThresholdPlaneInlier: 0.1,
		},
		CamHeightGround: 0.1,
	}
	_, err = conf.Validate("")
	test.That(t, err, test.ShouldBeNil)

	logger := golog.NewDebugLogger("test")
	_, err = newCameraMono(nil, config.Component{}, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "AttrConfig")
	goodC := config.Component{ConvertedAttributes: conf}
	_, err = newCameraMono(nil, goodC, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "missing from dependencies")
	deps := make(registry.Dependencies)

	deps[camera.Named(camName)] = &inject.Camera{
		Camera: nil,
		StreamFunc: func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
			return gostream.NewEmbeddedVideoStreamFromReader(gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
				return image.NewGray(image.Rect(0, 0, 1, 1)), func() {}, nil
			})), nil
		},
		NextPointCloudFunc: func(ctx context.Context) (pointcloud.PointCloud, error) {
			return nil, nil
		},
		CloseFunc: func(ctx context.Context) error {
			return nil
		},
	}
}

func TestFunctions(t *testing.T) {
	xy, z, err := tCo.Position(tCo.cancelCtx, make(map[string]interface{}))
	test.That(t, xy, test.ShouldResemble, geo.NewPoint(4, 5))
	test.That(t, z, test.ShouldEqual, 6)
	test.That(t, err, test.ShouldBeNil)

	ori, err := tCo.Orientation(tCo.cancelCtx, make(map[string]interface{}))
	test.That(t, ori, test.ShouldResemble, &spatialmath.OrientationVector{Theta: 90, OX: 1, OY: 2, OZ: 3})
	test.That(t, err, test.ShouldBeNil)

	linVel, err := tCo.LinearVelocity(tCo.cancelCtx, make(map[string]interface{}))
	test.That(t, linVel, test.ShouldResemble, r3.Vector{X: 40, Y: 50, Z: 60})
	test.That(t, err, test.ShouldBeNil)

	angVel, err := tCo.AngularVelocity(tCo.cancelCtx, make(map[string]interface{}))
	test.That(t, angVel, test.ShouldResemble, spatialmath.AngularVelocity{X: 10, Y: 20, Z: 30})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)

	props, err := tCo.Properties(tCo.cancelCtx, make(map[string]interface{}))
	test.That(t, props, test.ShouldResemble, &movementsensor.Properties{
		PositionSupported:        true,
		OrientationSupported:     true,
		LinearVelocitySupported:  true,
		AngularVelocitySupported: true,
	})
	test.That(t, err, test.ShouldBeNil)

	acc, err := tCo.Accuracy(tCo.cancelCtx, make(map[string]interface{}))
	test.That(t, acc, test.ShouldResemble, map[string]float32{})
	test.That(t, err,
		test.ShouldResemble,
		movementsensor.ErrMethodUnimplementedAccuracy)

	ch, err := tCo.CompassHeading(tCo.cancelCtx, make(map[string]interface{}))
	test.That(t, ch, test.ShouldEqual, 0)
	test.That(t, err,
		test.ShouldResemble,
		movementsensor.ErrMethodUnimplementedCompassHeading)

	read, err := tCo.Readings(tCo.cancelCtx, make(map[string]interface{}))
	test.That(t, read["linear_velocity"], test.ShouldResemble, r3.Vector{X: 40, Y: 50, Z: 60})
	test.That(t, err, test.ShouldBeNil)

	tCo.stream = mock{StreamName: "test"}
	tCo.Close()
}

func TestMathHelpers(t *testing.T) {
	t.Run("test package math", func(t *testing.T) {
		tMotion := &odometry.Motion3D{
			Rotation:    mat.NewDense(3, 3, []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}),
			Translation: mat.NewDense(3, 1, []float64{12, 14, 16}),
		}
		dt := 2.0
		lVout := calculateLinVel(tMotion, dt)
		test.That(t, lVout, test.ShouldResemble, r3.Vector{X: 6, Y: 7, Z: 8})

		r3Out := translationToR3(tMotion)
		test.That(t, r3Out, test.ShouldResemble, r3.Vector{X: 12, Y: 14, Z: 16})

		start := time.Now()
		time.Sleep(500 * time.Millisecond)
		end := time.Now()
		dt, moreThanZero := tCo.getDt(start, end)
		test.That(t, dt, test.ShouldBeGreaterThan, 0.49)
		test.That(t, moreThanZero, test.ShouldBeTrue)
	})

	t.Run("test extract images", func(t *testing.T) {
		// TODO(RSDK-586): Re-enable after testing new field changes during hack-week.
		t.Skip()
		tCo.logger = golog.NewTestLogger(t)
		// load cfg
		cfg, err := odometry.LoadMotionEstimationConfig(artifact.MustPath("vision/odometry/vo_config.json"))
		test.That(t, err, test.ShouldBeNil)
		// load images
		im1, err := rimage.NewImageFromFile(artifact.MustPath("vision/odometry/000001.png"))
		test.That(t, err, test.ShouldBeNil)
		im2, err := rimage.NewImageFromFile(artifact.MustPath("vision/odometry/000002.png"))
		test.That(t, err, test.ShouldBeNil)
		motion, err := tCo.extractMovementFromOdometer(im1, im2, 0.1, cfg)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, motion.Translation.At(2, 0), test.ShouldBeLessThan, -0.8)
		test.That(t, motion.Translation.At(1, 0), test.ShouldBeLessThan, 0.2)
	})
}

type mock struct {
	StreamName string
}

func (m mock) Close(ctx context.Context) error {
	return nil
}

func (m mock) Next(ctx context.Context) (image.Image, func(), error) {
	x0y0 := image.Point{0, 0}
	x1y1 := image.Point{1, 1}
	return image.Rectangle{x0y0, x1y1}, nil, nil
}
