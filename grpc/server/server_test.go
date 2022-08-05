package server_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/jhump/protoreflect/grpcreflect"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/movementsensor"
	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/grpc/server"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	armpb "go.viam.com/rdk/proto/api/component/arm/v1"
	pb "go.viam.com/rdk/proto/api/robot/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
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
		injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{} }
		server := server.New(injectRobot)

		resourceResp, err := server.ResourceNames(context.Background(), &pb.ResourceNamesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp, test.ShouldResemble, emptyResources)

		injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{serverNewResource} }

		resourceResp, err = server.ResourceNames(context.Background(), &pb.ResourceNamesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp.Resources, test.ShouldResemble, serverOneResourceResponse)
	})

	t.Run("Discovery", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{} }
		server := server.New(injectRobot)
		q := discovery.Query{arm.Named("arm").ResourceSubtype, "some arm"}
		disc := discovery.Discovery{Query: q, Results: struct{}{}}
		discoveries := []discovery.Discovery{disc}
		injectRobot.DiscoverComponentsFunc = func(ctx context.Context, keys []discovery.Query) ([]discovery.Discovery, error) {
			return discoveries, nil
		}
		req := &pb.DiscoverComponentsRequest{
			Queries: []*pb.DiscoveryQuery{{Subtype: string(q.SubtypeName), Model: q.Model}},
		}

		resp, err := server.DiscoverComponents(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.Discovery), test.ShouldEqual, 1)

		observed := resp.Discovery[0].Results.AsMap()
		expected := map[string]interface{}{}
		test.That(t, observed, test.ShouldResemble, expected)
	})

	t.Run("ResourceRPCSubtypes", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return nil }
		server := server.New(injectRobot)

		typesResp, err := server.ResourceRPCSubtypes(context.Background(), &pb.ResourceRPCSubtypesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, typesResp, test.ShouldResemble, &pb.ResourceRPCSubtypesResponse{
			ResourceRpcSubtypes: []*pb.ResourceRPCSubtype{},
		})

		desc1, err := grpcreflect.LoadServiceDescriptor(&pb.RobotService_ServiceDesc)
		test.That(t, err, test.ShouldBeNil)

		desc2, err := grpcreflect.LoadServiceDescriptor(&armpb.ArmService_ServiceDesc)
		test.That(t, err, test.ShouldBeNil)

		otherSubType := resource.NewSubtype("acme", resource.ResourceTypeComponent, "wat")
		respWith := []resource.RPCSubtype{
			{
				Subtype: serverNewResource.Subtype,
				Desc:    desc1,
			},
			{
				Subtype: resource.NewSubtype("acme", resource.ResourceTypeComponent, "wat"),
				Desc:    desc2,
			},
		}

		expectedResp := []*pb.ResourceRPCSubtype{
			{
				Subtype: &commonpb.ResourceName{
					Namespace: string(serverNewResource.Namespace),
					Type:      string(serverNewResource.ResourceType),
					Subtype:   string(serverNewResource.ResourceSubtype),
				},
				ProtoService: desc1.GetFullyQualifiedName(),
			},
			{
				Subtype: &commonpb.ResourceName{
					Namespace: string(otherSubType.Namespace),
					Type:      string(otherSubType.ResourceType),
					Subtype:   string(otherSubType.ResourceSubtype),
				},
				ProtoService: desc2.GetFullyQualifiedName(),
			},
		}

		injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return respWith }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return nil }

		typesResp, err = server.ResourceRPCSubtypes(context.Background(), &pb.ResourceRPCSubtypesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, typesResp.ResourceRpcSubtypes, test.ShouldResemble, expectedResp)
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

func TestServerGetStatus(t *testing.T) {
	t.Run("failed GetStatus", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		server := server.New(injectRobot)
		passedErr := errors.New("can't get status")
		injectRobot.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
			return nil, passedErr
		}
		_, err := server.GetStatus(context.Background(), &pb.GetStatusRequest{})
		test.That(t, err, test.ShouldBeError, passedErr)
	})

	t.Run("bad status response", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		server := server.New(injectRobot)
		aStatus := robot.Status{Name: arm.Named("arm"), Status: 1}
		readings := []robot.Status{aStatus}
		injectRobot.GetStatusFunc = func(ctx context.Context, status []resource.Name) ([]robot.Status, error) {
			return readings, nil
		}
		req := &pb.GetStatusRequest{
			ResourceNames: []*commonpb.ResourceName{},
		}

		_, err := server.GetStatus(context.Background(), req)
		test.That(
			t,
			err,
			test.ShouldBeError,
			errors.New(
				"unable to convert status for \"rdk:component:arm/arm\" to a form acceptable to structpb.NewStruct: "+
					"data of type int and kind int not a struct or a map-like object",
			),
		)
	})

	t.Run("working one status", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		server := server.New(injectRobot)
		aStatus := robot.Status{Name: arm.Named("arm"), Status: struct{}{}}
		readings := []robot.Status{aStatus}
		expected := map[resource.Name]interface{}{
			aStatus.Name: map[string]interface{}{},
		}
		injectRobot.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
			test.That(
				t,
				testutils.NewResourceNameSet(resourceNames...),
				test.ShouldResemble,
				testutils.NewResourceNameSet(aStatus.Name),
			)
			return readings, nil
		}
		req := &pb.GetStatusRequest{
			ResourceNames: []*commonpb.ResourceName{protoutils.ResourceNameToProto(aStatus.Name)},
		}

		resp, err := server.GetStatus(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.Status), test.ShouldEqual, 1)

		observed := map[resource.Name]interface{}{
			protoutils.ResourceNameFromProto(resp.Status[0].Name): resp.Status[0].Status.AsMap(),
		}
		test.That(t, observed, test.ShouldResemble, expected)
	})

	t.Run("working many statuses", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		server := server.New(injectRobot)
		iStatus := robot.Status{Name: imu.Named("imu"), Status: map[string]interface{}{"abc": []float64{1.2, 2.3, 3.4}}}
		gStatus := robot.Status{Name: movementsensor.Named("gps"), Status: map[string]interface{}{"efg": []string{"hello"}}}
		aStatus := robot.Status{Name: arm.Named("arm"), Status: struct{}{}}
		statuses := []robot.Status{iStatus, gStatus, aStatus}
		expected := map[resource.Name]interface{}{
			iStatus.Name: map[string]interface{}{"abc": []interface{}{1.2, 2.3, 3.4}},
			gStatus.Name: map[string]interface{}{"efg": []interface{}{"hello"}},
			aStatus.Name: map[string]interface{}{},
		}
		injectRobot.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
			test.That(
				t,
				testutils.NewResourceNameSet(resourceNames...),
				test.ShouldResemble,
				testutils.NewResourceNameSet(iStatus.Name, gStatus.Name, aStatus.Name),
			)
			return statuses, nil
		}
		req := &pb.GetStatusRequest{
			ResourceNames: []*commonpb.ResourceName{
				protoutils.ResourceNameToProto(iStatus.Name),
				protoutils.ResourceNameToProto(gStatus.Name),
				protoutils.ResourceNameToProto(aStatus.Name),
			},
		}

		resp, err := server.GetStatus(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.Status), test.ShouldEqual, 3)

		observed := map[resource.Name]interface{}{
			protoutils.ResourceNameFromProto(resp.Status[0].Name): resp.Status[0].Status.AsMap(),
			protoutils.ResourceNameFromProto(resp.Status[1].Name): resp.Status[1].Status.AsMap(),
			protoutils.ResourceNameFromProto(resp.Status[2].Name): resp.Status[2].Status.AsMap(),
		}
		test.That(t, observed, test.ShouldResemble, expected)
	})

	t.Run("failed StreamStatus", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		server := server.New(injectRobot)
		err1 := errors.New("whoops")
		injectRobot.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
			return nil, err1
		}

		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		messageCh := make(chan *pb.StreamStatusResponse)
		streamServer := &statusStreamServer{
			ctx:       cancelCtx,
			messageCh: messageCh,
		}
		err := server.StreamStatus(&pb.StreamStatusRequest{Every: durationpb.New(time.Second)}, streamServer)
		test.That(t, err, test.ShouldEqual, err1)
	})

	t.Run("failed StreamStatus server send", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		server := server.New(injectRobot)
		injectRobot.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
			return []robot.Status{{arm.Named("arm"), struct{}{}}}, nil
		}

		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		messageCh := make(chan *pb.StreamStatusResponse)
		streamServer := &statusStreamServer{
			ctx:       cancelCtx,
			messageCh: messageCh,
			fail:      true,
		}
		dur := 100 * time.Millisecond
		err := server.StreamStatus(&pb.StreamStatusRequest{Every: durationpb.New(dur)}, streamServer)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "send fail")
	})

	t.Run("timed out StreamStatus", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		server := server.New(injectRobot)
		injectRobot.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
			return []robot.Status{{arm.Named("arm"), struct{}{}}}, nil
		}

		timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		streamServer := &statusStreamServer{
			ctx:       timeoutCtx,
			messageCh: nil,
		}
		dur := 100 * time.Millisecond

		streamErr := server.StreamStatus(&pb.StreamStatusRequest{Every: durationpb.New(dur)}, streamServer)
		test.That(t, streamErr, test.ShouldResemble, context.DeadlineExceeded)
	})

	t.Run("working StreamStatus", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		server := server.New(injectRobot)
		injectRobot.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
			return []robot.Status{{arm.Named("arm"), struct{}{}}}, nil
		}

		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		messageCh := make(chan *pb.StreamStatusResponse)
		streamServer := &statusStreamServer{
			ctx:       cancelCtx,
			messageCh: messageCh,
			fail:      false,
		}
		dur := 100 * time.Millisecond
		var streamErr error
		start := time.Now()
		done := make(chan struct{})
		go func() {
			streamErr = server.StreamStatus(&pb.StreamStatusRequest{Every: durationpb.New(dur)}, streamServer)
			close(done)
		}()
		expectedStatus, err := structpb.NewStruct(map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		var messages []*pb.StreamStatusResponse
		messages = append(messages, <-messageCh)
		messages = append(messages, <-messageCh)
		messages = append(messages, <-messageCh)
		test.That(t, messages, test.ShouldResemble, []*pb.StreamStatusResponse{
			{Status: []*pb.Status{{Name: protoutils.ResourceNameToProto(arm.Named("arm")), Status: expectedStatus}}},
			{Status: []*pb.Status{{Name: protoutils.ResourceNameToProto(arm.Named("arm")), Status: expectedStatus}}},
			{Status: []*pb.Status{{Name: protoutils.ResourceNameToProto(arm.Named("arm")), Status: expectedStatus}}},
		})
		test.That(t, time.Since(start), test.ShouldBeGreaterThanOrEqualTo, 3*dur)
		test.That(t, time.Since(start), test.ShouldBeLessThanOrEqualTo, 6*dur)
		cancel()
		<-done
		test.That(t, streamErr, test.ShouldEqual, context.Canceled)
	})
}

type statusStreamServer struct {
	grpc.ServerStream // not set
	ctx               context.Context
	messageCh         chan<- *pb.StreamStatusResponse
	fail              bool
}

func (x *statusStreamServer) Context() context.Context {
	return x.ctx
}

func (x *statusStreamServer) Send(m *pb.StreamStatusResponse) error {
	if x.fail {
		return errors.New("send fail")
	}
	if x.messageCh == nil {
		return nil
	}
	x.messageCh <- m
	return nil
}
