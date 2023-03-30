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

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/internal/testhelper"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
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

// Create mock server that satisfies the pb.SLAMService_GetPointCloudMapStreamServer contract.
type pointCloudStreamServerMock struct {
	grpc.ServerStream
	rawBytes []byte
}
type internalStateStreamServerMock struct {
	grpc.ServerStream
	rawBytes []byte
}

func makePointCloudStreamServerMock() *pointCloudStreamServerMock {
	return &pointCloudStreamServerMock{}
}

// Concatenate received messages into single slice.
func (m *pointCloudStreamServerMock) Send(chunk *pb.GetPointCloudMapStreamResponse) error {
	m.rawBytes = append(m.rawBytes, chunk.PointCloudPcdChunk...)
	return nil
}

func makeInternalStateStreamServerMock() *internalStateStreamServerMock {
	return &internalStateStreamServerMock{}
}

// Concatenate received messages into single slice.
func (m *internalStateStreamServerMock) Send(chunk *pb.GetInternalStateStreamResponse) error {
	m.rawBytes = append(m.rawBytes, chunk.InternalStateChunk...)
	return nil
}

func TestWorkingServer(t *testing.T) {
	injectSvc := &inject.SLAMService{}
	resourceMap := map[resource.Name]interface{}{
		slam.Named(testSlamServiceName): injectSvc,
	}
	injectSubtypeSvc, err := subtype.New(resourceMap)
	test.That(t, err, test.ShouldBeNil)
	slamServer := slam.NewServer(injectSubtypeSvc)
	cloudPath := artifact.MustPath("slam/mock_lidar/0.pcd")
	pcd, err := os.ReadFile(cloudPath)
	test.That(t, err, test.ShouldBeNil)

	//nolint:dupl
	t.Run("working get position new function", func(t *testing.T) {
		poseSucc := spatial.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatial.OrientationVector{Theta: math.Pi / 2, OX: 0, OY: 0, OZ: -1})
		componentRefSucc := "cam"

		injectSvc.GetPositionFunc = func(ctx context.Context, name string) (spatial.Pose, string, error) {
			return poseSucc, componentRefSucc, nil
		}

		reqPos := &pb.GetPositionNewRequest{
			Name: testSlamServiceName,
		}
		respPos, err := slamServer.GetPositionNew(context.Background(), reqPos)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqual(poseSucc, spatial.NewPoseFromProtobuf(respPos.Pose)), test.ShouldBeTrue)
		test.That(t, respPos.ComponentReference, test.ShouldEqual, componentRefSucc)
	})

	//nolint:dupl
	t.Run("working GetPosition", func(t *testing.T) {
		poseSucc := spatial.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatial.OrientationVector{Theta: math.Pi / 2, OX: 0, OY: 0, OZ: -1})
		componentRefSucc := "cam"

		injectSvc.GetPositionFunc = func(ctx context.Context, name string) (spatial.Pose, string, error) {
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

	t.Run("working get pointcloud map stream function", func(t *testing.T) {
		injectSvc.GetPointCloudMapFunc = func(ctx context.Context, name string) (func() ([]byte, error), error) {
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

		reqPointCloudMapStream := &pb.GetPointCloudMapStreamRequest{Name: testSlamServiceName}
		mockServer := makePointCloudStreamServerMock()
		err = slamServer.GetPointCloudMapStream(reqPointCloudMapStream, mockServer)
		test.That(t, err, test.ShouldBeNil)

		// comparing raw bytes to ensure order is correct
		test.That(t, mockServer.rawBytes, test.ShouldResemble, pcd)
		// comparing pointclouds to ensure PCDs are correct
		testhelper.TestComparePointCloudsFromPCDs(t, mockServer.rawBytes, pcd)
	})

	t.Run("working GetPointCloudMap", func(t *testing.T) {
		injectSvc.GetPointCloudMapFunc = func(ctx context.Context, name string) (func() ([]byte, error), error) {
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

		reqPointCloudMapStream := &pb.GetPointCloudMapRequest{Name: testSlamServiceName}
		mockServer := makePointCloudServerMock()
		err = slamServer.GetPointCloudMap(reqPointCloudMapStream, mockServer)
		test.That(t, err, test.ShouldBeNil)

		// comparing raw bytes to ensure order is correct
		test.That(t, mockServer.rawBytes, test.ShouldResemble, pcd)
		// comparing pointclouds to ensure PCDs are correct
		testhelper.TestComparePointCloudsFromPCDs(t, mockServer.rawBytes, pcd)
	})

	//nolint:dupl
	t.Run("working get internal state functions", func(t *testing.T) {
		internalStateSucc := []byte{0, 1, 2, 3, 4}
		chunkSizeInternalState := 2
		injectSvc.GetInternalStateFunc = func(ctx context.Context, name string) (func() ([]byte, error), error) {
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

		req := &pb.GetInternalStateStreamRequest{
			Name: testSlamServiceName,
		}
		mockServer := makeInternalStateStreamServerMock()
		err := slamServer.GetInternalStateStream(req, mockServer)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mockServer.rawBytes, test.ShouldResemble, internalStateSucc)
	})

	//nolint:dupl
	t.Run("working GetInternalState", func(t *testing.T) {
		internalStateSucc := []byte{0, 1, 2, 3, 4}
		chunkSizeInternalState := 2
		injectSvc.GetInternalStateFunc = func(ctx context.Context, name string) (func() ([]byte, error), error) {
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

	t.Run("Multiple services Valid", func(t *testing.T) {
		resourceMap = map[resource.Name]interface{}{
			slam.Named(testSlamServiceName):  injectSvc,
			slam.Named(testSlamServiceName2): injectSvc,
		}
		injectSubtypeSvc, err := subtype.New(resourceMap)
		test.That(t, err, test.ShouldBeNil)
		slamServer = slam.NewServer(injectSubtypeSvc)
		poseSucc := spatial.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatial.OrientationVector{Theta: math.Pi / 2, OX: 0, OY: 0, OZ: -1})
		componentRefSucc := "cam"

		injectSvc.GetPositionFunc = func(ctx context.Context, name string) (spatial.Pose, string, error) {
			return poseSucc, componentRefSucc, nil
		}

		injectSvc.GetPointCloudMapFunc = func(ctx context.Context, name string) (func() ([]byte, error), error) {
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

		// test unary endpoint using GetPositionNew
		reqPosNew := &pb.GetPositionNewRequest{Name: testSlamServiceName}
		respPosNew, err := slamServer.GetPositionNew(context.Background(), reqPosNew)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqual(poseSucc, spatial.NewPoseFromProtobuf(respPosNew.Pose)), test.ShouldBeTrue)
		test.That(t, respPosNew.ComponentReference, test.ShouldEqual, componentRefSucc)

		reqPosNew = &pb.GetPositionNewRequest{Name: testSlamServiceName2}
		respPosNew, err = slamServer.GetPositionNew(context.Background(), reqPosNew)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqual(poseSucc, spatial.NewPoseFromProtobuf(respPosNew.Pose)), test.ShouldBeTrue)
		test.That(t, respPosNew.ComponentReference, test.ShouldEqual, componentRefSucc)

		// test streaming endpoint using GetPointCloudMapStream
		reqGetPointCloudMapStream := &pb.GetPointCloudMapStreamRequest{Name: testSlamServiceName}
		mockServer3 := makePointCloudStreamServerMock()
		err = slamServer.GetPointCloudMapStream(reqGetPointCloudMapStream, mockServer3)
		test.That(t, err, test.ShouldBeNil)
		// comparing raw bytes to ensure order is correct
		test.That(t, mockServer3.rawBytes, test.ShouldResemble, pcd)
		// comparing pointclouds to ensure PCDs are correct
		testhelper.TestComparePointCloudsFromPCDs(t, mockServer3.rawBytes, pcd)

		reqGetPointCloudMapStream = &pb.GetPointCloudMapStreamRequest{Name: testSlamServiceName2}
		mockServer4 := makePointCloudStreamServerMock()
		err = slamServer.GetPointCloudMapStream(reqGetPointCloudMapStream, mockServer4)
		test.That(t, err, test.ShouldBeNil)
		// comparing raw bytes to ensure order is correct
		test.That(t, mockServer4.rawBytes, test.ShouldResemble, pcd)
		// comparing pointclouds to ensure PCDs are correct
		testhelper.TestComparePointCloudsFromPCDs(t, mockServer4.rawBytes, pcd)
	})
}

func TestFailingServer(t *testing.T) {
	injectSvc := &inject.SLAMService{}
	resourceMap := map[resource.Name]interface{}{
		slam.Named(testSlamServiceName): injectSvc,
	}
	injectSubtypeSvc, err := subtype.New(resourceMap)
	test.That(t, err, test.ShouldBeNil)
	slamServer := slam.NewServer(injectSubtypeSvc)

	t.Run("failing get position function", func(t *testing.T) {
		injectSvc.GetPositionFunc = func(ctx context.Context, name string) (spatial.Pose, string, error) {
			return nil, "", errors.New("failure to get position")
		}

		req := &pb.GetPositionNewRequest{
			Name: testSlamServiceName,
		}
		resp, err := slamServer.GetPositionNew(context.Background(), req)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure to get position")
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("failing GetPosition", func(t *testing.T) {
		injectSvc.GetPositionFunc = func(ctx context.Context, name string) (spatial.Pose, string, error) {
			return nil, "", errors.New("failure to get position")
		}

		req := &pb.GetPositionRequest{
			Name: testSlamServiceName,
		}
		resp, err := slamServer.GetPosition(context.Background(), req)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure to get position")
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("failing GetPointCloudMapStream", func(t *testing.T) {
		// GetPointCloudMapStreamFunc failure
		injectSvc.GetPointCloudMapFunc = func(ctx context.Context, name string) (func() ([]byte, error), error) {
			return nil, errors.New("failure to get pointcloud map")
		}

		reqPointCloudMap := &pb.GetPointCloudMapRequest{Name: testSlamServiceName}

		mockServer := makePointCloudServerMock()
		err = slamServer.GetPointCloudMap(reqPointCloudMap, mockServer)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure to get pointcloud map")

		// Callback failure
		injectSvc.GetPointCloudMapFunc = func(ctx context.Context, name string) (func() ([]byte, error), error) {
			f := func() ([]byte, error) {
				return []byte{}, errors.New("callback error")
			}
			return f, nil
		}

		mockServer = makePointCloudServerMock()
		err = slamServer.GetPointCloudMap(reqPointCloudMap, mockServer)
		test.That(t, err.Error(), test.ShouldContainSubstring, "callback error")
	})

	//nolint:dupl
	t.Run("failing get internal state stream function", func(t *testing.T) {
		// GetInternalStateStreamFunc error
		injectSvc.GetInternalStateFunc = func(ctx context.Context, name string) (func() ([]byte, error), error) {
			return nil, errors.New("failure to get internal state")
		}

		req := &pb.GetInternalStateStreamRequest{Name: testSlamServiceName}
		mockServer := makeInternalStateStreamServerMock()
		err := slamServer.GetInternalStateStream(req, mockServer)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure to get internal state")

		// Callback failure
		injectSvc.GetInternalStateFunc = func(ctx context.Context, name string) (func() ([]byte, error), error) {
			f := func() ([]byte, error) {
				return []byte{}, errors.New("callback error")
			}
			return f, nil
		}

		err = slamServer.GetInternalStateStream(req, mockServer)
		test.That(t, err.Error(), test.ShouldContainSubstring, "callback error")
	})

	//nolint:dupl
	t.Run("failing GetInternalState", func(t *testing.T) {
		// GetInternalStateStreamFunc error
		injectSvc.GetInternalStateFunc = func(ctx context.Context, name string) (func() ([]byte, error), error) {
			return nil, errors.New("failure to get internal state")
		}

		req := &pb.GetInternalStateRequest{Name: testSlamServiceName}
		mockServer := makeInternalStateServerMock()
		err := slamServer.GetInternalState(req, mockServer)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure to get internal state")

		// Callback failure
		injectSvc.GetInternalStateFunc = func(ctx context.Context, name string) (func() ([]byte, error), error) {
			f := func() ([]byte, error) {
				return []byte{}, errors.New("callback error")
			}
			return f, nil
		}

		err = slamServer.GetInternalState(req, mockServer)
		test.That(t, err.Error(), test.ShouldContainSubstring, "callback error")
	})

	resourceMap = map[resource.Name]interface{}{
		slam.Named(testSlamServiceName): "not a frame system",
	}
	injectSubtypeSvc, _ = subtype.New(resourceMap)
	slamServer = slam.NewServer(injectSubtypeSvc)

	t.Run("failing on improper service interface", func(t *testing.T) {
		improperImplErr := slam.NewUnimplementedInterfaceError("string")

		// Get position
		getPositionReq := &pb.GetPositionRequest{Name: testSlamServiceName}
		getPositionResp, err := slamServer.GetPosition(context.Background(), getPositionReq)
		test.That(t, getPositionResp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, improperImplErr)

		// Get pointcloud map
		mockPointCloudServer := makePointCloudServerMock()
		getPointCloudMapReq := &pb.GetPointCloudMapRequest{Name: testSlamServiceName}
		err = slamServer.GetPointCloudMap(getPointCloudMapReq, mockPointCloudServer)
		test.That(t, err, test.ShouldBeError, improperImplErr)

		// Get internal state
		getInternalStateReq := &pb.GetInternalStateRequest{Name: testSlamServiceName}
		mockInternalStateServer := makeInternalStateServerMock()
		err = slamServer.GetInternalState(getInternalStateReq, mockInternalStateServer)
		test.That(t, err, test.ShouldBeError, improperImplErr)

		// Get position
		getPositionNewReq := &pb.GetPositionNewRequest{Name: testSlamServiceName}
		getPositionNewResp, err := slamServer.GetPositionNew(context.Background(), getPositionNewReq)
		test.That(t, getPositionNewResp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, improperImplErr)

		// Get pointcloud map stream
		mockPointCloudStreamServer := makePointCloudStreamServerMock()
		getPointCloudMapStreamReq := &pb.GetPointCloudMapStreamRequest{Name: testSlamServiceName}
		err = slamServer.GetPointCloudMapStream(getPointCloudMapStreamReq, mockPointCloudStreamServer)
		test.That(t, err, test.ShouldBeError, improperImplErr)

		// Get internal state stream
		getInternalStateStreamReq := &pb.GetInternalStateStreamRequest{Name: testSlamServiceName}
		mockInternalStateStreamServer := makeInternalStateStreamServerMock()
		err = slamServer.GetInternalStateStream(getInternalStateStreamReq, mockInternalStateStreamServer)
		test.That(t, err, test.ShouldBeError, improperImplErr)
	})

	injectSubtypeSvc, _ = subtype.New(map[resource.Name]interface{}{})
	slamServer = slam.NewServer(injectSubtypeSvc)
	t.Run("failing on nonexistent server", func(t *testing.T) {
		// test unary endpoint using GetPositionNew
		reqGetPositionNewRequest := &pb.GetPositionNewRequest{Name: testSlamServiceName}
		respPosNew, err := slamServer.GetPositionNew(context.Background(), reqGetPositionNewRequest)
		test.That(t, respPosNew, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(slam.Named(testSlamServiceName)))

		// test streaming endpoint using GetPointCloudMapStream
		mockStreamServer := makePointCloudStreamServerMock()
		reqGetPointCloudMapStreamRequest := &pb.GetPointCloudMapStreamRequest{Name: testSlamServiceName}
		err = slamServer.GetPointCloudMapStream(reqGetPointCloudMapStreamRequest, mockStreamServer)
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(slam.Named(testSlamServiceName)))
	})
}

func TestServerDoCommand(t *testing.T) {
	resourceMap := map[resource.Name]interface{}{
		slam.Named(testSvcName1): &inject.SLAMService{
			DoCommandFunc: generic.EchoFunc,
		},
	}
	injectSubtypeSvc, err := subtype.New(resourceMap)
	test.That(t, err, test.ShouldBeNil)
	server := slam.NewServer(injectSubtypeSvc)

	cmd, err := protoutils.StructToStructPb(generic.TestCommand)
	test.That(t, err, test.ShouldBeNil)
	doCommandRequest := &commonpb.DoCommandRequest{
		Name:    testSvcName1,
		Command: cmd,
	}
	doCommandResponse, err := server.DoCommand(context.Background(), doCommandRequest)
	test.That(t, err, test.ShouldBeNil)

	// Assert that do command response is an echoed request.
	respMap := doCommandResponse.Result.AsMap()
	test.That(t, respMap["command"], test.ShouldResemble, "test")
	test.That(t, respMap["data"], test.ShouldResemble, 500.0)
}
