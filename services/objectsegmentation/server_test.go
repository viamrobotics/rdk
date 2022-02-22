package objectsegmentation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/pointcloud"
	pb "go.viam.com/rdk/proto/api/service/objectsegmentation/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/objectsegmentation"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/segmentation"
)

func newServer(osMap map[resource.Name]interface{}) (pb.ObjectSegmentationServiceServer, error) {
	osSvc, err := subtype.New(osMap)
	if err != nil {
		return nil, err
	}
	return objectsegmentation.NewServer(osSvc), nil
}

func TestServerGetObjectPointClouds(t *testing.T) {
	osMap := map[resource.Name]interface{}{}
	server, err := newServer(osMap)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.GetObjectPointClouds(context.Background(), &pb.GetObjectPointCloudsRequest{})
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:object_segmentation\" not found"))

	// set up the robot with something that is not an objectsegmentation service
	osMap = map[resource.Name]interface{}{objectsegmentation.Name: "not object segmentation"}
	server, err = newServer(osMap)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.GetObjectPointClouds(context.Background(), &pb.GetObjectPointCloudsRequest{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected implementation of objectsegmentation.Service")

	// error
	injectOSS := &inject.ObjectSegmentationService{}
	osMap = map[resource.Name]interface{}{
		objectsegmentation.Name: injectOSS,
	}
	server, err = newServer(osMap)
	test.That(t, err, test.ShouldBeNil)
	passedErr := errors.New("fake object point clouds error")
	injectOSS.GetObjectPointCloudsFunc = func(ctx context.Context, cameraName string, params *vision.Parameters3D) ([]*vision.Object, error) {
		return nil, passedErr
	}
	req := &pb.GetObjectPointCloudsRequest{
		Name:               "fakeCamera",
		MimeType:           utils.MimeTypePCD,
		MinPointsInPlane:   5,
		MinPointsInSegment: 5,
		ClusteringRadiusMm: 5.,
	}
	_, err = server.GetObjectPointClouds(context.Background(), req)
	test.That(t, err, test.ShouldBeError, passedErr)

	// returns response
	// request the two segments in the point cloud
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
	segs, err := server.GetObjectPointClouds(context.Background(), &pb.GetObjectPointCloudsRequest{
		Name:               "fakeCamera",
		MinPointsInPlane:   100,
		MinPointsInSegment: 3,
		ClusteringRadiusMm: 5.,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(segs.Objects), test.ShouldEqual, 2)
	boxExpected, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 5}), r3.Vector{0, 0, 2})
	test.That(t, err, test.ShouldBeNil)
	box, err := spatialmath.NewGeometryFromProto(segs.Objects[0].Geometries.Geometries[0])
	test.That(t, err, test.ShouldBeNil)
	test.That(t, box.AlmostEqual(boxExpected), test.ShouldBeTrue)
	box, err = spatialmath.NewGeometryFromProto(segs.Objects[1].Geometries.Geometries[0])
	test.That(t, err, test.ShouldBeNil)
	test.That(t, box.AlmostEqual(boxExpected), test.ShouldBeTrue)
}
