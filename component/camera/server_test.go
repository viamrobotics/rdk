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
)

func newServer() (pb.CameraServiceServer, *inject.Camera, *inject.Camera, error) {
	injectCamera := &inject.Camera{}
	injectCamera2 := &inject.Camera{}
	cameras := map[resource.Name]interface{}{
		camera.Named(testCameraName): injectCamera,
		camera.Named(failCameraName): injectCamera2,
		camera.Named(fakeCameraName): "notCamera",
	}
	cameraSvc, err := subtype.New(cameras)
	if err != nil {
		return nil, nil, nil, err
	}
	return camera.NewServer(cameraSvc), injectCamera, injectCamera2, nil
}

func TestServer(t *testing.T) {
	cameraServer, injectCamera, injectCamera2, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var imgBuf bytes.Buffer
	test.That(t, png.Encode(&imgBuf, img), test.ShouldBeNil)
	var imgBufJpeg bytes.Buffer
	test.That(t, jpeg.Encode(&imgBufJpeg, img, nil), test.ShouldBeNil)

	pcA := pointcloud.New()
	err = pcA.Set(pointcloud.NewVector(5, 5, 5), nil)
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

	var imageReleased bool
	injectCamera.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return img, func() { imageReleased = true }, nil
	}
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pcA, nil
	}
	injectCamera.GetPropertiesFunc = func(ctx context.Context) (rimage.Projector, error) {
		return projA, nil
	}

	injectCamera2.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return nil, nil, errors.New("can't generate next frame")
	}
	injectCamera2.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return nil, errors.New("can't generate next point cloud")
	}
	injectCamera2.GetPropertiesFunc = func(ctx context.Context) (rimage.Projector, error) {
		return nil, errors.New("can't get camera properties")
	}
	t.Run("GetFrame", func(t *testing.T) {
		_, err := cameraServer.GetFrame(context.Background(), &pb.GetFrameRequest{Name: missingCameraName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")

		_, err = cameraServer.GetFrame(context.Background(), &pb.GetFrameRequest{Name: fakeCameraName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a camera")

		resp, err := cameraServer.GetFrame(context.Background(), &pb.GetFrameRequest{Name: testCameraName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imageReleased, test.ShouldBeTrue)
		test.That(t, resp.MimeType, test.ShouldEqual, "image/raw-rgba")
		test.That(t, resp.Image, test.ShouldResemble, img.Pix)

		imageReleased = false
		resp, err = cameraServer.GetFrame(context.Background(), &pb.GetFrameRequest{
			Name:     testCameraName,
			MimeType: "image/png",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imageReleased, test.ShouldBeTrue)
		test.That(t, resp.MimeType, test.ShouldEqual, "image/png")
		test.That(t, resp.Image, test.ShouldResemble, imgBuf.Bytes())

		imageReleased = false
		_, err = cameraServer.GetFrame(context.Background(), &pb.GetFrameRequest{
			Name:     testCameraName,
			MimeType: "image/who",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how")
		test.That(t, imageReleased, test.ShouldBeTrue)

		_, err = cameraServer.GetFrame(context.Background(), &pb.GetFrameRequest{Name: failCameraName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't generate next frame")
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
		test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how")
		test.That(t, imageReleased, test.ShouldBeTrue)

		_, err = cameraServer.RenderFrame(context.Background(), &pb.RenderFrameRequest{Name: failCameraName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't generate next frame")
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
		test.That(t, resp.IntrinsicParameters.WidthPixels, test.ShouldEqual, 1280)
		test.That(t, resp.IntrinsicParameters.HeightPixels, test.ShouldEqual, 720)
		test.That(t, resp.IntrinsicParameters.FocalXPixels, test.ShouldEqual, 200)
		test.That(t, resp.IntrinsicParameters.FocalYPixels, test.ShouldEqual, 200)
		test.That(t, resp.IntrinsicParameters.CenterXPixels, test.ShouldEqual, 100)
		test.That(t, resp.IntrinsicParameters.CenterYPixels, test.ShouldEqual, 100)
	})
}
