package client

import (
	"context"
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	armpb "go.viam.com/api/component/arm/v1"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	gotestutils "go.viam.com/utils/testutils"
	"gonum.org/v1/gonum/num/quat"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/movementsensor"
	rgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/robot/server"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var pose1 = spatialmath.NewZeroPose()

func TestClientDisconnect(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{arm.Named("arm1")}
	}

	// TODO(RSDK-882): will update this so that this is not necessary
	injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{}, nil
	}

	go gServer.Serve(listener)

	start := time.Now()

	test.That(t, err, test.ShouldBeNil)

	dur := 100 * time.Millisecond
	client, err := New(
		context.Background(),
		listener.Addr().String(),
		logger,
		WithCheckConnectedEvery(dur),
		WithReconnectEvery(2*dur),
	)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()

	test.That(t, client.Connected(), test.ShouldBeTrue)
	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 1)
	_, err = client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)

	gServer.Stop()
	test.That(t, <-client.Changed(), test.ShouldBeTrue)
	test.That(t, client.Connected(), test.ShouldBeFalse)
	timeSinceStart := time.Since(start)
	test.That(t, timeSinceStart, test.ShouldBeBetweenOrEqual, dur, 4*dur)
	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 0)
	_, err = client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeError, client.checkConnected())
}

func TestClientUnaryDisconnectHandler(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	var unaryStatusCallReceived bool
	justOneUnaryStatusCall := grpc.ChainUnaryInterceptor(
		func(
			ctx context.Context,
			req interface{},
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler,
		) (interface{}, error) {
			if strings.HasSuffix(info.FullMethod, "RobotService/GetStatus") {
				if unaryStatusCallReceived {
					return nil, status.Error(codes.Unknown, io.ErrClosedPipe.Error())
				}
				unaryStatusCallReceived = true
			}
			var resp interface{}
			return resp, nil
		},
	)
	gServer := grpc.NewServer(justOneUnaryStatusCall)

	injectRobot := &inject.Robot{}
	injectRobot.StatusFunc = func(ctx context.Context, rs []resource.Name) ([]robot.Status, error) {
		return []robot.Status{}, nil
	}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener)

	never := -1 * time.Second
	client, err := New(
		context.Background(),
		listener.Addr().String(),
		logger,
		WithCheckConnectedEvery(never),
		WithReconnectEvery(never),
	)
	test.That(t, err, test.ShouldBeNil)

	t.Run("unary call to connected remote", func(t *testing.T) {
		t.Helper()

		client.connected.Store(false)
		_, err = client.Status(context.Background(), []resource.Name{})
		test.That(t, status.Code(err), test.ShouldEqual, codes.Unavailable)
		test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("not connected to remote robot at %s", listener.Addr().String()))
		test.That(t, unaryStatusCallReceived, test.ShouldBeFalse)
		client.connected.Store(true)
	})

	t.Run("unary call to disconnected remote", func(t *testing.T) {
		t.Helper()

		_, err = client.Status(context.Background(), []resource.Name{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, unaryStatusCallReceived, test.ShouldBeTrue)
	})

	t.Run("unary call to undetected disconnected remote", func(t *testing.T) {
		test.That(t, unaryStatusCallReceived, test.ShouldBeTrue)
		_, err = client.Status(context.Background(), []resource.Name{})
		test.That(t, status.Code(err), test.ShouldEqual, codes.Unavailable)
		test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("not connected to remote robot at %s", listener.Addr().String()))
	})

	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()
	gServer.Stop()
}

func TestClientStreamDisconnectHandler(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	var streamStatusCallReceived bool
	interceptStreamStatusCall := grpc.ChainStreamInterceptor(
		func(
			srv interface{},
			ss grpc.ServerStream,
			info *grpc.StreamServerInfo,
			handler grpc.StreamHandler,
		) error {
			if strings.HasSuffix(info.FullMethod, "RobotService/StreamStatus") {
				streamStatusCallReceived = true
			}
			return handler(srv, ss)
		},
	)

	gServer := grpc.NewServer(interceptStreamStatusCall)

	injectRobot := &inject.Robot{}
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	injectRobot.ResourceNamesFunc = func() []resource.Name { return nil }
	injectRobot.StatusFunc = func(ctx context.Context, rs []resource.Name) ([]robot.Status, error) {
		return []robot.Status{}, nil
	}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener)

	never := -1 * time.Second
	client, err := New(
		context.Background(),
		listener.Addr().String(),
		logger,
		WithCheckConnectedEvery(never),
		WithReconnectEvery(never),
	)
	test.That(t, err, test.ShouldBeNil)

	t.Run("stream call to disconnected remote", func(t *testing.T) {
		t.Helper()

		client.connected.Store(false)
		_, err = client.client.StreamStatus(context.Background(), &pb.StreamStatusRequest{})
		test.That(t, status.Code(err), test.ShouldEqual, codes.Unavailable)
		test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("not connected to remote robot at %s", listener.Addr().String()))
		test.That(t, streamStatusCallReceived, test.ShouldBeFalse)
		client.connected.Store(true)
	})

	t.Run("stream call to connected remote", func(t *testing.T) {
		t.Helper()

		ssc, err := client.client.StreamStatus(context.Background(), &pb.StreamStatusRequest{})
		test.That(t, err, test.ShouldBeNil)
		ssc.Recv()
		test.That(t, streamStatusCallReceived, test.ShouldBeTrue)
	})

	t.Run("receive call from stream of disconnected remote", func(t *testing.T) {
		t.Helper()

		ssc, err := client.client.StreamStatus(context.Background(), &pb.StreamStatusRequest{})
		test.That(t, err, test.ShouldBeNil)

		client.connected.Store(false)
		_, err = ssc.Recv()
		test.That(t, status.Code(err), test.ShouldEqual, codes.Unavailable)
		test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("not connected to remote robot at %s", listener.Addr().String()))
		client.connected.Store(true)
	})

	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()
	gServer.Stop()
}

type mockType struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
}

func TestClientReconnect(t *testing.T) {
	someAPI := resource.APINamespace("acme").WithComponentType(uuid.New().String())
	var called int64
	resource.RegisterAPI(
		someAPI,
		resource.APIRegistration[resource.Resource]{
			RPCClient: func(
				ctx context.Context,
				conn rpc.ClientConn,
				remoteName string,
				name resource.Name,
				logger golog.Logger,
			) (resource.Resource, error) {
				atomic.AddInt64(&called, 1)
				return &mockType{Named: name.AsNamed()}, nil
			},
		},
	)

	logger := golog.NewTestLogger(t)

	var listener net.Listener = gotestutils.ReserveRandomListener(t)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	thing1Name := resource.NewName(someAPI, "thing1")
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{arm.Named("arm1"), thing1Name}
	}

	// TODO(RSDK-882): will update this so that this is not necessary
	injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{}, nil
	}

	injectArm := &inject.Arm{}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return pose1, nil
	}

	armSvc2, err := resource.NewAPIResourceCollection(arm.API, map[resource.Name]arm.Arm{arm.Named("arm1"): injectArm})
	test.That(t, err, test.ShouldBeNil)
	gServer.RegisterService(&armpb.ArmService_ServiceDesc, arm.NewRPCServiceServer(armSvc2))

	go gServer.Serve(listener)

	dur := 100 * time.Millisecond
	client, err := New(
		context.Background(),
		listener.Addr().String(),
		logger,
		WithCheckConnectedEvery(dur),
		WithReconnectEvery(dur),
	)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()

	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 2)
	_, err = client.ResourceByName(thing1Name)
	test.That(t, err, test.ShouldBeNil)
	a, err := client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = a.(arm.Arm).EndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, atomic.LoadInt64(&called), test.ShouldEqual, 1)

	gServer.Stop()

	test.That(t, <-client.Changed(), test.ShouldBeTrue)
	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 0)
	_, err = client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeError, client.checkConnected())

	gServer2 := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer2, server.New(injectRobot))
	gServer2.RegisterService(&armpb.ArmService_ServiceDesc, arm.NewRPCServiceServer(armSvc2))

	// Note: There's a slight chance this test can fail if someone else
	// claims the port we just released by closing the server.
	listener, err = net.Listen("tcp", listener.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	go gServer2.Serve(listener)
	defer gServer2.Stop()

	test.That(t, <-client.Changed(), test.ShouldBeTrue)
	test.That(t, client.Connected(), test.ShouldBeTrue)
	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 2)
	_, err = client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = a.(arm.Arm).EndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, atomic.LoadInt64(&called), test.ShouldEqual, 1)
}

func TestClientRefreshNoReconfigure(t *testing.T) {
	someAPI := resource.APINamespace("acme").WithComponentType(uuid.New().String())
	var called int64
	resource.RegisterAPI(
		someAPI,
		resource.APIRegistration[resource.Resource]{
			RPCClient: func(
				ctx context.Context,
				conn rpc.ClientConn,
				remoteName string,
				name resource.Name,
				logger golog.Logger,
			) (resource.Resource, error) {
				atomic.AddInt64(&called, 1)
				return &mockType{Named: name.AsNamed()}, nil
			},
		},
	)

	logger := golog.NewTestLogger(t)

	var listener net.Listener = gotestutils.ReserveRandomListener(t)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	thing1Name := resource.NewName(someAPI, "thing1")

	var callCount int
	calledEnough := make(chan struct{})

	allow := make(chan struct{})
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		if callCount == 1 {
			<-allow
		}
		if callCount == 5 {
			close(calledEnough)
		}
		callCount++

		return []resource.Name{arm.Named("arm1"), thing1Name}
	}

	go gServer.Serve(listener)
	defer gServer.Stop()

	dur := 100 * time.Millisecond
	client, err := New(
		context.Background(),
		listener.Addr().String(),
		logger,
		WithRefreshEvery(dur),
	)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()

	close(allow)
	<-calledEnough

	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 2)

	_, err = client.ResourceByName(thing1Name)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, atomic.LoadInt64(&called), test.ShouldEqual, 1)
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := New(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	client2, err := New(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = client1.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
	err = client2.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func ensurePartsAreEqual(part, otherPart *referenceframe.FrameSystemPart) error {
	if part.FrameConfig.Name() != otherPart.FrameConfig.Name() {
		return errors.Errorf("part had name %s while other part had name %s", part.FrameConfig.Name(), otherPart.FrameConfig.Name())
	}
	frameConfig := part.FrameConfig
	otherFrameConfig := otherPart.FrameConfig
	if frameConfig.Parent() != otherFrameConfig.Parent() {
		return errors.Errorf("part had parent %s while other part had parent %s", frameConfig.Parent(), otherFrameConfig.Parent())
	}
	if !spatialmath.R3VectorAlmostEqual(frameConfig.Pose().Point(), otherFrameConfig.Pose().Point(), 1e-8) {
		return errors.New("translations of parts not equal")
	}

	orient := frameConfig.Pose().Orientation()
	otherOrient := otherFrameConfig.Pose().Orientation()

	switch {
	case orient == nil && otherOrient != nil:
		if !spatialmath.QuaternionAlmostEqual(otherOrient.Quaternion(), quat.Number{1, 0, 0, 0}, 1e-5) {
			return errors.New("orientations of parts not equal")
		}
	case otherOrient == nil:
		return errors.New("orientation not returned for other part")
	case !spatialmath.OrientationAlmostEqual(orient, otherOrient):
		return errors.New("orientations of parts not equal")
	}
	return nil
}

func TestClientConfig(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	workingServer := grpc.NewServer()
	failingServer := grpc.NewServer()

	resourcesFunc := func() []resource.Name { return []resource.Name{} }
	workingRobot := &inject.Robot{
		ResourceNamesFunc:   resourcesFunc,
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
	}
	failingRobot := &inject.Robot{
		ResourceNamesFunc:   resourcesFunc,
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
	}

	o1 := &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1}
	o1Cfg, err := spatialmath.NewOrientationConfig(o1)
	test.That(t, err, test.ShouldBeNil)

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

	workingRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{Parts: fsConfigs}, nil
	}

	configErr := errors.New("failed to retrieve config")
	failingRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return nil, configErr
	}

	pb.RegisterRobotServiceServer(workingServer, server.New(workingRobot))
	pb.RegisterRobotServiceServer(failingServer, server.New(failingRobot))

	go workingServer.Serve(listener1)
	defer workingServer.Stop()

	ctx := context.Background()

	t.Run("Failing client due to cancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()
		_, err = New(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	workingFSClient, err := New(ctx, listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("client test config for working frame service", func(t *testing.T) {
		config, err := workingFSClient.FrameSystemConfig(ctx)
		test.That(t, err, test.ShouldBeNil)
		err = ensurePartsAreEqual(fsConfigs[0], config.Parts[0])
		test.That(t, err, test.ShouldBeNil)
		err = ensurePartsAreEqual(fsConfigs[1], config.Parts[1])
		test.That(t, err, test.ShouldBeNil)
	})

	err = workingFSClient.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	t.Run("dialed client test config for working frame service", func(t *testing.T) {
		workingDialedClient, err := New(ctx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		config, err := workingDialedClient.FrameSystemConfig(ctx)
		test.That(t, err, test.ShouldBeNil)
		err = ensurePartsAreEqual(fsConfigs[0], config.Parts[0])
		test.That(t, err, test.ShouldBeNil)
		err = ensurePartsAreEqual(fsConfigs[1], config.Parts[1])
		test.That(t, err, test.ShouldBeNil)
		err = workingDialedClient.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})

	go failingServer.Serve(listener2)
	defer failingServer.Stop()

	failingFSClient, err := New(ctx, listener2.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("client test config for failing frame service", func(t *testing.T) {
		frameSystemParts, err := failingFSClient.FrameSystemConfig(ctx)
		test.That(t, frameSystemParts, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	err = failingFSClient.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	t.Run("dialed client test config for failing frame service with failing config", func(t *testing.T) {
		failingDialedClient, err := New(ctx, listener2.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		parts, err := failingDialedClient.FrameSystemConfig(ctx)
		test.That(t, parts, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)

		err = failingDialedClient.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestClientStatus(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	gServer2 := grpc.NewServer()

	injectRobot := &inject.Robot{
		ResourceNamesFunc:   func() []resource.Name { return []resource.Name{} },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
	}
	injectRobot2 := &inject.Robot{
		ResourceNamesFunc:   func() []resource.Name { return []resource.Name{} },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
	}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	pb.RegisterRobotServiceServer(gServer2, server.New(injectRobot2))

	go gServer.Serve(listener1)
	defer gServer.Stop()

	go gServer2.Serve(listener2)
	defer gServer2.Stop()

	t.Run("failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = New(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("working status service", func(t *testing.T) {
		client, err := New(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		gStatus := robot.Status{Name: movementsensor.Named("gps"), Status: map[string]interface{}{"efg": []string{"hello"}}}
		aStatus := robot.Status{Name: arm.Named("arm"), Status: struct{}{}}
		statusMap := map[resource.Name]robot.Status{
			gStatus.Name: gStatus,
			aStatus.Name: aStatus,
		}
		injectRobot.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
			statuses := make([]robot.Status, 0, len(resourceNames))
			for _, n := range resourceNames {
				statuses = append(statuses, statusMap[n])
			}
			return statuses, nil
		}
		expected := map[resource.Name]interface{}{
			gStatus.Name: map[string]interface{}{"efg": []interface{}{"hello"}},
			aStatus.Name: map[string]interface{}{},
		}
		resp, err := client.Status(context.Background(), []resource.Name{aStatus.Name})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		test.That(t, resp[0].Status, test.ShouldResemble, expected[resp[0].Name])

		result := struct{}{}
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &result})
		test.That(t, err, test.ShouldBeNil)
		err = decoder.Decode(resp[0].Status)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldResemble, aStatus.Status)

		resp, err = client.Status(context.Background(), []resource.Name{gStatus.Name, aStatus.Name})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 2)

		observed := map[resource.Name]interface{}{
			resp[0].Name: resp[0].Status,
			resp[1].Name: resp[1].Status,
		}
		test.That(t, observed, test.ShouldResemble, expected)

		err = client.Close(context.Background())
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("failing status client", func(t *testing.T) {
		client2, err := New(context.Background(), listener2.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		passedErr := errors.New("can't get status")
		injectRobot2.StatusFunc = func(ctx context.Context, status []resource.Name) ([]robot.Status, error) {
			return nil, passedErr
		}
		_, err = client2.Status(context.Background(), []resource.Name{})
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())

		test.That(t, client2.Close(context.Background()), test.ShouldBeNil)
	})
}

func TestForeignResource(t *testing.T) {
	injectRobot := &inject.Robot{}

	desc1, err := grpcreflect.LoadServiceDescriptor(&pb.RobotService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	desc2, err := grpcreflect.LoadServiceDescriptor(&armpb.ArmService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	subtype1 := resource.APINamespace("acme").WithComponentType("huwat")
	subtype2 := resource.APINamespace("acme").WithComponentType("wat")
	respWith := []resource.RPCAPI{
		{
			API:  resource.APINamespace("acme").WithComponentType("huwat"),
			Desc: desc1,
		},
		{
			API:  resource.APINamespace("acme").WithComponentType("wat"),
			Desc: desc2,
		},
	}

	respWithResources := []resource.Name{
		arm.Named("arm1"),
		resource.NewName(subtype1, "thing1"),
		resource.NewName(subtype2, "thing2"),
	}

	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return respWith }
	injectRobot.ResourceNamesFunc = func() []resource.Name { return respWithResources }
	// TODO(RSDK-882): will update this so that this is not necessary
	injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{}, nil
	}

	gServer := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	reflection.Register(gServer)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	logger := golog.NewTestLogger(t)

	go gServer.Serve(listener)
	defer gServer.Stop()

	client, err := New(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	res1, err := client.ResourceByName(respWithResources[0])
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res1, test.ShouldImplement, (*arm.Arm)(nil))

	res2, err := client.ResourceByName(respWithResources[1])
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res2, test.ShouldHaveSameTypeAs, (*rgrpc.ForeignResource)(nil))
	test.That(t, res2.(*rgrpc.ForeignResource).Name(), test.ShouldResemble, respWithResources[1])

	res3, err := client.ResourceByName(respWithResources[2])
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res3, test.ShouldHaveSameTypeAs, (*rgrpc.ForeignResource)(nil))
	test.That(t, res3.(*rgrpc.ForeignResource).Name(), test.ShouldResemble, respWithResources[2])

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestClientStopAll(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	resourcesFunc := func() []resource.Name { return []resource.Name{} }
	stopAllCalled := false
	injectRobot1 := &inject.Robot{
		ResourceNamesFunc:   resourcesFunc,
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
		StopAllFunc: func(ctx context.Context, extra map[resource.Name]map[string]interface{}) error {
			stopAllCalled = true
			return nil
		},
	}
	pb.RegisterRobotServiceServer(gServer1, server.New(injectRobot1))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()

	client, err := New(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	err = client.StopAll(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stopAllCalled, test.ShouldBeTrue)

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestRemoteClientMatch(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	validResources := []resource.Name{arm.Named("remote:arm1")}
	injectRobot1 := &inject.Robot{
		ResourceNamesFunc:   func() []resource.Name { return validResources },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
	}

	// TODO(RSDK-882): will update this so that this is not necessary
	injectRobot1.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{}, nil
	}
	pb.RegisterRobotServiceServer(gServer1, server.New(injectRobot1))

	injectArm := &inject.Arm{}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return pose1, nil
	}

	armSvc1, err := resource.NewAPIResourceCollection(arm.API, map[resource.Name]arm.Arm{arm.Named("remote:arm1"): injectArm})
	test.That(t, err, test.ShouldBeNil)
	gServer1.RegisterService(&armpb.ArmService_ServiceDesc, arm.NewRPCServiceServer(armSvc1))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()

	// working
	dur := 100 * time.Millisecond
	client, err := New(
		context.Background(),
		listener1.Addr().String(),
		logger,
		WithRefreshEvery(dur),
	)
	test.That(t, err, test.ShouldBeNil)

	resource1, err := client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, client.resourceClients[arm.Named("remote:arm1")], test.ShouldEqual, resource1)
	pos, err := resource1.(arm.Arm).EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostEqual(pos, pose1), test.ShouldBeTrue)

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestRemoteClientDuplicate(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	validResources := []resource.Name{arm.Named("remote1:arm1"), arm.Named("remote2:arm1")}
	injectRobot1 := &inject.Robot{
		ResourceNamesFunc:   func() []resource.Name { return validResources },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
	}
	pb.RegisterRobotServiceServer(gServer1, server.New(injectRobot1))

	injectArm := &inject.Arm{}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return pose1, nil
	}

	armSvc1, err := resource.NewAPIResourceCollection(arm.API, map[resource.Name]arm.Arm{
		arm.Named("remote1:arm1"): injectArm,
		arm.Named("remote2:arm1"): injectArm,
	})
	test.That(t, err, test.ShouldBeNil)
	gServer1.RegisterService(&armpb.ArmService_ServiceDesc, arm.NewRPCServiceServer(armSvc1))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()

	// working
	dur := 100 * time.Millisecond
	client, err := New(
		context.Background(),
		listener1.Addr().String(),
		logger,
		WithRefreshEvery(dur),
	)
	test.That(t, err, test.ShouldBeNil)

	_, err = client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(arm.Named("arm1")))

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestClientOperationIntercept(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	injectRobot := &inject.Robot{
		ResourceNamesFunc:   func() []resource.Name { return []resource.Name{} },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
	}

	gServer := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener1)
	defer gServer.Stop()

	ctx := context.Background()
	var fakeArgs interface{}
	fakeManager := operation.NewManager(logger)
	ctx, done := fakeManager.Create(ctx, "fake", fakeArgs)
	defer done()
	fakeOp := operation.Get(ctx)
	test.That(t, fakeOp, test.ShouldNotBeNil)

	client, err := New(ctx, listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	injectRobot.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
		meta, ok := metadata.FromIncomingContext(ctx)
		test.That(t, ok, test.ShouldBeTrue)
		receivedOpID, err := operation.GetOrCreateFromMetadata(meta)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, receivedOpID.String(), test.ShouldEqual, fakeOp.ID.String())
		return []robot.Status{}, nil
	}

	resp, err := client.Status(ctx, []resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(resp), test.ShouldEqual, 0)

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestGetUnknownResource(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	injectRobot := &inject.Robot{
		ResourceNamesFunc:   func() []resource.Name { return []resource.Name{arm.Named("myArm")} },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
	}

	// TODO(RSDK-882): will update this so that this is not necessary
	injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{}, nil
	}

	gServer := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener1)
	defer gServer.Stop()

	client, err := New(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	// grabbing known resource is fine
	myArm, err := client.ResourceByName(arm.Named("myArm"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, myArm, test.ShouldNotBeNil)

	// grabbing unknown resource returns error
	_, err = client.ResourceByName(base.Named("notABase"))
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(base.Named("notABase")))

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}
