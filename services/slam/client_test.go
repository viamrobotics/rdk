// Package slam_test client_test.go tests the client for the SLAM service's GRPC server.
package slam_test

import (
	"bytes"
	"context"
	"math"
	"net"
	"os"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/rpc"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/internal/testhelper"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
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
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	workingServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)
	poseSucc := spatial.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatial.OrientationVector{Theta: math.Pi / 2, OX: 0, OY: 0, OZ: -1})
	componentRefSucc := "cam"
	pcSucc := &vision.Object{}
	pcSucc.PointCloud = pointcloud.New()
	pcdPath := artifact.MustPath("slam/mock_lidar/0.pcd")
	pcd, err := os.ReadFile(pcdPath)
	test.That(t, err, test.ShouldBeNil)

	timestampSucc := time.Now().UTC()
	propSucc := slam.Properties{
		CloudSlam:   false,
		MappingMode: slam.MappingModeNewMap,
	}

	err = pcSucc.PointCloud.Set(pointcloud.NewVector(5, 5, 5), nil)
	test.That(t, err, test.ShouldBeNil)
	internalStateSucc := []byte{0, 1, 2, 3, 4}

	workingSLAMService := &inject.SLAMService{}

	workingSLAMService.PositionFunc = func(ctx context.Context) (spatial.Pose, string, error) {
		return poseSucc, componentRefSucc, nil
	}

	workingSLAMService.PointCloudMapFunc = func(ctx context.Context) (func() ([]byte, error), error) {
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

	workingSLAMService.InternalStateFunc = func(ctx context.Context) (func() ([]byte, error), error) {
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

	workingSLAMService.LatestMapInfoFunc = func(ctx context.Context) (time.Time, error) {
		return timestampSucc, nil
	}

	workingSLAMService.PropertiesFunc = func(ctx context.Context) (slam.Properties, error) {
		return propSucc, nil
	}

	workingSvc, err := resource.NewAPIResourceCollection(slam.API, map[resource.Name]slam.Service{slam.Named(nameSucc): workingSLAMService})
	test.That(t, err, test.ShouldBeNil)

	resourceAPI, ok, err := resource.LookupAPIRegistration[slam.Service](slam.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	resourceAPI.RegisterRPCService(context.Background(), workingServer, workingSvc)

	go workingServer.Serve(listener)
	defer workingServer.Stop()

	t.Run("test that context canceled stops client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = viamgrpc.Dial(cancelCtx, listener.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	//nolint:dupl
	t.Run("client tests for using working SLAM client connection", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		workingSLAMClient, err := slam.NewClientFromConn(context.Background(), conn, "", slam.Named(nameSucc), logger)
		test.That(t, err, test.ShouldBeNil)
		// test position
		pose, componentRef, err := workingSLAMClient.Position(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqual(poseSucc, pose), test.ShouldBeTrue)
		test.That(t, componentRef, test.ShouldEqual, componentRefSucc)

		// test point cloud map
		fullBytesPCD, err := slam.PointCloudMapFull(context.Background(), workingSLAMClient)
		test.That(t, err, test.ShouldBeNil)
		// comparing raw bytes to ensure order is correct
		test.That(t, fullBytesPCD, test.ShouldResemble, pcd)
		// comparing pointclouds to ensure PCDs are correct
		testhelper.TestComparePointCloudsFromPCDs(t, fullBytesPCD, pcd)

		// test internal state
		fullBytesInternalState, err := slam.InternalStateFull(context.Background(), workingSLAMClient)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fullBytesInternalState, test.ShouldResemble, internalStateSucc)

		// test latest map info
		timestamp, err := workingSLAMClient.LatestMapInfo(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, timestamp, test.ShouldResemble, timestampSucc)

		// test properties
		prop, err := workingSLAMClient.Properties(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, prop.CloudSlam, test.ShouldBeFalse)
		test.That(t, prop.CloudSlam, test.ShouldEqual, propSucc.CloudSlam)
		test.That(t, prop.MappingMode, test.ShouldEqual, propSucc.MappingMode)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("client tests using working GRPC dial connection", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingDialedClient, err := slam.NewClientFromConn(context.Background(), conn, "", slam.Named(nameSucc), logger)
		test.That(t, err, test.ShouldBeNil)

		// test position
		pose, componentRef, err := workingDialedClient.Position(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqual(poseSucc, pose), test.ShouldBeTrue)
		test.That(t, componentRef, test.ShouldEqual, componentRefSucc)

		// test point cloud map
		fullBytesPCD, err := slam.PointCloudMapFull(context.Background(), workingDialedClient)
		test.That(t, err, test.ShouldBeNil)
		// comparing raw bytes to ensure order is correct
		test.That(t, fullBytesPCD, test.ShouldResemble, pcd)
		// comparing pointclouds to ensure PCDs are correct
		testhelper.TestComparePointCloudsFromPCDs(t, fullBytesPCD, pcd)

		// test internal state
		fullBytesInternalState, err := slam.InternalStateFull(context.Background(), workingDialedClient)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fullBytesInternalState, test.ShouldResemble, internalStateSucc)

		// test latest map info
		timestamp, err := workingDialedClient.LatestMapInfo(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, timestamp, test.ShouldResemble, timestampSucc)

		// test properties
		prop, err := workingDialedClient.Properties(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, prop.CloudSlam, test.ShouldBeFalse)
		test.That(t, prop.CloudSlam, test.ShouldEqual, propSucc.CloudSlam)
		test.That(t, prop.MappingMode, test.ShouldEqual, propSucc.MappingMode)

		// test do command
		workingSLAMService.DoCommandFunc = testutils.EchoFunc
		resp, err := workingDialedClient.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	//nolint:dupl
	t.Run("client tests using working GRPC dial connection converted to SLAM client", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		dialedClient, err := resourceAPI.RPCClient(context.Background(), conn, "", slam.Named(nameSucc), logger)
		test.That(t, err, test.ShouldBeNil)

		// test position
		pose, componentRef, err := dialedClient.Position(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqual(poseSucc, pose), test.ShouldBeTrue)
		test.That(t, componentRef, test.ShouldEqual, componentRefSucc)

		// test point cloud map
		fullBytesPCD, err := slam.PointCloudMapFull(context.Background(), dialedClient)
		test.That(t, err, test.ShouldBeNil)
		// comparing raw bytes to ensure order is correct
		test.That(t, fullBytesPCD, test.ShouldResemble, pcd)
		// comparing pointclouds to ensure PCDs are correct
		testhelper.TestComparePointCloudsFromPCDs(t, fullBytesPCD, pcd)

		// test internal state
		fullBytesInternalState, err := slam.InternalStateFull(context.Background(), dialedClient)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fullBytesInternalState, test.ShouldResemble, internalStateSucc)

		// test latest map info
		timestamp, err := dialedClient.LatestMapInfo(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, timestamp, test.ShouldResemble, timestampSucc)

		// test properties
		prop, err := dialedClient.Properties(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, prop.CloudSlam, test.ShouldBeFalse)
		test.That(t, prop.CloudSlam, test.ShouldEqual, propSucc.CloudSlam)
		test.That(t, prop.MappingMode, test.ShouldEqual, propSucc.MappingMode)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestFailingClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	failingServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	pcFail := &vision.Object{}
	pcFail.PointCloud = pointcloud.New()
	err = pcFail.PointCloud.Set(pointcloud.NewVector(5, 5, 5), nil)
	test.That(t, err, test.ShouldBeNil)

	failingSLAMService := &inject.SLAMService{}

	failingSLAMService.PositionFunc = func(ctx context.Context) (spatial.Pose, string, error) {
		return nil, "", errors.New("failure to get position")
	}

	failingSLAMService.PointCloudMapFunc = func(ctx context.Context) (func() ([]byte, error), error) {
		return nil, errors.New("failure during get pointcloud map")
	}

	failingSLAMService.InternalStateFunc = func(ctx context.Context) (func() ([]byte, error), error) {
		return nil, errors.New("failure during get internal state")
	}

	failingSLAMService.LatestMapInfoFunc = func(ctx context.Context) (time.Time, error) {
		return time.Time{}, errors.New("failure to get latest map info")
	}

	errBadProperties := errors.New("failure to get properties")
	failingSLAMService.PropertiesFunc = func(ctx context.Context) (slam.Properties, error) {
		return slam.Properties{}, errBadProperties
	}

	failingSvc, err := resource.NewAPIResourceCollection(slam.API, map[resource.Name]slam.Service{slam.Named(nameFail): failingSLAMService})
	test.That(t, err, test.ShouldBeNil)

	resourceAPI, ok, err := resource.LookupAPIRegistration[slam.Service](slam.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	resourceAPI.RegisterRPCService(context.Background(), failingServer, failingSvc)

	go failingServer.Serve(listener)
	defer failingServer.Stop()

	t.Run("client test using bad SLAM client connection", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		failingSLAMClient, err := slam.NewClientFromConn(context.Background(), conn, "", slam.Named(nameFail), logger)
		test.That(t, err, test.ShouldBeNil)

		// testing context cancel for streaming apis
		ctx := context.Background()
		cancelCtx, cancelFunc := context.WithCancel(ctx)
		cancelFunc()

		_, err = failingSLAMClient.PointCloudMap(cancelCtx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "context cancel")
		_, err = failingSLAMClient.InternalState(cancelCtx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "context cancel")

		// test position
		pose, componentRef, err := failingSLAMClient.Position(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure to get position")
		test.That(t, pose, test.ShouldBeNil)
		test.That(t, componentRef, test.ShouldBeEmpty)

		// test pointcloud map
		fullBytesPCD, err := slam.PointCloudMapFull(context.Background(), failingSLAMClient)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure during get pointcloud map")
		test.That(t, fullBytesPCD, test.ShouldBeNil)

		// test internal state
		fullBytesInternalState, err := slam.InternalStateFull(context.Background(), failingSLAMClient)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure during get internal state")
		test.That(t, fullBytesInternalState, test.ShouldBeNil)

		// test latest map info
		timestamp, err := failingSLAMClient.LatestMapInfo(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure to get latest map info")
		test.That(t, timestamp, test.ShouldResemble, time.Time{})

		// test properties
		prop, err := failingSLAMClient.Properties(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure to get properties")
		test.That(t, prop, test.ShouldResemble, slam.Properties{})

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	failingSLAMService.PointCloudMapFunc = func(ctx context.Context) (func() ([]byte, error), error) {
		f := func() ([]byte, error) {
			return nil, errors.New("failure during callback")
		}
		return f, nil
	}

	failingSLAMService.InternalStateFunc = func(ctx context.Context) (func() ([]byte, error), error) {
		f := func() ([]byte, error) {
			return nil, errors.New("failure during callback")
		}
		return f, nil
	}

	t.Run("client test with failed streaming callback function", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		failingSLAMClient, err := slam.NewClientFromConn(context.Background(), conn, "", slam.Named(nameFail), logger)
		test.That(t, err, test.ShouldBeNil)

		// test pointcloud map
		fullBytesPCD, err := slam.PointCloudMapFull(context.Background(), failingSLAMClient)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure during callback")
		test.That(t, fullBytesPCD, test.ShouldBeNil)

		// test internal state
		fullBytesInternalState, err := slam.InternalStateFull(context.Background(), failingSLAMClient)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure during callback")
		test.That(t, fullBytesInternalState, test.ShouldBeNil)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}
