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
	"go.viam.com/rdk/utils"
)

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

		cam, err := NewCamera(context.Background(), nil, cfg, logger)
		test.That(t, err, test.ShouldBeNil)

		img, err := camera.DecodeImageFromCamera(context.Background(), utils.MimeTypeRawRGBA, nil, cam)
		test.That(t, err, test.ShouldBeNil)
		// GetImage returns the world jpeg
		test.That(t, img.Bounds(), test.ShouldResemble, image.Rectangle{Max: image.Point{X: 480, Y: 270}})
		test.That(t, cam, test.ShouldNotBeNil)

		// implements rtppassthrough.Source
		passthroughCam, ok := cam.(rtppassthrough.Source)
		test.That(t, ok, test.ShouldBeTrue)
		var called atomic.Bool
		pktChan := make(chan []*rtp.Packet)
		// SubscribeRTP succeeds
		sub, err := passthroughCam.SubscribeRTP(context.Background(), 512, func(pkts []*rtp.Packet) {
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
		test.That(t, passthroughCam.Unsubscribe(context.Background(), uuid.New()), test.ShouldBeError, errors.New("id not found"))

		test.That(t, sub.Terminated.Err(), test.ShouldBeNil)
		// Unsubscribe succeeds when provided an ID for which there is a subscription
		test.That(t, passthroughCam.Unsubscribe(context.Background(), sub.ID), test.ShouldBeNil)
		// Unsubscribe cancels the subscription
		test.That(t, sub.Terminated.Err(), test.ShouldBeError, context.Canceled)

		// subscriptions are cleaned up after Close is called
		sub2, err := passthroughCam.SubscribeRTP(context.Background(), 512, func(pkts []*rtp.Packet) {})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sub2.Terminated.Err(), test.ShouldBeNil)
		test.That(t, cam.Close(context.Background()), test.ShouldBeNil)
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

func TestPropertiesToggle(t *testing.T) {
	// Test fake camera without setting model
	// IntrinsicParams and DistortionParams Properties should be nil
	ctx := context.Background()
	cfg1 := resource.Config{
		Name:                "test1",
		API:                 camera.API,
		Model:               Model,
		ConvertedAttributes: &Config{},
	}
	cam1, err := NewCamera(ctx, nil, cfg1, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cam1, test.ShouldNotBeNil)
	propsRes1, err := cam1.Properties(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, propsRes1, test.ShouldNotBeNil)
	test.That(t, propsRes1.IntrinsicParams, test.ShouldBeNil)
	test.That(t, propsRes1.DistortionParams, test.ShouldBeNil)
	test.That(t, cam1.Close(ctx), test.ShouldBeNil)

	// Test fake camera with model set to true
	// IntrinsicParams and DistortionParams Properties should be set
	cfg2 := resource.Config{
		Name:  "test2",
		API:   camera.API,
		Model: Model,
		ConvertedAttributes: &Config{
			Model: true,
		},
	}
	cam2, err := NewCamera(ctx, nil, cfg2, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cam2, test.ShouldNotBeNil)
	propsRes2, err := cam2.Properties(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, propsRes2, test.ShouldNotBeNil)
	test.That(t, propsRes2.IntrinsicParams, test.ShouldNotBeNil)
	test.That(t, propsRes2.DistortionParams, test.ShouldNotBeNil)
	test.That(t, cam2.Close(ctx), test.ShouldBeNil)
}
