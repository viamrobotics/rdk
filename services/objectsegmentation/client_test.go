package objectsegmentation_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/pointcloud"
	servicepb "go.viam.com/rdk/proto/api/service/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/objectsegmentation"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/segmentation"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectOSS := &inject.ObjectSegmentationService{}
	osMap := map[resource.Name]interface{}{
		objectsegmentation.Name: injectOSS,
	}
	svc, err := subtype.New(osMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(objectsegmentation.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = objectsegmentation.NewClient(cancelCtx, "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("working client", func(t *testing.T) {
		client, err := objectsegmentation.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		pcA := pointcloud.New()
		err = pcA.Set(pointcloud.NewBasicPoint(5, 5, 5))
		test.That(t, err, test.ShouldBeNil)
		err = pcA.Set(pointcloud.NewBasicPoint(5, 5, 6))
		test.That(t, err, test.ShouldBeNil)
		err = pcA.Set(pointcloud.NewBasicPoint(5, 5, 4))
		test.That(t, err, test.ShouldBeNil)
		err = pcA.Set(pointcloud.NewBasicPoint(-5, -5, 5))
		test.That(t, err, test.ShouldBeNil)
		err = pcA.Set(pointcloud.NewBasicPoint(-5, -5, 6))
		test.That(t, err, test.ShouldBeNil)
		err = pcA.Set(pointcloud.NewBasicPoint(-5, -5, 4))
		test.That(t, err, test.ShouldBeNil)

		injectOSS.GetObjectPointCloudsFunc = func(ctx context.Context, cameraName string, params *vision.Parameters3D) ([]*vision.Object, error) {
			seg, err := segmentation.NewObjectSegmentation(ctx, pcA, params)
			if err != nil {
				return nil, err
			}
			return seg.Objects(), nil
		}

		segs, err := client.GetObjectPointClouds(context.Background(), "", &vision.Parameters3D{100, 3, 5.})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(segs), test.ShouldEqual, 2)
		test.That(t, segs[0].Center.Z, test.ShouldEqual, 5.)
		test.That(t, segs[1].Center.Z, test.ShouldEqual, 5.)
		test.That(t, segs[0].BoundingBox.WidthMm, test.ShouldEqual, 0)
		test.That(t, segs[0].BoundingBox.LengthMm, test.ShouldEqual, 0)
		test.That(t, segs[0].BoundingBox.DepthMm, test.ShouldEqual, 2)
		test.That(t, segs[1].BoundingBox.WidthMm, test.ShouldEqual, 0)
		test.That(t, segs[1].BoundingBox.LengthMm, test.ShouldEqual, 0)
		test.That(t, segs[1].BoundingBox.DepthMm, test.ShouldEqual, 2)

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	})

	t.Run("broken client", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, "", logger)
		client2, ok := client.(objectsegmentation.Service)
		test.That(t, ok, test.ShouldBeTrue)

		passedErr := errors.New("fake get objects error")
		injectOSS.GetObjectPointCloudsFunc = func(ctx context.Context, cameraName string, params *vision.Parameters3D) ([]*vision.Object, error) {
			return nil, passedErr
		}

		resp, err := client2.GetObjectPointClouds(context.Background(), "", &vision.Parameters3D{})
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, utils.TryClose(context.Background(), client2), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	injectOSS := &inject.ObjectSegmentationService{}
	osMap := map[resource.Name]interface{}{
		objectsegmentation.Name: injectOSS,
	}
	server, err := newServer(osMap)
	test.That(t, err, test.ShouldBeNil)
	servicepb.RegisterObjectSegmentationServiceServer(gServer, server)

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := objectsegmentation.NewClient(ctx, "", listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	client2, err := objectsegmentation.NewClient(ctx, "", listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}
