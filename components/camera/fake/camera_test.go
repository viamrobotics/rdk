package fake

import (
	"context"
	"errors"
	"image"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/pion/rtp"
	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
)

//nolint:dupl
func TestFakeCameraHighResolution(t *testing.T) {
	model, width, height := fakeModel(1280, 720)
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	camOri := &Camera{
		ctx: cancelCtx, cancelFn: cancelFn,
		Named: camera.Named("test_high").AsNamed(), Model: model, Width: width, Height: height,
	}
	src, err := camera.NewVideoSourceFromReader(context.Background(), camOri, model, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	cameraTest(t, src, 1280, 720, 921600, model.PinholeCameraIntrinsics, model.Distortion)
	// (0,0) entry defaults to (1280, 720)
	model, width, height = fakeModel(0, 0)
	cancelCtx2, cancelFn2 := context.WithCancel(context.Background())
	camOri = &Camera{
		ctx: cancelCtx2, cancelFn: cancelFn2,
		Named: camera.Named("test_high_zero").AsNamed(), Model: model, Width: width, Height: height,
	}
	src, err = camera.NewVideoSourceFromReader(context.Background(), camOri, model, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	cameraTest(t, src, 1280, 720, 921600, model.PinholeCameraIntrinsics, model.Distortion)
}

func TestFakeCameraMedResolution(t *testing.T) {
	model, width, height := fakeModel(640, 360)
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	camOri := &Camera{
		ctx: cancelCtx, cancelFn: cancelFn,
		Named: camera.Named("test_high").AsNamed(), Model: model, Width: width, Height: height,
	}
	src, err := camera.NewVideoSourceFromReader(context.Background(), camOri, model, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	cameraTest(t, src, 640, 360, 230400, model.PinholeCameraIntrinsics, model.Distortion)
	err = src.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

//nolint:dupl
func TestFakeCameraUnspecified(t *testing.T) {
	// one unspecified side should keep 16:9 aspect ratio
	// (320, 0) -> (320, 180)
	model, width, height := fakeModel(320, 0)
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	camOri := &Camera{
		ctx: cancelCtx, cancelFn: cancelFn,
		Named: camera.Named("test_320").AsNamed(), Model: model, Width: width, Height: height,
	}
	src, err := camera.NewVideoSourceFromReader(context.Background(), camOri, model, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	cameraTest(t, src, 320, 180, 57600, model.PinholeCameraIntrinsics, model.Distortion)
	// (0, 180) -> (320, 180)
	model, width, height = fakeModel(0, 180)
	cancelCtx2, cancelFn2 := context.WithCancel(context.Background())
	camOri = &Camera{
		ctx: cancelCtx2, cancelFn: cancelFn2,
		Named: camera.Named("test_180").AsNamed(), Model: model, Width: width, Height: height,
	}
	src, err = camera.NewVideoSourceFromReader(context.Background(), camOri, model, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	cameraTest(t, src, 320, 180, 57600, model.PinholeCameraIntrinsics, model.Distortion)
}

func TestFakeCameraParams(t *testing.T) {
	// test odd width and height
	cfg := &Config{
		Width:  321,
		Height: 0,
	}
	_, err := cfg.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	cfg = &Config{
		Width:  0,
		Height: 321,
	}
	_, err = cfg.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
}

func cameraTest(
	t *testing.T,
	cam camera.VideoSource,
	width, height, points int,
	intrinsics *transform.PinholeCameraIntrinsics,
	distortion transform.Distorter,
) {
	t.Helper()
	stream, err := cam.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)
	img, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img.Bounds().Dx(), test.ShouldEqual, width)
	test.That(t, img.Bounds().Dy(), test.ShouldEqual, height)
	pc, err := cam.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, points)
	prop, err := cam.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, prop.IntrinsicParams, test.ShouldResemble, intrinsics)
	if distortion == nil {
		test.That(t, prop.DistortionParams, test.ShouldBeNil)
	} else {
		test.That(t, prop.DistortionParams, test.ShouldResemble, distortion)
	}
	err = cam.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestCameraValidationAndCreation(t *testing.T) {
	attrCfg := &Config{Width: 200000, Height: 10}
	cfg := resource.Config{
		Name:                "test1",
		API:                 camera.API,
		Model:               Model,
		ConvertedAttributes: attrCfg,
	}

	// error with a ridiculously large pixel value
	deps, err := cfg.Validate("", camera.API.SubtypeName)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, deps, test.ShouldBeNil)

	// error with a zero pixel value
	attrCfg.Width = 0
	cfg.ConvertedAttributes = attrCfg
	deps, err = cfg.Validate("", camera.API.SubtypeName)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, deps, test.ShouldBeNil)

	// error with a negative pixel value
	attrCfg.Width = -20
	cfg.ConvertedAttributes = attrCfg
	deps, err = cfg.Validate("", camera.API.SubtypeName)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, deps, test.ShouldBeNil)

	attrCfg.Width = 10
	cfg.ConvertedAttributes = attrCfg
	deps, err = cfg.Validate("", camera.API.SubtypeName)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, deps, test.ShouldBeNil)

	logger := logging.NewTestLogger(t)
	camera, err := NewCamera(context.Background(), nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, camera, test.ShouldNotBeNil)

	test.That(t, camera.Close(context.Background()), test.ShouldBeNil)
}

func TestRTPPassthrough(t *testing.T) {
	logger := logging.NewTestLogger(t)

	t.Run("when rtp_passthrough is enabled", func(t *testing.T) {
		cfg := resource.Config{
			Name:                "test1",
			API:                 camera.API,
			Model:               Model,
			ConvertedAttributes: &Config{RTPPassthrough: true},
		}

		// passes validations
		_, err := cfg.Validate("", camera.API.SubtypeName)
		test.That(t, err, test.ShouldBeNil)

		camera, err := NewCamera(context.Background(), nil, cfg, logger)
		test.That(t, err, test.ShouldBeNil)

		stream, err := camera.Stream(context.Background())
		test.That(t, err, test.ShouldBeNil)
		img, _, err := stream.Next(context.Background())
		test.That(t, err, test.ShouldBeNil)
		// GetImage returns the world jpeg
		test.That(t, img.Bounds(), test.ShouldResemble, image.Rectangle{Max: image.Point{X: 480, Y: 270}})
		test.That(t, camera, test.ShouldNotBeNil)

		// implements rtppassthrough.Source
		cam, ok := camera.(rtppassthrough.Source)
		test.That(t, ok, test.ShouldBeTrue)
		var called atomic.Bool
		pktChan := make(chan []*rtp.Packet)
		// SubscribeRTP succeeds
		sub, err := cam.SubscribeRTP(context.Background(), 512, func(pkts []*rtp.Packet) {
			if called.Load() {
				return
			}
			called.Store(true)
			pktChan <- pkts
		})
		test.That(t, err, test.ShouldBeNil)
		pkts := <-pktChan
		test.That(t, len(pkts), test.ShouldEqual, 4)

		// Unsubscribe fails when provided an ID for which there is no subscription
		test.That(t, cam.Unsubscribe(context.Background(), uuid.New()), test.ShouldBeError, errors.New("id not found"))

		test.That(t, sub.Terminated.Err(), test.ShouldBeNil)
		// Unsubscribe succeeds when provided an ID for which there is a subscription
		test.That(t, cam.Unsubscribe(context.Background(), sub.ID), test.ShouldBeNil)
		// Unsubscribe cancels the subscription
		test.That(t, sub.Terminated.Err(), test.ShouldBeError, context.Canceled)

		// subscriptions are cleaned up after Close is called
		sub2, err := cam.SubscribeRTP(context.Background(), 512, func(pkts []*rtp.Packet) {})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sub2.Terminated.Err(), test.ShouldBeNil)
		test.That(t, camera.Close(context.Background()), test.ShouldBeNil)
		test.That(t, sub2.Terminated.Err(), test.ShouldBeError, context.Canceled)
	})

	t.Run("when rtp_passthrough is not enabled", func(t *testing.T) {
		cfg := resource.Config{
			Name:                "test1",
			API:                 camera.API,
			Model:               Model,
			ConvertedAttributes: &Config{},
		}
		camera, err := NewCamera(context.Background(), nil, cfg, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, camera, test.ShouldNotBeNil)

		cam, ok := camera.(rtppassthrough.Source)
		test.That(t, ok, test.ShouldBeTrue)

		// SubscribeRTP returns rtppassthrough.NilSubscription, ErrRTPPassthroughNotEnabled
		sub, err := cam.SubscribeRTP(context.Background(), 512, func(pkts []*rtp.Packet) {})
		test.That(t, err, test.ShouldBeError, ErrRTPPassthroughNotEnabled)
		test.That(t, sub, test.ShouldResemble, rtppassthrough.NilSubscription)

		// Unsubscribe returns ErrRTPPassthroughNotEnabled
		test.That(t, cam.Unsubscribe(context.Background(), uuid.New()), test.ShouldBeError, ErrRTPPassthroughNotEnabled)
		test.That(t, camera.Close(context.Background()), test.ShouldBeNil)
	})
}
