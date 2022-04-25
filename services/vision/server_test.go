package vision_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/config"
	pb "go.viam.com/rdk/proto/api/service/vision/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/objectdetection"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func newServer(m map[resource.Name]interface{}) (pb.ObjectDetectionServiceServer, error) {
	svc, err := subtype.New(m)
	if err != nil {
		return nil, err
	}
	return objectdetection.NewServer(svc), nil
}

func TestDetectionServer(t *testing.T) {
	nameRequest := &pb.DetectorNamesRequest{}

	// no service
	m := map[resource.Name]interface{}{}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.DetectorNames(context.Background(), nameRequest)
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:object_detection\" not found"))

	// set up the robot with something that is not an object detection service
	m = map[resource.Name]interface{}{objectdetection.Name: "not what you want"}
	server, err = newServer(m)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.DetectorNames(context.Background(), nameRequest)
	test.That(t, err, test.ShouldBeError, utils.NewUnimplementedInterfaceError("objectdetection.Service", "string"))

	// correct server
	injectODS := &inject.ObjectDetectionService{}
	m = map[resource.Name]interface{}{
		objectdetection.Name: injectODS,
	}
	server, err = newServer(m)
	test.That(t, err, test.ShouldBeNil)
	// error
	passedErr := errors.New("fake error")
	injectODS.DetectorNamesFunc = func(ctx context.Context) ([]string, error) {
		return nil, passedErr
	}

	_, err = server.DetectorNames(context.Background(), nameRequest)
	test.That(t, err, test.ShouldBeError, passedErr)

	// returns response
	expSlice := []string{"test name"}
	injectODS.DetectorNamesFunc = func(ctx context.Context) ([]string, error) {
		return expSlice, nil
	}
	resp, err := server.DetectorNames(context.Background(), nameRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetDetectorNames(), test.ShouldResemble, expSlice)
}

func TestServerAddDetector(t *testing.T) {
	srv := createService(t, "data/empty.json")
	m := map[resource.Name]interface{}{
		objectdetection.Name: srv,
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
	resp, err := server.AddDetector(context.Background(), &pb.AddDetectorRequest{
		DetectorName:       "test",
		DetectorModelType:  "color",
		DetectorParameters: params,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Success, test.ShouldBeTrue)
	// failure
	resp, err = server.AddDetector(context.Background(), &pb.AddDetectorRequest{
		DetectorName:       "failing",
		DetectorModelType:  "no_such_type",
		DetectorParameters: params,
	})
	test.That(t, err.Error(), test.ShouldContainSubstring, "is not implemented")
	test.That(t, resp, test.ShouldBeNil)
}

func TestServerDetect(t *testing.T) {
	r := buildRobotWithFakeCamera(t)
	srv, err := objectdetection.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	m := map[resource.Name]interface{}{
		objectdetection.Name: srv,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)
	// success
	resp, err := server.Detect(context.Background(), &pb.DetectRequest{
		CameraName:   "fake_cam",
		DetectorName: "detect_red",
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Detections, test.ShouldHaveLength, 1)
	test.That(t, resp.Detections[0].Confidence, test.ShouldEqual, 1.0)
	test.That(t, resp.Detections[0].ClassName, test.ShouldEqual, "red")
	test.That(t, resp.Detections[0].XMin, test.ShouldEqual, 110)
	test.That(t, resp.Detections[0].YMin, test.ShouldEqual, 288)
	test.That(t, resp.Detections[0].XMax, test.ShouldEqual, 183)
	test.That(t, resp.Detections[0].YMax, test.ShouldEqual, 349)
	// failure - empty request
	_, err = server.Detect(context.Background(), &pb.DetectRequest{})
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
}
