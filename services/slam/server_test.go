// Package slam_test server_test.go tests the SLAM service's GRPC server.
package slam_test

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"testing"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/slam/v1"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/protoutils"
	"google.golang.org/grpc"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/internal/testhelper"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testSlamServiceName  = "slam1"
	testSlamServiceName2 = "slam2"
	chunkSizeServer      = 100
)

// Create mock server that satisfies the pb.SLAMService_GetPointCloudMapServer contract.
type pointCloudServerMock struct {
	grpc.ServerStream
	rawBytes []byte
}
type internalStateServerMock struct {
	grpc.ServerStream
	rawBytes []byte
}

func makePointCloudServerMock() *pointCloudServerMock {
	return &pointCloudServerMock{}
}

// Concatenate received messages into single slice.
func (m *pointCloudServerMock) Send(chunk *pb.GetPointCloudMapResponse) error {
	m.rawBytes = append(m.rawBytes, chunk.PointCloudPcdChunk...)
	return nil
}

func makeInternalStateServerMock() *internalStateServerMock {
	return &internalStateServerMock{}
}

// Concatenate received messages into single slice.
func (m *internalStateServerMock) Send(chunk *pb.GetInternalStateResponse) error {
	m.rawBytes = append(m.rawBytes, chunk.InternalStateChunk...)
	return nil
}

func TestWorkingServer(t *testing.T) {
	injectSvc := &inject.SLAMService{}
	resourceMap := map[resource.Name]slam.Service{
		slam.Named(testSlamServiceName): injectSvc,
	}
	injectAPISvc, err := resource.NewAPIResourceCollection(slam.API, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	slamServer := slam.NewRPCServiceServer(injectAPISvc).(pb.SLAMServiceServer)
	cloudPath := artifact.MustPath("slam/mock_lidar/0.pcd")
	pcd, err := os.ReadFile(cloudPath)
	test.That(t, err, test.ShouldBeNil)

	t.Run("working GetPosition", func(t *testing.T) {
		poseSucc := spatial.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatial.OrientationVector{Theta: math.Pi / 2, OX: 0, OY: 0, OZ: -1})
		componentRefSucc := "cam"

		injectSvc.PositionFunc = func(ctx context.Context) (spatial.Pose, string, error) {
			return poseSucc, componentRefSucc, nil
		}

		reqPos := &pb.GetPositionRequest{
			Name: testSlamServiceName,
		}
		respPos, err := slamServer.GetPosition(context.Background(), reqPos)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqual(poseSucc, spatial.NewPoseFromProtobuf(respPos.Pose)), test.ShouldBeTrue)
		test.That(t, respPos.ComponentReference, test.ShouldEqual, componentRefSucc)
	})

	t.Run("working GetPointCloudMap", func(t *testing.T) {
		injectSvc.PointCloudMapFunc = func(ctx context.Context) (func() ([]byte, error), error) {
			reader := bytes.NewReader(pcd)
			serverBuffer := make([]byte, chunkSizeServer)
			f := func() ([]byte, error) {
				n, err := reader.Read(serverBuffer)
				if err != nil {
					return nil, err
				}

				return serverBuffer[:n], err
			}

			return f, nil
		}

		reqPointCloudMap := &pb.GetPointCloudMapRequest{Name: testSlamServiceName}
		mockServer := makePointCloudServerMock()
		err = slamServer.GetPointCloudMap(reqPointCloudMap, mockServer)
		test.That(t, err, test.ShouldBeNil)

		// comparing raw bytes to ensure order is correct
		test.That(t, mockServer.rawBytes, test.ShouldResemble, pcd)
		// comparing pointclouds to ensure PCDs are correct
		testhelper.TestComparePointCloudsFromPCDs(t, mockServer.rawBytes, pcd)
	})

	t.Run("working GetInternalState", func(t *testing.T) {
		internalStateSucc := []byte{0, 1, 2, 3, 4}
		chunkSizeInternalState := 2
		injectSvc.InternalStateFunc = func(ctx context.Context) (func() ([]byte, error), error) {
			reader := bytes.NewReader(internalStateSucc)
			f := func() ([]byte, error) {
				serverBuffer := make([]byte, chunkSizeInternalState)
				n, err := reader.Read(serverBuffer)
				if err != nil {
					return nil, err
				}

				return serverBuffer[:n], err
			}
			return f, nil
		}

		req := &pb.GetInternalStateRequest{
			Name: testSlamServiceName,
		}
		mockServer := makeInternalStateServerMock()
		err := slamServer.GetInternalState(req, mockServer)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mockServer.rawBytes, test.ShouldResemble, internalStateSucc)
	})

	t.Run("working GetProperties", func(t *testing.T) {
		prop := slam.Properties{
			CloudSlam:   false,
			MappingMode: slam.MappingModeNewMap,
		}
		injectSvc.PropertiesFunc = func(ctx context.Context) (slam.Properties, error) {
			return prop, nil
		}

		reqInfo := &pb.GetPropertiesRequest{
			Name: testSlamServiceName,
		}

		respInfo, err := slamServer.GetProperties(context.Background(), reqInfo)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, respInfo.CloudSlam, test.ShouldResemble, prop.CloudSlam)
		test.That(t, respInfo.MappingMode, test.ShouldEqual, pb.MappingMode_MAPPING_MODE_CREATE_NEW_MAP)
	})

	t.Run("Multiple services Valid", func(t *testing.T) {
		resourceMap = map[resource.Name]slam.Service{
			slam.Named(testSlamServiceName):  injectSvc,
			slam.Named(testSlamServiceName2): injectSvc,
		}
		injectAPISvc, err := resource.NewAPIResourceCollection(slam.API, resourceMap)
		test.That(t, err, test.ShouldBeNil)
		slamServer = slam.NewRPCServiceServer(injectAPISvc).(pb.SLAMServiceServer)
		poseSucc := spatial.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatial.OrientationVector{Theta: math.Pi / 2, OX: 0, OY: 0, OZ: -1})
		componentRefSucc := "cam"

		injectSvc.PositionFunc = func(ctx context.Context) (spatial.Pose, string, error) {
			return poseSucc, componentRefSucc, nil
		}

		injectSvc.PointCloudMapFunc = func(ctx context.Context) (func() ([]byte, error), error) {
			reader := bytes.NewReader(pcd)
			serverBuffer := make([]byte, chunkSizeServer)
			f := func() ([]byte, error) {
				n, err := reader.Read(serverBuffer)
				if err != nil {
					return nil, err
				}

				return serverBuffer[:n], err
			}
			return f, nil
		}
		// test unary endpoint using GetPosition
		reqPos := &pb.GetPositionRequest{Name: testSlamServiceName}
		respPos, err := slamServer.GetPosition(context.Background(), reqPos)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqual(poseSucc, spatial.NewPoseFromProtobuf(respPos.Pose)), test.ShouldBeTrue)
		test.That(t, respPos.ComponentReference, test.ShouldEqual, componentRefSucc)

		reqPos = &pb.GetPositionRequest{Name: testSlamServiceName2}
		respPos, err = slamServer.GetPosition(context.Background(), reqPos)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqual(poseSucc, spatial.NewPoseFromProtobuf(respPos.Pose)), test.ShouldBeTrue)
		test.That(t, respPos.ComponentReference, test.ShouldEqual, componentRefSucc)

		// test streaming endpoint using GetPointCloudMap
		reqGetPointCloudMap := &pb.GetPointCloudMapRequest{Name: testSlamServiceName}
		mockServer1 := makePointCloudServerMock()
		err = slamServer.GetPointCloudMap(reqGetPointCloudMap, mockServer1)
		test.That(t, err, test.ShouldBeNil)
		// comparing raw bytes to ensure order is correct
		test.That(t, mockServer1.rawBytes, test.ShouldResemble, pcd)
		// comparing pointclouds to ensure PCDs are correct
		testhelper.TestComparePointCloudsFromPCDs(t, mockServer1.rawBytes, pcd)

		reqGetPointCloudMap = &pb.GetPointCloudMapRequest{Name: testSlamServiceName2}
		mockServer2 := makePointCloudServerMock()
		err = slamServer.GetPointCloudMap(reqGetPointCloudMap, mockServer2)
		test.That(t, err, test.ShouldBeNil)
		// comparing raw bytes to ensure order is correct
		test.That(t, mockServer2.rawBytes, test.ShouldResemble, pcd)
		// comparing pointclouds to ensure PCDs are correct
		testhelper.TestComparePointCloudsFromPCDs(t, mockServer2.rawBytes, pcd)
	})
}

func TestFailingServer(t *testing.T) {
	injectSvc := &inject.SLAMService{}
	resourceMap := map[resource.Name]slam.Service{
		slam.Named(testSlamServiceName): injectSvc,
	}
	injectAPISvc, err := resource.NewAPIResourceCollection(slam.API, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	slamServer := slam.NewRPCServiceServer(injectAPISvc).(pb.SLAMServiceServer)

	t.Run("failing GetPosition", func(t *testing.T) {
		injectSvc.PositionFunc = func(ctx context.Context) (spatial.Pose, string, error) {
			return nil, "", errors.New("failure to get position")
		}

		req := &pb.GetPositionRequest{
			Name: testSlamServiceName,
		}
		resp, err := slamServer.GetPosition(context.Background(), req)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure to get position")
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("failing GetPointCloudMap", func(t *testing.T) {
		// PointCloudMapFunc failure
		injectSvc.PointCloudMapFunc = func(ctx context.Context) (func() ([]byte, error), error) {
			return nil, errors.New("failure to get pointcloud map")
		}

		reqPointCloudMap := &pb.GetPointCloudMapRequest{Name: testSlamServiceName}

		mockServer := makePointCloudServerMock()
		err = slamServer.GetPointCloudMap(reqPointCloudMap, mockServer)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure to get pointcloud map")

		// Callback failure
		injectSvc.PointCloudMapFunc = func(ctx context.Context) (func() ([]byte, error), error) {
			f := func() ([]byte, error) {
				return []byte{}, errors.New("callback error")
			}
			return f, nil
		}

		mockServer = makePointCloudServerMock()
		err = slamServer.GetPointCloudMap(reqPointCloudMap, mockServer)
		test.That(t, err.Error(), test.ShouldContainSubstring, "callback error")
	})

	t.Run("failing GetInternalState", func(t *testing.T) {
		// InternalStateFunc error
		injectSvc.InternalStateFunc = func(ctx context.Context) (func() ([]byte, error), error) {
			return nil, errors.New("failure to get internal state")
		}

		req := &pb.GetInternalStateRequest{Name: testSlamServiceName}
		mockServer := makeInternalStateServerMock()
		err := slamServer.GetInternalState(req, mockServer)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure to get internal state")

		// Callback failure
		injectSvc.InternalStateFunc = func(ctx context.Context) (func() ([]byte, error), error) {
			f := func() ([]byte, error) {
				return []byte{}, errors.New("callback error")
			}
			return f, nil
		}

		err = slamServer.GetInternalState(req, mockServer)
		test.That(t, err.Error(), test.ShouldContainSubstring, "callback error")
	})

	t.Run("failing GetProperties", func(t *testing.T) {
		injectSvc.PropertiesFunc = func(ctx context.Context) (slam.Properties, error) {
			return slam.Properties{}, errors.New("failure to get properties")
		}
		reqInfo := &pb.GetPropertiesRequest{Name: testSlamServiceName}

		respInfo, err := slamServer.GetProperties(context.Background(), reqInfo)
		test.That(t, err, test.ShouldBeError, errors.New("failure to get properties"))
		test.That(t, respInfo, test.ShouldBeNil)
	})

	injectAPISvc, _ = resource.NewAPIResourceCollection(slam.API, map[resource.Name]slam.Service{})
	slamServer = slam.NewRPCServiceServer(injectAPISvc).(pb.SLAMServiceServer)
	t.Run("failing on nonexistent server", func(t *testing.T) {
		// test unary endpoint using GetPosition
		reqGetPositionRequest := &pb.GetPositionRequest{Name: testSlamServiceName}
		respPosNew, err := slamServer.GetPosition(context.Background(), reqGetPositionRequest)
		test.That(t, respPosNew, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(slam.Named(testSlamServiceName)))

		// test streaming endpoint using GetPointCloudMap
		mockPointCloudServer := makePointCloudServerMock()
		getPointCloudMapReq := &pb.GetPointCloudMapRequest{Name: testSlamServiceName}
		err = slamServer.GetPointCloudMap(getPointCloudMapReq, mockPointCloudServer)
		test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(slam.Named(testSlamServiceName)))
	})
}

func TestServerDoCommand(t *testing.T) {
	testSvcName1 := slam.Named("svc1")
	resourceMap := map[resource.Name]slam.Service{
		testSvcName1: &inject.SLAMService{
			DoCommandFunc: testutils.EchoFunc,
		},
	}
	injectAPISvc, err := resource.NewAPIResourceCollection(slam.API, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	server := slam.NewRPCServiceServer(injectAPISvc).(pb.SLAMServiceServer)

	cmd, err := protoutils.StructToStructPb(testutils.TestCommand)
	test.That(t, err, test.ShouldBeNil)
	doCommandRequest := &commonpb.DoCommandRequest{
		Name:    testSvcName1.ShortName(),
		Command: cmd,
	}
	doCommandResponse, err := server.DoCommand(context.Background(), doCommandRequest)
	test.That(t, err, test.ShouldBeNil)

	// Assert that do command response is an echoed request.
	respMap := doCommandResponse.Result.AsMap()
	test.That(t, respMap["command"], test.ShouldResemble, "test")
	test.That(t, respMap["data"], test.ShouldResemble, 500.0)
}
