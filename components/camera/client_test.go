// nolint
package camera_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pion/rtp"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/fake"
	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec/opus"
	"go.viam.com/rdk/gostream/codec/x264"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/testutils/robottestutils"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/utils/contextutils"
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
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
		return images, resource.ResponseMetadata{CapturedAt: ts}, nil
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
	t.Run("camera client extra", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		camClient, err := camera.NewClientFromConn(context.Background(), conn, "", camera.Named(testCameraName), logger)
		test.That(t, err, test.ShouldBeNil)

		injectCamera.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
			extra, ok := camera.FromContext(ctx)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, extra, test.ShouldBeEmpty)
			return nil, errStreamFailed
		}

		ctx := context.Background()
		_, _, err = camera.ReadImage(ctx, camClient)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errStreamFailed.Error())

		injectCamera.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
			extra, ok := camera.FromContext(ctx)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, len(extra), test.ShouldEqual, 1)
			test.That(t, extra["hello"], test.ShouldEqual, "world")
			return nil, errStreamFailed
		}

		// one kvp created with camera.Extra
		ext := camera.Extra{"hello": "world"}
		ctx = camera.NewContext(ctx, ext)
		_, _, err = camera.ReadImage(ctx, camClient)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errStreamFailed.Error())

		injectCamera.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
			extra, ok := camera.FromContext(ctx)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, len(extra), test.ShouldEqual, 1)
			test.That(t, extra[data.FromDMString], test.ShouldBeTrue)

			return nil, errStreamFailed
		}

		// one kvp created with data.FromDMContextKey
		ctx = context.WithValue(context.Background(), data.FromDMContextKey{}, true)
		_, _, err = camera.ReadImage(ctx, camClient)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errStreamFailed.Error())

		injectCamera.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
			extra, ok := camera.FromContext(ctx)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, len(extra), test.ShouldEqual, 2)
			test.That(t, extra["hello"], test.ShouldEqual, "world")
			test.That(t, extra[data.FromDMString], test.ShouldBeTrue)
			return nil, errStreamFailed
		}

		// merge values from data and camera
		ext = camera.Extra{"hello": "world"}
		ctx = camera.NewContext(ctx, ext)
		_, _, err = camera.ReadImage(ctx, camClient)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errStreamFailed.Error())

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientProperties(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	server, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
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
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
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
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
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

func TestClientStreamAfterClose(t *testing.T) {
	// Set up gRPC server
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	// Set up camera that can stream images
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	injectCamera := &inject.Camera{}
	injectCamera.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}
	injectCamera.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
		return gostream.NewEmbeddedVideoStreamFromReader(gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
			return img, func() {}, nil
		})), nil
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
	go rpcServer.Serve(listener)
	defer rpcServer.Stop()

	// Make client connection
	conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client, err := camera.NewClientFromConn(context.Background(), conn, "", camera.Named(testCameraName), logger)
	test.That(t, err, test.ShouldBeNil)

	// Get a stream
	stream, err := client.Stream(context.Background())
	test.That(t, stream, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	// Read from stream
	media, _, err := stream.Next(context.Background())
	test.That(t, media, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	// Close client and read from stream
	test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	media, _, err = stream.Next(context.Background())
	test.That(t, media, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "context canceled")

	// Get a new stream
	stream, err = client.Stream(context.Background())
	test.That(t, stream, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	// Read from the new stream
	media, _, err = stream.Next(context.Background())
	test.That(t, media, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	// Close client and connection
	test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
}

// See modmanager_test.go for the happy path (aka, when the
// client has a webrtc connection).
func TestRTPPassthroughWithoutWebRTC(t *testing.T) {
	logger := logging.NewTestLogger(t)
	camName := "rtp_passthrough_camera"
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectCamera := &inject.Camera{}
	resources := map[resource.Name]camera.Camera{
		camera.Named(camName): injectCamera,
	}
	cameraSvc, err := resource.NewAPIResourceCollection(camera.API, resources)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[camera.Camera](camera.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, cameraSvc), test.ShouldBeNil)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})

	t.Run("rtp_passthrough client fails without webrtc connection", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		camera1Client, err := camera.NewClientFromConn(context.Background(), conn, "", camera.Named(camName), logger)
		test.That(t, err, test.ShouldBeNil)
		rtpPassthroughClient, ok := camera1Client.(rtppassthrough.Source)
		test.That(t, ok, test.ShouldBeTrue)
		sub, err := rtpPassthroughClient.SubscribeRTP(context.Background(), 512, func(pkts []*rtp.Packet) {
			t.Log("should not be called")
			t.FailNow()
		})
		test.That(t, err, test.ShouldBeError, camera.ErrNoPeerConnection)
		test.That(t, sub, test.ShouldResemble, rtppassthrough.NilSubscription)
		err = rtpPassthroughClient.Unsubscribe(context.Background(), rtppassthrough.NilSubscription.ID)
		test.That(t, err, test.ShouldBeError, camera.ErrNoPeerConnection)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func setupRealRobot(
	t *testing.T,
	robotConfig *config.Config,
	logger logging.Logger,
) (context.Context, robot.LocalRobot, string, web.Service) {
	t.Helper()

	ctx := context.Background()
	robot, err := robotimpl.RobotFromConfig(ctx, robotConfig, logger)
	test.That(t, err, test.ShouldBeNil)

	// We initialize with a stream config such that the stream server is capable of creating video stream and
	// audio stream data.
	webSvc := web.New(robot, logger, web.WithStreamConfig(gostream.StreamConfig{
		AudioEncoderFactory: opus.NewEncoderFactory(),
		VideoEncoderFactory: x264.NewEncoderFactory(),
	}))
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = webSvc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	return ctx, robot, addr, webSvc
}

func setupRealRobot2(
	t *testing.T,
	robotConfig *config.Config,
	logger logging.Logger,
) (context.Context, robot.LocalRobot, string, web.Service, weboptions.Options) {
	t.Helper()

	ctx := context.Background()
	robot, err := robotimpl.RobotFromConfig(ctx, robotConfig, logger)
	test.That(t, err, test.ShouldBeNil)

	// We initialize with a stream config such that the stream server is capable of creating video stream and
	// audio stream data.
	webSvc := web.New(robot, logger, web.WithStreamConfig(gostream.StreamConfig{
		AudioEncoderFactory: opus.NewEncoderFactory(),
		VideoEncoderFactory: x264.NewEncoderFactory(),
	}))
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = webSvc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	return ctx, robot, addr, webSvc, options
}

func setupRealRobotWithAddr(
	t *testing.T,
	robotConfig *config.Config,
	logger logging.Logger,
	options weboptions.Options,
	addr string,
) (context.Context, robot.LocalRobot, web.Service) {
	t.Helper()

	ctx := context.Background()
	robot, err := robotimpl.RobotFromConfig(ctx, robotConfig, logger)
	test.That(t, err, test.ShouldBeNil)

	// We initialize with a stream config such that the stream server is capable of creating video stream and
	// audio stream data.
	webSvc := web.New(robot, logger, web.WithStreamConfig(gostream.StreamConfig{
		AudioEncoderFactory: opus.NewEncoderFactory(),
		VideoEncoderFactory: x264.NewEncoderFactory(),
	}))
	err = webSvc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	return ctx, robot, webSvc
}

var (
	Green   = "\033[32m"
	Red     = "\033[31m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Yellow  = "\033[33m"
	Reset   = "\033[0m"
)

// this helps make the test case much easier to read.
func greenLog(t *testing.T, msg string) {
	t.Log(Green + msg + Reset)
}

// this helps make the test case much easier to read.
func redLog(t *testing.T, msg string) {
	t.Log(Red + msg + Reset)
}

// Skipped due to
// https://viam.atlassian.net/browse/RSDK-7637
func TestMultiplexOverRemoteConnection(t *testing.T) {
	t.Skip()
	logger := logging.NewTestLogger(t).Sublogger(t.Name())

	remoteCfg := &config.Config{Components: []resource.Config{
		{
			Name:  "rtpPassthroughCamera",
			API:   resource.NewAPI("rdk", "component", "camera"),
			Model: resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{
				RTPPassthrough: true,
			},
		},
	}}

	// Create a robot with a single fake camera.
	remoteCtx, remoteRobot, addr, remoteWebSvc := setupRealRobot(t, remoteCfg, logger.Sublogger("remote"))
	defer remoteRobot.Close(remoteCtx)
	defer remoteWebSvc.Close(remoteCtx)

	mainCfg := &config.Config{Remotes: []config.Remote{
		{
			Name:     "remote",
			Address:  addr,
			Insecure: true,
		},
	}}
	mainCtx, mainRobot, _, mainWebSvc := setupRealRobot(t, mainCfg, logger.Sublogger("main"))
	greenLog(t, "robot setup")
	defer mainRobot.Close(mainCtx)
	defer mainWebSvc.Close(mainCtx)

	cameraClient, err := camera.FromRobot(mainRobot, "remote:rtpPassthroughCamera")
	test.That(t, err, test.ShouldBeNil)

	image, _, err := cameraClient.Images(mainCtx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, image, test.ShouldNotBeNil)
	greenLog(t, "got images")

	recvPktsCtx, recvPktsFn := context.WithCancel(context.Background())
	sub, err := cameraClient.(rtppassthrough.Source).SubscribeRTP(mainCtx, 2, func(pkts []*rtp.Packet) {
		recvPktsFn()
	})
	test.That(t, err, test.ShouldBeNil)
	<-recvPktsCtx.Done()
	greenLog(t, "got packets")

	err = cameraClient.(rtppassthrough.Source).Unsubscribe(mainCtx, sub.ID)
	test.That(t, err, test.ShouldBeNil)
	greenLog(t, "unsubscribe")
}

// Skipped due to
// https://viam.atlassian.net/browse/RSDK-7637
func TestMultiplexOverMultiHopRemoteConnection(t *testing.T) {
	logger := logging.NewTestLogger(t).Sublogger(t.Name())

	remoteCfg2 := &config.Config{
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{Sessions: config.SessionsConfig{HeartbeatWindow: time.Hour}}},
		Components: []resource.Config{
			{
				Name:  "rtpPassthroughCamera",
				API:   resource.NewAPI("rdk", "component", "camera"),
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					RTPPassthrough: true,
				},
			},
		},
	}

	r2Logger := logger.Sublogger("remote-2")
	r2Logger.SetLevel(logging.ERROR)
	// Create a robot with a single fake camera.
	remote2Ctx, remoteRobot2, addr2, remoteWebSvc2, webSvc2Options := setupRealRobot2(t, remoteCfg2, r2Logger)
	logger.Info("SETUP ROBOT. Addr2:", addr2, "Options:", webSvc2Options)
	defer remoteRobot2.Close(remote2Ctx)
	defer remoteWebSvc2.Close(remote2Ctx)

	remoteCfg1 := &config.Config{
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{Sessions: config.SessionsConfig{HeartbeatWindow: time.Hour}}},
		Remotes: []config.Remote{
			{
				Name:     "remote-2",
				Address:  addr2,
				Insecure: true,
			},
		},
	}

	r1Logger := logger.Sublogger("remote-1")
	remote1Ctx, remoteRobot1, addr1, remoteWebSvc1 := setupRealRobot(t, remoteCfg1, r1Logger)
	defer remoteRobot1.Close(remote1Ctx)
	defer remoteWebSvc1.Close(remote1Ctx)

	mainCfg := &config.Config{
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{Sessions: config.SessionsConfig{HeartbeatWindow: time.Hour}}},
		Remotes: []config.Remote{
			{
				Name:     "remote-1",
				Address:  addr1,
				Insecure: true,
			},
		},
	}

	mLogger := logger.Sublogger("main")
	// mLogger.SetLevel(logging.INFO)
	mainCtx, mainRobot, _, mainWebSvc := setupRealRobot(t, mainCfg, mLogger)
	defer mainRobot.Close(mainCtx)
	defer mainWebSvc.Close(mainCtx)

	cameraClient, err := camera.FromRobot(mainRobot, "remote-1:remote-2:rtpPassthroughCamera")
	test.That(t, err, test.ShouldBeNil)

	image, _, err := cameraClient.Images(mainCtx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, image, test.ShouldNotBeNil)
	greenLog(t, "got images")

	time.Sleep(time.Second)

	// recvPktsCtx, recvPktsFn := context.WithCancel(context.Background())
	// sub, err := cameraClient.(rtppassthrough.Source).SubscribeRTP(mainCtx, 512, func(pkts []*rtp.Packet) {
	//  	recvPktsFn()
	// })
	// test.That(t, err, test.ShouldBeNil)
	// <-recvPktsCtx.Done()
	// greenLog(t, "got packets")
	//
	// test.That(t, cameraClient.(rtppassthrough.Source).Unsubscribe(mainCtx, sub.ID), test.ShouldBeNil)

	greenLog(t, "Closing robot2")
	remoteWebSvc2.Close(context.Background())
	remoteRobot2.Close(context.Background())
	greenLog(t, "robot2 closed")

	mLogger.SetLevel(logging.INFO)
	for {
		_, err := camera.FromRobot(remoteRobot1, "remote-2:rtpPassthroughCamera")
		if err != nil && (strings.Contains(err.Error(), "resource not initialized") ||
			strings.Contains(err.Error(), "remote blipped")) {
			break
		}

		time.Sleep(time.Second)
	}

	greenLog(t, "Checking listResources Output")
	mainToRemote1ResourceName := resource.NewName(client.RemoteAPI, "remote-1")
	mainToRemote1Res, err := robot.ResourceFromRobot[resource.Resource](mainRobot, mainToRemote1ResourceName)
	logger.Infof("Main remote client: %T err: %v", mainToRemote1Res, err)
	logger.Info("Main refreshing remote1. Err:", mainToRemote1Res.(*client.RobotClient).Refresh(context.Background()))
	logger.Info("Main's resources", mainToRemote1Res.(*client.RobotClient).ResourceNames())

	remoteRobot2 = robotimpl.SetupLocalRobot(t, context.Background(), remoteCfg2, r2Logger)
	defer remoteRobot2.Close(context.Background())
	reopenedListener, err := net.Listen("tcp", addr2)
	test.That(t, err, test.ShouldBeNil)
	webSvc2Options.Network.Listener = reopenedListener
	test.That(t, remoteRobot2.StartWeb(context.Background(), webSvc2Options), test.ShouldBeNil)
	greenLog(t, "robot2 opened")
	for {
		time.Sleep(time.Second)

		_, err := camera.FromRobot(remoteRobot1, "remote-2:rtpPassthroughCamera")
		_, _, err = cameraClient.Images(mainCtx)
		if err == nil {
			break
		}
	}

	image, _, err = cameraClient.Images(mainCtx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, image, test.ShouldNotBeNil)
	greenLog(t, "got images")
}

// go test -race -v -run=TestWhyMustTimeoutOnReadRTP -timeout 10s
// TestWhyMustTimeoutOnReadRTP shows that if we don't timeout on ReadRTP (and also don't call RemoveStream) on close
// calling Close() on main's camera client blocks forever if there is a live SubscribeRTP subscription with a remote
// due to the fact that the TrackRemote.ReadRTP method blocking forever.
func TestWhyMustTimeoutOnReadRTP(t *testing.T) {
	logger := logging.NewTestLogger(t).Sublogger(t.Name())

	remoteCfg2 := &config.Config{
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{Sessions: config.SessionsConfig{HeartbeatWindow: time.Hour}}},
		Components: []resource.Config{
			{
				Name:  "rtpPassthroughCamera",
				API:   resource.NewAPI("rdk", "component", "camera"),
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					RTPPassthrough: true,
				},
			},
		},
	}
	// Create a robot with a single fake camera.
	remote2Ctx, remoteRobot2, addr2, remoteWebSvc2 := setupRealRobot(t, remoteCfg2, logger.Sublogger("remote-2"))
	defer remoteRobot2.Close(remote2Ctx)
	defer remoteWebSvc2.Close(remote2Ctx)

	remoteCfg1 := &config.Config{
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{Sessions: config.SessionsConfig{HeartbeatWindow: time.Hour}}},
		Remotes: []config.Remote{
			{
				Name:     "remote-2",
				Address:  addr2,
				Insecure: true,
			},
		},
	}

	remote1Ctx, remoteRobot1, addr1, remoteWebSvc1 := setupRealRobot(t, remoteCfg1, logger.Sublogger("remote-1"))
	defer remoteRobot1.Close(remote1Ctx)
	defer remoteWebSvc1.Close(remote1Ctx)

	mainCfg := &config.Config{
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{Sessions: config.SessionsConfig{HeartbeatWindow: time.Hour}}},
		Remotes: []config.Remote{
			{
				Name:     "remote-1",
				Address:  addr1,
				Insecure: true,
			},
		},
	}

	mainCtx, mainRobot, _, mainWebSvc := setupRealRobot(t, mainCfg, logger.Sublogger("main"))
	greenLog(t, "robot setup")
	defer mainRobot.Close(mainCtx)
	defer mainWebSvc.Close(mainCtx)

	cameraClient, err := camera.FromRobot(mainRobot, "remote-1:remote-2:rtpPassthroughCamera")
	test.That(t, err, test.ShouldBeNil)

	image, _, err := cameraClient.Images(mainCtx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, image, test.ShouldNotBeNil)
	greenLog(t, "got images")

	greenLog(t, fmt.Sprintf("calling SubscribeRTP on %T, %p", cameraClient, cameraClient))
	time.Sleep(time.Second)

	pktsChan := make(chan []*rtp.Packet)
	recvPktsCtx, recvPktsFn := context.WithCancel(context.Background())
	sub, err := cameraClient.(rtppassthrough.Source).SubscribeRTP(mainCtx, 512, func(pkts []*rtp.Packet) {
		// first packet
		recvPktsFn()
		// at some point packets are no longer published
		select {
		case pktsChan <- pkts:
		default:
		}
	})
	test.That(t, err, test.ShouldBeNil)
	<-recvPktsCtx.Done()
	greenLog(t, "got packets")

	greenLog(t, "calling close")
	test.That(t, remoteRobot2.Close(context.Background()), test.ShouldBeNil)
	test.That(t, remoteWebSvc2.Close(context.Background()), test.ShouldBeNil)
	greenLog(t, "close called")

	greenLog(t, "waiting for SubscribeRTP to stop receiving packets")

	timeoutCtx, timeoutFn := context.WithTimeout(context.Background(), time.Second)
	defer timeoutFn()

	var (
		pktTimeoutCtx context.Context
		pktTimeoutFn  context.CancelFunc
	)

	// Once we have not received packets for half a second we can assume that no more packets will be published
	// by the first instance of remote2
Loop:
	for {
		if pktTimeoutFn != nil {
			pktTimeoutFn()
		}
		pktTimeout := time.Millisecond * 500
		pktTimeoutCtx, pktTimeoutFn = context.WithTimeout(context.Background(), pktTimeout)
		select {
		case <-pktsChan:
			continue
		case <-pktTimeoutCtx.Done():
			greenLog(t, fmt.Sprintf("it has been at least %s since SubscribeRTP has received a packet", pktTimeout))
			pktTimeoutFn()
			// https://go.dev/ref/spec#Break_statements
			// The 'Loop' label is needed so that we break out of the loop
			// rather than out of the select statement
			break Loop
		case <-timeoutCtx.Done():
			t.Log("timed out waiting for SubscribeRTP packets to drain")
			t.FailNow()
		}
	}

	greenLog(t, "sleeping")
	time.Sleep(time.Second)

	// sub should still be alive
	test.That(t, sub.Terminated.Err(), test.ShouldBeNil)
}

// NOT WORKING as I don't know how to get a remote to come back online after calling Close
// go test -race -v -run=TestWhyMustCallUnsubscribe -timeout 10s
// TestWhyMustTimeoutOnReadRTP shows that if we don't call Unsubscribe on camera client Close (even if we are timing out in ReadRTP)
// when talking to a remote all subsequent AddStream calls to the remote will inevitably fail due to the previous track still being alive.
func TestWhyMustCallUnsubscribe(t *testing.T) {
	logger := logging.NewTestLogger(t).Sublogger(t.Name())

	remoteCfg2 := &config.Config{
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{Sessions: config.SessionsConfig{HeartbeatWindow: time.Hour}}},
		Components: []resource.Config{
			{
				Name:  "rtpPassthroughCamera",
				API:   resource.NewAPI("rdk", "component", "camera"),
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					RTPPassthrough: true,
				},
			},
		},
	}

	// Create a robot with a single fake camera.
	options2, _, addr2 := robottestutils.CreateBaseOptionsAndListener(t)
	remote2Ctx, remoteRobot2, remoteWebSvc2 := setupRealRobotWithAddr(t, remoteCfg2, logger.Sublogger("remote-2"), options2, addr2)

	remoteCfg1 := &config.Config{
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{Sessions: config.SessionsConfig{HeartbeatWindow: time.Hour}}},
		Remotes: []config.Remote{
			{
				Name:     "remote-2",
				Address:  addr2,
				Insecure: true,
			},
		},
	}

	remote1Ctx, remoteRobot1, addr1, remoteWebSvc1 := setupRealRobot(t, remoteCfg1, logger.Sublogger("remote-1"))
	defer remoteRobot1.Close(remote1Ctx)
	defer remoteWebSvc1.Close(remote1Ctx)

	mainCfg := &config.Config{
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{Sessions: config.SessionsConfig{HeartbeatWindow: time.Hour}}},
		Remotes: []config.Remote{
			{
				Name:     "remote-1",
				Address:  addr1,
				Insecure: true,
			},
		},
	}

	mainCtx, mainRobot, _, mainWebSvc := setupRealRobot(t, mainCfg, logger.Sublogger("main"))
	greenLog(t, "robot setup")
	defer mainRobot.Close(mainCtx)
	defer mainWebSvc.Close(mainCtx)

	cameraClient, err := camera.FromRobot(mainRobot, "remote-1:remote-2:rtpPassthroughCamera")
	test.That(t, err, test.ShouldBeNil)

	image, _, err := cameraClient.Images(mainCtx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, image, test.ShouldNotBeNil)
	greenLog(t, "got images")

	greenLog(t, fmt.Sprintf("calling SubscribeRTP on %T, %p", cameraClient, cameraClient))
	// time.Sleep(time.Second)

	pktsChan := make(chan []*rtp.Packet)
	recvPktsCtx, recvPktsFn := context.WithCancel(context.Background())
	sub, err := cameraClient.(rtppassthrough.Source).SubscribeRTP(mainCtx, 512, func(pkts []*rtp.Packet) {
		// first packet
		recvPktsFn()
		// at some point packets are no longer published
		select {
		case pktsChan <- pkts:
		default:
		}
	})
	test.That(t, err, test.ShouldBeNil)
	<-recvPktsCtx.Done()
	greenLog(t, "got packets")

	greenLog(t, "calling close")
	test.That(t, remoteRobot2.Close(remote2Ctx), test.ShouldBeNil)
	test.That(t, remoteWebSvc2.Close(remote2Ctx), test.ShouldBeNil)
	greenLog(t, "close called")

	greenLog(t, "waiting for SubscribeRTP to stop receiving packets")

	timeoutCtx, timeoutFn := context.WithTimeout(context.Background(), time.Second)
	defer timeoutFn()

	var (
		pktTimeoutCtx context.Context
		pktTimeoutFn  context.CancelFunc
	)

	// Once we have not received packets for half a second we can assume that no more packets will be published
	// by the first instance of remote2
Loop:
	for {
		if pktTimeoutFn != nil {
			pktTimeoutFn()
		}
		pktTimeout := time.Millisecond * 500
		pktTimeoutCtx, pktTimeoutFn = context.WithTimeout(context.Background(), pktTimeout)
		select {
		case <-pktsChan:
			continue
		case <-pktTimeoutCtx.Done():
			greenLog(t, fmt.Sprintf("it has been at least %s since SubscribeRTP has received a packet", pktTimeout))
			pktTimeoutFn()
			// https://go.dev/ref/spec#Break_statements
			// The 'Loop' label is needed so that we break out of the loop
			// rather than out of the select statement
			break Loop
		case <-timeoutCtx.Done():
			t.Log("timed out waiting for SubscribeRTP packets to drain")
			t.FailNow()
		}
	}

	// sub should still be alive
	test.That(t, sub.Terminated.Err(), test.ShouldBeNil)

	// I'm trying to get the remote-2 to come back online at the same address under the hopes that remote-1 will treat it the same as it would
	// if a real robot crasehed & came back online without changing its name.
	// The expectation is that SubscribeRTP should start receiving packets from remote-1 when remote-1 starts receiving packets from the new remote-2
	// It is not working as remote 1 never detects remote 2 & as a result main calls Close() on it's client with remote-1 which can be detectd
	// by the fact that sub.Terminated.Done() is always the path this test goes down
	remote2CtxSecond, remoteRobot2Second, remoteWebSvc2Second := setupRealRobotWithAddr(t, remoteCfg2, logger.Sublogger("remote-2"), options2, addr2)
	defer remoteRobot2Second.Close(remote2CtxSecond)
	defer remoteWebSvc2Second.Close(remote2CtxSecond)
	sndPktTimeoutCtx, sndPktTimeoutFn := context.WithTimeout(context.Background(), time.Second*20)
	defer sndPktTimeoutFn()
	select {
	case <-sub.Terminated.Done():
		// Right now we are going down this path b/c main's
		redLog(t, "main's sub terminated due to clos")
		t.FailNow()
	case <-pktsChan:
		// Right now we never go down this path as the test is not able to get remote1 to reconnect to the new remote 2
		greenLog(t, "SubscribeRTP got packets")
	case <-sndPktTimeoutCtx.Done():
		redLog(t, "timed out waiting for SubscribeRTP to receive packets")
		t.FailNow()
	}
}
