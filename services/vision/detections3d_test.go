package vision_test

import (
	"context"
	"image"
	"net"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/data"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
	visionObject "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/detection3d"
	"go.viam.com/rdk/vision/viscapture"
)

func TestDetections3DClientServerRoundTrip(t *testing.T) {
	logger := logging.NewTestLogger(t)
	//nolint: noctx
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	box, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: 82, Y: 82, Z: 95}, "body")
	test.That(t, err, test.ShouldBeNil)
	root := referenceframe.NewLinkInFrame("cam", spatialmath.NewPoseFromPoint(r3.Vector{X: 412}), "vision/mug", box)
	root.SetUUID([]byte("mug-1"))
	root.SetMetadata(map[string]interface{}{"color": "red"})
	handle := referenceframe.NewLinkInFrame("vision/mug", spatialmath.NewPoseFromPoint(r3.Vector{X: 52}), "vision/mug/handle", nil)
	want := []*detection3d.Detection{{
		Transforms:      []*referenceframe.LinkInFrame{root, handle},
		Classifications: classification.Classifications{classification.NewClassification(0.93, "mug")},
		Metadata:        map[string]interface{}{"model": "sam3-segments"},
	}}

	srv := &inject.VisionService{}
	srv.GetDetections3DFunc = func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*detection3d.Detection, error) {
		return want, nil
	}
	srv.GetPropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*vision.Properties, error) {
		return &vision.Properties{Detections3DSupported: true}, nil
	}
	srv.CaptureAllFromCameraFunc = func(
		ctx context.Context, cameraName string, opts viscapture.CaptureOptions, extra map[string]interface{},
	) (viscapture.VisCapture, error) {
		test.That(t, opts.ReturnDetections3D, test.ShouldBeTrue)
		return viscapture.VisCapture{Detections3D: want}, nil
	}
	svc, err := resource.NewAPIResourceCollection(vision.API, map[resource.Name]vision.Service{visName1: srv})
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[vision.Service](vision.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, svc, logger), test.ShouldBeNil)
	go rpcServer.Serve(listener)
	defer rpcServer.Stop()

	conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()
	client, err := vision.NewClientFromConn(context.Background(), conn, "", visName1, logger)
	test.That(t, err, test.ShouldBeNil)

	checkDetections := func(got []*detection3d.Detection) {
		test.That(t, len(got), test.ShouldEqual, 1)
		test.That(t, len(got[0].Transforms), test.ShouldEqual, 2)
		gotRoot, gotHandle := got[0].Transforms[0], got[0].Transforms[1]
		test.That(t, gotRoot.Name(), test.ShouldEqual, "vision/mug")
		test.That(t, gotRoot.Parent(), test.ShouldEqual, "cam")
		test.That(t, gotRoot.UUID(), test.ShouldResemble, []byte("mug-1"))
		test.That(t, gotRoot.Metadata(), test.ShouldResemble, map[string]interface{}{"color": "red"})
		test.That(t, spatialmath.GeometriesAlmostEqual(gotRoot.Geometry(), box), test.ShouldBeTrue)
		test.That(t, gotHandle.Parent(), test.ShouldEqual, "vision/mug")
		test.That(t, gotHandle.Geometry(), test.ShouldBeNil)
		test.That(t, got[0].Classifications[0].Label(), test.ShouldEqual, "mug")
		test.That(t, got[0].Classifications[0].Score(), test.ShouldEqual, 0.93)
		test.That(t, got[0].Metadata, test.ShouldResemble, map[string]interface{}{"model": "sam3-segments"})
	}

	got, err := client.GetDetections3D(context.Background(), "cam", nil)
	test.That(t, err, test.ShouldBeNil)
	checkDetections(got)

	capt, err := client.CaptureAllFromCamera(context.Background(), "cam", viscapture.CaptureOptions{ReturnDetections3D: true}, nil)
	test.That(t, err, test.ShouldBeNil)
	checkDetections(capt.Detections3D)

	props, err := client.GetProperties(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.Detections3DSupported, test.ShouldBeTrue)
}

func TestBuilderGetDetections3D(t *testing.T) {
	cloud := pointcloud.NewBasicEmpty()
	test.That(t, cloud.Set(r3.Vector{X: 10}, pointcloud.NewBasicData()), test.ShouldBeNil)
	test.That(t, cloud.Set(r3.Vector{X: 12}, pointcloud.NewBasicData()), test.ShouldBeNil)
	mug, err := visionObject.NewObjectWithLabel(cloud, "mug", nil)
	test.That(t, err, test.ShouldBeNil)

	segmentCalls := 0
	segmenter := func(ctx context.Context, src camera.Camera) ([]*visionObject.Object, error) {
		segmentCalls++
		return []*visionObject.Object{mug, visionObject.NewEmptyObject()}, nil
	}
	cam := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context, filterSourceNames []string, extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			img, err := camera.NamedImageFromImage(image.NewRGBA(image.Rect(0, 0, 3, 3)), "", utils.MimeTypePNG, data.Annotations{})
			return []camera.NamedImage{img}, resource.ResponseMetadata{}, err
		},
	}
	deps := resource.Dependencies{camera.Named(testCameraName): cam}
	svc, err := vision.NewService(vision.Named("seg"), deps, logging.NewTestLogger(t), nil, nil, nil, segmenter, testCameraName)
	test.That(t, err, test.ShouldBeNil)

	props, err := svc.GetProperties(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.Detections3DSupported, test.ShouldBeTrue)

	// An empty camera name falls back to the default camera, which becomes the root's parent frame.
	dets, err := svc.GetDetections3D(context.Background(), "", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(dets), test.ShouldEqual, 2)

	t.Run("object becomes a recentered root plus a points child", func(t *testing.T) {
		test.That(t, len(dets[0].Transforms), test.ShouldEqual, 2)
		root, points := dets[0].Transforms[0], dets[0].Transforms[1]
		test.That(t, root.Name(), test.ShouldEqual, "seg/object-0")
		test.That(t, root.Parent(), test.ShouldEqual, testCameraName)
		test.That(t, spatialmath.R3VectorAlmostEqual(root.Pose().Point(), r3.Vector{X: 11}, 1e-6), test.ShouldBeTrue)
		test.That(t, spatialmath.PoseAlmostEqual(root.Geometry().Pose(), spatialmath.NewZeroPose()), test.ShouldBeTrue)
		test.That(t, root.Geometry().Label(), test.ShouldEqual, "mug")

		test.That(t, points.Name(), test.ShouldEqual, "seg/object-0/points")
		test.That(t, points.Parent(), test.ShouldEqual, "seg/object-0")
		octree, ok := points.Geometry().(*pointcloud.BasicOctree)
		test.That(t, ok, test.ShouldBeTrue)
		var xs []float64
		octree.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
			xs = append(xs, p.X)
			return true
		})
		test.That(t, xs, test.ShouldHaveLength, 2)
		test.That(t, xs, test.ShouldContain, -1.0)
		test.That(t, xs, test.ShouldContain, 1.0)
	})

	t.Run("empty object has no geometry and no points child", func(t *testing.T) {
		test.That(t, len(dets[1].Transforms), test.ShouldEqual, 1)
		test.That(t, dets[1].Transforms[0].Geometry(), test.ShouldBeNil)
	})

	t.Run("CaptureAll segments once for both 3D outputs", func(t *testing.T) {
		segmentCalls = 0
		capt, err := svc.CaptureAllFromCamera(context.Background(), "", viscapture.CaptureOptions{
			ReturnObject:       true,
			ReturnDetections3D: true,
		}, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, segmentCalls, test.ShouldEqual, 1)
		test.That(t, len(capt.Objects), test.ShouldEqual, 2)
		test.That(t, len(capt.Detections3D), test.ShouldEqual, 2)
	})
}
