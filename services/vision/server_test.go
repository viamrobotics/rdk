package vision_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"google.golang.org/protobuf/types/known/structpb"

	// register cameras for testing.
	_ "go.viam.com/rdk/component/camera/register"
	"go.viam.com/rdk/config"
	pb "go.viam.com/rdk/proto/api/service/vision/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/segmentation"
)

func newServer(m map[resource.Name]interface{}) (pb.VisionServiceServer, error) {
	svc, err := subtype.New(m)
	if err != nil {
		return nil, err
	}
	return vision.NewServer(svc), nil
}

func TestVisionServerFailures(t *testing.T) {
	nameRequest := &pb.GetDetectorNamesRequest{
		Name: testVisionServiceName,
	}

	// no service
	m := map[resource.Name]interface{}{}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.GetDetectorNames(context.Background(), nameRequest)
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:vision/vision1\" not found"))

	// set up the robot with something that is not a vision service
	m = map[resource.Name]interface{}{vision.Named(testVisionServiceName): "not what you want"}
	server, err = newServer(m)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.GetDetectorNames(context.Background(), nameRequest)
	test.That(t, err, test.ShouldBeError, utils.NewUnimplementedInterfaceError("vision.Service", "string"))

	// correct server
	injectODS := &inject.VisionService{}
	m = map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): injectODS,
	}
	server, err = newServer(m)
	test.That(t, err, test.ShouldBeNil)
	// error
	passedErr := errors.New("fake error")
	injectODS.GetDetectorNamesFunc = func(ctx context.Context) ([]string, error) {
		return nil, passedErr
	}
	_, err = server.GetDetectorNames(context.Background(), nameRequest)
	test.That(t, err, test.ShouldBeError, passedErr)
}

func TestServerGetDetectorNames(t *testing.T) {
	injectODS := &inject.VisionService{}
	m := map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): injectODS,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)

	// returns response
	expSlice := []string{"test name"}
	injectODS.GetDetectorNamesFunc = func(ctx context.Context) ([]string, error) {
		return expSlice, nil
	}
	nameRequest := &pb.GetDetectorNamesRequest{Name: testVisionServiceName}
	resp, err := server.GetDetectorNames(context.Background(), nameRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetDetectorNames(), test.ShouldResemble, expSlice)
}

func TestServerAddDetector(t *testing.T) {
	srv, r := createService(t, "data/empty.json")
	m := map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): srv,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)
	params, err := structpb.NewStruct(config.AttributeMap{
		"detect_color": "#112233",
		"tolerance":    0.4,
		"segment_size": 200,
	})
	test.That(t, err, test.ShouldBeNil)
	// success
	_, err = server.AddDetector(context.Background(), &pb.AddDetectorRequest{
		Name:               testVisionServiceName,
		DetectorName:       "test",
		DetectorModelType:  "color",
		DetectorParameters: params,
	})
	test.That(t, err, test.ShouldBeNil)
	// did it add the detector
	detRequest := &pb.GetDetectorNamesRequest{Name: testVisionServiceName}
	detResp, err := server.GetDetectorNames(context.Background(), detRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, detResp.GetDetectorNames(), test.ShouldContain, "test")
	// did it also add the segmenter
	segRequest := &pb.GetSegmenterNamesRequest{Name: testVisionServiceName}
	segResp, err := server.GetSegmenterNames(context.Background(), segRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, segResp.GetSegmenterNames(), test.ShouldContain, "test")
	// failure
	resp, err := server.AddDetector(context.Background(), &pb.AddDetectorRequest{
		Name:               testVisionServiceName,
		DetectorName:       "failing",
		DetectorModelType:  "no_such_type",
		DetectorParameters: params,
	})
	test.That(t, err.Error(), test.ShouldContainSubstring, "is not implemented")
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

func TestServerGetDetections(t *testing.T) {
	r := buildRobotWithFakeCamera(t)
	visName := vision.FindVisionName(r)
	srv, err := vision.FromRobot(r, visName)
	test.That(t, err, test.ShouldBeNil)
	m := map[resource.Name]interface{}{
		vision.Named(visName): srv,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)
	// success
	resp, err := server.GetDetectionsFromCamera(context.Background(), &pb.GetDetectionsFromCameraRequest{
		Name:         visName,
		CameraName:   "fake_cam",
		DetectorName: "detect_red",
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Detections, test.ShouldHaveLength, 1)
	test.That(t, resp.Detections[0].Confidence, test.ShouldEqual, 1.0)
	test.That(t, resp.Detections[0].ClassName, test.ShouldEqual, "red")
	test.That(t, *(resp.Detections[0].XMin), test.ShouldEqual, 110)
	test.That(t, *(resp.Detections[0].YMin), test.ShouldEqual, 288)
	test.That(t, *(resp.Detections[0].XMax), test.ShouldEqual, 183)
	test.That(t, *(resp.Detections[0].YMax), test.ShouldEqual, 349)
	// failure - empty request
	_, err = server.GetDetectionsFromCamera(context.Background(), &pb.GetDetectionsFromCameraRequest{Name: testVisionServiceName})
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)

	// test new getdetections straight from image
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	params, err := structpb.NewStruct(config.AttributeMap{
		"model_path":  modelLoc,
		"num_threads": 1,
	})
	test.That(t, err, test.ShouldBeNil)
	// success
	addDetResp, err := server.AddDetector(context.Background(), &pb.AddDetectorRequest{
		Name:               visName,
		DetectorName:       "test",
		DetectorModelType:  "tflite",
		DetectorParameters: params,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, addDetResp, test.ShouldNotBeNil)
	img, _ := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	imgBytes, err := rimage.EncodeImage(context.Background(), img, utils.MimeTypeJPEG)
	test.That(t, err, test.ShouldBeNil)

	resp2, err := server.GetDetections(context.Background(), &pb.GetDetectionsRequest{
		Name:         visName,
		Image:        imgBytes,
		Width:        int64(img.Width()),
		Height:       int64(img.Height()),
		MimeType:     utils.MimeTypeJPEG,
		DetectorName: "test",
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp2.Detections, test.ShouldNotBeNil)
	test.That(t, resp2.Detections[0].ClassName, test.ShouldResemble, "17")
	test.That(t, resp2.Detections[0].Confidence, test.ShouldBeGreaterThan, 0.79)
	test.That(t, resp2.Detections[1].ClassName, test.ShouldResemble, "17")
	test.That(t, resp2.Detections[1].Confidence, test.ShouldBeGreaterThan, 0.73)
}

func TestServerObjectSegmentation(t *testing.T) {
	// create a working segmenter
	injCam := &cloudSource{}

	injectOSS := &inject.VisionService{}
	injectOSS.GetObjectPointCloudsFunc = func(ctx context.Context,
		cameraName string,
		segmenterName string,
		params config.AttributeMap,
	) ([]*viz.Object, error) {
		switch segmenterName {
		case vision.RadiusClusteringSegmenter:
			segments, err := segmentation.RadiusClustering(ctx, injCam, params)
			if err != nil {
				return nil, err
			}
			return segments, nil
		default:
			return nil, errors.Errorf("no Segmenter with name %s", segmenterName)
		}
	}
	injectOSS.GetSegmenterParametersFunc = func(ctx context.Context, segmenterName string) ([]utils.TypedName, error) {
		switch segmenterName {
		case vision.RadiusClusteringSegmenter:
			return utils.JSONTags(segmentation.RadiusClusteringConfig{}), nil
		default:
			return nil, errors.Errorf("no Segmenter with name %s", segmenterName)
		}
	}
	injectOSS.GetSegmenterNamesFunc = func(ctx context.Context) ([]string, error) {
		return []string{vision.RadiusClusteringSegmenter}, nil
	}
	// make server
	m := map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): injectOSS,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)
	// request segmenters
	segReq := &pb.GetSegmenterNamesRequest{
		Name: testVisionServiceName,
	}
	segResp, err := server.GetSegmenterNames(context.Background(), segReq)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, segResp.SegmenterNames, test.ShouldHaveLength, 1)
	test.That(t, segResp.SegmenterNames[0], test.ShouldEqual, vision.RadiusClusteringSegmenter)

	// no such segmenter in registry
	_, err = server.GetSegmenterParameters(context.Background(), &pb.GetSegmenterParametersRequest{
		Name:          testVisionServiceName,
		SegmenterName: "no_such_segmenter",
	})
	test.That(t, err.Error(), test.ShouldContainSubstring, "no Segmenter with name")

	params, err := structpb.NewStruct(config.AttributeMap{})
	test.That(t, err, test.ShouldBeNil)
	_, err = server.GetObjectPointClouds(context.Background(), &pb.GetObjectPointCloudsRequest{
		Name:          testVisionServiceName,
		CameraName:    "fakeCamera",
		SegmenterName: "no_such_segmenter",
		MimeType:      utils.MimeTypePCD,
		Parameters:    params,
	})
	test.That(t, err.Error(), test.ShouldContainSubstring, "no Segmenter with name")

	// successful request
	paramNamesResp, err := server.GetSegmenterParameters(context.Background(), &pb.GetSegmenterParametersRequest{
		Name:          testVisionServiceName,
		SegmenterName: vision.RadiusClusteringSegmenter,
	})
	test.That(t, err, test.ShouldBeNil)
	paramNames := paramNamesResp.SegmenterParameters
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
		Name:          testVisionServiceName,
		CameraName:    "fakeCamera",
		SegmenterName: vision.RadiusClusteringSegmenter,
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
