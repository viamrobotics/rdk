// Package slam_test server_test.go tests the SLAM service's GRPC server.
package slam_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"io"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/slam/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

const (
	testSlamServiceName  = "slam1"
	testSlamServiceName2 = "slam2"
)

// Create mock server that satisfies the pb.SLAMService_GetPointCloudMapStreamServer contract.
type PointCloudStreamServerMock struct {
	grpc.ServerStream
	rawBytes []byte
}
type InternalStateStreamServerMock struct {
	grpc.ServerStream
	rawBytes []byte
}

func makePointCloudStreamServerMock() *PointCloudStreamServerMock {
	return &PointCloudStreamServerMock{}
}

// Concatenate received messages into single slice.
func (m *PointCloudStreamServerMock) Send(chunk *pb.GetPointCloudMapStreamResponse) error {
	m.rawBytes = append(m.rawBytes, chunk.PointCloudPcdChunk...)
	return nil
}

func makeInternalStateStreamServerMock() *InternalStateStreamServerMock {
	return &InternalStateStreamServerMock{}
}

// Concatenate received messages into single slice.
func (m *InternalStateStreamServerMock) Send(chunk *pb.GetInternalStateStreamResponse) error {
	m.rawBytes = append(m.rawBytes, chunk.InternalStateChunk...)
	return nil
}

func TestServer(t *testing.T) {
	injectSvc := &inject.SLAMService{}
	resourceMap := map[resource.Name]interface{}{
		slam.Named(testSlamServiceName): injectSvc,
	}
	injectSubtypeSvc, err := subtype.New(resourceMap)
	test.That(t, err, test.ShouldBeNil)
	slamServer := slam.NewServer(injectSubtypeSvc)

	t.Run("working get position functions", func(t *testing.T) {
		pose := spatial.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatial.OrientationVector{Theta: math.Pi / 2, OX: 0, OY: 0, OZ: -1})
		pSucc := referenceframe.NewPoseInFrame("frame", pose)

		var extraOptions map[string]interface{}
		injectSvc.PositionFunc = func(ctx context.Context, name string, extra map[string]interface{}) (*referenceframe.PoseInFrame, error) {
			extraOptions = extra
			return pSucc, nil
		}
		extra := map[string]interface{}{"foo": "Position"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)

		reqPos := &pb.GetPositionRequest{
			Name:  testSlamServiceName,
			Extra: ext,
		}
		respPos, err := slamServer.GetPosition(context.Background(), reqPos)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, referenceframe.ProtobufToPoseInFrame(respPos.Pose).Parent(), test.ShouldEqual, pSucc.Parent())
		test.That(t, extraOptions, test.ShouldResemble, extra)
	})

	t.Run("working get map function", func(t *testing.T) {
		pose := spatial.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatial.OrientationVector{Theta: math.Pi / 2, OX: 0, OY: 0, OZ: -1})
		pSucc := referenceframe.NewPoseInFrame("frame", pose)
		pcSucc := &vision.Object{}
		pcSucc.PointCloud = pointcloud.New()
		err = pcSucc.PointCloud.Set(pointcloud.NewVector(5, 5, 5), nil)
		test.That(t, err, test.ShouldBeNil)
		imSucc := image.NewNRGBA(image.Rect(0, 0, 4, 4))

		var extraOptions map[string]interface{}
		injectSvc.GetMapFunc = func(ctx context.Context, name, mimeType string, cp *referenceframe.PoseInFrame,
			include bool, extra map[string]interface{},
		) (string, image.Image, *vision.Object, error) {
			extraOptions = extra
			return mimeType, imSucc, pcSucc, nil
		}
		extra := map[string]interface{}{"foo": "GetMap"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)

		reqMap := &pb.GetMapRequest{
			Name:               testSlamServiceName,
			MimeType:           utils.MimeTypePCD,
			CameraPosition:     referenceframe.PoseInFrameToProtobuf(pSucc).Pose,
			IncludeRobotMarker: true,
			Extra:              ext,
		}
		respMap, err := slamServer.GetMap(context.Background(), reqMap)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, respMap.MimeType, test.ShouldEqual, utils.MimeTypePCD)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		reqMap = &pb.GetMapRequest{
			Name:               testSlamServiceName,
			MimeType:           utils.MimeTypeJPEG,
			CameraPosition:     referenceframe.PoseInFrameToProtobuf(pSucc).Pose,
			IncludeRobotMarker: true,
		}
		respMap, err = slamServer.GetMap(context.Background(), reqMap)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, respMap.MimeType, test.ShouldEqual, utils.MimeTypeJPEG)
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{})
	})

	t.Run("working get internal state functions", func(t *testing.T) {
		internalStateSucc := []byte{1, 2, 3, 4}
		injectSvc.GetInternalStateFunc = func(ctx context.Context, name string) ([]byte, error) {
			return internalStateSucc, nil
		}

		req := &pb.GetInternalStateRequest{
			Name: testSlamServiceName,
		}
		respInternalState, err := slamServer.GetInternalState(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, respInternalState.GetInternalState(), test.ShouldResemble, internalStateSucc)
	})

	t.Run("working get internal state stream functions", func(t *testing.T) {
		internalStateSucc := []byte{0, 1, 2, 3, 4}
		chunkSizeInternalState := 2
		injectSvc.GetInternalStateStreamFunc = func(ctx context.Context, name string) (func() ([]byte, error), error) {
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

	t.Run("failing get position function", func(t *testing.T) {
		injectSvc.PositionFunc = func(ctx context.Context, name string, extra map[string]interface{}) (*referenceframe.PoseInFrame, error) {
			return nil, errors.New("failure to get position")
		}

		req := &pb.GetPositionRequest{
			Name: testSlamServiceName,
		}
		resp, err := slamServer.GetPosition(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("failing get map function", func(t *testing.T) {
		pose := spatial.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatial.OrientationVector{Theta: math.Pi / 2, OX: 0, OY: 0, OZ: -1})

		injectSvc.GetMapFunc = func(ctx context.Context, name, mimeType string, cp *referenceframe.PoseInFrame,
			include bool, extra map[string]interface{},
		) (string, image.Image, *vision.Object, error) {
			return mimeType, nil, nil, errors.New("failure to get map")
		}

		req := &pb.GetMapRequest{MimeType: utils.MimeTypeJPEG, CameraPosition: spatial.PoseToProtobuf(pose)}
		resp, err := slamServer.GetMap(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("failing get internal state function", func(t *testing.T) {
		injectSvc.GetInternalStateFunc = func(ctx context.Context, name string) ([]byte, error) {
			return nil, errors.New("failure to get internal state")
		}

		req := &pb.GetInternalStateRequest{
			Name: testSlamServiceName,
		}
		resp, err := slamServer.GetInternalState(context.Background(), req)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure to get internal state")
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("failing get internal state stream function", func(t *testing.T) {
		// GetInternalStateStreamFunc error
		injectSvc.GetInternalStateStreamFunc = func(ctx context.Context, name string) (func() ([]byte, error), error) {
			f := func() ([]byte, error) {
				return []byte{}, io.EOF
			}
			return f, errors.New("failure to get internal state")
		}

		req := &pb.GetInternalStateStreamRequest{
			Name: testSlamServiceName,
		}
		mockServer := makeInternalStateStreamServerMock()
		err := slamServer.GetInternalStateStream(req, mockServer)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failure to get internal state")

		// Callback failure
		injectSvc.GetInternalStateStreamFunc = func(ctx context.Context, name string) (func() ([]byte, error), error) {
			f := func() ([]byte, error) {
				return []byte{}, errors.New("callback error")
			}
			return f, nil
		}

		err = slamServer.GetInternalStateStream(req, mockServer)
		test.That(t, err.Error(), test.ShouldContainSubstring, "callback error")

	})

	resourceMap = map[resource.Name]interface{}{
		slam.Named(testSlamServiceName): "not a frame system",
	}
	injectSubtypeSvc, _ = subtype.New(resourceMap)
	slamServer = slam.NewServer(injectSubtypeSvc)

	t.Run("failing on improper service interface", func(t *testing.T) {
		improperImplErr := slam.NewUnimplementedInterfaceError("string")

		getPositionReq := &pb.GetPositionRequest{Name: testSlamServiceName}
		getPositionResp, err := slamServer.GetPosition(context.Background(), getPositionReq)
		test.That(t, getPositionResp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, improperImplErr)

		getMapReq := &pb.GetMapRequest{Name: testSlamServiceName}
		getMapResp, err := slamServer.GetMap(context.Background(), getMapReq)
		test.That(t, getMapResp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, improperImplErr)

		getInternalStateReq := &pb.GetInternalStateRequest{Name: testSlamServiceName}
		getInternalStateResp, err := slamServer.GetInternalState(context.Background(), getInternalStateReq)
		test.That(t, getInternalStateResp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, improperImplErr)

		getInternalStateStreamReq := &pb.GetInternalStateStreamRequest{Name: testSlamServiceName}
		mockServer := makeInternalStateStreamServerMock()
		err = slamServer.GetInternalStateStream(getInternalStateStreamReq, mockServer)
		test.That(t, err, test.ShouldBeError, improperImplErr)
	})

	injectSubtypeSvc, _ = subtype.New(map[resource.Name]interface{}{})
	slamServer = slam.NewServer(injectSubtypeSvc)
	t.Run("failing on nonexistent server", func(t *testing.T) {
		req := &pb.GetPositionRequest{
			Name: testSlamServiceName,
		}
		resp, err := slamServer.GetPosition(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(slam.Named(testSlamServiceName)))
	})
	t.Run("Multiple services Valid", func(t *testing.T) {
		resourceMap = map[resource.Name]interface{}{
			slam.Named(testSlamServiceName):  injectSvc,
			slam.Named(testSlamServiceName2): injectSvc,
		}
		injectSubtypeSvc, err := subtype.New(resourceMap)
		test.That(t, err, test.ShouldBeNil)
		slamServer = slam.NewServer(injectSubtypeSvc)
		pose := spatial.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatial.OrientationVector{Theta: math.Pi / 2, OX: 0, OY: 0, OZ: -1})
		pSucc := referenceframe.NewPoseInFrame("frame", pose)
		injectSvc.PositionFunc = func(ctx context.Context, name string, extra map[string]interface{}) (*referenceframe.PoseInFrame, error) {
			return pSucc, nil
		}

		reqPos := &pb.GetPositionRequest{
			Name: testSlamServiceName,
		}
		respPos, err := slamServer.GetPosition(context.Background(), reqPos)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, referenceframe.ProtobufToPoseInFrame(respPos.Pose).Parent(), test.ShouldEqual, pSucc.Parent())
		reqPos = &pb.GetPositionRequest{
			Name: testSlamServiceName2,
		}
		respPos, err = slamServer.GetPosition(context.Background(), reqPos)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, referenceframe.ProtobufToPoseInFrame(respPos.Pose).Parent(), test.ShouldEqual, pSucc.Parent())
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
