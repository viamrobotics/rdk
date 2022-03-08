package objectsegmentation_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/config"
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
	injectOSS.GetObjectPointCloudsFunc = func(ctx context.Context,
		cameraName string,
		segmenterName string,
		params config.AttributeMap) ([]*vision.Object, error) {
		return nil, passedErr
	}
	params, err := structpb.NewStruct(config.AttributeMap{})
	test.That(t, err, test.ShouldBeNil)
	req := &pb.GetObjectPointCloudsRequest{
		CameraName:    "fakeCamera",
		SegmenterName: segmentation.RadiusClusteringSegmenter,
		MimeType:      utils.MimeTypePCD,
		Parameters:    params,
	}
	_, err = server.GetObjectPointClouds(context.Background(), req)
	test.That(t, err, test.ShouldBeError, passedErr)

	// create a working segmenter
	injCam := &inject.Camera{}
	injCam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		pcA := pointcloud.New()
		for _, pt := range testPointCloud {
			test.That(t, pcA.Set(pt), test.ShouldBeNil)
		}
		return pcA, nil
	}

	injectOSS.GetObjectPointCloudsFunc = func(ctx context.Context,
		cameraName string,
		segmenterName string,
		params config.AttributeMap) ([]*vision.Object, error) {
		segmenter, err := segmentation.SegmenterLookup(segmenterName)
		if err != nil {
			return nil, err
		}
		return segmenter.Segmenter(ctx, injCam, params)
	}
	injectOSS.GetSegmenterParametersFunc = func(ctx context.Context, segmenterName string) ([]string, error) {
		segmenter, err := segmentation.SegmenterLookup(segmenterName)
		if err != nil {
			return nil, err
		}
		return segmenter.Parameters, nil
	}

	// no such segmenter in registry
	_, err = server.GetSegmenterParameters(context.Background(), &pb.GetSegmenterParametersRequest{
		SegmenterName: "no_such_segmenter",
	})
	test.That(t, err.Error(), test.ShouldContainSubstring, "no Segmenter with name")

	_, err = server.GetObjectPointClouds(context.Background(), &pb.GetObjectPointCloudsRequest{
		CameraName:    "fakeCamera",
		SegmenterName: "no_such_segmenter",
		MimeType:      utils.MimeTypePCD,
		Parameters:    params,
	})
	test.That(t, err.Error(), test.ShouldContainSubstring, "no Segmenter with name")

	// successful request
	paramNamesResp, err := server.GetSegmenterParameters(context.Background(), &pb.GetSegmenterParametersRequest{
		SegmenterName: segmentation.RadiusClusteringSegmenter,
	})
	test.That(t, err, test.ShouldBeNil)
	paramNames := paramNamesResp.Parameters

	params, err = structpb.NewStruct(config.AttributeMap{
		paramNames[0]: 100, // min points in plane
		paramNames[1]: 3,   // min points in segment
		paramNames[2]: 5.,  //  clustering radius
	})
	test.That(t, err, test.ShouldBeNil)
	segs, err := server.GetObjectPointClouds(context.Background(), &pb.GetObjectPointCloudsRequest{
		CameraName:    "fakeCamera",
		SegmenterName: segmentation.RadiusClusteringSegmenter,
		MimeType:      utils.MimeTypePCD,
		Parameters:    params,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(segs.Objects), test.ShouldEqual, 2)

	expectedBoxes := makeExpectedBoxes(t)
	for _, object := range segs.Objects {
		box, err := spatialmath.NewGeometryFromProto(object.Geometries.Geometries[0])
		test.That(t, err, test.ShouldBeNil)
		test.That(t, box.AlmostEqual(expectedBoxes[0]) || box.AlmostEqual(expectedBoxes[1]), test.ShouldBeTrue)
	}
}
