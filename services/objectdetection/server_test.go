package objectdetection_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	pb "go.viam.com/rdk/proto/api/service/objectdetection/v1"
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

	// error
	injectODS := &inject.ObjectDetectionService{}
	m = map[resource.Name]interface{}{
		objectdetection.Name: injectODS,
	}
	server, err = newServer(m)
	test.That(t, err, test.ShouldBeNil)
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
