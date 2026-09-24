package motion_test

import (
	"context"
	"errors"
	"io"
	"math"
	"net"
	"sync"
	"testing"

	armpb "go.viam.com/api/component/arm/v1"
	motionpb "go.viam.com/api/service/motion/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/components/arm"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	injectmotion "go.viam.com/rdk/testutils/inject/motion"
)

const testStreamedArmName = "arm1"

// setupStreamedServer stands up a real rpc server backed by injectMS and returns a dialed
// connection. Server and connection are torn down via t.Cleanup so goleak stays satisfied.
func setupStreamedServer(t *testing.T, logger logging.Logger, injectMS *injectmotion.MotionService) rpc.ClientConn {
	t.Helper()
	//nolint: noctx
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	svc, err := resource.NewAPIResourceCollection(motion.API, map[resource.Name]motion.Service{
		testMotionServiceName: injectMS,
	})
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[motion.Service](motion.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, svc, logger), test.ShouldBeNil)

	go rpcServer.Serve(listener)

	conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	t.Cleanup(func() {
		test.That(t, conn.Close(), test.ShouldBeNil)
		test.That(t, rpcServer.Stop(), test.ShouldBeNil)
	})
	return conn
}

// driveStreamed plays the caller/framework side of the streamed RPC: it feeds waypoints, closes
// the targets channel, drains responses, and closes the responses channel only after the wrapper
// returns (per the channel-ownership contract). It returns the number of responses seen and the
// wrapper's terminal error.
func driveStreamed(
	ctx context.Context,
	client motion.Service,
	armName string,
	opts motion.TempStreamOptions,
	waypoints [][]referenceframe.Input,
	extra map[string]interface{},
) (int, error) {
	targets := make(chan []referenceframe.Input)
	responses := make(chan motion.TempStreamResponse)
	errCh := make(chan error, 1)
	go func() {
		errCh <- client.TempStreamArmJointPositions(ctx, armName, opts, targets, responses, extra)
	}()

	respCount := 0
	drained := make(chan struct{})
	go func() {
		for range responses {
			respCount++
		}
		close(drained)
	}()

	// Feed targets, but bail out if the wrapper returns first (e.g. a server-side error stops it
	// reading), so we never block forever on a send nobody is receiving.
	stopFeeding := make(chan struct{})
	go func() {
		defer close(targets)
		for _, wp := range waypoints {
			select {
			case targets <- wp:
			case <-stopFeeding:
				return
			}
		}
	}()

	err := <-errCh
	close(stopFeeding)
	close(responses)
	<-drained
	return respCount, err
}

func TestClientStreamed(t *testing.T) {
	logger := logging.NewTestLogger(t)

	t.Run("happy path round trip", func(t *testing.T) {
		var (
			mu         sync.Mutex
			gotArmName string
			gotOpts    motion.TempStreamOptions
			gotTargets [][]referenceframe.Input
			gotExtra   map[string]interface{}
		)
		injectMS := injectmotion.NewMotionService(testMotionServiceName.Name)
		injectMS.TempStreamArmJointPositionsFunc = func(
			ctx context.Context,
			armName string,
			opts motion.TempStreamOptions,
			targets <-chan []referenceframe.Input,
			responses chan<- motion.TempStreamResponse,
			extra map[string]interface{},
		) error {
			mu.Lock()
			gotArmName = armName
			gotOpts = opts
			gotExtra = extra
			mu.Unlock()
			for target := range targets {
				mu.Lock()
				gotTargets = append(gotTargets, target)
				mu.Unlock()
				responses <- motion.TempStreamResponse{}
			}
			return nil
		}
		conn := setupStreamedServer(t, logger, injectMS)
		client, err := motion.NewClientFromConn(context.Background(), conn, "", testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)

		runway, interval, window := int32(100), int32(10), int32(60)
		opts := motion.TempStreamOptions{
			ArmSideTargetRunwayMs: &runway,
			SendToArmIntervalMs:   &interval,
			DiagnosticsWindowSecs: &window,
			MoveOptions:           &arm.MoveOptions{MaxVelRads: 1.5, MaxAccRads: 2.5},
		}
		waypoints := [][]referenceframe.Input{{0, 1, 2}, {3, 4, 5}, {6, 7, 8}}
		respCount, err := driveStreamed(
			context.Background(), client, testStreamedArmName, opts, waypoints, map[string]interface{}{"foo": "bar"},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, respCount, test.ShouldEqual, len(waypoints))

		mu.Lock()
		defer mu.Unlock()
		test.That(t, gotArmName, test.ShouldEqual, testStreamedArmName)
		test.That(t, gotExtra, test.ShouldResemble, map[string]interface{}{"foo": "bar"})
		// Targets cross the wire in degrees and come back as radians, so compare within float error.
		test.That(t, len(gotTargets), test.ShouldEqual, len(waypoints))
		for i, wp := range waypoints {
			test.That(t, len(gotTargets[i]), test.ShouldEqual, len(wp))
			for j, v := range wp {
				test.That(t, gotTargets[i][j], test.ShouldAlmostEqual, v)
			}
		}
		test.That(t, *gotOpts.ArmSideTargetRunwayMs, test.ShouldEqual, runway)
		test.That(t, *gotOpts.SendToArmIntervalMs, test.ShouldEqual, interval)
		test.That(t, *gotOpts.DiagnosticsWindowSecs, test.ShouldEqual, window)
		test.That(t, gotOpts.MoveOptions, test.ShouldNotBeNil)
		test.That(t, gotOpts.MoveOptions.MaxVelRads, test.ShouldAlmostEqual, 1.5)
		test.That(t, gotOpts.MoveOptions.MaxAccRads, test.ShouldAlmostEqual, 2.5)
	})

	t.Run("impl error becomes terminal status", func(t *testing.T) {
		injectMS := injectmotion.NewMotionService(testMotionServiceName.Name)
		injectMS.TempStreamArmJointPositionsFunc = func(
			ctx context.Context,
			armName string,
			opts motion.TempStreamOptions,
			targets <-chan []referenceframe.Input,
			responses chan<- motion.TempStreamResponse,
			extra map[string]interface{},
		) error {
			for range targets {
			}
			return errors.New("boom")
		}
		conn := setupStreamedServer(t, logger, injectMS)
		client, err := motion.NewClientFromConn(context.Background(), conn, "", testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)

		waypoints := [][]referenceframe.Input{{0, 0}}
		_, err = driveStreamed(context.Background(), client, testStreamedArmName, motion.TempStreamOptions{}, waypoints, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "boom")
	})

	t.Run("client cancellation is honored", func(t *testing.T) {
		started := make(chan struct{})
		injectMS := injectmotion.NewMotionService(testMotionServiceName.Name)
		injectMS.TempStreamArmJointPositionsFunc = func(
			ctx context.Context,
			armName string,
			opts motion.TempStreamOptions,
			targets <-chan []referenceframe.Input,
			responses chan<- motion.TempStreamResponse,
			extra map[string]interface{},
		) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}
		conn := setupStreamedServer(t, logger, injectMS)
		client, err := motion.NewClientFromConn(context.Background(), conn, "", testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)

		ctx, cancel := context.WithCancel(context.Background())
		targets := make(chan []referenceframe.Input)
		responses := make(chan motion.TempStreamResponse)
		errCh := make(chan error, 1)
		go func() {
			errCh <- client.TempStreamArmJointPositions(ctx, testStreamedArmName, motion.TempStreamOptions{}, targets, responses, nil)
		}()
		drained := make(chan struct{})
		go func() {
			for range responses {
			}
			close(drained)
		}()

		<-started
		cancel()
		err = <-errCh
		close(targets)
		close(responses)
		<-drained
		test.That(t, err, test.ShouldNotBeNil)
	})

	// Raw-protocol faults must surface to the client as terminal InvalidArgument statuses. These use
	// the generated client directly to send malformed message sequences the typed wrapper would
	// never produce.
	t.Run("first message not Init", func(t *testing.T) {
		injectMS := injectmotion.NewMotionService(testMotionServiceName.Name)
		conn := setupStreamedServer(t, logger, injectMS)
		raw := motionpb.NewMotionServiceClient(conn)
		stream, err := raw.TempStreamArmJointPositions(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, stream.Send(&motionpb.TempStreamArmJointPositionsRequest{
			Name: testMotionServiceName.Name,
			Message: &motionpb.TempStreamArmJointPositionsRequest_Targets_{
				Targets: &motionpb.TempStreamArmJointPositionsRequest_Targets{
					Positions: []*armpb.JointPositions{{Values: []float64{0, 0}}},
				},
			},
		}), test.ShouldBeNil)
		test.That(t, stream.CloseSend(), test.ShouldBeNil)
		_, err = stream.Recv()
		test.That(t, status.Code(err), test.ShouldEqual, codes.InvalidArgument)
	})

	t.Run("Init without component name", func(t *testing.T) {
		injectMS := injectmotion.NewMotionService(testMotionServiceName.Name)
		conn := setupStreamedServer(t, logger, injectMS)
		raw := motionpb.NewMotionServiceClient(conn)
		stream, err := raw.TempStreamArmJointPositions(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, stream.Send(&motionpb.TempStreamArmJointPositionsRequest{
			Name:    testMotionServiceName.Name,
			Message: &motionpb.TempStreamArmJointPositionsRequest_Init_{Init: &motionpb.TempStreamArmJointPositionsRequest_Init{}},
		}), test.ShouldBeNil)
		test.That(t, stream.CloseSend(), test.ShouldBeNil)
		_, err = stream.Recv()
		test.That(t, status.Code(err), test.ShouldEqual, codes.InvalidArgument)
	})

	t.Run("non-Targets after Init", func(t *testing.T) {
		injectMS := injectmotion.NewMotionService(testMotionServiceName.Name)
		injectMS.TempStreamArmJointPositionsFunc = func(
			ctx context.Context,
			armName string,
			opts motion.TempStreamOptions,
			targets <-chan []referenceframe.Input,
			responses chan<- motion.TempStreamResponse,
			extra map[string]interface{},
		) error {
			for range targets {
			}
			return nil
		}
		conn := setupStreamedServer(t, logger, injectMS)
		raw := motionpb.NewMotionServiceClient(conn)
		stream, err := raw.TempStreamArmJointPositions(context.Background())
		test.That(t, err, test.ShouldBeNil)
		initMsg := &motionpb.TempStreamArmJointPositionsRequest{
			Name: testMotionServiceName.Name,
			Message: &motionpb.TempStreamArmJointPositionsRequest_Init_{
				Init: &motionpb.TempStreamArmJointPositionsRequest_Init{ComponentName: testStreamedArmName},
			},
		}
		test.That(t, stream.Send(initMsg), test.ShouldBeNil)
		// A second Init is not a Targets message and must be rejected.
		test.That(t, stream.Send(initMsg), test.ShouldBeNil)
		_, err = stream.Recv()
		test.That(t, status.Code(err), test.ShouldEqual, codes.InvalidArgument)
	})

	t.Run("nil options round trip as nil", func(t *testing.T) {
		var (
			mu      sync.Mutex
			gotOpts motion.TempStreamOptions
		)
		injectMS := injectmotion.NewMotionService(testMotionServiceName.Name)
		injectMS.TempStreamArmJointPositionsFunc = func(
			ctx context.Context,
			armName string,
			opts motion.TempStreamOptions,
			targets <-chan []referenceframe.Input,
			responses chan<- motion.TempStreamResponse,
			extra map[string]interface{},
		) error {
			mu.Lock()
			gotOpts = opts
			mu.Unlock()
			for range targets {
			}
			return nil
		}
		conn := setupStreamedServer(t, logger, injectMS)
		client, err := motion.NewClientFromConn(context.Background(), conn, "", testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)

		_, err = driveStreamed(context.Background(), client, testStreamedArmName, motion.TempStreamOptions{}, nil, nil)
		test.That(t, err, test.ShouldBeNil)

		mu.Lock()
		defer mu.Unlock()
		test.That(t, gotOpts.ArmSideTargetRunwayMs, test.ShouldBeNil)
		test.That(t, gotOpts.SendToArmIntervalMs, test.ShouldBeNil)
		test.That(t, gotOpts.DiagnosticsWindowSecs, test.ShouldBeNil)
		test.That(t, gotOpts.MoveOptions, test.ShouldBeNil)
	})

	t.Run("server converts wire degrees to radians", func(t *testing.T) {
		var (
			mu         sync.Mutex
			gotTargets [][]referenceframe.Input
		)
		injectMS := injectmotion.NewMotionService(testMotionServiceName.Name)
		injectMS.TempStreamArmJointPositionsFunc = func(
			ctx context.Context,
			armName string,
			opts motion.TempStreamOptions,
			targets <-chan []referenceframe.Input,
			responses chan<- motion.TempStreamResponse,
			extra map[string]interface{},
		) error {
			for target := range targets {
				mu.Lock()
				gotTargets = append(gotTargets, target)
				mu.Unlock()
			}
			return nil
		}
		conn := setupStreamedServer(t, logger, injectMS)
		raw := motionpb.NewMotionServiceClient(conn)
		stream, err := raw.TempStreamArmJointPositions(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, stream.Send(&motionpb.TempStreamArmJointPositionsRequest{
			Name: testMotionServiceName.Name,
			Message: &motionpb.TempStreamArmJointPositionsRequest_Init_{
				Init: &motionpb.TempStreamArmJointPositionsRequest_Init{ComponentName: testStreamedArmName},
			},
		}), test.ShouldBeNil)
		test.That(t, stream.Send(&motionpb.TempStreamArmJointPositionsRequest{
			Message: &motionpb.TempStreamArmJointPositionsRequest_Targets_{
				Targets: &motionpb.TempStreamArmJointPositionsRequest_Targets{
					Positions: []*armpb.JointPositions{{Values: []float64{180, 90, 0}}},
				},
			},
		}), test.ShouldBeNil)
		test.That(t, stream.CloseSend(), test.ShouldBeNil)
		for {
			if _, err := stream.Recv(); err != nil {
				test.That(t, errors.Is(err, io.EOF), test.ShouldBeTrue)
				break
			}
		}

		mu.Lock()
		defer mu.Unlock()
		test.That(t, len(gotTargets), test.ShouldEqual, 1)
		test.That(t, gotTargets[0][0], test.ShouldAlmostEqual, math.Pi)
		test.That(t, gotTargets[0][1], test.ShouldAlmostEqual, math.Pi/2)
		test.That(t, gotTargets[0][2], test.ShouldAlmostEqual, 0)
	})
}
