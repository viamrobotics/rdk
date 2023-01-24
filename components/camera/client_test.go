package camera_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"net"
	"sync"
	"testing"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	componentpb "go.viam.com/api/component/camera/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectCamera := &inject.Camera{}
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))

	var imgBuf bytes.Buffer
	test.That(t, png.Encode(&imgBuf, img), test.ShouldBeNil)
	imgPng, err := png.Decode(bytes.NewReader(imgBuf.Bytes()))
	test.That(t, err, test.ShouldBeNil)

	pcA := pointcloud.New()
	err = pcA.Set(pointcloud.NewVector(5, 5, 5), nil)
	test.That(t, err, test.ShouldBeNil)

	var projA transform.Projector
	intrinsics := &transform.PinholeCameraIntrinsics{ // not the real camera parameters -- fake for test
		Width:  1280,
		Height: 720,
		Fx:     200,
		Fy:     200,
		Ppx:    100,
		Ppy:    100,
	}
	projA = intrinsics

	var imageReleased bool
	var imageReleasedMu sync.Mutex
	// color camera
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pcA, nil
	}
	injectCamera.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{
			SupportsPCD:     true,
			IntrinsicParams: intrinsics,
		}, nil
	}
	injectCamera.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
		return projA, nil
	}
	injectCamera.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
		return gostream.NewEmbeddedVideoStreamFromReader(gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
			imageReleasedMu.Lock()
			imageReleased = true
			imageReleasedMu.Unlock()
			return imgPng, func() {}, nil
		})), nil
	}
	// depth camera
	injectCameraDepth := &inject.Camera{}
	depthImg := rimage.NewEmptyDepthMap(10, 20)
	depthImg.Set(0, 0, rimage.Depth(40))
	depthImg.Set(0, 1, rimage.Depth(1))
	depthImg.Set(5, 6, rimage.Depth(190))
	depthImg.Set(9, 12, rimage.Depth(3000))
	depthImg.Set(5, 9, rimage.MaxDepth-rimage.Depth(1))
	injectCameraDepth.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pcA, nil
	}
	injectCameraDepth.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{
			SupportsPCD:     true,
			IntrinsicParams: intrinsics,
		}, nil
	}
	injectCameraDepth.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
		return projA, nil
	}
	injectCameraDepth.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
		return gostream.NewEmbeddedVideoStreamFromReader(gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
			imageReleasedMu.Lock()
			imageReleased = true
			imageReleasedMu.Unlock()
			return depthImg, func() {}, nil
		})), nil
	}
	// bad camera
	injectCamera2 := &inject.Camera{}
	injectCamera2.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return nil, errors.New("can't generate next point cloud")
	}
	injectCamera2.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, errors.New("can't get camera properties")
	}
	injectCamera2.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
		return nil, errors.New("can't get camera properties")
	}
	injectCamera2.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
		return nil, errors.New("can't generate stream")
	}

	resources := map[resource.Name]interface{}{
		camera.Named(testCameraName):  injectCamera,
		camera.Named(failCameraName):  injectCamera2,
		camera.Named(depthCameraName): injectCameraDepth,
	}
	cameraSvc, err := subtype.New(resources)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(camera.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, cameraSvc)

	injectCamera.DoFunc = generic.EchoFunc
	generic.RegisterService(rpcServer, cameraSvc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("camera client 1", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		camera1Client := camera.NewClientFromConn(context.Background(), conn, testCameraName, logger)
		frame, _, err := camera.ReadImage(context.Background(), camera1Client)
		test.That(t, err, test.ShouldBeNil)
		compVal, _, err := rimage.CompareImages(img, frame)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, compVal, test.ShouldEqual, 0) // exact copy, no color conversion
		imageReleasedMu.Lock()
		test.That(t, imageReleased, test.ShouldBeTrue)
		imageReleasedMu.Unlock()

		pcB, err := camera1Client.NextPointCloud(context.Background())
		test.That(t, err, test.ShouldBeNil)
		_, got := pcB.At(5, 5, 5)
		test.That(t, got, test.ShouldBeTrue)

		projB, err := camera1Client.Projector(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, projB, test.ShouldNotBeNil)

		propsB, err := camera1Client.Properties(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, propsB.SupportsPCD, test.ShouldBeTrue)
		test.That(t, propsB.IntrinsicParams, test.ShouldResemble, intrinsics)

		// Do
		resp, err := camera1Client.DoCommand(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		test.That(t, utils.TryClose(context.Background(), camera1Client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	t.Run("camera client depth", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, depthCameraName, logger)
		cameraDepthClient, ok := client.(camera.Camera)
		test.That(t, ok, test.ShouldBeTrue)

		ctx := gostream.WithMIMETypeHint(
			context.Background(), rutils.WithLazyMIMEType(rutils.MimeTypePNG))
		frame, _, err := camera.ReadImage(ctx, cameraDepthClient)
		test.That(t, err, test.ShouldBeNil)
		dm, err := rimage.ConvertImageToDepthMap(context.Background(), frame)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dm, test.ShouldResemble, depthImg)
		imageReleasedMu.Lock()
		test.That(t, imageReleased, test.ShouldBeTrue)
		imageReleasedMu.Unlock()

		test.That(t, utils.TryClose(context.Background(), cameraDepthClient), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("camera client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, failCameraName, logger)
		camera2Client, ok := client.(camera.Camera)
		test.That(t, ok, test.ShouldBeTrue)

		_, _, err = camera.ReadImage(context.Background(), camera2Client)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't generate stream")

		_, err = camera2Client.NextPointCloud(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't generate next point cloud")

		_, err = camera2Client.Projector(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get camera properties")

		_, err = camera2Client.Properties(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get camera properties")

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientProperties(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	server, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectCamera := &inject.Camera{}
	resources := map[resource.Name]interface{}{camera.Named(testCameraName): injectCamera}
	svc, err := subtype.New(resources)
	test.That(t, err, test.ShouldBeNil)

	rSubType := registry.ResourceSubtypeLookup(camera.Subtype)
	test.That(t, rSubType.RegisterRPCService(context.Background(), server, svc), test.ShouldBeNil)
	test.That(t, generic.RegisterService(server, svc), test.ShouldBeNil)

	go test.That(t, server.Serve(listener), test.ShouldBeNil)
	defer func() { test.That(t, server.Stop(), test.ShouldBeNil) }()

	fakeIntrinsics := &transform.PinholeCameraIntrinsics{
		Width:  1,
		Height: 1,
		Fx:     1,
		Fy:     1,
		Ppx:    1,
		Ppy:    1,
	}
	fakeDistortion := &transform.BrownConrady{
		RadialK1:     1.0,
		RadialK2:     1.0,
		RadialK3:     1.0,
		TangentialP1: 1.0,
		TangentialP2: 1.0,
	}

	testCases := []struct {
		name  string
		props camera.Properties
	}{
		{
			name: "non-nil properties",
			props: camera.Properties{
				SupportsPCD:      true,
				ImageType:        camera.UnspecifiedStream,
				IntrinsicParams:  fakeIntrinsics,
				DistortionParams: fakeDistortion,
			},
		}, {
			name: "nil intrinsic params",
			props: camera.Properties{
				SupportsPCD:      true,
				ImageType:        camera.UnspecifiedStream,
				IntrinsicParams:  nil,
				DistortionParams: fakeDistortion,
			},
		}, {
			name: "nil distortion parameters",
			props: camera.Properties{
				SupportsPCD:      true,
				ImageType:        camera.UnspecifiedStream,
				IntrinsicParams:  fakeIntrinsics,
				DistortionParams: nil,
			},
		}, {
			name:  "empty properties",
			props: camera.Properties{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			injectCamera.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
				return testCase.props, nil
			}

			conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
			test.That(t, err, test.ShouldBeNil)

			client := camera.NewClientFromConn(context.Background(), conn, testCameraName, logger)
			actualProps, err := client.Properties(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, actualProps, test.ShouldResemble, testCase.props)

			test.That(t, conn.Close(), test.ShouldBeNil)
		})
	}
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectCamera := &inject.Camera{}

	cameraSvc, err := subtype.New(map[resource.Name]interface{}{camera.Named(testCameraName): injectCamera})
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterCameraServiceServer(gServer, camera.NewServer(cameraSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	conn1, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client1 := camera.NewClientFromConn(ctx, conn1, testCameraName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	conn2, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client2 := camera.NewClientFromConn(ctx, conn2, testCameraName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
}

func TestClientLazyImage(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectCamera := &inject.Camera{}
	img := image.NewNRGBA64(image.Rect(0, 0, 4, 8))

	var imgBuf bytes.Buffer
	test.That(t, png.Encode(&imgBuf, img), test.ShouldBeNil)
	imgPng, err := png.Decode(bytes.NewReader(imgBuf.Bytes()))
	test.That(t, err, test.ShouldBeNil)

	injectCamera.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
		return gostream.NewEmbeddedVideoStreamFromReader(gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
			mimeType, _ := rutils.CheckLazyMIMEType(gostream.MIMETypeHint(ctx, rutils.MimeTypeRawRGBA))
			switch mimeType {
			case rutils.MimeTypePNG:
				return imgPng, func() {}, nil
			default:
				return nil, nil, errors.New("invalid mime type")
			}
		})), nil
	}

	resources := map[resource.Name]interface{}{
		camera.Named(testCameraName): injectCamera,
	}
	cameraSvc, err := subtype.New(resources)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(camera.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, cameraSvc)

	generic.RegisterService(rpcServer, cameraSvc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	camera1Client := camera.NewClientFromConn(context.Background(), conn, testCameraName, logger)

	ctx := gostream.WithMIMETypeHint(context.Background(), rutils.MimeTypePNG)
	frame, _, err := camera.ReadImage(ctx, camera1Client)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame, test.ShouldNotHaveSameTypeAs, &rimage.LazyEncodedImage{})
	compVal, _, err := rimage.CompareImages(img, frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 0) // exact copy, no color conversion

	ctx = gostream.WithMIMETypeHint(context.Background(), rutils.WithLazyMIMEType(rutils.MimeTypePNG))
	frame, _, err = camera.ReadImage(ctx, camera1Client)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame, test.ShouldHaveSameTypeAs, &rimage.LazyEncodedImage{})
	frameLazy := frame.(*rimage.LazyEncodedImage)
	test.That(t, frameLazy.RawData(), test.ShouldResemble, imgBuf.Bytes())

	test.That(t, frameLazy.MIMEType(), test.ShouldEqual, rutils.MimeTypePNG)
	compVal, _, err = rimage.CompareImages(img, frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 0) // exact copy, no color conversion

	test.That(t, conn.Close(), test.ShouldBeNil)
}
