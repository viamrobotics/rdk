// Package builtin implements simultaneous localization and mapping.
// This is an Experimental package.
package builtin

import (
	"bytes"
	"context"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	commonv1 "go.viam.com/api/common/v1"
	v1 "go.viam.com/api/service/slam/v1"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	slamConfig "go.viam.com/rdk/services/slam/slam_copy/config"
	slamTesthelper "go.viam.com/rdk/services/slam/slam_copy/testhelper"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
	"go.viam.com/utils"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	testExecutableName = "true" // the program "true", not the boolean value
	testDataRateMsec   = 200
)

var (
	testMapRateSec  = 200
	chunkSizeClient = 1
	_true           = true
	_false          = false
)

type pointCloudClientMock struct {
	grpc.ClientStream
	reader *bytes.Reader
}

func makePointCloudClientMock(b []byte) *pointCloudClientMock {
	return &pointCloudClientMock{reader: bytes.NewReader(b)}
}

// Concatenate received messages into single slice.
func (m *pointCloudClientMock) Recv() (*v1.GetPointCloudMapResponse, error) {

	clientBuffer := make([]byte, chunkSizeClient)

	n, err := m.reader.Read(clientBuffer)
	if err != nil {
		return nil, err
	}

	resp := &v1.GetPointCloudMapResponse{
		PointCloudPcdChunk: clientBuffer[:n],
	}

	return resp, nil
}

type internalStateClientMock struct {
	grpc.ClientStream
	reader *bytes.Reader
}

func makeInternalStateClientMock(b []byte) *internalStateClientMock {
	return &internalStateClientMock{reader: bytes.NewReader(b)}
}

// Concatenate received messages into single slice.
func (m *internalStateClientMock) Recv() (*v1.GetInternalStateResponse, error) {

	clientBuffer := make([]byte, chunkSizeClient)

	n, err := m.reader.Read(clientBuffer)
	if err != nil {
		return nil, err
	}

	resp := &v1.GetInternalStateResponse{
		InternalStateChunk: clientBuffer[:n],
	}

	return resp, nil
}

func TestEndpointsSuccess(t *testing.T) {
	logger := golog.NewTestLogger(t)
	dataDir, err := slamTesthelper.CreateTempFolderArchitecture(logger)
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()

	grpcServer, port := setupTestGRPCServer(t)
	test.That(t, err, test.ShouldBeNil)
	attrCfg := &slamConfig.AttrConfig{
		Sensors:       []string{},
		ConfigParams:  map[string]string{"mode": "2d"},
		DataDirectory: dataDir,
		Port:          "localhost:" + strconv.Itoa(port),
		UseLiveData:   &_false,
	}

	svc, err := createSLAMService(t, attrCfg, "fake_cartographer", logger, false, true)
	test.That(t, err, test.ShouldBeNil)

	grpcServer.Stop()
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	mockSLAMClient := &inject.SLAMServiceClient{}

	t.Run("Successful GetPosition", func(t *testing.T) {
		reference_comp_success := "reference_comp"
		pose_succ := spatialmath.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, spatialmath.NewZeroOrientation())
		mockSLAMClient.GetPositionFunc = func(ctx context.Context, in *v1.GetPositionRequest, opts ...grpc.CallOption) (*v1.GetPositionResponse, error) {
			extra, err := structpb.NewStruct(map[string]interface{}{
				"quat": map[string]interface{}{
					"real": pose_succ.Orientation().Quaternion().Real,
					"imag": pose_succ.Orientation().Quaternion().Imag,
					"jmag": pose_succ.Orientation().Quaternion().Jmag,
					"kmag": pose_succ.Orientation().Quaternion().Kmag,
				},
			})
			if err != nil {
				return nil, err
			}

			resp := &v1.GetPositionResponse{
				Pose: &commonv1.Pose{
					X:     pose_succ.Point().X,
					Y:     pose_succ.Point().Y,
					Z:     pose_succ.Point().Z,
					OX:    pose_succ.Orientation().OrientationVectorDegrees().OX,
					OY:    pose_succ.Orientation().OrientationVectorDegrees().OY,
					OZ:    pose_succ.Orientation().OrientationVectorDegrees().OZ,
					Theta: pose_succ.Orientation().OrientationVectorDegrees().Theta,
				},
				ComponentReference: reference_comp_success,
				Extra:              extra,
			}
			return resp, nil
		}

		a := svc.(*builtIn)
		a.clientAlgo = mockSLAMClient

		pose, reference_comp, err := svc.GetPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, reference_comp, test.ShouldEqual, reference_comp_success)
		test.That(t, pose, test.ShouldResemble, pose_succ)
	})

	t.Run("Successful GetPointCloudMap", func(t *testing.T) {
		respBytes := []byte{1, 2, 3, 4, 5, 6}
		mockSLAMClient.GetPointCloudMapFunc = func(ctx context.Context, in *v1.GetPointCloudMapRequest, opts ...grpc.CallOption) (v1.SLAMService_GetPointCloudMapClient, error) {

			cm := makePointCloudClientMock(respBytes)
			return cm, nil
		}

		a := svc.(*builtIn)
		a.clientAlgo = mockSLAMClient

		_, err := svc.GetPointCloudMap(context.Background())
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("Successful GetInternalState", func(t *testing.T) {
		respBytes := []byte{1, 2, 3, 4, 5, 6}
		mockSLAMClient.GetInternalStateFunc = func(ctx context.Context, in *v1.GetInternalStateRequest, opts ...grpc.CallOption) (v1.SLAMService_GetInternalStateClient, error) {

			cm := makeInternalStateClientMock(respBytes)
			return cm, nil
		}

		a := svc.(*builtIn)
		a.clientAlgo = mockSLAMClient

		_, err := svc.GetInternalState(context.Background())
		test.That(t, err, test.ShouldBeNil)
	})

	grpcServer.Stop()
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	closeOutSLAMService(t, dataDir)
}

// ------------------------------------------------------ REMOVE THESE ------------------------------------------------------

// SetupTestGRPCServer sets up and starts a grpc server.
// It returns the grpc server and the port at which it is served.
func setupTestGRPCServer(tb testing.TB) (*grpc.Server, int) {
	listener, err := net.Listen("tcp", ":0")
	test.That(tb, err, test.ShouldBeNil)
	grpcServer := grpc.NewServer()
	go grpcServer.Serve(listener)

	return grpcServer, listener.Addr().(*net.TCPAddr).Port
}

func createSLAMService(
	t *testing.T,
	attrCfg *slamConfig.AttrConfig,
	model string,
	logger golog.Logger,
	bufferSLAMProcessLogs bool,
	success bool,
) (slam.Service, error) {
	//	t.Helper()

	ctx := context.Background()
	cfgService := config.Service{Name: "test", Type: "slam", Model: resource.NewDefaultModel(resource.ModelName(model))}
	cfgService.ConvertedAttributes = attrCfg

	sensorDeps, err := attrCfg.Validate("path")
	if err != nil {
		return nil, err
	}
	test.That(t, sensorDeps, test.ShouldResemble, attrCfg.Sensors)

	SetCameraValidationMaxTimeoutSecForTesting(1)
	SetDialMaxTimeoutSecForTesting(1)

	var deps map[resource.Name]interface{} // HELLO

	svc, err := NewBuiltIn(ctx, deps, cfgService, logger, bufferSLAMProcessLogs)

	if success {
		if err != nil {
			return nil, err
		}
		test.That(t, svc, test.ShouldNotBeNil)
		return svc, nil
	}

	test.That(t, svc, test.ShouldBeNil)
	return nil, err
}

func createFakeSLAMLibraries() {
	for _, s := range slam.SLAMLibraries {
		slam.SLAMLibraries["fake_"+s.AlgoName] = slam.LibraryMetadata{
			AlgoName:       "fake_" + s.AlgoName,
			AlgoType:       s.AlgoType,
			SlamMode:       s.SlamMode,
			BinaryLocation: "true",
		}
	}
}

func deleteFakeSLAMLibraries() {
	for k := range slam.SLAMLibraries {
		if strings.Contains(k, "fake") {
			delete(slam.SLAMLibraries, k)
		}
	}
}

func closeOutSLAMService(t *testing.T, name string) {
	t.Helper()

	if name != "" {
		err := slamTesthelper.ResetFolder(name)
		test.That(t, err, test.ShouldBeNil)
	}

	deleteFakeSLAMLibraries()
}
