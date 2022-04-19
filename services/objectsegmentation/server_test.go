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
)

func newServer(osMap map[resource.Name]interface{}) (pb.ObjectSegmentationServiceServer, error) {
	osSvc, err := subtype.New(osMap)
	if err != nil {
		return nil, err
	}
	return objectsegmentation.NewServer(osSvc), nil
}

func TestServerObjectSegmentation(t *testing.T) {
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

	// error GetSegmenters
	segErr := errors.New("segmenters error")
	injectOSS.GetSegmentersFunc = func(ctx context.Context) ([]string, error) {
		return nil, segErr
	}
	segReq := &pb.GetSegmentersRequest{}
	_, err = server.GetSegmenters(context.Background(), segReq)
	test.That(t, err, test.ShouldBeError, segErr)
	// error GetObjectPointClouds
	passedErr := errors.New("fake object point clouds error")
	injectOSS.GetObjectPointCloudsFunc = func(ctx context.Context,
		cameraName string,
		segmenterName string,
		params config.AttributeMap,
	) ([]*vision.Object, error) {
		return nil, passedErr
	}
	params, err := structpb.NewStruct(config.AttributeMap{})
	test.That(t, err, test.ShouldBeNil)
	req := &pb.GetObjectPointCloudsRequest{
		CameraName:    "fakeCamera",
		SegmenterName: objectsegmentation.RadiusClusteringSegmenter,
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
			test.That(t, pcA.Set(pt, nil), test.ShouldBeNil)
		}
		return pcA, nil
	}

	injectOSS.GetObjectPointCloudsFunc = func(ctx context.Context,
		cameraName string,
		segmenterName string,
		params config.AttributeMap) ([]*vision.Object, error) {
		segmenter, err := objectsegmentation.SegmenterLookup(segmenterName)
		if err != nil {
			return nil, err
		}
		return segmenter.Segmenter(ctx, injCam, params)
	}
	injectOSS.GetSegmenterParametersFunc = func(ctx context.Context, segmenterName string) ([]utils.TypedName, error) {
		segmenter, err := objectsegmentation.SegmenterLookup(segmenterName)
		if err != nil {
			return nil, err
		}
		return segmenter.Parameters, nil
	}
	injectOSS.GetSegmentersFunc = func(ctx context.Context) ([]string, error) {
		return []string{objectsegmentation.RadiusClusteringSegmenter}, nil
	}
	// request segmenters
	segResp, err := server.GetSegmenters(context.Background(), segReq)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, segResp.Segmenters, test.ShouldHaveLength, 1)
	test.That(t, segResp.Segmenters[0], test.ShouldEqual, objectsegmentation.RadiusClusteringSegmenter)

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
		SegmenterName: objectsegmentation.RadiusClusteringSegmenter,
	})
	test.That(t, err, test.ShouldBeNil)
	paramNames := paramNamesResp.Parameters
	test.That(t, paramNames[0].Type, test.ShouldEqual, "int")
	test.That(t, paramNames[1].Type, test.ShouldEqual, "int")
	test.That(t, paramNames[2].Type, test.ShouldEqual, "float64")
	test.That(t, paramNames[3].Type, test.ShouldEqual, "int")

	params, err = structpb.NewStruct(config.AttributeMap{
		paramNames[0].Name: 100, // min points in plane
		paramNames[1].Name: 3,   // min points in segment
		paramNames[2].Name: 5.,  //  clustering radius
		paramNames[3].Name: 10,  //  mean_k_filtering
	})
	test.That(t, err, test.ShouldBeNil)
	segs, err := server.GetObjectPointClouds(context.Background(), &pb.GetObjectPointCloudsRequest{
		CameraName:    "fakeCamera",
		SegmenterName: objectsegmentation.RadiusClusteringSegmenter,
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
