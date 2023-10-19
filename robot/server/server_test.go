package server_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	commonpb "go.viam.com/api/common/v1"
	armpb "go.viam.com/api/component/arm/v1"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	vprotoutils "go.viam.com/utils/protoutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/robot/server"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var emptyResources = &pb.ResourceNamesResponse{
	Resources: []*commonpb.ResourceName{},
}

var serverNewResource = arm.Named("")

var serverOneResourceResponse = []*commonpb.ResourceName{
	{
		Namespace: string(serverNewResource.API.Type.Namespace),
		Type:      serverNewResource.API.Type.Name,
		Subtype:   serverNewResource.API.SubtypeName,
		Name:      serverNewResource.Name,
	},
}

func TestServer(t *testing.T) {
	t.Run("Metadata", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{} }
		server := server.New(injectRobot)

		resourceResp, err := server.ResourceNames(context.Background(), &pb.ResourceNamesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp, test.ShouldResemble, emptyResources)

		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{serverNewResource} }

		resourceResp, err = server.ResourceNames(context.Background(), &pb.ResourceNamesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp.Resources, test.ShouldResemble, serverOneResourceResponse)
	})

	t.Run("Discovery", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{} }
		server := server.New(injectRobot)

		q := resource.DiscoveryQuery{arm.Named("arm").API, resource.DefaultModelFamily.WithModel("some-arm")}
		disc := resource.Discovery{Query: q, Results: struct{}{}}
		discoveries := []resource.Discovery{disc}
		injectRobot.DiscoverComponentsFunc = func(ctx context.Context, keys []resource.DiscoveryQuery) ([]resource.Discovery, error) {
			return discoveries, nil
		}

		t.Run("full api and model", func(t *testing.T) {
			req := &pb.DiscoverComponentsRequest{
				Queries: []*pb.DiscoveryQuery{{Subtype: q.API.String(), Model: q.Model.String()}},
			}

			resp, err := server.DiscoverComponents(context.Background(), req)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(resp.Discovery), test.ShouldEqual, 1)

			observed := resp.Discovery[0].Results.AsMap()
			expected := map[string]interface{}{}
			expectedQ := &pb.DiscoveryQuery{Subtype: "rdk:component:arm", Model: "rdk:builtin:some-arm"}
			test.That(t, resp.Discovery[0].Query, test.ShouldResemble, expectedQ)
			test.That(t, observed, test.ShouldResemble, expected)
		})
		t.Run("short api and model", func(t *testing.T) {
			req := &pb.DiscoverComponentsRequest{
				Queries: []*pb.DiscoveryQuery{{Subtype: "arm", Model: "some-arm"}},
			}

			resp, err := server.DiscoverComponents(context.Background(), req)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(resp.Discovery), test.ShouldEqual, 1)

			observed := resp.Discovery[0].Results.AsMap()
			expected := map[string]interface{}{}
			expectedQ := &pb.DiscoveryQuery{Subtype: "arm", Model: "some-arm"}
			test.That(t, resp.Discovery[0].Query, test.ShouldResemble, expectedQ)
			test.That(t, observed, test.ShouldResemble, expected)
		})
	})

	t.Run("ResourceRPCSubtypes", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
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

		otherAPI := resource.NewAPI("acme", "component", "wat")
		respWith := []resource.RPCAPI{
			{
				API:  serverNewResource.API,
				Desc: desc1,
			},
			{
				API:  resource.NewAPI("acme", "component", "wat"),
				Desc: desc2,
			},
		}

		expectedResp := []*pb.ResourceRPCSubtype{
			{
				Subtype: &commonpb.ResourceName{
					Namespace: string(serverNewResource.API.Type.Namespace),
					Type:      serverNewResource.API.Type.Name,
					Subtype:   serverNewResource.API.SubtypeName,
				},
				ProtoService: desc1.GetFullyQualifiedName(),
			},
			{
				Subtype: &commonpb.ResourceName{
					Namespace: string(otherAPI.Type.Namespace),
					Type:      otherAPI.Type.Name,
					Subtype:   otherAPI.SubtypeName,
				},
				ProtoService: desc2.GetFullyQualifiedName(),
			},
		}

		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return respWith }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return nil }

		typesResp, err = server.ResourceRPCSubtypes(context.Background(), &pb.ResourceRPCSubtypesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, typesResp.ResourceRpcSubtypes, test.ShouldResemble, expectedResp)
	})

	t.Run("GetOperations", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		injectRobot := &inject.Robot{}
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return nil }
		injectRobot.LoggerFunc = func() golog.Logger {
			return logger
		}
		server := server.New(injectRobot)

		opsResp, err := server.GetOperations(context.Background(), &pb.GetOperationsRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, opsResp, test.ShouldResemble, &pb.GetOperationsResponse{
			Operations: []*pb.Operation{},
		})

		sess1 := session.New(context.Background(), "owner1", time.Minute, nil)
		sess2 := session.New(context.Background(), "owner2", time.Minute, nil)
		sess1Ctx := session.ToContext(context.Background(), sess1)
		sess2Ctx := session.ToContext(context.Background(), sess2)
		op1, cancel1 := injectRobot.OperationManager().Create(context.Background(), "something1", nil)
		defer cancel1()

		op2, cancel2 := injectRobot.OperationManager().Create(sess1Ctx, "something2", nil)
		defer cancel2()

		op3, cancel3 := injectRobot.OperationManager().Create(sess2Ctx, "something3", nil)
		defer cancel3()

		opsResp, err = server.GetOperations(context.Background(), &pb.GetOperationsRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, opsResp.Operations, test.ShouldHaveLength, 3)

		for idx, searchFor := range []struct {
			opID   uuid.UUID
			sessID uuid.UUID
		}{
			{operation.Get(op1).ID, uuid.Nil},
			{operation.Get(op2).ID, sess1.ID()},
			{operation.Get(op3).ID, sess2.ID()},
		} {
			t.Run(fmt.Sprintf("check op=%d", idx), func(t *testing.T) {
				for _, op := range opsResp.Operations {
					if op.Id == searchFor.opID.String() {
						if searchFor.sessID == uuid.Nil {
							test.That(t, op.SessionId, test.ShouldBeNil)
							return
						}
						test.That(t, op.SessionId, test.ShouldNotBeNil)
						test.That(t, *op.SessionId, test.ShouldEqual, searchFor.sessID.String())
						return
					}
				}
				t.Fail()
			})
		}
	})

	t.Run("GetSessions", func(t *testing.T) {
		sessMgr := &sessionManager{}
		injectRobot := &inject.Robot{SessMgr: sessMgr}
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return nil }
		server := server.New(injectRobot)

		sessResp, err := server.GetSessions(context.Background(), &pb.GetSessionsRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sessResp, test.ShouldResemble, &pb.GetSessionsResponse{
			Sessions: []*pb.Session{},
		})

		ownerID1 := "owner1"
		remoteAddr1 := &net.TCPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 5}
		ctx1 := peer.NewContext(context.Background(), &peer.Peer{
			Addr: remoteAddr1,
		})
		remoteAddr2 := &net.TCPAddr{IP: net.IPv4(2, 2, 3, 8), Port: 9}
		ctx2 := peer.NewContext(context.Background(), &peer.Peer{
			Addr: remoteAddr2,
		})

		ownerID2 := "owner2"
		dur := time.Second

		sessions := []*session.Session{
			session.New(ctx1, ownerID1, dur, nil),
			session.New(ctx2, ownerID2, dur, nil),
		}

		sessMgr.mu.Lock()
		sessMgr.sessions = sessions
		sessMgr.mu.Unlock()

		sessResp, err = server.GetSessions(context.Background(), &pb.GetSessionsRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sessResp, test.ShouldResemble, &pb.GetSessionsResponse{
			Sessions: []*pb.Session{
				{
					Id:                 sessions[0].ID().String(),
					PeerConnectionInfo: sessions[0].PeerConnectionInfo(),
				},
				{
					Id:                 sessions[1].ID().String(),
					PeerConnectionInfo: sessions[1].PeerConnectionInfo(),
				},
			},
		})
	})
}

func TestServerFrameSystemConfig(t *testing.T) {
	injectRobot := &inject.Robot{}

	o1 := &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1}
	o1Cfg, err := spatialmath.NewOrientationConfig(o1)
	test.That(t, err, test.ShouldBeNil)

	// test working config function
	t.Run("test working config function", func(t *testing.T) {
		l1 := &referenceframe.LinkConfig{
			ID:          "frame1",
			Parent:      referenceframe.World,
			Translation: r3.Vector{X: 1, Y: 2, Z: 3},
			Orientation: o1Cfg,
			Geometry:    &spatialmath.GeometryConfig{Type: "box", X: 1, Y: 2, Z: 1},
		}
		lif1, err := l1.ParseConfig()
		test.That(t, err, test.ShouldBeNil)
		l2 := &referenceframe.LinkConfig{
			ID:          "frame2",
			Parent:      "frame1",
			Translation: r3.Vector{X: 1, Y: 2, Z: 3},
			Geometry:    &spatialmath.GeometryConfig{Type: "box", X: 1, Y: 2, Z: 1},
		}
		lif2, err := l2.ParseConfig()
		test.That(t, err, test.ShouldBeNil)
		fsConfigs := []*referenceframe.FrameSystemPart{
			{
				FrameConfig: lif1,
			},
			{
				FrameConfig: lif2,
			},
		}

		injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
			return &framesystem.Config{Parts: fsConfigs}, nil
		}
		server := server.New(injectRobot)
		req := &pb.FrameSystemConfigRequest{}
		resp, err := server.FrameSystemConfig(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.FrameSystemConfigs), test.ShouldEqual, len(fsConfigs))
		test.That(t, resp.FrameSystemConfigs[0].Frame.ReferenceFrame, test.ShouldEqual, fsConfigs[0].FrameConfig.Name())
		test.That(
			t,
			resp.FrameSystemConfigs[0].Frame.PoseInObserverFrame.ReferenceFrame,
			test.ShouldEqual,
			fsConfigs[0].FrameConfig.Parent(),
		)
		test.That(t,
			resp.FrameSystemConfigs[0].Frame.PoseInObserverFrame.Pose.X,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Pose().Point().X,
		)
		test.That(t,
			resp.FrameSystemConfigs[0].Frame.PoseInObserverFrame.Pose.Y,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Pose().Point().Y,
		)
		test.That(t,
			resp.FrameSystemConfigs[0].Frame.PoseInObserverFrame.Pose.Z,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Pose().Point().Z,
		)
		pose := fsConfigs[0].FrameConfig.Pose()
		test.That(t,
			resp.FrameSystemConfigs[0].Frame.PoseInObserverFrame.Pose.OX,
			test.ShouldAlmostEqual,
			pose.Orientation().OrientationVectorDegrees().OX,
		)
		test.That(t,
			resp.FrameSystemConfigs[0].Frame.PoseInObserverFrame.Pose.OY,
			test.ShouldAlmostEqual,
			pose.Orientation().OrientationVectorDegrees().OY,
		)
		test.That(t,
			resp.FrameSystemConfigs[0].Frame.PoseInObserverFrame.Pose.OZ,
			test.ShouldAlmostEqual,
			pose.Orientation().OrientationVectorDegrees().OZ,
		)
		test.That(t,
			resp.FrameSystemConfigs[0].Frame.PoseInObserverFrame.Pose.Theta,
			test.ShouldAlmostEqual,
			pose.Orientation().OrientationVectorDegrees().Theta,
		)
	})

	t.Run("test failing config function", func(t *testing.T) {
		expectedErr := errors.New("failed to retrieve config")
		injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
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
	// Sample lastReconfigured times to be used across status tests.
	lastReconfigured, err := time.Parse("2006-01-02 15:04:05", "1998-04-30 19:08:00")
	test.That(t, err, test.ShouldBeNil)
	lastReconfigured2, err := time.Parse("2006-01-02 15:04:05", "2011-11-11 00:00:00")
	test.That(t, err, test.ShouldBeNil)

	t.Run("failed GetStatus", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		server := server.New(injectRobot)
		passedErr := errors.New("can't get status")
		injectRobot.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
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
		injectRobot.StatusFunc = func(ctx context.Context, status []resource.Name) ([]robot.Status, error) {
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
				"unable to convert interface 1 to a form acceptable to structpb.NewStruct: "+
					"data of type int and kind int not a struct or a map-like object",
			),
		)
	})

	t.Run("working one status", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		server := server.New(injectRobot)
		aStatus := robot.Status{
			Name:             arm.Named("arm"),
			LastReconfigured: lastReconfigured,
			Status:           struct{}{},
		}
		readings := []robot.Status{aStatus}
		expected := map[resource.Name]interface{}{
			aStatus.Name: map[string]interface{}{},
		}
		expectedLR := map[resource.Name]time.Time{
			aStatus.Name: lastReconfigured,
		}
		injectRobot.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
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
		observedLR := map[resource.Name]time.Time{
			protoutils.ResourceNameFromProto(resp.Status[0].Name): resp.Status[0].
				LastReconfigured.AsTime(),
		}
		test.That(t, observed, test.ShouldResemble, expected)
		test.That(t, observedLR, test.ShouldResemble, expectedLR)
	})

	t.Run("working many statuses", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		server := server.New(injectRobot)
		gStatus := robot.Status{
			Name:             movementsensor.Named("gps"),
			LastReconfigured: lastReconfigured,
			Status:           map[string]interface{}{"efg": []string{"hello"}},
		}
		aStatus := robot.Status{
			Name:             arm.Named("arm"),
			LastReconfigured: lastReconfigured2,
			Status:           struct{}{},
		}
		statuses := []robot.Status{gStatus, aStatus}
		expected := map[resource.Name]interface{}{
			gStatus.Name: map[string]interface{}{"efg": []interface{}{"hello"}},
			aStatus.Name: map[string]interface{}{},
		}
		expectedLRs := map[resource.Name]time.Time{
			gStatus.Name: lastReconfigured,
			aStatus.Name: lastReconfigured2,
		}
		injectRobot.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
			test.That(
				t,
				testutils.NewResourceNameSet(resourceNames...),
				test.ShouldResemble,
				testutils.NewResourceNameSet(gStatus.Name, aStatus.Name),
			)
			return statuses, nil
		}
		req := &pb.GetStatusRequest{
			ResourceNames: []*commonpb.ResourceName{
				protoutils.ResourceNameToProto(gStatus.Name),
				protoutils.ResourceNameToProto(aStatus.Name),
			},
		}

		resp, err := server.GetStatus(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.Status), test.ShouldEqual, 2)

		observed := map[resource.Name]interface{}{
			protoutils.ResourceNameFromProto(resp.Status[0].Name): resp.Status[0].Status.AsMap(),
			protoutils.ResourceNameFromProto(resp.Status[1].Name): resp.Status[1].Status.AsMap(),
		}
		observedLRs := map[resource.Name]time.Time{
			protoutils.ResourceNameFromProto(resp.Status[0].Name): resp.Status[0].
				LastReconfigured.AsTime(),
			protoutils.ResourceNameFromProto(resp.Status[1].Name): resp.Status[1].
				LastReconfigured.AsTime(),
		}
		test.That(t, observed, test.ShouldResemble, expected)
		test.That(t, observedLRs, test.ShouldResemble, expectedLRs)
	})

	t.Run("failed StreamStatus", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		server := server.New(injectRobot)
		err1 := errors.New("whoops")
		injectRobot.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
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
		injectRobot.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
			return []robot.Status{{arm.Named("arm"), time.Time{}, struct{}{}}}, nil
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
		injectRobot.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
			return []robot.Status{{arm.Named("arm"), time.Time{}, struct{}{}}}, nil
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
		injectRobot.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
			return []robot.Status{
				{
					Name:             arm.Named("arm"),
					LastReconfigured: lastReconfigured,
					Status:           struct{}{},
				},
			}, nil
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
		expectedStatus, err := vprotoutils.StructToStructPb(map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		var messages []*pb.StreamStatusResponse
		messages = append(messages, <-messageCh)
		messages = append(messages, <-messageCh)
		messages = append(messages, <-messageCh)
		expectedRobotStatus := []*pb.Status{
			{
				Name:             protoutils.ResourceNameToProto(arm.Named("arm")),
				LastReconfigured: timestamppb.New(lastReconfigured),
				Status:           expectedStatus,
			},
		}
		test.That(t, messages, test.ShouldResemble, []*pb.StreamStatusResponse{
			{Status: expectedRobotStatus},
			{Status: expectedRobotStatus},
			{Status: expectedRobotStatus},
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

type sessionManager struct {
	mu       sync.Mutex
	sessions []*session.Session
}

func (mgr *sessionManager) Start(ctx context.Context, ownerID string) (*session.Session, error) {
	panic("unimplemented")
}

func (mgr *sessionManager) All() []*session.Session {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.sessions
}

func (mgr *sessionManager) FindByID(ctx context.Context, id uuid.UUID, ownerID string) (*session.Session, error) {
	panic("unimplemented")
}

func (mgr *sessionManager) AssociateResource(id uuid.UUID, resourceName resource.Name) {
	panic("unimplemented")
}

func (mgr *sessionManager) Close() {
}

func (mgr *sessionManager) ServerInterceptors() session.ServerInterceptors {
	panic("unimplemented")
}
