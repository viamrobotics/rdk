package server_test

import (
	"context"
	"errors"
	"fmt"
	"math"
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
	"google.golang.org/protobuf/types/known/durationpb"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
	"go.viam.com/rdk/robot/server"
	"go.viam.com/rdk/session"
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

	t.Run("GetOperations", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		injectRobot := &inject.Robot{}
		injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
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

		sess1 := session.New("owner1", nil, time.Minute, nil)
		sess2 := session.New("owner2", nil, time.Minute, nil)
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
		injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return nil }
		server := server.New(injectRobot)

		sessResp, err := server.GetSessions(context.Background(), &pb.GetSessionsRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sessResp, test.ShouldResemble, &pb.GetSessionsResponse{
			Sessions: []*pb.Session{},
		})

		ownerID1 := "owner1"
		remoteAddr1 := "rem1"
		localAddr1 := "loc1"
		info1 := &pb.PeerConnectionInfo{
			Type:          pb.PeerConnectionType_PEER_CONNECTION_TYPE_GRPC,
			RemoteAddress: &remoteAddr1,
			LocalAddress:  &localAddr1,
		}
		ownerID2 := "owner2"
		remoteAddr2 := "rem2"
		localAddr2 := "loc2"
		info2 := &pb.PeerConnectionInfo{
			Type:          pb.PeerConnectionType_PEER_CONNECTION_TYPE_GRPC,
			RemoteAddress: &remoteAddr2,
			LocalAddress:  &localAddr2,
		}
		dur := time.Second

		sessions := []*session.Session{
			session.New(ownerID1, info1, dur, nil),
			session.New(ownerID2, info2, dur, nil),
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

		injectRobot.FrameSystemConfigFunc = func(
			ctx context.Context, additionalTransforms []*referenceframe.LinkInFrame,
		) (framesystemparts.Parts, error) {
			return framesystemparts.Parts(fsConfigs), nil
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
		injectRobot.FrameSystemConfigFunc = func(
			ctx context.Context, additionalTransforms []*referenceframe.LinkInFrame,
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
		aStatus := robot.Status{Name: arm.Named("arm"), Status: struct{}{}}
		readings := []robot.Status{aStatus}
		expected := map[resource.Name]interface{}{
			aStatus.Name: map[string]interface{}{},
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
		test.That(t, observed, test.ShouldResemble, expected)
	})

	t.Run("working many statuses", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		server := server.New(injectRobot)
		gStatus := robot.Status{Name: movementsensor.Named("gps"), Status: map[string]interface{}{"efg": []string{"hello"}}}
		aStatus := robot.Status{Name: arm.Named("arm"), Status: struct{}{}}
		statuses := []robot.Status{gStatus, aStatus}
		expected := map[resource.Name]interface{}{
			gStatus.Name: map[string]interface{}{"efg": []interface{}{"hello"}},
			aStatus.Name: map[string]interface{}{},
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
		test.That(t, observed, test.ShouldResemble, expected)
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
		injectRobot.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
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
		injectRobot.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
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
		expectedStatus, err := vprotoutils.StructToStructPb(map[string]interface{}{})
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

type sessionManager struct {
	mu       sync.Mutex
	sessions []*session.Session
}

func (mgr *sessionManager) Start(ownerID string, peerConnInfo *pb.PeerConnectionInfo) (*session.Session, error) {
	panic("unimplemented")
}

func (mgr *sessionManager) All() []*session.Session {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.sessions
}

func (mgr *sessionManager) FindByID(id uuid.UUID, ownerID string) (*session.Session, error) {
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
