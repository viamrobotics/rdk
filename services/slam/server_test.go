package slam_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/slam/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func TestServer(t *testing.T) {
	injectSvc := &inject.SLAMService{}
	resourceMap := map[resource.Name]interface{}{
		slam.Name: injectSvc,
	}
	injectSubtypeSvc, err := subtype.New(resourceMap)
	test.That(t, err, test.ShouldBeNil)
	slamServer := slam.NewServer(injectSubtypeSvc)

	t.Run("working get position functions", func(t *testing.T) {
		pSucc := &commonpb.PoseInFrame{}

		injectSvc.CloseFunc = func() error {
			return nil
		}

		injectSvc.GetPositionFunc = func(ctx context.Context, name string) (*commonpb.PoseInFrame, error) {
			return pSucc, nil
		}

		reqPos := &pb.GetPositionRequest{
			Name: "viam",
		}
		respPos, err := slamServer.GetPosition(context.Background(), reqPos)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, respPos.Pose, test.ShouldEqual, pSucc)
	})

	t.Run("working get map function", func(t *testing.T) {
		pSucc := &commonpb.PoseInFrame{}
		pcSucc := &commonpb.PointCloudObject{}
		imSucc := []byte{}

		injectSvc.CloseFunc = func() error {
			return nil
		}

		injectSvc.GetMapFunc = func(ctx context.Context, name string, mimeType string, cp *commonpb.Pose,
			include bool) (string, []byte, *commonpb.PointCloudObject, error) {
			return mimeType, imSucc, pcSucc, nil
		}

		reqMap := &pb.GetMapRequest{
			Name:               "viam",
			MimeType:           utils.MimeTypePCD,
			CameraPosition:     pSucc.Pose,
			IncludeRobotMarker: true,
		}
		respMap, err := slamServer.GetMap(context.Background(), reqMap)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, respMap.MimeType, test.ShouldEqual, utils.MimeTypePCD)
		test.That(t, respMap.GetPointCloud(), test.ShouldResemble, pcSucc)

		reqMap = &pb.GetMapRequest{
			Name:               "viam",
			MimeType:           utils.MimeTypeJPEG,
			CameraPosition:     pSucc.Pose,
			IncludeRobotMarker: true,
		}
		respMap, err = slamServer.GetMap(context.Background(), reqMap)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, respMap.MimeType, test.ShouldEqual, utils.MimeTypeJPEG)
		test.That(t, respMap.GetImage(), test.ShouldResemble, imSucc)
	})

	t.Run("failing get position function", func(t *testing.T) {
		injectSvc.CloseFunc = func() error {
			return errors.New("failure to close")
		}

		injectSvc.GetPositionFunc = func(ctx context.Context, name string) (*commonpb.PoseInFrame, error) {
			return nil, errors.New("failure to get position")
		}

		req := &pb.GetPositionRequest{
			Name: "viam",
		}
		resp, err := slamServer.GetPosition(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("failing get map function", func(t *testing.T) {
		injectSvc.CloseFunc = func() error {
			return errors.New("failure to close")
		}

		injectSvc.GetMapFunc = func(ctx context.Context, name string, mimeType string, cp *commonpb.Pose,
			include bool) (string, []byte, *commonpb.PointCloudObject, error) {
			return mimeType, nil, nil, errors.New("failure to get map")
		}

		req := &pb.GetMapRequest{MimeType: utils.MimeTypeJPEG}
		resp, err := slamServer.GetMap(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
	})

	resourceMap = map[resource.Name]interface{}{
		slam.Name: "not a frame system",
	}
	injectSubtypeSvc, _ = subtype.New(resourceMap)
	slamServer = slam.NewServer(injectSubtypeSvc)

	t.Run("failing on improper service interface", func(t *testing.T) {
		improperImplErr := utils.NewUnimplementedInterfaceError("slam.Service", "string")

		getPositionReq := &pb.GetPositionRequest{}
		getModeResp, err := slamServer.GetPosition(context.Background(), getPositionReq)
		test.That(t, getModeResp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, improperImplErr)

		getMapReq := &pb.GetMapRequest{}
		setModeResp, err := slamServer.GetMap(context.Background(), getMapReq)
		test.That(t, err, test.ShouldBeError, improperImplErr)
		test.That(t, setModeResp, test.ShouldBeNil)
	})

	injectSubtypeSvc, _ = subtype.New(map[resource.Name]interface{}{})
	slamServer = slam.NewServer(injectSubtypeSvc)
	t.Run("failing on nonexistent server", func(t *testing.T) {
		req := &pb.GetPositionRequest{}
		resp, err := slamServer.GetPosition(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(slam.Name))
	})
}
