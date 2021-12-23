package camera_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/pointcloud"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

func newServer() (pb.CameraServiceServer, *inject.Camera, *inject.Camera, error) {
	injectCamera := &inject.Camera{}
	injectCamera2 := &inject.Camera{}
	cameras := map[resource.Name]interface{}{
		camera.Named("camera1"): injectCamera,
		camera.Named("camera2"): injectCamera2,
		camera.Named("camera3"): "notCamera",
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

	camera1 := "camera1"
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var imgBuf bytes.Buffer
	test.That(t, png.Encode(&imgBuf, img), test.ShouldBeNil)

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
	injectCamera2.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return nil, nil, errors.New("can't generate next frame")
	}
	injectCamera2.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return nil, errors.New("can't generate next point cloud")
	}
	t.Run("Frame", func(t *testing.T) {
		_, err := cameraServer.Frame(context.Background(), &pb.CameraServiceFrameRequest{Name: "g4"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")

		_, err = cameraServer.Frame(context.Background(), &pb.CameraServiceFrameRequest{Name: "camera3"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a camera")

		resp, err := cameraServer.Frame(context.Background(), &pb.CameraServiceFrameRequest{Name: camera1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imageReleased, test.ShouldBeTrue)
		test.That(t, resp.MimeType, test.ShouldEqual, "image/png")
		test.That(t, resp.Frame, test.ShouldResemble, imgBuf.Bytes())

		imageReleased = false
		resp, err = cameraServer.Frame(context.Background(), &pb.CameraServiceFrameRequest{
			Name:     camera1,
			MimeType: "image/png",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imageReleased, test.ShouldBeTrue)
		test.That(t, resp.MimeType, test.ShouldEqual, "image/png")
		test.That(t, resp.Frame, test.ShouldResemble, imgBuf.Bytes())

		imageReleased = false
		_, err = cameraServer.Frame(context.Background(), &pb.CameraServiceFrameRequest{
			Name:     camera1,
			MimeType: "image/who",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how")
		test.That(t, imageReleased, test.ShouldBeTrue)

		_, err = cameraServer.Frame(context.Background(), &pb.CameraServiceFrameRequest{Name: camera2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't generate next frame")
	})

	t.Run("RenderFrame", func(t *testing.T) {
		_, err := cameraServer.RenderFrame(context.Background(), &pb.CameraServiceRenderFrameRequest{Name: "g4"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")

		resp, err := cameraServer.RenderFrame(context.Background(), &pb.CameraServiceRenderFrameRequest{
			Name: camera1,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imageReleased, test.ShouldBeTrue)
		test.That(t, resp.ContentType, test.ShouldEqual, "image/png")
		test.That(t, resp.Data, test.ShouldResemble, imgBuf.Bytes())

		imageReleased = false
		resp, err = cameraServer.RenderFrame(context.Background(), &pb.CameraServiceRenderFrameRequest{
			Name:     camera1,
			MimeType: "image/png",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imageReleased, test.ShouldBeTrue)
		test.That(t, resp.ContentType, test.ShouldEqual, "image/png")
		test.That(t, resp.Data, test.ShouldResemble, imgBuf.Bytes())

		imageReleased = false
		_, err = cameraServer.RenderFrame(context.Background(), &pb.CameraServiceRenderFrameRequest{
			Name:     camera1,
			MimeType: "image/who",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how")
		test.That(t, imageReleased, test.ShouldBeTrue)

		_, err = cameraServer.RenderFrame(context.Background(), &pb.CameraServiceRenderFrameRequest{Name: camera2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't generate next frame")
	})

	t.Run("PointCloud", func(t *testing.T) {
		_, err := cameraServer.PointCloud(context.Background(), &pb.CameraServicePointCloudRequest{Name: "g4"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")

		pcA := pointcloud.New()
		err = pcA.Set(pointcloud.NewBasicPoint(5, 5, 5))
		test.That(t, err, test.ShouldBeNil)

		injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
			return pcA, nil
		}
		_, err = cameraServer.PointCloud(context.Background(), &pb.CameraServicePointCloudRequest{
			Name: camera1,
		})
		test.That(t, err, test.ShouldBeNil)

		_, err = cameraServer.PointCloud(context.Background(), &pb.CameraServicePointCloudRequest{
			Name: camera2,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't generate next point cloud")
	})

	t.Run("ObjectPointClouds", func(t *testing.T) {
		_, err := cameraServer.ObjectPointClouds(context.Background(), &pb.CameraServiceObjectPointCloudsRequest{Name: "g4"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")

		// request the two segments in the point cloud
		pcA := pointcloud.New()
		err = pcA.Set(pointcloud.NewBasicPoint(5, 5, 5))
		test.That(t, err, test.ShouldBeNil)
		err = pcA.Set(pointcloud.NewBasicPoint(5, 5, 6))
		test.That(t, err, test.ShouldBeNil)
		err = pcA.Set(pointcloud.NewBasicPoint(5, 5, 4))
		test.That(t, err, test.ShouldBeNil)
		err = pcA.Set(pointcloud.NewBasicPoint(-5, -5, 5))
		test.That(t, err, test.ShouldBeNil)
		err = pcA.Set(pointcloud.NewBasicPoint(-5, -5, 6))
		test.That(t, err, test.ShouldBeNil)
		err = pcA.Set(pointcloud.NewBasicPoint(-5, -5, 4))
		test.That(t, err, test.ShouldBeNil)

		injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
			return pcA, nil
		}
		segs, err := cameraServer.ObjectPointClouds(context.Background(), &pb.CameraServiceObjectPointCloudsRequest{
			Name:               camera1,
			MinPointsInPlane:   100,
			MinPointsInSegment: 3,
			ClusteringRadius:   5.,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(segs.Frames), test.ShouldEqual, 2)
		test.That(t, segs.Centers[0].Z, test.ShouldEqual, 5.)
		test.That(t, segs.Centers[1].Z, test.ShouldEqual, 5.)
		test.That(t, segs.BoundingBoxes[0].Width, test.ShouldEqual, 0)
		test.That(t, segs.BoundingBoxes[0].Length, test.ShouldEqual, 0)
		test.That(t, segs.BoundingBoxes[0].Depth, test.ShouldEqual, 2)
		test.That(t, segs.BoundingBoxes[1].Width, test.ShouldEqual, 0)
		test.That(t, segs.BoundingBoxes[1].Length, test.ShouldEqual, 0)
		test.That(t, segs.BoundingBoxes[1].Depth, test.ShouldEqual, 2)

		//empty pointcloud
		pcB := pointcloud.New()

		injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
			return pcB, nil
		}
		segs, err = cameraServer.ObjectPointClouds(context.Background(), &pb.CameraServiceObjectPointCloudsRequest{
			Name:               camera1,
			MinPointsInPlane:   100,
			MinPointsInSegment: 3,
			ClusteringRadius:   5.,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(segs.Frames), test.ShouldEqual, 0)

		_, err = cameraServer.ObjectPointClouds(context.Background(), &pb.CameraServiceObjectPointCloudsRequest{
			Name:               camera2,
			MinPointsInPlane:   100,
			MinPointsInSegment: 3,
			ClusteringRadius:   5.,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't generate next point cloud")
	})

}
