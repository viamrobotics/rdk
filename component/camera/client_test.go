package camera_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"net"
	"testing"

	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/pointcloud"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"google.golang.org/grpc"

	viamgrpc "go.viam.com/rdk/grpc"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()

	camera1 := "camera1"
	injectCamera := &inject.Camera{}
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var imgBuf bytes.Buffer
	test.That(t, jpeg.Encode(&imgBuf, img, nil), test.ShouldBeNil)

	pcA := pointcloud.New()
	err = pcA.Set(pointcloud.NewBasicPoint(5, 5, 5))
	test.That(t, err, test.ShouldBeNil)

	var imageReleased bool
	injectCamera.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return img, func() { imageReleased = true }, nil
	}
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pcA, nil
	}

	camera2 := "camera2"
	injectCamera2 := &inject.Camera{}
	injectCamera2.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return nil, nil, errors.New("can't generate next frame")
	}
	injectCamera2.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return nil, errors.New("can't generate next point cloud")
	}

	resources := map[resource.Name]interface{}{
		camera.Named(camera1): injectCamera,
		camera.Named(camera2): injectCamera2,
	}
	cameraSvc, err := subtype.New(resources)
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterCameraServiceServer(gServer1, camera.NewServer(cameraSvc))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = camera.NewClient(cancelCtx, camera1, listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("camera client 1", func(t *testing.T) {
		camera1Client, err := camera.NewClient(context.Background(), camera1, listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)
		frame, _, err := camera1Client.Next(context.Background())
		test.That(t, err, test.ShouldBeNil)
		compVal, _, err := rimage.CompareImages(img, frame)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, compVal, test.ShouldEqual, 0) // exact copy, no color conversion
		test.That(t, imageReleased, test.ShouldBeTrue)

		pcB, err := camera1Client.NextPointCloud(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pcB.At(5, 5, 5), test.ShouldNotBeNil)

		test.That(t, utils.TryClose(camera1Client), test.ShouldBeNil)
	})

	t.Run("camera client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)
		camera2Client := camera.NewClientFromConn(conn, camera2, logger)
		test.That(t, err, test.ShouldBeNil)

		_, _, err = camera2Client.Next(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't generate next frame")

		_, err = camera2Client.NextPointCloud(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't generate next point cloud")

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectCamera := &inject.Camera{}
	camera1 := "camera1"

	cameraSvc, err := subtype.New((map[resource.Name]interface{}{camera.Named(camera1): injectCamera}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterCameraServiceServer(gServer, camera.NewServer(cameraSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := camera.NewClient(ctx, camera1, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	client2, err := camera.NewClient(ctx, camera1, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.DialCalled, test.ShouldEqual, 2)

	err = utils.TryClose(client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(client2)
	test.That(t, err, test.ShouldBeNil)
}
