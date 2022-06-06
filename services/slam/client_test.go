// Package slam_test client_test.go tests the client for the SLAM service's GRPC server.
package slam_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	viamgrpc "go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	servicepb "go.viam.com/rdk/proto/api/service/slam/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	workingServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	failingServer := grpc.NewServer()

	pSucc := &commonpb.PoseInFrame{}
	pcSucc := &commonpb.PointCloudObject{}
	imSucc := []byte{}

	workingSLAMService := &inject.SLAMService{}

	workingSLAMService.CloseFunc = func() error {
		return nil
	}

	workingSLAMService.GetPositionFunc = func(ctx context.Context, name string) (*commonpb.PoseInFrame, error) {
		return pSucc, nil
	}

	workingSLAMService.GetMapFunc = func(ctx context.Context, name string, mimeType string, cp *commonpb.Pose,
		include bool) (string, []byte, *commonpb.PointCloudObject, error) {
		if mimeType == utils.MimeTypePCD {
			return mimeType, nil, pcSucc, nil
		}
		return mimeType, imSucc, nil, nil
	}

	workingSvc, err := subtype.New(map[resource.Name]interface{}{slam.Name: workingSLAMService})
	test.That(t, err, test.ShouldBeNil)

	pFail := &commonpb.PoseInFrame{}
	pcFail := &commonpb.PointCloudObject{}
	imFail := []byte{}

	failingSLAMService := &inject.SLAMService{}

	failingSLAMService.CloseFunc = func() error {
		return errors.New("failure to close")
	}

	failingSLAMService.GetPositionFunc = func(ctx context.Context, name string) (*commonpb.PoseInFrame, error) {
		return pFail, errors.New("failure to get position")
	}

	failingSLAMService.GetMapFunc = func(ctx context.Context, name string, mimeType string, cp *commonpb.Pose,
		include bool) (string, []byte, *commonpb.PointCloudObject, error) {
		return mimeType, imFail, pcFail, errors.New("failure to get map")
	}

	failingSvc, err := subtype.New(map[resource.Name]interface{}{slam.Name: failingSLAMService})
	test.That(t, err, test.ShouldBeNil)

	resourceSubtype := registry.ResourceSubtypeLookup(slam.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), workingServer, workingSvc)
	servicepb.RegisterSLAMServiceServer(failingServer, slam.NewServer(failingSvc))

	go workingServer.Serve(listener1)
	defer workingServer.Stop()

	t.Run("context canceled", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = slam.NewClient(cancelCtx, slam.Name.String(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	workingSLAMClient, err := slam.NewClient(
		context.Background(), slam.Name.String(),
		listener1.Addr().String(), logger,
	)
	test.That(t, err, test.ShouldBeNil)

	nameSucc := "viam"

	t.Run("client tests for working slam service", func(t *testing.T) {
		// test get position
		pInFrame, err := workingSLAMClient.GetPosition(context.Background(), nameSucc)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pInFrame, test.ShouldResemble, pSucc)

		// test get map
		mimeType, im, pc, err := workingSLAMClient.GetMap(context.Background(), nameSucc, utils.MimeTypePCD, pSucc.Pose, true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mimeType, test.ShouldEqual, utils.MimeTypePCD)
		test.That(t, im, test.ShouldResemble, imSucc)
		test.That(t, pc, test.ShouldResemble, pcSucc)

		mimeType, im, pc, err = workingSLAMClient.GetMap(context.Background(), nameSucc, utils.MimeTypeJPEG, pSucc.Pose, true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mimeType, test.ShouldEqual, utils.MimeTypeJPEG)
		test.That(t, im, test.ShouldResemble, imSucc)
		test.That(t, pc, test.ShouldResemble, pcSucc)

		// test close
		err = workingSLAMClient.Close()
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("dialed client tests for working slam service", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingDialedClient := slam.NewClientFromConn(context.Background(), conn, "", logger)

		// test get position
		p, err := workingDialedClient.GetPosition(context.Background(), nameSucc)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, p, test.ShouldResemble, pSucc)

		// test get map
		mimeType, im, pc, err := workingDialedClient.GetMap(context.Background(), nameSucc, utils.MimeTypePCD, pSucc.Pose, true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mimeType, test.ShouldEqual, utils.MimeTypePCD)
		test.That(t, im, test.ShouldResemble, imSucc)
		test.That(t, pc, test.ShouldResemble, pcSucc)

		// test close
		err = workingDialedClient.Close()
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("dialed client test 2 for working slam service", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		dialedClient := resourceSubtype.RPCClient(context.Background(), conn, "", logger)
		workingDialedClient, ok := dialedClient.(slam.Service)
		test.That(t, ok, test.ShouldBeTrue)

		// test get position
		p, err := workingDialedClient.GetPosition(context.Background(), nameSucc)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, p, test.ShouldResemble, pSucc)
	})

	go failingServer.Serve(listener2)
	defer failingServer.Stop()

	failingSLAMClient, err := slam.NewClient(
		context.Background(), slam.Name.String(),
		listener2.Addr().String(), logger,
	)
	test.That(t, err, test.ShouldBeNil)

	nameFail := "maiv"
	mimeTypeFail := utils.MimeTypeJPEG

	t.Run("dialed client test 2 for failing slam service", func(t *testing.T) {
		// test get position
		p, err := failingSLAMClient.GetPosition(context.Background(), nameFail)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, p, test.ShouldBeNil)

		// test get map
		mimeType, im, pc, err := failingSLAMClient.GetMap(context.Background(), nameFail, mimeTypeFail, pFail.Pose, true)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, mimeType, test.ShouldEqual, "")
		test.That(t, im, test.ShouldResemble, imFail)
		test.That(t, pc, test.ShouldResemble, pcFail)

		// test close
		err = failingSLAMClient.Close()
		test.That(t, err, test.ShouldBeNil)
	})
}
