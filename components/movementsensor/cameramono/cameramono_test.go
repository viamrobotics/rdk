package cameramono

import (
	"context"
	"image"
	"sync"
	"testing"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.uber.org/zap"
	"go.viam.com/test"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision/odometry"
)

var tCo = &cameramono{
	Unimplemented:           generic.Unimplemented{},
	activeBackgroundWorkers: sync.WaitGroup{},
	cancelCtx:               context.Background(),
	cancelFunc: func() {
	},
	output:        make(chan *odometry.Motion3D),
	logger:        &zap.SugaredLogger{},
	trackedPos:    r3.Vector{X: 4, Y: 5, Z: 6},
	trackedOrient: &spatialmath.OrientationVector{Theta: 90, OX: 1, OY: 2, OZ: 3},
	angVel:        spatialmath.AngularVelocity{X: 10, Y: 20, Z: 30},
	linVel:        r3.Vector{X: 40, Y: 50, Z: 60},
}

func TestInit(t *testing.T) {
	camName := "cam"
	conf := &AttrConfig{}
	err := conf.Validate()
	test.That(t, err.Error(), test.ShouldContainSubstring, "single camera")
	conf.Camera = camName
	err = conf.Validate()
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	logger := golog.NewDevelopmentLogger("test")
	_, err = newCameraMono(ctx, nil, config.Component{}, logger)
	test.That(t, err, test.ShouldNotBeNil)
	goodC := config.Component{ConvertedAttributes: conf}
	_, err = newCameraMono(ctx, nil, goodC, logger)
	test.That(t, err, test.ShouldNotBeNil)
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
		ProjectorFunc: func(ctx context.Context) (rimage.Projector, error) {
			return nil, nil
		},
		CloseFunc: func(ctx context.Context) error {
			return nil
		},
	}
	co, err := newCameraMono(ctx, deps, goodC, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, co, test.ShouldHaveSameTypeAs, &cameramono{})
}

func TestGetFunctions(t *testing.T) {
	xy, z, err := tCo.GetPosition(tCo.cancelCtx)
	test.That(t, xy, test.ShouldResemble, geo.NewPoint(4, 5))
	test.That(t, z, test.ShouldEqual, 6)
	test.That(t, err, test.ShouldBeNil)

	ori, err := tCo.GetOrientation(tCo.cancelCtx)
	test.That(t, ori, test.ShouldResemble, &spatialmath.OrientationVector{Theta: 90, OX: 1, OY: 2, OZ: 3})
	test.That(t, err, test.ShouldBeNil)

	linVel, err := tCo.GetLinearVelocity(tCo.cancelCtx)
	test.That(t, linVel, test.ShouldResemble, r3.Vector{X: 40, Y: 50, Z: 60})
	test.That(t, err, test.ShouldBeNil)

	angVel, err := tCo.GetAngularVelocity(tCo.cancelCtx)
	test.That(t, angVel, test.ShouldResemble, spatialmath.AngularVelocity{X: 10, Y: 20, Z: 30})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)

	props, err := tCo.GetProperties(tCo.cancelCtx)
	test.That(t, props, test.ShouldResemble, &movementsensor.Properties{
		PositionSupported:        true,
		OrientationSupported:     true,
		LinearVelocitySupported:  true,
		AngularVelocitySupported: true,
	})
	test.That(t, err, test.ShouldBeNil)

	acc, err := tCo.GetAccuracy(tCo.cancelCtx)
	test.That(t, acc, test.ShouldResemble, map[string]float32{})
	test.That(t, err, test.ShouldBeNil)

	ch, err := tCo.GetCompassHeading(tCo.cancelCtx)
	test.That(t, ch, test.ShouldEqual, 0)
	test.That(t, err, test.ShouldBeNil)

	read, err := tCo.GetReadings(tCo.cancelCtx)
	test.That(t, read["linear_velocity"], test.ShouldResemble, r3.Vector{X: 40, Y: 50, Z: 60})
	test.That(t, err, test.ShouldBeNil)

	tCo.Close()
}

func TestMathHelpers(t *testing.T) {
	tMotion := &odometry.Motion3D{
		Rotation:    mat.NewDense(3, 3, []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}),
		Translation: mat.NewDense(3, 1, []float64{12, 14, 16}),
	}
	dt := 2.0
	lVout := calculateLinVel(tMotion, dt)
	test.That(t, lVout, test.ShouldResemble, r3.Vector{X: 6, Y: 7, Z: 8})

	r3Out := translationToR3(tMotion)
	test.That(t, r3Out, test.ShouldResemble, r3.Vector{X: 12, Y: 14, Z: 16})
}
