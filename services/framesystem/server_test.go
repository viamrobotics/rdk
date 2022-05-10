package framesystem_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/framesystem/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/framesystem"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

func TestServerConfig(t *testing.T) {
	injectSvc := &inject.FrameSystemService{}
	resourceMap := map[resource.Name]interface{}{
		framesystem.Name: injectSvc,
	}
	injectSubtypeSvc, err := subtype.New(resourceMap)
	test.That(t, err, test.ShouldBeNil)
	fsServer := framesystem.NewServer(injectSubtypeSvc)

	// test working config function
	t.Run("test working config function", func(t *testing.T) {
		fsConfigs := []*config.FrameSystemPart{
			{
				Name: "frame1",
				FrameConfig: &config.Frame{
					Parent:      referenceframe.World,
					Translation: spatialmath.TranslationConfig{X: 1, Y: 2, Z: 3},
					Orientation: &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1},
				},
			},
			{
				Name: "frame2",
				FrameConfig: &config.Frame{
					Parent:      "frame1",
					Translation: spatialmath.TranslationConfig{X: 1, Y: 2, Z: 3},
				},
			},
		}

		injectSvc.ConfigFunc = func(
			ctx context.Context, additionalTransforms []*commonpb.Transform,
		) (framesystem.Parts, error) {
			return framesystem.Parts(fsConfigs), nil
		}
		req := &pb.ConfigRequest{}
		resp, err := fsServer.Config(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.FrameSystemConfigs), test.ShouldEqual, len(fsConfigs))
		test.That(t, resp.FrameSystemConfigs[0].Name, test.ShouldEqual, fsConfigs[0].Name)
		test.That(t, resp.FrameSystemConfigs[0].PoseInParentFrame.ReferenceFrame, test.ShouldEqual, fsConfigs[0].FrameConfig.Parent)
		test.That(t,
			resp.FrameSystemConfigs[0].PoseInParentFrame.Pose.X,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Translation.X,
		)
		test.That(t,
			resp.FrameSystemConfigs[0].PoseInParentFrame.Pose.Y,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Translation.Y,
		)
		test.That(t,
			resp.FrameSystemConfigs[0].PoseInParentFrame.Pose.Z,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Translation.Z,
		)
		test.That(t,
			resp.FrameSystemConfigs[0].PoseInParentFrame.Pose.OX,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Orientation.OrientationVectorDegrees().OX,
		)
		test.That(t,
			resp.FrameSystemConfigs[0].PoseInParentFrame.Pose.OY,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Orientation.OrientationVectorDegrees().OY,
		)
		test.That(t,
			resp.FrameSystemConfigs[0].PoseInParentFrame.Pose.OZ,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Orientation.OrientationVectorDegrees().OZ,
		)
		test.That(t,
			resp.FrameSystemConfigs[0].PoseInParentFrame.Pose.Theta,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Orientation.OrientationVectorDegrees().Theta,
		)
	})

	t.Run("test failing config function", func(t *testing.T) {
		expectedErr := errors.New("failed to retrieve config")
		injectSvc.ConfigFunc = func(
			ctx context.Context, additionalTransforms []*commonpb.Transform,
		) (framesystem.Parts, error) {
			return nil, expectedErr
		}
		req := &pb.ConfigRequest{}
		resp, err := fsServer.Config(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, expectedErr)
	})

	resourceMap = map[resource.Name]interface{}{
		framesystem.Name: "not a frame system",
	}
	injectSubtypeSvc, _ = subtype.New(resourceMap)
	fsServer = framesystem.NewServer(injectSubtypeSvc)

	t.Run("test failing on improper service interface", func(t *testing.T) {
		req := &pb.ConfigRequest{}
		resp, err := fsServer.Config(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	injectSubtypeSvc, _ = subtype.New(map[resource.Name]interface{}{})
	fsServer = framesystem.NewServer(injectSubtypeSvc)

	t.Run("test failing on nonexistent server", func(t *testing.T) {
		req := &pb.ConfigRequest{}
		resp, err := fsServer.Config(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
	})
}
