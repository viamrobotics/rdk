package arm_test

import (
	"context"
	"errors"
	"math"
	"net"
	"sync"
	"testing"
	"time"

	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/components/arm"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

// setupStreamedServer stands up a real rpc server backed by injectArm and returns a dialed
// connection. Server and connection are torn down via t.Cleanup so goleak stays satisfied.
func setupStreamedServer(t *testing.T, logger logging.Logger, injectArm *inject.Arm) rpc.ClientConn {
	t.Helper()
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	armSvc, err := resource.NewAPIResourceCollection(arm.API, map[resource.Name]arm.Arm{
		arm.Named(testArmName): injectArm,
	})
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[arm.Arm](arm.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, armSvc, logger), test.ShouldBeNil)

	go rpcServer.Serve(listener)

	conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	t.Cleanup(func() {
		test.That(t, conn.Close(), test.ShouldBeNil)
		test.That(t, rpcServer.Stop(), test.ShouldBeNil)
	})
	return conn
}

// driveStreamed plays the caller/framework side of the streamed RPC: it feeds wireBatches, closes
// the send channel, drains responses, and closes the response channel only after the wrapper
// returns (per the channel-ownership contract). It returns the number of responses seen and the
// wrapper's terminal error.
func driveStreamed(
	ctx context.Context,
	client arm.Arm,
	wireBatches [][]arm.TrajectoryPoint,
	extra map[string]interface{},
) (int, error) {
	batches := make(chan []arm.TrajectoryPoint)
	responses := make(chan arm.Response)
	errCh := make(chan error, 1)
	go func() {
		errCh <- client.MoveThroughJointPositionsStreamed(ctx, batches, responses, extra)
	}()

	respCount := 0
	drained := make(chan struct{})
	go func() {
		for range responses {
			respCount++
		}
		close(drained)
	}()

	// Feed batches, but bail out if the wrapper returns first (e.g. a server-side error stops it
	// reading), so we never block forever on a send nobody is receiving.
	stopFeeding := make(chan struct{})
	go func() {
		defer close(batches)
		for _, b := range wireBatches {
			select {
			case batches <- b:
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
			gotBatches [][]arm.TrajectoryPoint
			gotExtra   map[string]interface{}
		)
		injectArm := &inject.Arm{}
		injectArm.KinematicsFunc = func(ctx context.Context) (referenceframe.Model, error) {
			return nil, errKinematicsUnimplemented
		}
		injectArm.MoveThroughJointPositionsStreamedFunc = func(
			ctx context.Context,
			batches <-chan []arm.TrajectoryPoint,
			responses chan<- arm.Response,
			extra map[string]interface{},
		) error {
			mu.Lock()
			gotExtra = extra
			mu.Unlock()
			for batch := range batches {
				mu.Lock()
				gotBatches = append(gotBatches, batch)
				mu.Unlock()
				responses <- arm.Response{}
			}
			return nil
		}
		conn := setupStreamedServer(t, logger, injectArm)
		client, err := arm.NewClientFromConn(context.Background(), conn, "", arm.Named(testArmName), logger)
		test.That(t, err, test.ShouldBeNil)

		wireBatches := [][]arm.TrajectoryPoint{
			{{Time: 0, Positions: []referenceframe.Input{0, math.Pi / 2}, Constraints: &arm.KinematicConstraints{Velocities: []float64{1, 2}}}},
			{
				{Time: 100 * time.Millisecond, Positions: []referenceframe.Input{math.Pi, 0}},
				{Time: 200 * time.Millisecond, Positions: []referenceframe.Input{0, 0}},
			},
		}
		respCount, err := driveStreamed(context.Background(), client, wireBatches, map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, respCount, test.ShouldEqual, 2)

		mu.Lock()
		defer mu.Unlock()
		test.That(t, gotExtra, test.ShouldResemble, map[string]interface{}{"foo": "bar"})
		test.That(t, len(gotBatches), test.ShouldEqual, 2)
		test.That(t, len(gotBatches[0]), test.ShouldEqual, 1)
		test.That(t, len(gotBatches[1]), test.ShouldEqual, 2)
		test.That(t, gotBatches[0][0].Positions[1], test.ShouldAlmostEqual, math.Pi/2)
		test.That(t, gotBatches[0][0].Constraints.Velocities[0], test.ShouldAlmostEqual, 1)
		test.That(t, gotBatches[1][1].Time, test.ShouldEqual, 200*time.Millisecond)
	})

	t.Run("impl error becomes terminal status", func(t *testing.T) {
		injectArm := &inject.Arm{}
		injectArm.KinematicsFunc = func(ctx context.Context) (referenceframe.Model, error) {
			return nil, errKinematicsUnimplemented
		}
		injectArm.MoveThroughJointPositionsStreamedFunc = func(
			ctx context.Context,
			batches <-chan []arm.TrajectoryPoint,
			responses chan<- arm.Response,
			extra map[string]interface{},
		) error {
			for range batches {
			}
			return errors.New("boom")
		}
		conn := setupStreamedServer(t, logger, injectArm)
		client, err := arm.NewClientFromConn(context.Background(), conn, "", arm.Named(testArmName), logger)
		test.That(t, err, test.ShouldBeNil)

		wireBatches := [][]arm.TrajectoryPoint{{{Time: 0, Positions: []referenceframe.Input{0, 0}}}}
		_, err = driveStreamed(context.Background(), client, wireBatches, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "boom")
	})

	t.Run("client cancellation is honored", func(t *testing.T) {
		started := make(chan struct{})
		injectArm := &inject.Arm{}
		injectArm.KinematicsFunc = func(ctx context.Context) (referenceframe.Model, error) {
			return nil, errKinematicsUnimplemented
		}
		injectArm.MoveThroughJointPositionsStreamedFunc = func(
			ctx context.Context,
			batches <-chan []arm.TrajectoryPoint,
			responses chan<- arm.Response,
			extra map[string]interface{},
		) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}
		conn := setupStreamedServer(t, logger, injectArm)
		client, err := arm.NewClientFromConn(context.Background(), conn, "", arm.Named(testArmName), logger)
		test.That(t, err, test.ShouldBeNil)

		ctx, cancel := context.WithCancel(context.Background())
		batches := make(chan []arm.TrajectoryPoint)
		responses := make(chan arm.Response)
		errCh := make(chan error, 1)
		go func() {
			errCh <- client.MoveThroughJointPositionsStreamed(ctx, batches, responses, nil)
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
		close(batches)
		close(responses)
		<-drained
		test.That(t, err, test.ShouldNotBeNil)
	})

	// Raw-protocol faults must surface as terminal InvalidArgument statuses (task: recv-side error
	// surfacing). These use the generated client directly to send malformed message sequences the
	// typed wrapper would never produce.
	t.Run("first message not Init", func(t *testing.T) {
		injectArm := &inject.Arm{}
		conn := setupStreamedServer(t, logger, injectArm)
		raw := pb.NewArmServiceClient(conn)
		stream, err := raw.MoveThroughJointPositionsStreamed(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, stream.Send(&pb.MoveThroughJointPositionsStreamedRequest{
			Name:    testArmName,
			Message: &pb.MoveThroughJointPositionsStreamedRequest_Batch{Batch: &pb.MoveThroughJointPositionsStreamedRequest_TrajectoryBatch{}},
		}), test.ShouldBeNil)
		test.That(t, stream.CloseSend(), test.ShouldBeNil)
		_, err = stream.Recv()
		test.That(t, status.Code(err), test.ShouldEqual, codes.InvalidArgument)
	})

	t.Run("non-batch after Init", func(t *testing.T) {
		injectArm := &inject.Arm{}
		injectArm.KinematicsFunc = func(ctx context.Context) (referenceframe.Model, error) {
			return nil, errKinematicsUnimplemented
		}
		injectArm.MoveThroughJointPositionsStreamedFunc = func(
			ctx context.Context,
			batches <-chan []arm.TrajectoryPoint,
			responses chan<- arm.Response,
			extra map[string]interface{},
		) error {
			for range batches {
			}
			return nil
		}
		conn := setupStreamedServer(t, logger, injectArm)
		raw := pb.NewArmServiceClient(conn)
		stream, err := raw.MoveThroughJointPositionsStreamed(context.Background())
		test.That(t, err, test.ShouldBeNil)
		initMsg := &pb.MoveThroughJointPositionsStreamedRequest{
			Name:    testArmName,
			Message: &pb.MoveThroughJointPositionsStreamedRequest_Init_{Init: &pb.MoveThroughJointPositionsStreamedRequest_Init{}},
		}
		test.That(t, stream.Send(initMsg), test.ShouldBeNil)
		// A second Init is not a TrajectoryBatch and must be rejected.
		test.That(t, stream.Send(initMsg), test.ShouldBeNil)
		_, err = stream.Recv()
		test.That(t, status.Code(err), test.ShouldEqual, codes.InvalidArgument)
	})

	t.Run("malformed point", func(t *testing.T) {
		injectArm := &inject.Arm{}
		injectArm.KinematicsFunc = func(ctx context.Context) (referenceframe.Model, error) {
			return referenceframe.ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/ur5e.json"), "foo")
		}
		injectArm.MoveThroughJointPositionsStreamedFunc = func(
			ctx context.Context,
			batches <-chan []arm.TrajectoryPoint,
			responses chan<- arm.Response,
			extra map[string]interface{},
		) error {
			for range batches {
			}
			return nil
		}
		conn := setupStreamedServer(t, logger, injectArm)
		raw := pb.NewArmServiceClient(conn)
		stream, err := raw.MoveThroughJointPositionsStreamed(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, stream.Send(&pb.MoveThroughJointPositionsStreamedRequest{
			Name:    testArmName,
			Message: &pb.MoveThroughJointPositionsStreamedRequest_Init_{Init: &pb.MoveThroughJointPositionsStreamedRequest_Init{}},
		}), test.ShouldBeNil)
		// The model has 6 DoF; a 3-value point cannot be interpreted and must be rejected.
		test.That(t, stream.Send(&pb.MoveThroughJointPositionsStreamedRequest{
			Message: &pb.MoveThroughJointPositionsStreamedRequest_Batch{
				Batch: &pb.MoveThroughJointPositionsStreamedRequest_TrajectoryBatch{
					Points: []*pb.TrajectoryPoint{{Positions: &pb.JointPositions{Values: []float64{1, 2, 3}}}},
				},
			},
		}), test.ShouldBeNil)
		_, err = stream.Recv()
		test.That(t, status.Code(err), test.ShouldEqual, codes.InvalidArgument)
	})
}
