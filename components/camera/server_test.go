package camera_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
	"time"

	pb "go.viam.com/api/component/camera/v1"
	"go.viam.com/test"
	goprotoutils "go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

var (
	errGeneratePointCloudFailed = errors.New("can't generate next point cloud")
	errPropertiesFailed         = errors.New("can't get camera properties")
	errCameraProjectorFailed    = errors.New("can't get camera properties")
	errGetImageFailed           = errors.New("can't get image")
	errCameraUnimplemented      = errors.New("not found")
)

func newServer() (pb.CameraServiceServer, *inject.Camera, *inject.Camera, *inject.Camera, error) {
	injectCamera := &inject.Camera{}
	injectCameraDepth := &inject.Camera{}
	injectCamera2 := &inject.Camera{}
	cameras := map[resource.Name]camera.Camera{
		camera.Named(testCameraName):  injectCamera,
		camera.Named(depthCameraName): injectCameraDepth,
		camera.Named(failCameraName):  injectCamera2,
	}
	cameraSvc, err := resource.NewAPIResourceCollection(camera.API, cameras)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return camera.NewRPCServiceServer(cameraSvc).(pb.CameraServiceServer), injectCamera, injectCameraDepth, injectCamera2, nil
}

func TestServer(t *testing.T) {
	cameraServer, injectCamera, injectCameraDepth, injectCamera2, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var imgBuf bytes.Buffer
	test.That(t, png.Encode(&imgBuf, img), test.ShouldBeNil)
	var imgBufJpeg bytes.Buffer

	test.That(t, jpeg.Encode(&imgBufJpeg, img, &jpeg.Options{Quality: 75}), test.ShouldBeNil)

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
	pcA := pointcloud.NewBasicEmpty()
	err = pcA.Set(pointcloud.NewVector(5, 5, 5), nil)
	test.That(t, err, test.ShouldBeNil)

	injectCamera.NextPointCloudFunc = func(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error) {
		return pcA, nil
	}
	injectCamera.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{
			SupportsPCD:     true,
			IntrinsicParams: intrinsics,
			MimeTypes:       []string{utils.MimeTypeJPEG, utils.MimeTypePNG, utils.MimeTypeH264},
			FrameRate:       float32(10.0),
		}, nil
	}
	injectCamera.ImagesFunc = func(
		ctx context.Context,
		filterSourceNames []string,
		extra map[string]interface{},
	) ([]camera.NamedImage, resource.ResponseMetadata, error) {
		color := rimage.NewImage(40, 50)
		colorImg, err := camera.NamedImageFromImage(color, "color", utils.MimeTypeJPEG, annotations1)
		if err != nil {
			return nil, resource.ResponseMetadata{}, err
		}
		depth := rimage.NewEmptyDepthMap(10, 20)
		depthImg, err := camera.NamedImageFromImage(depth, "depth", utils.MimeTypeRawDepth, annotations2)
		if err != nil {
			return nil, resource.ResponseMetadata{}, err
		}

		if len(filterSourceNames) == 0 {
			ts := time.UnixMilli(12345)
			return []camera.NamedImage{colorImg, depthImg}, resource.ResponseMetadata{CapturedAt: ts}, nil
		}

		images := make([]camera.NamedImage, 0, len(filterSourceNames))
		for _, src := range filterSourceNames {
			switch src {
			case "color":
				images = append(images, colorImg)
			case "depth":
				images = append(images, depthImg)
			default:
				return nil, resource.ResponseMetadata{}, fmt.Errorf("unknown source name: %s", src)
			}
		}
		ts := time.UnixMilli(12345)
		return images, resource.ResponseMetadata{CapturedAt: ts}, nil
	}
	injectCamera.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
		return projA, nil
	}
	// depth camera
	depthImage := rimage.NewEmptyDepthMap(10, 20)
	depthImage.Set(0, 0, rimage.Depth(40))
	depthImage.Set(0, 1, rimage.Depth(1))
	depthImage.Set(5, 6, rimage.Depth(190))
	depthImage.Set(9, 12, rimage.Depth(3000))
	depthImage.Set(5, 9, rimage.MaxDepth-rimage.Depth(1))
	var depthBuf bytes.Buffer
	test.That(t, png.Encode(&depthBuf, depthImage), test.ShouldBeNil)
	injectCameraDepth.NextPointCloudFunc = func(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error) {
		return pcA, nil
	}
	// no frame rate camera
	injectCameraDepth.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{
			SupportsPCD:     true,
			IntrinsicParams: intrinsics,
			ImageType:       camera.DepthStream,
		}, nil
	}
	injectCameraDepth.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
		return projA, nil
	}
	injectCameraDepth.ImagesFunc = func(
		ctx context.Context,
		filterSourceNames []string,
		extra map[string]interface{},
	) ([]camera.NamedImage, resource.ResponseMetadata, error) {
		namedImg, err := camera.NamedImageFromImage(depthImage, "", utils.MimeTypeRawDepth, data.Annotations{})
		if err != nil {
			return nil, resource.ResponseMetadata{}, err
		}
		return []camera.NamedImage{namedImg}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
	}
	// bad camera
	injectCamera2.NextPointCloudFunc = func(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error) {
		return nil, errGeneratePointCloudFailed
	}
	injectCamera2.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, errPropertiesFailed
	}
	injectCamera2.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
		return nil, errCameraProjectorFailed
	}
	injectCamera2.ImagesFunc = func(
		ctx context.Context,
		filterSourceNames []string,
		extra map[string]interface{},
	) ([]camera.NamedImage, resource.ResponseMetadata, error) {
		return nil, resource.ResponseMetadata{}, errGetImageFailed
	}

	t.Run("GetPointCloud", func(t *testing.T) {
		_, err := cameraServer.GetPointCloud(context.Background(), &pb.GetPointCloudRequest{Name: missingCameraName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCameraUnimplemented.Error())

		pcA := pointcloud.NewBasicEmpty()
		err = pcA.Set(pointcloud.NewVector(5, 5, 5), nil)
		test.That(t, err, test.ShouldBeNil)

		injectCamera.NextPointCloudFunc = func(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error) {
			if val, ok := extra["empty"].(bool); ok && val {
				return pointcloud.NewBasicEmpty(), nil
			}
			return pcA, nil
		}
		pcProto, err := cameraServer.GetPointCloud(context.Background(), &pb.GetPointCloudRequest{
			Name: testCameraName,
		})

		test.That(t, err, test.ShouldBeNil)
		pc, err := pointcloud.ReadPCD(bytes.NewReader(pcProto.GetPointCloud()), "")
		test.That(t, err, test.ShouldBeNil)
		_, got := pc.At(5, 5, 5)
		test.That(t, got, test.ShouldBeTrue)

		ext, err := goprotoutils.StructToStructPb(map[string]any{"empty": "true"})
		test.That(t, err, test.ShouldBeNil)
		emptyPcProto, err := cameraServer.GetPointCloud(context.Background(), &pb.GetPointCloudRequest{
			Name:  testCameraName,
			Extra: ext,
		})
		test.That(t, err, test.ShouldBeNil)

		emptyPc, err := pointcloud.ReadPCD(bytes.NewReader(emptyPcProto.GetPointCloud()), "")
		test.That(t, err, test.ShouldBeNil)
		_, got = emptyPc.At(5, 5, 5)
		test.That(t, got, test.ShouldBeTrue)

		_, err = cameraServer.GetPointCloud(context.Background(), &pb.GetPointCloudRequest{
			Name: failCameraName,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGeneratePointCloudFailed.Error())
	})
	t.Run("GetImages", func(t *testing.T) {
		_, err := cameraServer.GetImages(context.Background(), &pb.GetImagesRequest{Name: missingCameraName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCameraUnimplemented.Error())

		resp, err := cameraServer.GetImages(context.Background(), &pb.GetImagesRequest{Name: testCameraName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.ResponseMetadata.CapturedAt.AsTime(), test.ShouldEqual, time.UnixMilli(12345))
		test.That(t, len(resp.Images), test.ShouldEqual, 2)
		test.That(t, resp.Images[0].MimeType, test.ShouldEqual, utils.MimeTypeJPEG)
		test.That(t, resp.Images[0].SourceName, test.ShouldEqual, "color")
		test.That(t, resp.Images[1].MimeType, test.ShouldEqual, utils.MimeTypeRawDepth)
		test.That(t, resp.Images[1].SourceName, test.ShouldEqual, "depth")

		// filter only color
		resp, err = cameraServer.GetImages(context.Background(), &pb.GetImagesRequest{Name: testCameraName, FilterSourceNames: []string{"color"}})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.ResponseMetadata.CapturedAt.AsTime(), test.ShouldEqual, time.UnixMilli(12345))
		test.That(t, len(resp.Images), test.ShouldEqual, 1)
		test.That(t, resp.Images[0].SourceName, test.ShouldEqual, "color")
		test.That(t, resp.Images[0].MimeType, test.ShouldEqual, utils.MimeTypeJPEG)
		// validate decoded image
		decodedColor, err := rimage.DecodeImage(context.Background(), resp.Images[0].Image, utils.MimeTypeJPEG)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, decodedColor.Bounds().Dx(), test.ShouldEqual, 40)
		test.That(t, decodedColor.Bounds().Dy(), test.ShouldEqual, 50)

		// filter only depth
		resp, err = cameraServer.GetImages(context.Background(), &pb.GetImagesRequest{Name: testCameraName, FilterSourceNames: []string{"depth"}})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.ResponseMetadata.CapturedAt.AsTime(), test.ShouldEqual, time.UnixMilli(12345))
		test.That(t, len(resp.Images), test.ShouldEqual, 1)
		test.That(t, resp.Images[0].SourceName, test.ShouldEqual, "depth")
		test.That(t, resp.Images[0].MimeType, test.ShouldEqual, utils.MimeTypeRawDepth)
		// validate decoded depth map
		decodedDepthImg, err := rimage.DecodeImage(context.Background(), resp.Images[0].Image, utils.MimeTypeRawDepth)
		test.That(t, err, test.ShouldBeNil)
		dm, err := rimage.ConvertImageToDepthMap(context.Background(), decodedDepthImg)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dm.Width(), test.ShouldEqual, 10)
		test.That(t, dm.Height(), test.ShouldEqual, 20)

		// filter both
		resp, err = cameraServer.GetImages(
			context.Background(),
			&pb.GetImagesRequest{
				Name:              testCameraName,
				FilterSourceNames: []string{"color", "depth"},
			},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.ResponseMetadata.CapturedAt.AsTime(), test.ShouldEqual, time.UnixMilli(12345))
		test.That(t, len(resp.Images), test.ShouldEqual, 2)
		seen := map[string]bool{}
		for _, im := range resp.Images {
			switch im.SourceName {
			case "color":
				test.That(t, im.MimeType, test.ShouldEqual, utils.MimeTypeJPEG)
				decoded, err := rimage.DecodeImage(context.Background(), im.Image, utils.MimeTypeJPEG)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, decoded.Bounds().Dx(), test.ShouldEqual, 40)
				test.That(t, decoded.Bounds().Dy(), test.ShouldEqual, 50)
				seen["color"] = true
			case "depth":
				test.That(t, im.MimeType, test.ShouldEqual, utils.MimeTypeRawDepth)
				decodedDepth, err := rimage.DecodeImage(context.Background(), im.Image, utils.MimeTypeRawDepth)
				test.That(t, err, test.ShouldBeNil)
				dm, err := rimage.ConvertImageToDepthMap(context.Background(), decodedDepth)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, dm.Width(), test.ShouldEqual, 10)
				test.That(t, dm.Height(), test.ShouldEqual, 20)
				seen["depth"] = true
			default:
				t.Fatalf("unexpected source name: %s", im.SourceName)
			}
		}
		test.That(t, seen["color"], test.ShouldBeTrue)
		test.That(t, seen["depth"], test.ShouldBeTrue)

		// duplicate should error at server
		_, err = cameraServer.GetImages(
			context.Background(),
			&pb.GetImagesRequest{
				Name:              testCameraName,
				FilterSourceNames: []string{"color", "color"},
			},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "duplicate source name in filter: color")

		// unknown source should error from mock
		_, err = cameraServer.GetImages(
			context.Background(),
			&pb.GetImagesRequest{
				Name:              testCameraName,
				FilterSourceNames: []string{"unknown"},
			},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown source name: unknown")
	})

	t.Run("GetProperties", func(t *testing.T) {
		_, err := cameraServer.GetProperties(context.Background(), &pb.GetPropertiesRequest{Name: missingCameraName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCameraUnimplemented.Error())

		resp, err := cameraServer.GetProperties(context.Background(), &pb.GetPropertiesRequest{Name: testCameraName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.SupportsPcd, test.ShouldBeTrue)
		test.That(t, resp.IntrinsicParameters.WidthPx, test.ShouldEqual, 1280)
		test.That(t, resp.IntrinsicParameters.HeightPx, test.ShouldEqual, 720)
		test.That(t, resp.IntrinsicParameters.FocalXPx, test.ShouldEqual, 200)
		test.That(t, resp.IntrinsicParameters.FocalYPx, test.ShouldEqual, 200)
		test.That(t, resp.IntrinsicParameters.CenterXPx, test.ShouldEqual, 100)
		test.That(t, resp.IntrinsicParameters.CenterYPx, test.ShouldEqual, 100)
		test.That(t, resp.MimeTypes, test.ShouldContain, utils.MimeTypeJPEG)
		test.That(t, resp.MimeTypes, test.ShouldContain, utils.MimeTypePNG)
		test.That(t, resp.MimeTypes, test.ShouldContain, utils.MimeTypeH264)
		test.That(t, resp.FrameRate, test.ShouldNotBeNil)
		test.That(t, *resp.FrameRate, test.ShouldEqual, 10.0)

		// test property when we don't set frame rate
		resp2, err := cameraServer.GetProperties(context.Background(), &pb.GetPropertiesRequest{Name: depthCameraName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp2.FrameRate, test.ShouldBeNil)
	})

	t.Run("GetImages with extra", func(t *testing.T) {
		injectCamera.ImagesFunc = func(
			ctx context.Context,
			filterSourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			test.That(t, extra, test.ShouldBeEmpty)
			return nil, resource.ResponseMetadata{}, errGetImageFailed
		}

		_, err := cameraServer.GetImages(context.Background(), &pb.GetImagesRequest{
			Name: testCameraName,
		})

		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGetImageFailed.Error())

		injectCamera.ImagesFunc = func(
			ctx context.Context,
			filterSourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			test.That(t, len(extra), test.ShouldEqual, 1)
			test.That(t, extra["hello"], test.ShouldEqual, "world")
			return nil, resource.ResponseMetadata{}, errGetImageFailed
		}

		ext, err := goprotoutils.StructToStructPb(map[string]interface{}{"hello": "world"})
		test.That(t, err, test.ShouldBeNil)

		_, err = cameraServer.GetImages(context.Background(), &pb.GetImagesRequest{
			Name:  testCameraName,
			Extra: ext,
		})

		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGetImageFailed.Error())

		injectCamera.ImagesFunc = func(
			ctx context.Context,
			filterSourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			test.That(t, len(extra), test.ShouldEqual, 1)
			test.That(t, extra[data.FromDMString], test.ShouldBeTrue)

			return nil, resource.ResponseMetadata{}, errGetImageFailed
		}

		// one kvp created with data.FromDMContextKey
		ext, err = goprotoutils.StructToStructPb(map[string]interface{}{data.FromDMString: true})
		test.That(t, err, test.ShouldBeNil)

		_, err = cameraServer.GetImages(context.Background(), &pb.GetImagesRequest{
			Name:  testCameraName,
			Extra: ext,
		})

		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGetImageFailed.Error())

		injectCamera.ImagesFunc = func(
			ctx context.Context,
			filterSourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			test.That(t, len(extra), test.ShouldEqual, 2)
			test.That(t, extra["hello"], test.ShouldEqual, "world")
			test.That(t, extra[data.FromDMString], test.ShouldBeTrue)
			return nil, resource.ResponseMetadata{}, errGetImageFailed
		}

		// use values from data and camera
		ext, err = goprotoutils.StructToStructPb(
			map[string]interface{}{
				data.FromDMString: true,
				"hello":           "world",
			},
		)
		test.That(t, err, test.ShouldBeNil)

		_, err = cameraServer.GetImages(context.Background(), &pb.GetImagesRequest{
			Name:  testCameraName,
			Extra: ext,
		})

		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGetImageFailed.Error())
	})
}
