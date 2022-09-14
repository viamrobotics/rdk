// Package slam_test server_test.go tests the SLAM service's GRPC server.
package slam_test

import (
	"context"
	"errors"
	"image"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/pointcloud"
	pb "go.viam.com/rdk/proto/api/service/slam/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

const (
	testSlamServiceName  = "slam1"
	testSlamServiceName2 = "slam2"
)

func TestServer(t *testing.T) {
	injectSvc := &inject.SLAMService{}
	resourceMap := map[resource.Name]interface{}{
		slam.Named(testSlamServiceName): injectSvc,
	}
	injectSubtypeSvc, err := subtype.New(resourceMap)
	test.That(t, err, test.ShouldBeNil)
	slamServer := slam.NewServer(injectSubtypeSvc)

	t.Run("working get position functions", func(t *testing.T) {
		pose := spatial.NewPoseFromOrientation(r3.Vector{1, 2, 3}, &spatial.OrientationVector{math.Pi / 2, 0, 0, -1})
		pSucc := referenceframe.NewPoseInFrame("frame", pose)

		injectSvc.GetPositionFunc = func(ctx context.Context, name string) (*referenceframe.PoseInFrame, error) {
			return pSucc, nil
		}

		reqPos := &pb.GetPositionRequest{
			Name: testSlamServiceName,
		}
		respPos, err := slamServer.GetPosition(context.Background(), reqPos)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, referenceframe.ProtobufToPoseInFrame(respPos.Pose).FrameName(), test.ShouldEqual, pSucc.FrameName())
	})

	t.Run("working get map function", func(t *testing.T) {
		pose := spatial.NewPoseFromOrientation(r3.Vector{1, 2, 3}, &spatial.OrientationVector{math.Pi / 2, 0, 0, -1})
		pSucc := referenceframe.NewPoseInFrame("frame", pose)
		pcSucc := &vision.Object{}
		pcSucc.PointCloud = pointcloud.New()
		err = pcSucc.PointCloud.Set(pointcloud.NewVector(5, 5, 5), nil)
		test.That(t, err, test.ShouldBeNil)
		imSucc := image.NewNRGBA(image.Rect(0, 0, 4, 4))

		injectSvc.GetMapFunc = func(ctx context.Context, name, mimeType string, cp *referenceframe.PoseInFrame,
			include bool,
		) (string, image.Image, *vision.Object, error) {
			return mimeType, imSucc, pcSucc, nil
		}

		reqMap := &pb.GetMapRequest{
			Name:               testSlamServiceName,
			MimeType:           utils.MimeTypePCD,
			CameraPosition:     referenceframe.PoseInFrameToProtobuf(pSucc).Pose,
			IncludeRobotMarker: true,
		}
		respMap, err := slamServer.GetMap(context.Background(), reqMap)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, respMap.MimeType, test.ShouldEqual, utils.MimeTypePCD)

		reqMap = &pb.GetMapRequest{
			Name:               testSlamServiceName,
			MimeType:           utils.MimeTypeJPEG,
			CameraPosition:     referenceframe.PoseInFrameToProtobuf(pSucc).Pose,
			IncludeRobotMarker: true,
		}
		respMap, err = slamServer.GetMap(context.Background(), reqMap)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, respMap.MimeType, test.ShouldEqual, utils.MimeTypeJPEG)
	})

	t.Run("failing get position function", func(t *testing.T) {
		injectSvc.GetPositionFunc = func(ctx context.Context, name string) (*referenceframe.PoseInFrame, error) {
			return nil, errors.New("failure to get position")
		}

		req := &pb.GetPositionRequest{
			Name: testSlamServiceName,
		}
		resp, err := slamServer.GetPosition(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("failing get map function", func(t *testing.T) {
		pose := spatial.NewPoseFromOrientation(r3.Vector{1, 2, 3}, &spatial.OrientationVector{math.Pi / 2, 0, 0, -1})

		injectSvc.GetMapFunc = func(ctx context.Context, name, mimeType string, cp *referenceframe.PoseInFrame,
			include bool,
		) (string, image.Image, *vision.Object, error) {
			return mimeType, nil, nil, errors.New("failure to get map")
		}

		req := &pb.GetMapRequest{MimeType: utils.MimeTypeJPEG, CameraPosition: spatial.PoseToProtobuf(pose)}
		resp, err := slamServer.GetMap(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
	})

	resourceMap = map[resource.Name]interface{}{
		slam.Named(testSlamServiceName): "not a frame system",
	}
	injectSubtypeSvc, _ = subtype.New(resourceMap)
	slamServer = slam.NewServer(injectSubtypeSvc)

	t.Run("failing on improper service interface", func(t *testing.T) {
		improperImplErr := slam.NewUnimplementedInterfaceError("string")

		getPositionReq := &pb.GetPositionRequest{Name: testSlamServiceName}
		getModeResp, err := slamServer.GetPosition(context.Background(), getPositionReq)
		test.That(t, getModeResp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, improperImplErr)

		getMapReq := &pb.GetMapRequest{Name: testSlamServiceName}
		setModeResp, err := slamServer.GetMap(context.Background(), getMapReq)
		test.That(t, err, test.ShouldBeError, improperImplErr)
		test.That(t, setModeResp, test.ShouldBeNil)
	})

	injectSubtypeSvc, _ = subtype.New(map[resource.Name]interface{}{})
	slamServer = slam.NewServer(injectSubtypeSvc)
	t.Run("failing on nonexistent server", func(t *testing.T) {
		req := &pb.GetPositionRequest{
			Name: testSlamServiceName,
		}
		resp, err := slamServer.GetPosition(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(slam.Named(testSlamServiceName)))
	})
	t.Run("Multiple services Valid", func(t *testing.T) {
		resourceMap = map[resource.Name]interface{}{
			slam.Named(testSlamServiceName):  injectSvc,
			slam.Named(testSlamServiceName2): injectSvc,
		}
		injectSubtypeSvc, err := subtype.New(resourceMap)
		test.That(t, err, test.ShouldBeNil)
		slamServer = slam.NewServer(injectSubtypeSvc)
		pose := spatial.NewPoseFromOrientation(r3.Vector{1, 2, 3}, &spatial.OrientationVector{math.Pi / 2, 0, 0, -1})
		pSucc := referenceframe.NewPoseInFrame("frame", pose)
		injectSvc.GetPositionFunc = func(ctx context.Context, name string) (*referenceframe.PoseInFrame, error) {
			return pSucc, nil
		}

		reqPos := &pb.GetPositionRequest{
			Name: testSlamServiceName,
		}
		respPos, err := slamServer.GetPosition(context.Background(), reqPos)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, referenceframe.ProtobufToPoseInFrame(respPos.Pose).FrameName(), test.ShouldEqual, pSucc.FrameName())
		reqPos = &pb.GetPositionRequest{
			Name: testSlamServiceName2,
		}
		respPos, err = slamServer.GetPosition(context.Background(), reqPos)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, referenceframe.ProtobufToPoseInFrame(respPos.Pose).FrameName(), test.ShouldEqual, pSucc.FrameName())
	})
}
