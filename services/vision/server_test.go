package vision_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/invopop/jsonschema"
	"github.com/pkg/errors"
	// register cameras for testing.
	pb "go.viam.com/api/service/vision/v1"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/camera"
	_ "go.viam.com/rdk/components/camera/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/services/vision/builtin"
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
	test.That(t, err, test.ShouldBeError, vision.NewUnimplementedInterfaceError("string"))

	// correct server
	injectVS := &inject.VisionService{}
	m = map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): injectVS,
	}
	server, err = newServer(m)
	test.That(t, err, test.ShouldBeNil)
	// error
	passedErr := errors.New("fake error")
	injectVS.GetDetectorNamesFunc = func(ctx context.Context, extra map[string]interface{}) ([]string, error) {
		return nil, passedErr
	}
	_, err = server.GetDetectorNames(context.Background(), nameRequest)
	test.That(t, err, test.ShouldBeError, passedErr)
}

func TestServerGetParameterSchema(t *testing.T) {
	srv, r := createService(t)
	m := map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): srv,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)
	paramsRequest := &pb.GetModelParameterSchemaRequest{Name: testVisionServiceName, ModelType: string(builtin.RCSegmenter)}
	params, err := server.GetModelParameterSchema(context.Background(), paramsRequest)
	test.That(t, err, test.ShouldBeNil)
	outp := &jsonschema.Schema{}
	err = json.Unmarshal(params.ModelParameterSchema, outp)
	test.That(t, err, test.ShouldBeNil)
	parameterNames := outp.Definitions["RadiusClusteringConfig"].Required
	test.That(t, parameterNames, test.ShouldContain, "min_points_in_plane")
	test.That(t, parameterNames, test.ShouldContain, "min_points_in_segment")
	test.That(t, parameterNames, test.ShouldContain, "clustering_radius_mm")
	test.That(t, parameterNames, test.ShouldContain, "mean_k_filtering")
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

func TestServerGetDetectorNames(t *testing.T) {
	injectVS := &inject.VisionService{}
	m := map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): injectVS,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)

	// returns response
	expSlice := []string{"test name"}
	var extraOptions map[string]interface{}
	injectVS.GetDetectorNamesFunc = func(ctx context.Context, extra map[string]interface{}) ([]string, error) {
		extraOptions = extra
		return expSlice, nil
	}
	extra := map[string]interface{}{"foo": "GetDetectorNames"}
	ext, err := protoutils.StructToStructPb(extra)
	test.That(t, err, test.ShouldBeNil)
	nameRequest := &pb.GetDetectorNamesRequest{Name: testVisionServiceName, Extra: ext}
	resp, err := server.GetDetectorNames(context.Background(), nameRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetDetectorNames(), test.ShouldResemble, expSlice)
	test.That(t, extraOptions, test.ShouldResemble, extra)
}

func TestServerAddDetector(t *testing.T) {
	srv, r := createService(t)
	m := map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): srv,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)
	params, err := protoutils.StructToStructPb(config.AttributeMap{
		"detect_color":      "#112233",
		"hue_tolerance_pct": 0.4,
		"value_cutoff_pct":  0.2,
		"segment_size_px":   200,
	})
	test.That(t, err, test.ShouldBeNil)
	// success
	_, err = server.AddDetector(context.Background(), &pb.AddDetectorRequest{
		Name:               testVisionServiceName,
		DetectorName:       "test",
		DetectorModelType:  "color_detector",
		DetectorParameters: params,
	})
	test.That(t, err, test.ShouldBeNil)
	// did it add the detector
	detRequest := &pb.GetDetectorNamesRequest{Name: testVisionServiceName}
	detResp, err := server.GetDetectorNames(context.Background(), detRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, detResp.GetDetectorNames(), test.ShouldContain, "test")
	// was a segmenter added too
	segRequest := &pb.GetSegmenterNamesRequest{Name: testVisionServiceName}
	segResp, err := server.GetSegmenterNames(context.Background(), segRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, segResp.GetSegmenterNames(), test.ShouldContain, "test_segmenter")

	// now remove it
	_, err = server.RemoveDetector(context.Background(), &pb.RemoveDetectorRequest{
		Name:         testVisionServiceName,
		DetectorName: "test",
	})
	test.That(t, err, test.ShouldBeNil)
	// check that it is gone
	detRequest = &pb.GetDetectorNamesRequest{Name: testVisionServiceName}
	detResp, err = server.GetDetectorNames(context.Background(), detRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, detResp.GetDetectorNames(), test.ShouldNotContain, "test")
	// checkt to see that segmenter is gone too
	segRequest = &pb.GetSegmenterNamesRequest{Name: testVisionServiceName}
	segResp, err = server.GetSegmenterNames(context.Background(), segRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, segResp.GetSegmenterNames(), test.ShouldNotContain, "test_segmenter")

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
	r, err := buildRobotWithFakeCamera(t)
	test.That(t, err, test.ShouldBeNil)
	visName := vision.FindFirstName(r)
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
	params, err := protoutils.StructToStructPb(config.AttributeMap{
		"model_path":  modelLoc,
		"num_threads": 1,
	})
	test.That(t, err, test.ShouldBeNil)
	// success
	addDetResp, err := server.AddDetector(context.Background(), &pb.AddDetectorRequest{
		Name:               visName,
		DetectorName:       "test",
		DetectorModelType:  "tflite_detector",
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
	test.That(t, resp2.Detections[0].Confidence, test.ShouldBeGreaterThan, 0.78)
	test.That(t, resp2.Detections[1].ClassName, test.ShouldResemble, "17")
	test.That(t, resp2.Detections[1].Confidence, test.ShouldBeGreaterThan, 0.73)
}

func TestServerAddRemoveSegmenter(t *testing.T) {
	srv, r := createService(t)
	m := map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): srv,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)
	params, err := protoutils.StructToStructPb(config.AttributeMap{
		"min_points_in_plane":   100,
		"min_points_in_segment": 3,
		"clustering_radius_mm":  5.,
		"mean_k_filtering":      10.,
	})
	test.That(t, err, test.ShouldBeNil)
	// add segmenter
	_, err = server.AddSegmenter(context.Background(), &pb.AddSegmenterRequest{
		Name:                testVisionServiceName,
		SegmenterName:       "test",
		SegmenterModelType:  string(builtin.RCSegmenter),
		SegmenterParameters: params,
	})
	test.That(t, err, test.ShouldBeNil)
	// success
	request := &pb.GetSegmenterNamesRequest{Name: testVisionServiceName}
	resp, err := server.GetSegmenterNames(context.Background(), request)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetSegmenterNames(), test.ShouldContain, "test")

	// remove it
	_, err = server.RemoveSegmenter(context.Background(), &pb.RemoveSegmenterRequest{
		Name:          testVisionServiceName,
		SegmenterName: "test",
	})
	test.That(t, err, test.ShouldBeNil)
	// check that it is gone
	request = &pb.GetSegmenterNamesRequest{Name: testVisionServiceName}
	resp, err = server.GetSegmenterNames(context.Background(), request)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetSegmenterNames(), test.ShouldNotContain, "test")

	// failure
	respAdd, err := server.AddSegmenter(context.Background(), &pb.AddSegmenterRequest{
		Name:                testVisionServiceName,
		SegmenterName:       "failing",
		SegmenterModelType:  "no_such_type",
		SegmenterParameters: params,
	})
	test.That(t, err.Error(), test.ShouldContainSubstring, "is not implemented")
	test.That(t, respAdd, test.ShouldBeNil)
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

func TestServerSegmentationGetObjects(t *testing.T) {
	expectedLabel := "test_label"
	params := config.AttributeMap{
		"min_points_in_plane":   100,
		"min_points_in_segment": 3,
		"clustering_radius_mm":  5.,
		"mean_k_filtering":      10.,
		"label":                 expectedLabel,
	}
	segmenter, err := segmentation.NewRadiusClustering(params)
	test.That(t, err, test.ShouldBeNil)

	_cam := &cloudSource{}
	cam, err := camera.NewFromReader(context.Background(), _cam, nil, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	injectVision := &inject.VisionService{}
	var extraOptions map[string]interface{}
	injectVision.GetObjectPointCloudsFunc = func(ctx context.Context, cameraName, segmenterName string, extra map[string]interface{},
	) ([]*viz.Object, error) {
		extraOptions = extra
		if segmenterName == RadiusClusteringSegmenter {
			return segmenter(ctx, cam)
		}
		return nil, errors.Errorf("no segmenter with name %s", segmenterName)
	}
	m := map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): injectVision,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)

	// no such segmenter in registry
	_, err = server.GetObjectPointClouds(context.Background(), &pb.GetObjectPointCloudsRequest{
		Name:          testVisionServiceName,
		CameraName:    "fakeCamera",
		SegmenterName: "no_such_segmenter",
		MimeType:      utils.MimeTypePCD,
	})
	test.That(t, err.Error(), test.ShouldContainSubstring, "no segmenter with name")
	test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{})

	// successful request
	extra := map[string]interface{}{"foo": "GetObjectPointClouds"}
	ext, err := protoutils.StructToStructPb(extra)
	test.That(t, err, test.ShouldBeNil)
	segs, err := server.GetObjectPointClouds(context.Background(), &pb.GetObjectPointCloudsRequest{
		Name:          testVisionServiceName,
		CameraName:    "fakeCamera",
		SegmenterName: RadiusClusteringSegmenter,
		MimeType:      utils.MimeTypePCD,
		Extra:         ext,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(segs.Objects), test.ShouldEqual, 2)
	test.That(t, extraOptions, test.ShouldResemble, extra)

	expectedBoxes := makeExpectedBoxes(t)
	for _, object := range segs.Objects {
		box, err := spatialmath.NewGeometryFromProto(object.Geometries.Geometries[0])
		test.That(t, err, test.ShouldBeNil)
		test.That(t, box.AlmostEqual(expectedBoxes[0]) || box.AlmostEqual(expectedBoxes[1]), test.ShouldBeTrue)
		test.That(t, box.Label(), test.ShouldEqual, expectedLabel)
	}
}

func TestServerSegmentationAddRemove(t *testing.T) {
	srv, r := createService(t)
	m := map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): srv,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)
	// add a segmenter
	params, err := protoutils.StructToStructPb(config.AttributeMap{
		"min_points_in_plane":   100,
		"min_points_in_segment": 3,
		"clustering_radius_mm":  5.,
		"mean_k_filtering":      10.,
	})
	test.That(t, err, test.ShouldBeNil)
	// add segmenter
	_, err = server.AddSegmenter(context.Background(), &pb.AddSegmenterRequest{
		Name:                testVisionServiceName,
		SegmenterName:       RadiusClusteringSegmenter,
		SegmenterModelType:  string(builtin.RCSegmenter),
		SegmenterParameters: params,
	})
	test.That(t, err, test.ShouldBeNil)
	// segmenter names
	segReq := &pb.GetSegmenterNamesRequest{
		Name: testVisionServiceName,
	}
	segResp, err := server.GetSegmenterNames(context.Background(), segReq)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, segResp.SegmenterNames, test.ShouldHaveLength, 1)
	test.That(t, segResp.SegmenterNames[0], test.ShouldEqual, RadiusClusteringSegmenter)
	// remove segmenter
	_, err = server.RemoveSegmenter(context.Background(), &pb.RemoveSegmenterRequest{
		Name:          testVisionServiceName,
		SegmenterName: RadiusClusteringSegmenter,
	})
	test.That(t, err, test.ShouldBeNil)
	// test that it was removed
	segReq = &pb.GetSegmenterNamesRequest{
		Name: testVisionServiceName,
	}
	segResp, err = server.GetSegmenterNames(context.Background(), segReq)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, segResp.SegmenterNames, test.ShouldHaveLength, 0)

	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

func TestServerAddRemoveClassifier(t *testing.T) {
	srv, r := createService(t)
	m := map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): srv,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)
	params, err := protoutils.StructToStructPb(config.AttributeMap{
		"model_path":  artifact.MustPath("vision/tflite/effnet0.tflite"),
		"label_path":  "",
		"num_threads": 2,
	})
	test.That(t, err, test.ShouldBeNil)
	// success
	_, err = server.AddClassifier(context.Background(), &pb.AddClassifierRequest{
		Name:                 testVisionServiceName,
		ClassifierName:       "test",
		ClassifierModelType:  "tflite_classifier",
		ClassifierParameters: params,
	})
	test.That(t, err, test.ShouldBeNil)
	// did it add the classifier
	classRequest := &pb.GetClassifierNamesRequest{Name: testVisionServiceName}
	classResp, err := server.GetClassifierNames(context.Background(), classRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, classResp.GetClassifierNames(), test.ShouldContain, "test")

	// now remove it
	_, err = server.RemoveClassifier(context.Background(), &pb.RemoveClassifierRequest{
		Name:           testVisionServiceName,
		ClassifierName: "test",
	})
	test.That(t, err, test.ShouldBeNil)
	// check that it is gone
	classRequest = &pb.GetClassifierNamesRequest{Name: testVisionServiceName}
	classResp, err = server.GetClassifierNames(context.Background(), classRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, classResp.GetClassifierNames(), test.ShouldNotContain, "test")

	// failure
	resp, err := server.AddClassifier(context.Background(), &pb.AddClassifierRequest{
		Name:                 testVisionServiceName,
		ClassifierName:       "failing",
		ClassifierModelType:  "no_such_type",
		ClassifierParameters: params,
	})
	test.That(t, err.Error(), test.ShouldContainSubstring, "is not implemented")
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

func TestServerGetClassifications(t *testing.T) {
	r, err := buildRobotWithFakeCamera(t)
	test.That(t, err, test.ShouldBeNil)
	visName := vision.FindFirstName(r)
	srv, err := vision.FromRobot(r, visName)
	test.That(t, err, test.ShouldBeNil)
	m := map[resource.Name]interface{}{
		vision.Named(visName): srv,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)
	// add a classifier
	params, err := protoutils.StructToStructPb(config.AttributeMap{
		"model_path":  artifact.MustPath("vision/tflite/effnet0.tflite"),
		"label_path":  "",
		"num_threads": 2,
	})
	test.That(t, err, test.ShouldBeNil)
	// success
	_, err = server.AddClassifier(context.Background(), &pb.AddClassifierRequest{
		Name:                 testVisionServiceName,
		ClassifierName:       "test_classifier",
		ClassifierModelType:  "tflite_classifier",
		ClassifierParameters: params,
	})
	test.That(t, err, test.ShouldBeNil)
	// success
	resp, err := server.GetClassificationsFromCamera(context.Background(), &pb.GetClassificationsFromCameraRequest{
		Name:           visName,
		CameraName:     "fake_cam2",
		ClassifierName: "test_classifier",
		N:              5,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Classifications, test.ShouldHaveLength, 5)
	test.That(t, resp.Classifications[0].Confidence, test.ShouldBeGreaterThan, 0.82)
	test.That(t, resp.Classifications[0].ClassName, test.ShouldResemble, "291")

	// failure - empty request
	_, err = server.GetClassificationsFromCamera(context.Background(), &pb.GetClassificationsFromCameraRequest{Name: testVisionServiceName})
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)

	// test new getclassifications straight from image
	modelLoc := artifact.MustPath("vision/tflite/effnet0.tflite")
	params, err = protoutils.StructToStructPb(config.AttributeMap{
		"model_path":  modelLoc,
		"num_threads": 1,
	})
	test.That(t, err, test.ShouldBeNil)
	// success
	addClassResp, err := server.AddClassifier(context.Background(), &pb.AddClassifierRequest{
		Name:                 visName,
		ClassifierName:       "test",
		ClassifierModelType:  "tflite_classifier",
		ClassifierParameters: params,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, addClassResp, test.ShouldNotBeNil)
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/lion.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	imgBytes, err := rimage.EncodeImage(context.Background(), img, utils.MimeTypeJPEG)
	test.That(t, err, test.ShouldBeNil)

	resp2, err := server.GetClassifications(context.Background(), &pb.GetClassificationsRequest{
		Name:           visName,
		Image:          imgBytes,
		Width:          int32(img.Width()),
		Height:         int32(img.Height()),
		MimeType:       utils.MimeTypeJPEG,
		ClassifierName: "test",
		N:              10,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp2.Classifications, test.ShouldNotBeNil)
	test.That(t, resp2.Classifications, test.ShouldHaveLength, 10)
	test.That(t, resp2.Classifications[0].ClassName, test.ShouldResemble, "291")
	test.That(t, resp2.Classifications[0].Confidence, test.ShouldBeGreaterThan, 0.82)
}
