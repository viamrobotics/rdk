// Package slam_test client_test.go tests the client for the SLAM service's GRPC server.
package slam_test

import (
	"context"
	"image"
	"math"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

func TestClientWorkingService(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	workingServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	pose := spatial.NewPoseFromOrientationVector(r3.Vector{1, 2, 3}, &spatial.OrientationVector{math.Pi / 2, 0, 0, -1})
	pSucc := referenceframe.NewPoseInFrame("frame", pose)

	pcSucc := &vision.Object{}
	pcSucc.PointCloud = pointcloud.New()
	err = pcSucc.PointCloud.Set(pointcloud.NewVector(5, 5, 5), nil)
	test.That(t, err, test.ShouldBeNil)

	imSucc := image.NewNRGBA(image.Rect(0, 0, 4, 4))

	workingSLAMService := &inject.SLAMService{}

	workingSLAMService.CloseFunc = func() error {
		return nil
	}

	workingSLAMService.GetPositionFunc = func(ctx context.Context, name string) (*referenceframe.PoseInFrame, error) {
		return pSucc, nil
	}

	workingSLAMService.GetMapFunc = func(ctx context.Context, name string, mimeType string, cp *referenceframe.PoseInFrame,
		include bool) (string, image.Image, *vision.Object, error) {
		if mimeType == utils.MimeTypePCD {
			return mimeType, nil, pcSucc, nil
		}
		return mimeType, imSucc, nil, nil
	}

	workingSvc, err := subtype.New(map[resource.Name]interface{}{slam.Name: workingSLAMService})
	test.That(t, err, test.ShouldBeNil)

	resourceSubtype := registry.ResourceSubtypeLookup(slam.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), workingServer, workingSvc)

	go workingServer.Serve(listener)
	defer workingServer.Stop()

	nameSucc := "viam"

	t.Run("test that context canceled stops client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = slam.NewClient(cancelCtx, slam.Name.String(), listener.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("client tests for using working SLAM client connection", func(t *testing.T) {
		workingSLAMClient, err := slam.NewClient(
			context.Background(), slam.Name.String(),
			listener.Addr().String(), logger,
		)
		test.That(t, err, test.ShouldBeNil)

		// test get position
		pInFrame, err := workingSLAMClient.GetPosition(context.Background(), nameSucc)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pInFrame.FrameName(), test.ShouldEqual, pSucc.FrameName())

		// test get map
		mimeType, im, pc, err := workingSLAMClient.GetMap(context.Background(), nameSucc, utils.MimeTypePCD, pSucc, true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mimeType, test.ShouldEqual, utils.MimeTypePCD)
		test.That(t, im, test.ShouldBeNil)
		test.That(t, pc.PointCloud, test.ShouldNotBeNil)

		mimeType, im, pc, err = workingSLAMClient.GetMap(context.Background(), nameSucc, utils.MimeTypeJPEG, pSucc, true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mimeType, test.ShouldEqual, utils.MimeTypeJPEG)
		test.That(t, im, test.ShouldNotBeNil)
		test.That(t, pc.PointCloud, test.ShouldBeNil)

		// test close
		err = workingSLAMClient.Close()
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("client tests using working GRPC dial connection", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingDialedClient := slam.NewClientFromConn(context.Background(), conn, "", logger)

		// test get position
		pInFrame, err := workingDialedClient.GetPosition(context.Background(), nameSucc)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pInFrame.FrameName(), test.ShouldEqual, pSucc.FrameName())

		// test get map
		mimeType, im, pc, err := workingDialedClient.GetMap(context.Background(), nameSucc, utils.MimeTypePCD, pSucc, true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mimeType, test.ShouldEqual, utils.MimeTypePCD)
		test.That(t, im, test.ShouldBeNil)
		test.That(t, pc, test.ShouldNotBeNil)

		// test close
		err = workingDialedClient.Close()
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("client tests using working GRPC dial connection converted to SLAM client", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		dialedClient := resourceSubtype.RPCClient(context.Background(), conn, "", logger)
		workingDialedClient, ok := dialedClient.(slam.Service)
		test.That(t, ok, test.ShouldBeTrue)

		// test get position
		p, err := workingDialedClient.GetPosition(context.Background(), nameSucc)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, p.FrameName(), test.ShouldEqual, pSucc.FrameName())
	})
}

func TestClientFailingService(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	failingServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	pose := spatial.NewPoseFromOrientationVector(r3.Vector{1, 2, 3}, &spatial.OrientationVector{math.Pi / 2, 0, 0, -1})
	pFail := referenceframe.NewPoseInFrame("frame", pose)
	pcFail := &vision.Object{}
	pcFail.PointCloud = pointcloud.New()
	err = pcFail.PointCloud.Set(pointcloud.NewVector(5, 5, 5), nil)
	test.That(t, err, test.ShouldBeNil)
	imFail := image.NewNRGBA(image.Rect(0, 0, 4, 4))

	failingSLAMService := &inject.SLAMService{}

	failingSLAMService.CloseFunc = func() error {
		return errors.New("failure to close")
	}

	failingSLAMService.GetPositionFunc = func(ctx context.Context, name string) (*referenceframe.PoseInFrame, error) {
		return pFail, errors.New("failure to get position")
	}

	failingSLAMService.GetMapFunc = func(ctx context.Context, name string, mimeType string, cp *referenceframe.PoseInFrame,
		include bool) (string, image.Image, *vision.Object, error) {
		return mimeType, imFail, pcFail, errors.New("failure to get map")
	}

	failingSvc, err := subtype.New(map[resource.Name]interface{}{slam.Name: failingSLAMService})
	test.That(t, err, test.ShouldBeNil)

	resourceSubtype := registry.ResourceSubtypeLookup(slam.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), failingServer, failingSvc)

	go failingServer.Serve(listener)
	defer failingServer.Stop()

	nameFail := "maiv"

	t.Run("client test using bad SLAM client connection", func(t *testing.T) {
		failingSLAMClient, err := slam.NewClient(
			context.Background(), slam.Name.String(),
			listener.Addr().String(), logger,
		)
		test.That(t, err, test.ShouldBeNil)

		// test get position
		p, err := failingSLAMClient.GetPosition(context.Background(), nameFail)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, p, test.ShouldBeNil)

		// test get map
		mimeType, im, pc, err := failingSLAMClient.GetMap(context.Background(), nameFail, utils.MimeTypeJPEG, pFail, true)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, mimeType, test.ShouldEqual, "")
		test.That(t, im, test.ShouldBeNil)
		test.That(t, pc.PointCloud, test.ShouldBeNil)

		// test close
		err = failingSLAMClient.Close()
		test.That(t, err, test.ShouldBeNil)
	})
}
