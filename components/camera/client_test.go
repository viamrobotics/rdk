package camera_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/viamrobotics/gostream"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"go.viam.com/rdk/components/camera"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/utils/contextutils"
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
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
	injectCamera.ImagesFunc = func(ctx context.Context) ([]camera.NamedImage, resource.ResponseMetadata, error) {
		images := []camera.NamedImage{}
		// one color image
		color := rimage.NewImage(40, 50)
		images = append(images, camera.NamedImage{color, "color"})
		// one depth image
		depth := rimage.NewEmptyDepthMap(10, 20)
		images = append(images, camera.NamedImage{depth, "depth"})
		// a timestamp of 12345
		ts := time.UnixMilli(12345)
		return images, resource.ResponseMetadata{ts}, nil
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
		return nil, errGeneratePointCloudFailed
	}
	injectCamera2.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, errPropertiesFailed
	}
	injectCamera2.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
		return nil, errCameraProjectorFailed
	}
	injectCamera2.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
		return nil, errStreamFailed
	}

	resources := map[resource.Name]camera.Camera{
		camera.Named(testCameraName):  injectCamera,
		camera.Named(failCameraName):  injectCamera2,
		camera.Named(depthCameraName): injectCameraDepth,
	}
	cameraSvc, err := resource.NewAPIResourceCollection(camera.API, resources)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[camera.Camera](camera.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, cameraSvc), test.ShouldBeNil)

	injectCamera.DoFunc = testutils.EchoFunc

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})

	t.Run("camera client 1", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		camera1Client, err := camera.NewClientFromConn(context.Background(), conn, "", camera.Named(testCameraName), logger)
		test.That(t, err, test.ShouldBeNil)
		ctx := gostream.WithMIMETypeHint(context.Background(), rutils.MimeTypeRawRGBA)
		frame, _, err := camera.ReadImage(ctx, camera1Client)
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

		images, meta, err := camera1Client.Images(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, meta.CapturedAt, test.ShouldEqual, time.UnixMilli(12345))
		test.That(t, len(images), test.ShouldEqual, 2)
		test.That(t, images[0].SourceName, test.ShouldEqual, "color")
		test.That(t, images[0].Image.Bounds().Dx(), test.ShouldEqual, 40)
		test.That(t, images[0].Image.Bounds().Dy(), test.ShouldEqual, 50)
		test.That(t, images[0].Image, test.ShouldHaveSameTypeAs, &rimage.LazyEncodedImage{})
		test.That(t, images[0].Image.ColorModel(), test.ShouldHaveSameTypeAs, color.RGBAModel)
		test.That(t, images[1].SourceName, test.ShouldEqual, "depth")
		test.That(t, images[1].Image.Bounds().Dx(), test.ShouldEqual, 10)
		test.That(t, images[1].Image.Bounds().Dy(), test.ShouldEqual, 20)
		test.That(t, images[1].Image, test.ShouldHaveSameTypeAs, &rimage.LazyEncodedImage{})
		test.That(t, images[1].Image.ColorModel(), test.ShouldHaveSameTypeAs, color.Gray16Model)

		// Do
		resp, err := camera1Client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		test.That(t, camera1Client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	t.Run("camera client depth", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := resourceAPI.RPCClient(context.Background(), conn, "", camera.Named(depthCameraName), logger)
		test.That(t, err, test.ShouldBeNil)

		ctx := gostream.WithMIMETypeHint(
			context.Background(), rutils.WithLazyMIMEType(rutils.MimeTypePNG))
		frame, _, err := camera.ReadImage(ctx, client)
		test.That(t, err, test.ShouldBeNil)
		dm, err := rimage.ConvertImageToDepthMap(context.Background(), frame)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dm, test.ShouldResemble, depthImg)
		imageReleasedMu.Lock()
		test.That(t, imageReleased, test.ShouldBeTrue)
		imageReleasedMu.Unlock()

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("camera client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client2, err := resourceAPI.RPCClient(context.Background(), conn, "", camera.Named(failCameraName), logger)
		test.That(t, err, test.ShouldBeNil)

		_, _, err = camera.ReadImage(context.Background(), client2)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errStreamFailed.Error())

		_, err = client2.NextPointCloud(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGeneratePointCloudFailed.Error())

		_, err = client2.Projector(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCameraProjectorFailed.Error())

		_, err = client2.Properties(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errPropertiesFailed.Error())

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientProperties(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	server, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectCamera := &inject.Camera{}
	resources := map[resource.Name]camera.Camera{camera.Named(testCameraName): injectCamera}
	svc, err := resource.NewAPIResourceCollection(camera.API, resources)
	test.That(t, err, test.ShouldBeNil)

	rSubType, ok, err := resource.LookupAPIRegistration[camera.Camera](camera.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, rSubType.RegisterRPCService(context.Background(), server, svc), test.ShouldBeNil)

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

			client, err := camera.NewClientFromConn(context.Background(), conn, "", camera.Named(testCameraName), logger)
			test.That(t, err, test.ShouldBeNil)
			actualProps, err := client.Properties(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, actualProps, test.ShouldResemble, testCase.props)

			test.That(t, conn.Close(), test.ShouldBeNil)
		})
	}
}

func TestClientLazyImage(t *testing.T) {
	logger := logging.NewTestLogger(t)
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
				return nil, nil, errInvalidMimeType
			}
		})), nil
	}

	resources := map[resource.Name]camera.Camera{
		camera.Named(testCameraName): injectCamera,
	}
	cameraSvc, err := resource.NewAPIResourceCollection(camera.API, resources)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[camera.Camera](camera.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, cameraSvc), test.ShouldBeNil)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	camera1Client, err := camera.NewClientFromConn(context.Background(), conn, "", camera.Named(testCameraName), logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := gostream.WithMIMETypeHint(context.Background(), rutils.MimeTypePNG)
	frame, _, err := camera.ReadImage(ctx, camera1Client)
	test.That(t, err, test.ShouldBeNil)
	// Should always lazily decode
	test.That(t, frame, test.ShouldHaveSameTypeAs, &rimage.LazyEncodedImage{})
	frameLazy := frame.(*rimage.LazyEncodedImage)
	test.That(t, frameLazy.RawData(), test.ShouldResemble, imgBuf.Bytes())

	ctx = gostream.WithMIMETypeHint(context.Background(), rutils.WithLazyMIMEType(rutils.MimeTypePNG))
	frame, _, err = camera.ReadImage(ctx, camera1Client)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame, test.ShouldHaveSameTypeAs, &rimage.LazyEncodedImage{})
	frameLazy = frame.(*rimage.LazyEncodedImage)
	test.That(t, frameLazy.RawData(), test.ShouldResemble, imgBuf.Bytes())

	test.That(t, frameLazy.MIMEType(), test.ShouldEqual, rutils.MimeTypePNG)
	compVal, _, err := rimage.CompareImages(img, frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 0) // exact copy, no color conversion

	test.That(t, conn.Close(), test.ShouldBeNil)
}

func TestClientWithInterceptor(t *testing.T) {
	// Set up gRPC server
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	// Set up camera that adds timestamps into the gRPC response header.
	injectCamera := &inject.Camera{}

	pcA := pointcloud.New()
	err = pcA.Set(pointcloud.NewVector(5, 5, 5), nil)
	test.That(t, err, test.ShouldBeNil)

	k, v := "hello", "world"
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		var grpcMetadata metadata.MD = make(map[string][]string)
		grpcMetadata.Set(k, v)
		grpc.SendHeader(ctx, grpcMetadata)
		return pcA, nil
	}

	// Register CameraService API in our gRPC server.
	resources := map[resource.Name]camera.Camera{
		camera.Named(testCameraName): injectCamera,
	}
	cameraSvc, err := resource.NewAPIResourceCollection(camera.API, resources)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[camera.Camera](camera.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, cameraSvc), test.ShouldBeNil)

	// Start serving requests.
	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// Set up gRPC client with context with metadata interceptor.
	conn, err := viamgrpc.Dial(
		context.Background(),
		listener1.Addr().String(),
		logger,
		rpc.WithUnaryClientInterceptor(contextutils.ContextWithMetadataUnaryClientInterceptor),
	)
	test.That(t, err, test.ShouldBeNil)
	camera1Client, err := camera.NewClientFromConn(context.Background(), conn, "", camera.Named(testCameraName), logger)
	test.That(t, err, test.ShouldBeNil)

	// Construct a ContextWithMetadata to pass into NextPointCloud and check that the
	// interceptor correctly injected the metadata from the gRPC response header into the
	// context.
	ctx, md := contextutils.ContextWithMetadata(context.Background())
	pcB, err := camera1Client.NextPointCloud(ctx)
	test.That(t, err, test.ShouldBeNil)
	_, got := pcB.At(5, 5, 5)
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, md[k][0], test.ShouldEqual, v)

	test.That(t, conn.Close(), test.ShouldBeNil)
}
