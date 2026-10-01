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
	box := func(dims float64, label string) spatialmath.Geometry {
		b, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: dims, Y: dims, Z: dims}, label)
		test.That(t, err, test.ShouldBeNil)
		return b
	}
	pointsAt := func(xs ...float64) spatialmath.Geometry {
		cloud := pointcloud.NewBasicEmpty()
		for _, x := range xs {
			test.That(t, cloud.Set(r3.Vector{X: x}, pointcloud.NewBasicData()), test.ShouldBeNil)
		}
		octree, err := pointcloud.ToBasicOctree(cloud, 0)
		test.That(t, err, test.ShouldBeNil)
		return octree
	}
	at := func(x float64) spatialmath.Pose { return spatialmath.NewPoseFromPoint(r3.Vector{X: x}) }

	mug := &detection3d.Detection{Transforms: []*referenceframe.LinkInFrame{
		referenceframe.NewLinkInFrame(testCameraName, at(100), "mug", box(10, "body")),
		referenceframe.NewLinkInFrame("mug", at(20), "mug/handle", box(2, "handle")),
		referenceframe.NewLinkInFrame("mug", spatialmath.NewZeroPose(), "mug/points", pointsAt(-1, 1)),
	}}
	pointsOnly := &detection3d.Detection{Transforms: []*referenceframe.LinkInFrame{
		referenceframe.NewLinkInFrame(testCameraName, at(50), "blob", nil),
		referenceframe.NewLinkInFrame("blob", spatialmath.NewZeroPose(), "blob/points", pointsAt(-2, 2)),
	}}
	detections := []*detection3d.Detection{mug, pointsOnly}

	detectCalls := 0
	var gotCamera string
	detector := func(ctx context.Context, src camera.Camera) ([]*detection3d.Detection, error) {
		detectCalls++
		gotCamera = src.Name().ShortName()
		return detections, nil
	}
	cam := inject.NewCamera(testCameraName)
	cam.ImagesFunc = func(
		ctx context.Context, filterSourceNames []string, extra map[string]interface{},
	) ([]camera.NamedImage, resource.ResponseMetadata, error) {
		img, err := camera.NamedImageFromImage(image.NewRGBA(image.Rect(0, 0, 3, 3)), "", utils.MimeTypePNG, data.Annotations{})
		return []camera.NamedImage{img}, resource.ResponseMetadata{}, err
	}
	deps := resource.Dependencies{camera.Named(testCameraName): cam}
	svc, err := vision.NewService(vision.Named("det"), deps, logging.NewTestLogger(t), nil, nil, nil, detector, testCameraName)
	test.That(t, err, test.ShouldBeNil)

	props, err := svc.GetProperties(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.Detections3DSupported, test.ShouldBeTrue)
	test.That(t, props.ObjectPCDsSupported, test.ShouldBeTrue)

	t.Run("GetDetections3D forwards the detector's output unchanged", func(t *testing.T) {
		got, err := svc.GetDetections3D(context.Background(), "", nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gotCamera, test.ShouldEqual, testCameraName)
		test.That(t, got, test.ShouldResemble, detections)
	})

	t.Run("GetObjectPointClouds flattens each tree into the root's parent frame", func(t *testing.T) {
		objects, err := svc.GetObjectPointClouds(context.Background(), "", nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(objects), test.ShouldEqual, 2)

		// The root box wins over the handle, and the points are re-expressed around the root's pose.
		test.That(t, objects[0].Geometry.Label(), test.ShouldEqual, "body")
		test.That(t, spatialmath.R3VectorAlmostEqual(objects[0].Geometry.Pose().Point(), r3.Vector{X: 100}, 1e-6), test.ShouldBeTrue)
		var xs []float64
		objects[0].Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
			xs = append(xs, p.X)
			return true
		})
		test.That(t, xs, test.ShouldHaveLength, 2)
		test.That(t, xs, test.ShouldContain, 99.0)
		test.That(t, xs, test.ShouldContain, 101.0)

		// A detection with only points still gets a geometry, since obstacle consumers rely on it.
		test.That(t, objects[1].Geometry, test.ShouldNotBeNil)
		test.That(t, spatialmath.R3VectorAlmostEqual(objects[1].Geometry.Pose().Point(), r3.Vector{X: 50}, 1e-6), test.ShouldBeTrue)
	})

	t.Run("CaptureAll calls the detector once for both 3D outputs", func(t *testing.T) {
		detectCalls = 0
		capt, err := svc.CaptureAllFromCamera(context.Background(), "", viscapture.CaptureOptions{
			ReturnObject:       true,
			ReturnDetections3D: true,
		}, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, detectCalls, test.ShouldEqual, 1)
		test.That(t, len(capt.Objects), test.ShouldEqual, 2)
		test.That(t, capt.Detections3D, test.ShouldResemble, detections)
	})

	t.Run("a transform whose parent is not earlier in the tree fails flattening", func(t *testing.T) {
		detections = []*detection3d.Detection{{Transforms: []*referenceframe.LinkInFrame{
			referenceframe.NewLinkInFrame(testCameraName, at(0), "root", nil),
			referenceframe.NewLinkInFrame("missing", at(0), "orphan", box(1, "")),
		}}}
		_, err := svc.GetObjectPointClouds(context.Background(), "", nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `"missing"`)
	})
}
