package forcematrix_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/forcematrix"
	viamgrpc "go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClientFailing(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	t.Run("cancelled", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = forcematrix.NewClient(cancelCtx, testForceMatrixName, listener.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("failing", func(t *testing.T) {
		injectFsm := &inject.ForceMatrix{}
		injectFsm.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return nil, errors.New("bad matrix")
		}
		injectFsm.DetectSlipFunc = func(ctx context.Context) (bool, error) {
			return false, errors.New("slip detection error")
		}

		forceMatrixSvc, err := subtype.New(
			map[resource.Name]interface{}{forcematrix.Named(testForceMatrixName): injectFsm})
		test.That(t, err, test.ShouldBeNil)
		pb.RegisterForceMatrixServiceServer(gServer, forcematrix.NewServer(forceMatrixSvc))

		go gServer.Serve(listener)
		defer gServer.Stop()

		t.Run("client 1", func(t *testing.T) {
			forceMatrixClient, err := forcematrix.NewClient(
				context.Background(),
				testForceMatrixName,
				listener.Addr().String(),
				logger,
				rpc.WithInsecure(),
			)
			test.That(t, err, test.ShouldBeNil)

			m, err := forceMatrixClient.ReadMatrix(context.Background())
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "bad matrix")
			test.That(t, m, test.ShouldBeNil)

			isSlipping, err := forceMatrixClient.DetectSlip(context.Background())
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "slip detection error")
			test.That(t, isSlipping, test.ShouldBeFalse)

			test.That(t, utils.TryClose(context.Background(), forceMatrixClient), test.ShouldBeNil)
		})

		t.Run("client 2", func(t *testing.T) {
			conn, err := viamgrpc.Dial(context.Background(),
				listener.Addr().String(), logger, rpc.WithInsecure())
			test.That(t, err, test.ShouldBeNil)
			forceMatrixClient := forcematrix.NewClientFromConn(context.Background(),
				conn, testForceMatrixName, logger)

			m, err := forceMatrixClient.ReadMatrix(context.Background())
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "bad matrix")
			test.That(t, m, test.ShouldBeNil)

			isSlipping, err := forceMatrixClient.DetectSlip(context.Background())
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "slip detection error")
			test.That(t, isSlipping, test.ShouldBeFalse)

			test.That(t, utils.TryClose(context.Background(), forceMatrixClient), test.ShouldBeNil)
		})
	})
}

func TestClientWorking(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	t.Run("working", func(t *testing.T) {
		injectFsm := &inject.ForceMatrix{}
		expectedMatrix := make([][]int, 4)
		for i := 0; i < len(expectedMatrix); i++ {
			expectedMatrix[i] = []int{1, 2, 3, 4}
		}
		injectFsm.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return expectedMatrix, nil
		}
		injectFsm.DetectSlipFunc = func(ctx context.Context) (bool, error) {
			return true, nil
		}

		forceMatrixSvc, err := subtype.New(
			map[resource.Name]interface{}{forcematrix.Named(testForceMatrixName): injectFsm})
		test.That(t, err, test.ShouldBeNil)
		resourceSubtype := registry.ResourceSubtypeLookup(forcematrix.Subtype)
		resourceSubtype.RegisterRPCService(context.Background(), rpcServer, forceMatrixSvc)

		go rpcServer.Serve(listener)
		defer rpcServer.Stop()

		t.Run("client 1", func(t *testing.T) {
			forceMatrixClient, err := forcematrix.NewClient(
				context.Background(),
				testForceMatrixName,
				listener.Addr().String(),
				logger,
				rpc.WithInsecure(),
			)
			test.That(t, err, test.ShouldBeNil)

			m, err := forceMatrixClient.ReadMatrix(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, m, test.ShouldResemble, expectedMatrix)

			isSlipping, err := forceMatrixClient.DetectSlip(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isSlipping, test.ShouldBeTrue)

			rs, err := forceMatrixClient.GetReadings(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rs, test.ShouldResemble, []interface{}{expectedMatrix})

			test.That(t, utils.TryClose(context.Background(), forceMatrixClient), test.ShouldBeNil)
		})

		t.Run("client 2", func(t *testing.T) {
			conn, err := viamgrpc.Dial(context.Background(),
				listener.Addr().String(), logger, rpc.WithInsecure())
			test.That(t, err, test.ShouldBeNil)
			client := resourceSubtype.RPCClient(context.Background(), conn, testForceMatrixName, logger)
			forceMatrixClient, ok := client.(forcematrix.ForceMatrix)
			test.That(t, ok, test.ShouldBeTrue)

			m, err := forceMatrixClient.ReadMatrix(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, m, test.ShouldResemble, expectedMatrix)

			isSlipping, err := forceMatrixClient.DetectSlip(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isSlipping, test.ShouldBeTrue)

			rs, err := forceMatrixClient.GetReadings(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rs, test.ShouldResemble, []interface{}{expectedMatrix})

			test.That(t, conn.Close(), test.ShouldBeNil)
		})
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectForceMatrix := &inject.ForceMatrix{}

	forceMatrixSvc, err := subtype.New(
		map[resource.Name]interface{}{forcematrix.Named(testForceMatrixName): injectForceMatrix})
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterForceMatrixServiceServer(gServer, forcematrix.NewServer(forceMatrixSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := forcematrix.NewClient(ctx, testForceMatrixName, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	client2, err := forcematrix.NewClient(ctx, testForceMatrixName, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.DialCalled, test.ShouldEqual, 2)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}
