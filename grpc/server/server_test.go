package server_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/grpc/server"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/robot/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

var emptyResources = &pb.ResourceNamesResponse{
	Resources: []*commonpb.ResourceName{},
}

var serverNewResource = resource.NewName(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	arm.SubtypeName,
	"",
)

var serverOneResourceResponse = []*commonpb.ResourceName{
	{
		Namespace: string(serverNewResource.Namespace),
		Type:      string(serverNewResource.ResourceType),
		Subtype:   string(serverNewResource.ResourceSubtype),
		Name:      serverNewResource.Name,
	},
}

func TestServer(t *testing.T) {
	t.Run("Metadata", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{} }
		server := server.New(injectRobot)

		resourceResp, err := server.ResourceNames(context.Background(), &pb.ResourceNamesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp, test.ShouldResemble, emptyResources)

		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{serverNewResource} }

		resourceResp, err = server.ResourceNames(context.Background(), &pb.ResourceNamesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp.Resources, test.ShouldResemble, serverOneResourceResponse)
	})

	t.Run("Discovery", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{} }
		server := server.New(injectRobot)
		q := discovery.Query{arm.Named("arm").ResourceSubtype, "some arm"}
		disc := discovery.Discovery{Query: q, Discovered: struct{}{}}
		discoveries := []discovery.Discovery{disc}
		injectRobot.DiscoverComponentsFunc = func(ctx context.Context, keys []discovery.Query) ([]discovery.Discovery, error) {
			return discoveries, nil
		}
		req := &pb.DiscoverComponentsRequest{
			Queries: []*pb.Query{{Subtype: string(q.SubtypeName), Model: q.Model}},
		}

		resp, err := server.DiscoverComponents(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.Discovery), test.ShouldEqual, 1)

		observed := resp.Discovery[0].Discovered.AsMap()
		expected := map[string]interface{}{}
		test.That(t, observed, test.ShouldResemble, expected)
	})
}

func TestServerFrameSystemConfig(t *testing.T) {
	injectRobot := &inject.Robot{}

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

		injectRobot.FrameSystemConfigFunc = func(
			ctx context.Context, additionalTransforms []*commonpb.Transform,
		) (framesystemparts.Parts, error) {
			return framesystemparts.Parts(fsConfigs), nil
		}
		server := server.New(injectRobot)
		req := &pb.FrameSystemConfigRequest{}
		resp, err := server.FrameSystemConfig(context.Background(), req)
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
		injectRobot.FrameSystemConfigFunc = func(
			ctx context.Context, additionalTransforms []*commonpb.Transform,
		) (framesystemparts.Parts, error) {
			return nil, expectedErr
		}
		req := &pb.FrameSystemConfigRequest{}
		server := server.New(injectRobot)
		resp, err := server.FrameSystemConfig(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, expectedErr)
	})

	injectRobot = &inject.Robot{}

	t.Run("test failing on nonexistent server", func(t *testing.T) {
		req := &pb.FrameSystemConfigRequest{}
		server := server.New(injectRobot)
		test.That(t, func() { server.FrameSystemConfig(context.Background(), req) }, test.ShouldPanic)
	})
}
