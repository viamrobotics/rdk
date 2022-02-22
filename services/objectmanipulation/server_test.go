package objectmanipulation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/objectmanipulation/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/objectmanipulation"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func newServer(omMap map[resource.Name]interface{}) (pb.ObjectManipulationServiceServer, error) {
	omSvc, err := subtype.New(omMap)
	if err != nil {
		return nil, err
	}
	return objectmanipulation.NewServer(omSvc), nil
}

func TestServerDoGrab(t *testing.T) {
	omMap := map[resource.Name]interface{}{}
	server, err := newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.DoGrab(context.Background(), &pb.DoGrabRequest{})
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:object_manipulation\" not found"))

	// set up the robot with something that is not an objectmanipulation service
	omMap = map[resource.Name]interface{}{objectmanipulation.Name: "not object manipulation"}
	server, err = newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.DoGrab(context.Background(), &pb.DoGrabRequest{})
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("objectmanipulation.Service", "string"))

	// error
	injectOMS := &inject.ObjectManipulationService{}
	omMap = map[resource.Name]interface{}{
		objectmanipulation.Name: injectOMS,
	}
	server, err = newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	passedErr := errors.New("fake dograb error")
	injectOMS.DoGrabFunc = func(ctx context.Context, gripperName, armName, cameraName string, cameraPoint *r3.Vector) (bool, error) {
		return false, passedErr
	}
	req := &pb.DoGrabRequest{
		CameraName:  "fakeC",
		GripperName: "fakeG",
		ArmName:     "fakeA",
		CameraPoint: &commonpb.Vector3{X: 0, Y: 0, Z: 0},
	}
	_, err = server.DoGrab(context.Background(), req)
	test.That(t, err, test.ShouldBeError, passedErr)

	// returns response
	injectOMS.DoGrabFunc = func(ctx context.Context, gripperName, armName, cameraName string, cameraPoint *r3.Vector) (bool, error) {
		return true, nil
	}
	resp, err := server.DoGrab(context.Background(), req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetSuccess(), test.ShouldBeTrue)
}
