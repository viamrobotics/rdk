package camera_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net"
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
	injectCamera.ImageFunc = func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
		if val, ok := extra["empty"].(bool); ok && val {
			return []byte{}, camera.ImageMetadata{}, nil
		}
		resBytes, err := rimage.EncodeImage(ctx, imgPng, mimeType)
		test.That(t, err, test.ShouldBeNil)
		return resBytes, camera.ImageMetadata{MimeType: mimeType}, nil
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
	injectCameraDepth.ImageFunc = func(
		ctx context.Context,
		mimeType string,
		extra map[string]interface{},
	) ([]byte, camera.ImageMetadata, error) {
		resBytes, err := rimage.EncodeImage(ctx, depthImg, mimeType)
		test.That(t, err, test.ShouldBeNil)
		return resBytes, camera.ImageMetadata{MimeType: mimeType}, nil
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
	injectCamera2.ImageFunc = func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
		return nil, camera.ImageMetadata{}, errGetImageFailed
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

		frame, err := camera.DecodeImageFromCamera(context.Background(), rutils.MimeTypeRawRGBA, nil, camera1Client)
		test.That(t, err, test.ShouldBeNil)
		compVal, _, err := rimage.CompareImages(img, frame)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, compVal, test.ShouldEqual, 0) // exact copy, no color conversion
		_, err = camera.DecodeImageFromCamera(context.Background(), rutils.MimeTypeRawRGBA, map[string]interface{}{"empty": true}, camera1Client)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "received empty bytes from Image method")

		pcB, err := camera1Client.NextPointCloud(context.Background())
		test.That(t, err, test.ShouldBeNil)
		_, got := pcB.At(5, 5, 5)
		test.That(t, got, test.ShouldBeTrue)

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

		ctx := context.Background()
		frame, err := camera.DecodeImageFromCamera(ctx, rutils.WithLazyMIMEType(rutils.MimeTypePNG), nil, client)
		test.That(t, err, test.ShouldBeNil)
		dm, err := rimage.ConvertImageToDepthMap(context.Background(), frame)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dm, test.ShouldResemble, depthImg)

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
		test.That(t, err.Error(), test.ShouldContainSubstring, errGetImageFailed.Error())

		_, _, err = client2.Image(context.Background(), "", nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGetImageFailed.Error())

		_, err = client2.NextPointCloud(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGeneratePointCloudFailed.Error())

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

		injectCamera.ImageFunc = func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
			test.That(t, extra, test.ShouldBeEmpty)
			return nil, camera.ImageMetadata{}, errGetImageFailed
		}

		ctx := context.Background()
		_, _, err = camClient.Image(ctx, "", nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGetImageFailed.Error())

		injectCamera.ImageFunc = func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
			test.That(t, len(extra), test.ShouldEqual, 1)
			test.That(t, extra[data.FromDMString], test.ShouldBeTrue)

			return nil, camera.ImageMetadata{}, errGetImageFailed
		}

		_, _, err = camClient.Image(context.Background(), "", map[string]interface{}{data.FromDMString: true})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGetImageFailed.Error())

		injectCamera.ImageFunc = func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
			test.That(t, len(extra), test.ShouldEqual, 2)
			test.That(t, extra["hello"], test.ShouldEqual, "world")
			test.That(t, extra[data.FromDMString], test.ShouldBeTrue)
			return nil, camera.ImageMetadata{}, errGetImageFailed
		}

		// merge values from data and camera
		ext := data.FromDMExtraMap
		ext["hello"] = "world"
		ctx = context.Background()
		_, _, err = camClient.Image(ctx, "", ext)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGetImageFailed.Error())

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
	fakeFrameRate := float32(10.0)

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
				FrameRate:        fakeFrameRate,
			},
		},
		{
			name: "nil intrinsic params",
			props: camera.Properties{
				SupportsPCD:      true,
				ImageType:        camera.UnspecifiedStream,
				IntrinsicParams:  nil,
				DistortionParams: fakeDistortion,
				FrameRate:        fakeFrameRate,
			},
		},
		{
			name: "nil distortion parameters",
			props: camera.Properties{
				SupportsPCD:      true,
				ImageType:        camera.UnspecifiedStream,
				IntrinsicParams:  fakeIntrinsics,
				DistortionParams: nil,
				FrameRate:        fakeFrameRate,
			},
		},
		{
			name: "no frame rate parameters",
			props: camera.Properties{
				SupportsPCD:      true,
				ImageType:        camera.UnspecifiedStream,
				IntrinsicParams:  fakeIntrinsics,
				DistortionParams: fakeDistortion,
			},
		},
		{
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
	var jpegBuf bytes.Buffer
	test.That(t, jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: 100}), test.ShouldBeNil)
	imgJpeg, err := jpeg.Decode(bytes.NewBuffer(jpegBuf.Bytes()))
	test.That(t, err, test.ShouldBeNil)

	injectCamera.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}
	injectCamera.ImageFunc = func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
		if mimeType == "" {
			mimeType = rutils.MimeTypeRawRGBA
		}
		mimeType, _ = rutils.CheckLazyMIMEType(mimeType)
		switch mimeType {
		case rutils.MimeTypePNG:
			resBytes, err := rimage.EncodeImage(ctx, imgPng, mimeType)
			test.That(t, err, test.ShouldBeNil)
			return resBytes, camera.ImageMetadata{MimeType: mimeType}, nil
		default:
			return nil, camera.ImageMetadata{}, errInvalidMimeType
		}
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

	ctx := context.Background()
	frame, err := camera.DecodeImageFromCamera(ctx, rutils.MimeTypePNG, nil, camera1Client)
	test.That(t, err, test.ShouldBeNil)
	// Should always lazily decode
	test.That(t, frame, test.ShouldHaveSameTypeAs, &rimage.LazyEncodedImage{})
	frameLazy := frame.(*rimage.LazyEncodedImage)
	test.That(t, frameLazy.RawData(), test.ShouldResemble, imgBuf.Bytes())

	frame, err = camera.DecodeImageFromCamera(ctx, rutils.WithLazyMIMEType(rutils.MimeTypePNG), nil, camera1Client)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame, test.ShouldHaveSameTypeAs, &rimage.LazyEncodedImage{})
	frameLazy = frame.(*rimage.LazyEncodedImage)
	test.That(t, frameLazy.RawData(), test.ShouldResemble, imgBuf.Bytes())

	test.That(t, frameLazy.MIMEType(), test.ShouldEqual, rutils.MimeTypePNG)
	compVal, _, err := rimage.CompareImages(img, frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 0) // exact copy, no color conversion

	// when DecodeImageFromCamera is called without a mime type, defaults to JPEG
	var called bool
	injectCamera.ImageFunc = func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
		called = true
		test.That(t, mimeType, test.ShouldResemble, rutils.WithLazyMIMEType(rutils.MimeTypeJPEG))
		resBytes, err := rimage.EncodeImage(ctx, imgPng, mimeType)
		test.That(t, err, test.ShouldBeNil)
		return resBytes, camera.ImageMetadata{MimeType: mimeType}, nil
	}
	frame, err = camera.DecodeImageFromCamera(ctx, "", nil, camera1Client)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame, test.ShouldHaveSameTypeAs, &rimage.LazyEncodedImage{})
	frameLazy = frame.(*rimage.LazyEncodedImage)
	test.That(t, frameLazy.MIMEType(), test.ShouldEqual, rutils.MimeTypeJPEG)
	compVal, _, err = rimage.CompareImages(imgJpeg, frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 0) // exact copy, no color conversion
	test.That(t, called, test.ShouldBeTrue)
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

func TestClientStreamAfterClose(t *testing.T) {
	// Set up gRPC server
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	// Set up camera that can stream images
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	injectCamera := &inject.Camera{}
	injectCamera.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}
	injectCamera.ImageFunc = func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
		imgBytes, err := rimage.EncodeImage(ctx, img, mimeType)
		test.That(t, err, test.ShouldBeNil)
		return imgBytes, camera.ImageMetadata{MimeType: mimeType}, nil
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
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
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

func setupRealRobotWithOptions(
	t *testing.T,
	robotConfig *config.Config,
	logger logging.Logger,
	options weboptions.Options,
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

func TestMultiplexOverRemoteConnection(t *testing.T) {
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
	logger.Info("robot setup")
	defer mainRobot.Close(mainCtx)
	defer mainWebSvc.Close(mainCtx)

	cameraClient, err := camera.FromRobot(mainRobot, "remote:rtpPassthroughCamera")
	test.That(t, err, test.ShouldBeNil)

	image, _, err := cameraClient.Images(mainCtx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, image, test.ShouldNotBeNil)
	logger.Info("got images")

	recvPktsCtx, recvPktsFn := context.WithCancel(context.Background())
	sub, err := cameraClient.(rtppassthrough.Source).SubscribeRTP(mainCtx, 2, func(pkts []*rtp.Packet) {
		recvPktsFn()
	})
	test.That(t, err, test.ShouldBeNil)
	<-recvPktsCtx.Done()
	logger.Info("got packets")

	err = cameraClient.(rtppassthrough.Source).Unsubscribe(mainCtx, sub.ID)
	test.That(t, err, test.ShouldBeNil)
	logger.Info("unsubscribe")
}

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
	defer mainRobot.Close(mainCtx)
	defer mainWebSvc.Close(mainCtx)

	cameraClient, err := camera.FromRobot(mainRobot, "remote-1:remote-2:rtpPassthroughCamera")
	test.That(t, err, test.ShouldBeNil)

	image, _, err := cameraClient.Images(mainCtx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, image, test.ShouldNotBeNil)
	logger.Info("got images")

	time.Sleep(time.Second)

	recvPktsCtx, recvPktsFn := context.WithCancel(context.Background())
	sub, err := cameraClient.(rtppassthrough.Source).SubscribeRTP(mainCtx, 512, func(pkts []*rtp.Packet) {
		recvPktsFn()
	})
	test.That(t, err, test.ShouldBeNil)
	<-recvPktsCtx.Done()
	logger.Info("got packets")

	test.That(t, cameraClient.(rtppassthrough.Source).Unsubscribe(mainCtx, sub.ID), test.ShouldBeNil)
}

//nolint
// NOTE: These tests fail when this condition occurs:
//
//	logger.go:130: 2024-06-17T16:56:14.097-0400 DEBUG   TestGrandRemoteRebooting.remote-1.rdk:remote:/remote-2.webrtc   rpc/wrtc_client_channel.go:299  no stream for id; discarding    {"ch": 0, "id": 11}
//
// https://github.com/viamrobotics/goutils/blob/main/rpc/wrtc_client_channel.go#L299
//
// go test -race -v -run=TestWhyMustTimeoutOnReadRTP -timeout 10s
// TestWhyMustTimeoutOnReadRTP shows that if we don't timeout on ReadRTP (and also don't call RemoveStream) on close
// calling Close() on main's camera client blocks forever if there is a live SubscribeRTP subscription with a remote
// due to the fact that the TrackRemote.ReadRTP method blocking forever.
func TestWhyMustTimeoutOnReadRTP(t *testing.T) {
	t.Skip("Depends on RSDK-7903")
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
	logger.Info("robot setup")
	defer mainRobot.Close(mainCtx)
	defer mainWebSvc.Close(mainCtx)

	cameraClient, err := camera.FromRobot(mainRobot, "remote-1:remote-2:rtpPassthroughCamera")
	test.That(t, err, test.ShouldBeNil)

	image, _, err := cameraClient.Images(mainCtx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, image, test.ShouldNotBeNil)
	logger.Info("got images")

	logger.Infof("calling SubscribeRTP on %T, %p", cameraClient, cameraClient)
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
	logger.Info("got packets")

	logger.Info("calling close")
	test.That(t, remoteRobot2.Close(context.Background()), test.ShouldBeNil)
	test.That(t, remoteWebSvc2.Close(context.Background()), test.ShouldBeNil)
	logger.Info("close called")

	logger.Info("waiting for SubscribeRTP to stop receiving packets")

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
			logger.Infof("it has been at least %s since SubscribeRTP has received a packet", pktTimeout)
			pktTimeoutFn()
			// https://go.dev/ref/spec#Break_statements
			// The 'Loop' label is needed so that we break out of the loop
			// rather than out of the select statement
			break Loop
		case <-timeoutCtx.Done():
			// Failure case. The following assertion always fails. We use this to get a failure line
			// number + error message.
			test.That(t, true, test.ShouldEqual, "timed out waiting for SubscribeRTP packets to drain")
		}
	}

	logger.Info("sleeping")
	time.Sleep(time.Second)

	// sub should still be alive
	test.That(t, sub.Terminated.Err(), test.ShouldBeNil)
}

// Notes:
// - Ideally, we'd lower robot client reconnect timers down from 10 seconds.
// - We need to force robot client webrtc connections
// - WebRTC connections need to disable SRTP replay protection
//
// This tests the following scenario:
//  1. main-part (main) -> remote-part-1 (r1) -> remote-part-2 (r2) where r2 has a camera
//  2. the client in the main part makes an AddStream(r1:r2:rtpPassthroughCamera) request, starting a
//     webrtc video track to be streamed from r2 -> r1 -> main -> client
//  3. r2 reboots
//  4. expect that r1 & main stop getting packets
//  5. when the new instance of r2 comes back online main gets new rtp packets from it's track with
//     r1.
func TestGrandRemoteRebooting(t *testing.T) {
	t.Skip("Depends on RSDK-7903")
	logger := logging.NewTestLogger(t).Sublogger(t.Name())

	remoteCfg2 := &config.Config{
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
	remote2Ctx, remoteRobot2, remoteWebSvc2 := setupRealRobotWithOptions(t, remoteCfg2, logger.Sublogger("remote-2"), options2)

	remoteCfg1 := &config.Config{
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
		Remotes: []config.Remote{
			{
				Name:     "remote-1",
				Address:  addr1,
				Insecure: true,
			},
		},
	}

	mainCtx, mainRobot, _, mainWebSvc := setupRealRobot(t, mainCfg, logger.Sublogger("main"))
	logger.Info("robot setup")
	defer mainRobot.Close(mainCtx)
	defer mainWebSvc.Close(mainCtx)

	mainCameraClient, err := camera.FromRobot(mainRobot, "remote-1:remote-2:rtpPassthroughCamera")
	test.That(t, err, test.ShouldBeNil)

	image, _, err := mainCameraClient.Images(mainCtx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, image, test.ShouldNotBeNil)
	logger.Info("got images")

	logger.Infof("calling SubscribeRTP on %T, %p", mainCameraClient, mainCameraClient)
	time.Sleep(time.Second)

	pktsChan := make(chan []*rtp.Packet, 10)
	recvPktsCtx, recvPktsFn := context.WithCancel(context.Background())
	testDone := make(chan struct{})
	defer close(testDone)
	sub, err := mainCameraClient.(rtppassthrough.Source).SubscribeRTP(mainCtx, 512, func(pkts []*rtp.Packet) {
		// first packet
		recvPktsFn()
		// at some point packets are no longer published
		lastPkt := pkts[len(pkts)-1]
		logger.Info("Pushing packets: ", len(pkts), " TS:", lastPkt.Timestamp)
		select {
		case <-testDone:
		case pktsChan <- pkts:
		}
		logger.Info("Pkt pushed. TS:", lastPkt.Timestamp)
	})
	test.That(t, err, test.ShouldBeNil)
	<-recvPktsCtx.Done()
	logger.Info("got packets")

	logger.Info("calling close")
	test.That(t, remoteRobot2.Close(remote2Ctx), test.ShouldBeNil)
	test.That(t, remoteWebSvc2.Close(remote2Ctx), test.ShouldBeNil)
	logger.Info("close called")

	logger.Info("waiting for SubscribeRTP to stop receiving packets")

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
		case pkts := <-pktsChan:
			lastPkt := pkts[len(pkts)-1]
			logger.Infof("First RTP packet received. TS: %v", lastPkt.Timestamp)
			continue
		case <-pktTimeoutCtx.Done():
			logger.Infow("SubscribeRTP timed out waiting for a packet. The remote is offline.", "pktTimeout", pktTimeout)
			pktTimeoutFn()
			// https://go.dev/ref/spec#Break_statements
			// The 'Loop' label is needed so that we break out of the loop
			// rather than out of the select statement
			break Loop
		case <-timeoutCtx.Done():
			// Failure case. The following assertion always fails. We use this to get a failure line
			// number + error message.
			test.That(t, true, test.ShouldEqual, "timed out waiting for SubscribeRTP packets to drain")
		}
	}

	// sub should still be alive
	test.That(t, sub.Terminated.Err(), test.ShouldBeNil)

	// I'm trying to get the remote-2 to come back online at the same address under the hopes that remote-1 will
	// treat it the same as it would if a real robot crasehed & came back online without changing its name.
	// The expectation is that SubscribeRTP should start receiving packets from remote-1 when remote-1 starts
	// receiving packets from the new remote-2
	// It is not working as remote 1 never detects remote 2 & as a result main calls Close() on it's client with
	// remote-1 which can be detectd
	// by the fact that sub.Terminated.Done() is always the path this test goes down

	logger.Infow("old robot address", "address", addr2)
	tcpAddr, ok := options2.Network.Listener.Addr().(*net.TCPAddr)
	test.That(t, ok, test.ShouldBeTrue)
	newListener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: tcpAddr.Port})
	test.That(t, err, test.ShouldBeNil)
	options2.Network.Listener = newListener

	logger.Info("setting up new robot at address %s", newListener.Addr().String())

	remote2CtxSecond, remoteRobot2Second, remoteWebSvc2Second := setupRealRobotWithOptions(
		t,
		remoteCfg2,
		logger.Sublogger("remote-2SecondInstance"),
		options2,
	)
	defer remoteRobot2Second.Close(remote2CtxSecond)
	defer remoteWebSvc2Second.Close(remote2CtxSecond)
	sndPktTimeoutCtx, sndPktTimeoutFn := context.WithTimeout(context.Background(), time.Second*20)
	defer sndPktTimeoutFn()
	testPassed := false
	for !testPassed {
		select {
		case <-sub.Terminated.Done():
			// Failure case. The following assertion always fails. We use this to get a failure line
			// number + error message.
			test.That(t, true, test.ShouldEqual, "main's sub terminated due to close")
		case pkts := <-pktsChan:
			lastPkt := pkts[len(pkts)-1]
			logger.Info("Test finale RTP packet received. TS: %v", lastPkt.Timestamp)
			// Right now we never go down this path as the test is not able to get remote1 to reconnect to the new remote 2
			logger.Info("SubscribeRTP got packets")
			testPassed = true
		case <-sndPktTimeoutCtx.Done():
			// Failure case. The following assertion always fails. We use this to get a failure line
			// number + error message.
			test.That(t, true, test.ShouldEqual, "timed out waiting for SubscribeRTP to receive packets")
		case <-time.After(time.Second):
			logger.Info("still waiting for RTP packets")
		}
	}
}
