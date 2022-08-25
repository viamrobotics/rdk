package camera_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/pointcloud"
	pb "go.viam.com/rdk/proto/api/component/camera/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func newServer() (pb.CameraServiceServer, *inject.Camera, *inject.Camera, *inject.Camera, error) {
	injectCamera := &inject.Camera{}
	injectCameraDepth := &inject.Camera{}
	injectCamera2 := &inject.Camera{}
	cameras := map[resource.Name]interface{}{
		camera.Named(testCameraName):  injectCamera,
		camera.Named(depthCameraName): injectCameraDepth,
		camera.Named(failCameraName):  injectCamera2,
		camera.Named(fakeCameraName):  "notCamera",
	}
	cameraSvc, err := subtype.New(cameras)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return camera.NewServer(cameraSvc), injectCamera, injectCameraDepth, injectCamera2, nil
}

func TestServer(t *testing.T) {
	cameraServer, injectCamera, injectCameraDepth, injectCamera2, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	var projA rimage.Projector
	intrinsics := &transform.PinholeCameraIntrinsics{ // not the real camera parameters -- fake for test
		Width:  1280,
		Height: 720,
		Fx:     200,
		Fy:     200,
		Ppx:    100,
		Ppy:    100,
	}
	projA = intrinsics
	pcA := pointcloud.New()
	err = pcA.Set(pointcloud.NewVector(5, 5, 5), nil)
	test.That(t, err, test.ShouldBeNil)

	var imageReleased bool
	// color camera
	img := image.NewNRGBA64(image.Rect(0, 0, 4, 4))
	var imgBuf bytes.Buffer
	test.That(t, png.Encode(&imgBuf, img), test.ShouldBeNil)
	var imgBufJpeg bytes.Buffer
	test.That(t, jpeg.Encode(&imgBufJpeg, img, nil), test.ShouldBeNil)
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pcA, nil
	}
	injectCamera.GetPropertiesFunc = func(ctx context.Context) (rimage.Projector, error) {
		return projA, nil
	}
	injectCamera.GetFrameFunc = func(ctx context.Context, mimeType string) ([]byte, string, int64, int64, error) {
		imageReleased = true
		if mimeType == "" {
			mimeType = utils.MimeTypeRawRGBA
		}
		imgBytes, err := rimage.EncodeImage(ctx, img, mimeType)
		return imgBytes, mimeType, int64(img.Bounds().Dx()), int64(img.Bounds().Dy()), err
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
	injectCameraDepth.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pcA, nil
	}
	injectCameraDepth.GetPropertiesFunc = func(ctx context.Context) (rimage.Projector, error) {
		return projA, nil
	}
	injectCameraDepth.GetFrameFunc = func(ctx context.Context, mimeType string) ([]byte, string, int64, int64, error) {
		imageReleased = true
		if mimeType == "" {
			mimeType = utils.MimeTypeRawRGBA
		}
		depthBytes, err := rimage.EncodeImage(ctx, depthImage, mimeType)
		return depthBytes, mimeType, int64(depthImage.Bounds().Dx()), int64(depthImage.Bounds().Dy()), err
	}
	// bad camera
	injectCamera2.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return nil, errors.New("can't generate next point cloud")
	}
	injectCamera2.GetPropertiesFunc = func(ctx context.Context) (rimage.Projector, error) {
		return nil, errors.New("can't get camera properties")
	}
	injectCamera2.GetFrameFunc = func(ctx context.Context, mimeType string) ([]byte, string, int64, int64, error) {
		return nil, "", 0, 0, errors.New("can't generate frame")
	}
	// does a depth camera transfer its depth image properly
	t.Run("GetFrame", func(t *testing.T) {
		_, err := cameraServer.GetFrame(context.Background(), &pb.GetFrameRequest{Name: missingCameraName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")

		_, err = cameraServer.GetFrame(context.Background(), &pb.GetFrameRequest{Name: fakeCameraName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a camera")
		// color camera
		resp, err := cameraServer.GetFrame(
			context.Background(),
			&pb.GetFrameRequest{Name: testCameraName, MimeType: utils.MimeTypeRawRGBA},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imageReleased, test.ShouldBeTrue)
		test.That(t, resp.MimeType, test.ShouldEqual, utils.MimeTypeRawRGBA)
		test.That(t, resp.Image, test.ShouldResemble, img.Pix)

		resp, err = cameraServer.GetFrame(
			context.Background(),
			&pb.GetFrameRequest{Name: testCameraName, MimeType: ""},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imageReleased, test.ShouldBeTrue)
		test.That(t, resp.MimeType, test.ShouldEqual, utils.MimeTypeRawRGBA)
		test.That(t, resp.Image, test.ShouldNotBeNil)

		imageReleased = false
		resp, err = cameraServer.GetFrame(context.Background(), &pb.GetFrameRequest{
			Name:     testCameraName,
			MimeType: "image/png",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imageReleased, test.ShouldBeTrue)
		test.That(t, resp.MimeType, test.ShouldEqual, utils.MimeTypePNG)
		test.That(t, resp.Image, test.ShouldResemble, imgBuf.Bytes())

		imageReleased = false
		_, err = cameraServer.GetFrame(context.Background(), &pb.GetFrameRequest{
			Name:     testCameraName,
			MimeType: "image/who",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how to encode")
		test.That(t, imageReleased, test.ShouldBeTrue)
		// depth camera
		imageReleased = false
		resp, err = cameraServer.GetFrame(
			context.Background(),
			&pb.GetFrameRequest{Name: depthCameraName, MimeType: ""},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imageReleased, test.ShouldBeTrue)
		test.That(t, resp.MimeType, test.ShouldEqual, utils.MimeTypeRawRGBA)
		test.That(t, resp.Image, test.ShouldNotBeNil)
		decodedDepth, err := rimage.DecodeImage(
			context.Background(),
			resp.Image,
			resp.MimeType,
			int(resp.WidthPx), int(resp.HeightPx),
		)
		test.That(t, err, test.ShouldBeNil)
		dm, err := rimage.ConvertImageToDepthMap(decodedDepth)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dm, test.ShouldResemble, depthImage)

		imageReleased = false
		resp, err = cameraServer.GetFrame(context.Background(), &pb.GetFrameRequest{
			Name:     depthCameraName,
			MimeType: "image/png",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imageReleased, test.ShouldBeTrue)
		test.That(t, resp.MimeType, test.ShouldEqual, utils.MimeTypePNG)
		test.That(t, resp.Image, test.ShouldResemble, depthBuf.Bytes())
		// bad camera
		_, err = cameraServer.GetFrame(context.Background(), &pb.GetFrameRequest{Name: failCameraName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't generate frame")
	})

	t.Run("RenderFrame", func(t *testing.T) {
		_, err := cameraServer.RenderFrame(context.Background(), &pb.RenderFrameRequest{Name: missingCameraName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")

		resp, err := cameraServer.RenderFrame(context.Background(), &pb.RenderFrameRequest{
			Name: testCameraName,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imageReleased, test.ShouldBeTrue)
		test.That(t, resp.ContentType, test.ShouldEqual, "image/jpeg")
		test.That(t, resp.Data, test.ShouldResemble, imgBufJpeg.Bytes())

		imageReleased = false
		resp, err = cameraServer.RenderFrame(context.Background(), &pb.RenderFrameRequest{
			Name:     testCameraName,
			MimeType: "image/png",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imageReleased, test.ShouldBeTrue)
		test.That(t, resp.ContentType, test.ShouldEqual, "image/png")
		test.That(t, resp.Data, test.ShouldResemble, imgBuf.Bytes())

		imageReleased = false
		_, err = cameraServer.RenderFrame(context.Background(), &pb.RenderFrameRequest{
			Name:     testCameraName,
			MimeType: "image/who",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how to encode")
		test.That(t, imageReleased, test.ShouldBeTrue)

		_, err = cameraServer.RenderFrame(context.Background(), &pb.RenderFrameRequest{Name: failCameraName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't generate frame")
	})

	t.Run("GetPointCloud", func(t *testing.T) {
		_, err := cameraServer.GetPointCloud(context.Background(), &pb.GetPointCloudRequest{Name: missingCameraName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")

		pcA := pointcloud.New()
		err = pcA.Set(pointcloud.NewVector(5, 5, 5), nil)
		test.That(t, err, test.ShouldBeNil)

		injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
			return pcA, nil
		}
		_, err = cameraServer.GetPointCloud(context.Background(), &pb.GetPointCloudRequest{
			Name: testCameraName,
		})
		test.That(t, err, test.ShouldBeNil)

		_, err = cameraServer.GetPointCloud(context.Background(), &pb.GetPointCloudRequest{
			Name: failCameraName,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't generate next point cloud")
	})

	t.Run("GetProperties", func(t *testing.T) {
		_, err := cameraServer.GetProperties(context.Background(), &pb.GetPropertiesRequest{Name: missingCameraName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")

		_, err = cameraServer.GetProperties(context.Background(), &pb.GetPropertiesRequest{Name: fakeCameraName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a camera")

		resp, err := cameraServer.GetProperties(context.Background(), &pb.GetPropertiesRequest{Name: testCameraName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.IntrinsicParameters.WidthPx, test.ShouldEqual, 1280)
		test.That(t, resp.IntrinsicParameters.HeightPx, test.ShouldEqual, 720)
		test.That(t, resp.IntrinsicParameters.FocalXPx, test.ShouldEqual, 200)
		test.That(t, resp.IntrinsicParameters.FocalYPx, test.ShouldEqual, 200)
		test.That(t, resp.IntrinsicParameters.CenterXPx, test.ShouldEqual, 100)
		test.That(t, resp.IntrinsicParameters.CenterYPx, test.ShouldEqual, 100)
	})
}
