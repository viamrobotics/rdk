// Package slam_test client_test.go tests the client for the SLAM service's GRPC server.
package slam_test

import (
	"bytes"
	"context"
	"errors"
	"math"
	"net"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/internal/testhelper"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision"
)

const (
	nameSucc               = "viam"
	nameFail               = "maiv"
	chunkSizeInternalState = 2
	chunkSizePointCloud    = 100
)

func TestClientWorkingService(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	workingServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)
	poseSucc := spatial.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatial.OrientationVector{Theta: math.Pi / 2, OX: 0, OY: 0, OZ: -1})
	componentRefSucc := "cam"
	pcSucc := &vision.Object{}
	pcSucc.PointCloud = pointcloud.New()
	pcdPath := artifact.MustPath("slam/mock_lidar/0.pcd")
	pcd, err := os.ReadFile(pcdPath)
	test.That(t, err, test.ShouldBeNil)

	err = pcSucc.PointCloud.Set(pointcloud.NewVector(5, 5, 5), nil)
	test.That(t, err, test.ShouldBeNil)
	internalStateSucc := []byte{0, 1, 2, 3, 4}

	workingSLAMService := &inject.SLAMService{}

	workingSLAMService.GetPositionFunc = func(ctx context.Context) (spatial.Pose, string, error) {
		return poseSucc, componentRefSucc, nil
	}

	workingSLAMService.GetPointCloudMapFunc = func(ctx context.Context) (func() ([]byte, error), error) {
		reader := bytes.NewReader(pcd)
		clientBuffer := make([]byte, chunkSizePointCloud)
		f := func() ([]byte, error) {
			n, err := reader.Read(clientBuffer)
			if err != nil {
				return nil, err
			}
			return clientBuffer[:n], err
		}
		return f, nil
	}

	workingSLAMService.GetInternalStateFunc = func(ctx context.Context) (func() ([]byte, error), error) {
		reader := bytes.NewReader(internalStateSucc)
		clientBuffer := make([]byte, chunkSizeInternalState)
		f := func() ([]byte, error) {
			n, err := reader.Read(clientBuffer)
			if err != nil {
				return nil, err
			}

			return clientBuffer[:n], err
		}
		return f, nil
	}

	workingSvc, err := subtype.New(map[resource.Name]interface{}{slam.Named(nameSucc): workingSLAMService})
	test.That(t, err, test.ShouldBeNil)

	resourceSubtype := registry.ResourceSubtypeLookup(slam.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), workingServer, workingSvc)

	go workingServer.Serve(listener)
	defer workingServer.Stop()

	t.Run("test that context canceled stops client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = viamgrpc.Dial(cancelCtx, listener.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("client tests for using working SLAM client connection", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		workingSLAMClient := slam.NewClientFromConn(context.Background(), conn, slam.Named(nameSucc).String(), logger)
		// test get position
		pose, componentRef, err := workingSLAMClient.GetPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqual(poseSucc, pose), test.ShouldBeTrue)
		test.That(t, componentRef, test.ShouldEqual, componentRefSucc)

		// test get point cloud map
		fullBytesPCD, err := slam.GetPointCloudMapFull(context.Background(), workingSLAMClient)
		test.That(t, err, test.ShouldBeNil)
		// comparing raw bytes to ensure order is correct
		test.That(t, fullBytesPCD, test.ShouldResemble, pcd)
		// comparing pointclouds to ensure PCDs are correct
		testhelper.TestComparePointCloudsFromPCDs(t, fullBytesPCD, pcd)

		// test get internal state
		fullBytesInternalState, err := slam.GetInternalStateFull(context.Background(), workingSLAMClient)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fullBytesInternalState, test.ShouldResemble, internalStateSucc)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("client tests using working GRPC dial connection", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingDialedClient := slam.NewClientFromConn(context.Background(), conn, nameSucc, logger)

		// test get position
		pose, componentRef, err := workingDialedClient.GetPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqual(poseSucc, pose), test.ShouldBeTrue)
		test.That(t, componentRef, test.ShouldEqual, componentRefSucc)

		// test get point cloud map
		fullBytesPCD, err := slam.GetPointCloudMapFull(context.Background(), workingDialedClient)
		test.That(t, err, test.ShouldBeNil)
		// comparing raw bytes to ensure order is correct
		test.That(t, fullBytesPCD, test.ShouldResemble, pcd)
		// comparing pointclouds to ensure PCDs are correct
		testhelper.TestComparePointCloudsFromPCDs(t, fullBytesPCD, pcd)

		// test get internal state
		fullBytesInternalState, err := slam.GetInternalStateFull(context.Background(), workingDialedClient)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fullBytesInternalState, test.ShouldResemble, internalStateSucc)

		// test do command
		workingSLAMService.DoCommandFunc = generic.EchoFunc
		resp, err := workingDialedClient.DoCommand(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("client tests using working GRPC dial connection converted to SLAM client", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		dialedClient := resourceSubtype.RPCClient(context.Background(), conn, nameSucc, logger)
		workingDialedClient, ok := dialedClient.(slam.Service)
		test.That(t, ok, test.ShouldBeTrue)

		// test get position
		pose, componentRef, err := workingDialedClient.GetPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqual(poseSucc, pose), test.ShouldBeTrue)
		test.That(t, componentRef, test.ShouldEqual, componentRefSucc)

		// test get point cloud map
		fullBytesPCD, err := slam.GetPointCloudMapFull(context.Background(), workingDialedClient)
		test.That(t, err, test.ShouldBeNil)
		// comparing raw bytes to ensure order is correct
		test.That(t, fullBytesPCD, test.ShouldResemble, pcd)
		// comparing pointclouds to ensure PCDs are correct
		testhelper.TestComparePointCloudsFromPCDs(t, fullBytesPCD, pcd)

		// test get internal state
		fullBytesInternalState, err := slam.GetInternalStateFull(context.Background(), workingDialedClient)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fullBytesInternalState, test.ShouldResemble, internalStateSucc)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestFailingClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	failingServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	pcFail := &vision.Object{}
	pcFail.PointCloud = pointcloud.New()
	err = pcFail.PointCloud.Set(pointcloud.NewVector(5, 5, 5), nil)
	test.That(t, err, test.ShouldBeNil)

	failingSLAMService := &inject.SLAMService{}

	failingSLAMService.GetPositionFunc = func(ctx context.Context) (spatial.Pose, string, error) {
		return nil, "", errors.New("failure to get position")
	}

	failingSLAMService.GetPointCloudMapFunc = func(ctx context.Context) (func() ([]byte, error), error) {
		return nil, errors.New("failure during get pointcloud map")
	}

	failingSLAMService.GetInternalStateFunc = func(ctx context.Context) (func() ([]byte, error), error) {
		return nil, errors.New("failure during get internal state")
	}

	failingSvc, err := subtype.New(map[resource.Name]interface{}{slam.Named(nameFail): failingSLAMService})
	test.That(t, err, test.ShouldBeNil)

	resourceSubtype := registry.ResourceSubtypeLookup(slam.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), failingServer, failingSvc)

	go failingServer.Serve(listener)
	defer failingServer.Stop()

	t.Run("client test using bad SLAM client connection", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		failingSLAMClient := slam.NewClientFromConn(context.Background(), conn, slam.Named(nameFail).String(), logger)

		// testing context cancel for streaming apis
		ctx := context.Background()
		cancelCtx, cancelFunc := context.WithCancel(ctx)
		cancelFunc()
		_, err = failingSLAMClient.GetPointCloudMap(cancelCtx)
		test.That(t, err.Error(), test.ShouldContainSubstring, "context cancel")
		_, err = failingSLAMClient.GetInternalState(cancelCtx)
		test.That(t, err.Error(), test.ShouldContainSubstring, "context cancel")

		// test get position
		pose, componentRef, err := failingSLAMClient.GetPosition(context.Background())
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure to get position")
		test.That(t, pose, test.ShouldBeNil)
		test.That(t, componentRef, test.ShouldBeEmpty)

		// test get pointcloud map
		fullBytesPCD, err := slam.GetPointCloudMapFull(context.Background(), failingSLAMClient)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure during get pointcloud map")
		test.That(t, fullBytesPCD, test.ShouldBeNil)

		// test get internal state
		fullBytesInternalState, err := slam.GetInternalStateFull(context.Background(), failingSLAMClient)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure during get internal state")
		test.That(t, fullBytesInternalState, test.ShouldBeNil)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	failingSLAMService.GetPointCloudMapFunc = func(ctx context.Context) (func() ([]byte, error), error) {
		f := func() ([]byte, error) {
			return nil, errors.New("failure during callback")
		}
		return f, nil
	}

	failingSLAMService.GetInternalStateFunc = func(ctx context.Context) (func() ([]byte, error), error) {
		f := func() ([]byte, error) {
			return nil, errors.New("failure during callback")
		}
		return f, nil
	}

	t.Run("client test with failed streaming callback function", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		failingSLAMClient := slam.NewClientFromConn(context.Background(), conn, slam.Named(nameFail).String(), logger)

		// test get pointcloud map
		fullBytesPCD, err := slam.GetPointCloudMapFull(context.Background(), failingSLAMClient)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure during callback")
		test.That(t, fullBytesPCD, test.ShouldBeNil)

		// test get internal state
		fullBytesInternalState, err := slam.GetInternalStateFull(context.Background(), failingSLAMClient)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure during callback")
		test.That(t, fullBytesInternalState, test.ShouldBeNil)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}
