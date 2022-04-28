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

	"go.viam.com/rdk/config"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/pointcloud"
	servicepb "go.viam.com/rdk/proto/api/service/objectsegmentation/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/objectsegmentation"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
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

		injCam := &inject.Camera{}
		injCam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
			pcA := pointcloud.New()
			for _, pt := range testPointCloud {
				test.That(t, pcA.Set(pt, nil), test.ShouldBeNil)
			}
			return pcA, nil
		}

		injectOSS.GetSegmenterParametersFunc = func(ctx context.Context, segmenterName string) ([]rdkutils.TypedName, error) {
			return rdkutils.JSONTags(segmentation.RadiusClusteringConfig{}), nil
		}
		injectOSS.GetObjectPointCloudsFunc = func(ctx context.Context,
			cameraName string,
			segmenterName string,
			params config.AttributeMap,
		) ([]*vision.Object, error) {
			segments, err := segmentation.RadiusClustering(ctx, injCam, params)
			if err != nil {
				return nil, err
			}
			return segments, nil
		}
		injectOSS.GetSegmentersFunc = func(ctx context.Context) ([]string, error) {
			return []string{objectsegmentation.RadiusClusteringSegmenter}, nil
		}

		segNames, err := client.GetSegmenters(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, segNames, test.ShouldHaveLength, 1)
		test.That(t, segNames[0], test.ShouldEqual, objectsegmentation.RadiusClusteringSegmenter)

		paramNames, err := client.GetSegmenterParameters(context.Background(), segNames[0])
		test.That(t, err, test.ShouldBeNil)
		expParams := []rdkutils.TypedName{
			{"min_points_in_plane", "int"},
			{"min_points_in_segment", "int"},
			{"clustering_radius_mm", "float64"},
			{"mean_k_filtering", "int"},
		}
		test.That(t, paramNames, test.ShouldResemble, expParams)
		params := config.AttributeMap{
			paramNames[0].Name: 100,
			paramNames[1].Name: 3,
			paramNames[2].Name: 5.0,
			paramNames[3].Name: 10,
		}
		segs, err := client.GetObjectPointClouds(context.Background(), "", segNames[0], params)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(segs), test.ShouldEqual, 2)

		expectedBoxes := makeExpectedBoxes(t)
		for _, seg := range segs {
			box, err := pointcloud.BoundingBoxFromPointCloud(seg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, box, test.ShouldNotBeNil)
			test.That(t, box.AlmostEqual(expectedBoxes[0]) || box.AlmostEqual(expectedBoxes[1]), test.ShouldBeTrue)
		}

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	})

	t.Run("broken client", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, "", logger)
		client2, ok := client.(objectsegmentation.Service)
		test.That(t, ok, test.ShouldBeTrue)

		passedErr := errors.New("fake get objects error")
		injectOSS.GetObjectPointCloudsFunc = func(ctx context.Context,
			cameraName string,
			segmenterName string,
			params config.AttributeMap,
		) ([]*vision.Object, error) {
			return nil, passedErr
		}

		resp, err := client2.GetObjectPointClouds(context.Background(), "", "", config.AttributeMap{})
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
